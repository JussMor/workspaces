package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	docker_network "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
)

const (
	sandboxNetName   = "sandbox-net"
	sandboxNetSubnet = "10.0.5.0/24"
	sandboxNetGW     = "10.0.5.1"

	defaultMemMB = 8 * 1024 // 8 GB
	defaultCPUs  = 2.0

	// defaultSandboxImage is a pre-baked image (Dockerfile.sandbox) that
	// includes a `forge` user, git, gh, go, node, python, kubectl, docker CLI,
	// awscli, build-essential, and sudo (passwordless). The agent can install
	// anything else at runtime via `sudo apt-get install`.
	defaultSandboxImage = "forge-sandbox:latest"
	sandboxUser         = "forge"
	sandboxHome         = "/home/forge"
	sandboxWorkdir      = "/home/forge/workspace"
)

// DockerDriver implements Driver using Docker containers.
// Uses the official Docker Go SDK — does not shell out to the docker CLI.
type DockerDriver struct {
	cli     *client.Client
	netOnce sync.Once

	// userMu protects the hasForgeUser map below.
	userMu sync.RWMutex
	// hasForgeUser caches per-container whether the `forge` user exists,
	// so we don't probe on every Exec() call.
	hasForgeUser map[string]bool
}

// compile-time interface assertion
var _ Driver = (*DockerDriver)(nil)

// initClient lazily initialises the Docker client.
func (d *DockerDriver) initClient() error {
	if d.cli != nil {
		return nil
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	d.cli = cli
	return nil
}

// ensureNetwork creates the sandbox-net network if it does not exist.
func (d *DockerDriver) ensureNetwork(ctx context.Context) error {
	var ensureErr error
	d.netOnce.Do(func() {
		_, err := d.cli.NetworkInspect(ctx, sandboxNetName, docker_network.InspectOptions{})
		if err == nil {
			return
		}
		if !client.IsErrNotFound(err) {
			ensureErr = fmt.Errorf("sandbox-net inspect: %w", err)
			return
		}
		_, cerr := d.cli.NetworkCreate(ctx, sandboxNetName, docker_network.CreateOptions{
			Driver: "bridge",
			IPAM: &docker_network.IPAM{
				Driver: "default",
				Config: []docker_network.IPAMConfig{
					{Subnet: sandboxNetSubnet, Gateway: sandboxNetGW},
				},
			},
		})
		if cerr != nil {
			ensureErr = fmt.Errorf("sandbox-net create: %w", cerr)
			return
		}
		slog.Info("sandbox-net created", "subnet", sandboxNetSubnet)
	})
	if ensureErr != nil {
		d.netOnce = sync.Once{} // reset so next call retries
	}
	return ensureErr
}

// ensureImage pulls img if it is not already present locally.
func (d *DockerDriver) ensureImage(ctx context.Context, img string) error {
	_, err := d.cli.ImageInspect(ctx, img)
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return fmt.Errorf("image inspect %q: %w", img, err)
	}
	slog.Info("pulling image", "image", img)
	rc, err := d.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %q: %w", img, err)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}

// Create starts a new Docker container with the given configuration and
// returns its container name (used as sandbox ID for subsequent calls).
func (d *DockerDriver) Create(ctx context.Context, cfg Config) (string, error) {
	if err := d.initClient(); err != nil {
		return "", err
	}
	if err := d.ensureNetwork(ctx); err != nil {
		return "", err
	}

	img := cfg.Image
	if img == "" {
		img = defaultSandboxImage
	}
	if err := d.ensureImage(ctx, img); err != nil {
		return "", err
	}

	memMB := cfg.MemMB
	if memMB <= 0 {
		memMB = envIntOr("FORGE_MEM_MB", defaultMemMB)
	}
	cpus := cfg.CPUs
	if cpus <= 0 {
		cpus = envFloatOr("FORGE_CPUS", defaultCPUs)
	}

	var env []string
	// Defaults so bash tool calls behave like a real interactive shell session.
	env = append(env,
		"HOME="+sandboxHome,
		"USER="+sandboxUser,
		"LOGNAME="+sandboxUser,
		"SHELL=/bin/bash",
		"PATH="+sandboxHome+"/.local/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM=xterm",
		"LANG=C.UTF-8",
	)
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	sandboxID := uuid.New().String()
	containerName := "forge-sandbox-" + sandboxID[:8]

	// Idle forever. The `forge` user and workspace dir are expected to exist
	// in the image (forge-sandbox:latest). The CMD runs as root (the image
	// no longer ends with USER forge), so we can safely chown the workspace
	// here in case a bind-mount or prior run left stale ownership. Exec()
	// calls drop to the forge user explicitly.
	startScript := "mkdir -p " + sandboxWorkdir +
		" && (id forge >/dev/null 2>&1 && chown -R forge:forge " + sandboxHome + " 2>/dev/null || true)" +
		" && cd " + sandboxWorkdir +
		" && while true; do sleep 3600; done"

	resp, err := d.cli.ContainerCreate(ctx,
		&container.Config{
			Image:      img,
			Env:        env,
			WorkingDir: sandboxWorkdir,
			// Start as root so we can create the workspace dir on any image.
			// Exec calls then drop to `forge` when that user exists.
			Cmd:       []string{"sh", "-c", startScript},
			Tty:       false,
			OpenStdin: false,
		},
		&container.HostConfig{
			Resources: container.Resources{
				Memory:   int64(memMB) * 1024 * 1024,
				NanoCPUs: int64(cpus * 1e9),
			},
			NetworkMode: container.NetworkMode(sandboxNetName),
		},
		&docker_network.NetworkingConfig{
			EndpointsConfig: map[string]*docker_network.EndpointSettings{
				sandboxNetName: {},
			},
		},
		nil,
		containerName,
	)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = d.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("container start: %w", err)
	}

	slog.Info("container started", "sandbox_id", containerName, "container_id", resp.ID[:12])
	return containerName, nil
}

// Exec runs a command inside the container identified by id (container name).
// Commands run as the non-root `forge` user in /home/forge/workspace.
// The user has passwordless sudo if the command needs root (e.g. apt-get install).
func (d *DockerDriver) Exec(ctx context.Context, id, cmd string) (ExecResult, error) {
	if err := d.initClient(); err != nil {
		return ExecResult{}, err
	}

	runExec := func(user, home, shell string) (ExecResult, error) {
		login := sandboxUser
		if user == "" {
			login = "root"
		}
		execCmd := []string{shell, "-lc", cmd}
		if shell == "sh" {
			execCmd = []string{"sh", "-c", cmd}
		}
		execResp, err := d.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
			AttachStdout: true,
			AttachStderr: true,
			User:         user,
			WorkingDir:   sandboxWorkdir,
			Env: []string{
				"HOME=" + home,
				"USER=" + login,
				"LOGNAME=" + login,
				"SHELL=" + shell,
				"PATH=" + sandboxHome + "/.local/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
				"LANG=C.UTF-8",
			},
			Cmd: execCmd,
		})
		if err != nil {
			return ExecResult{}, err
		}

		attach, err := d.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
		if err != nil {
			return ExecResult{}, fmt.Errorf("exec attach: %w", err)
		}
		defer attach.Close()

		var stdoutBuf, stderrBuf bytes.Buffer
		if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attach.Reader); err != nil {
			slog.Warn("exec stream demux", "err", err)
		}

		inspect, err := d.cli.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return ExecResult{}, fmt.Errorf("exec inspect: %w", err)
		}

		return ExecResult{
			Stdout:   stdoutBuf.String(),
			Stderr:   stderrBuf.String(),
			ExitCode: inspect.ExitCode,
		}, nil
	}

	// Preferred path: run as forge with bash in the batteries-included image.
	// We cache per-container whether the forge user exists, because docker
	// exec with a missing user returns exit code 126 + stdout "unable to
	// find user forge" (NOT a Go error), which is expensive to detect.
	useForge := true
	d.userMu.RLock()
	if d.hasForgeUser != nil {
		if v, ok := d.hasForgeUser[id]; ok {
			useForge = v
		}
	}
	d.userMu.RUnlock()

	if useForge {
		res, err := runExec(sandboxUser, sandboxHome, "bash")
		if err == nil {
			// Detect "unable to find user forge" (exit 126) and remember.
			if res.ExitCode == 126 && strings.Contains(res.Stdout+res.Stderr, "unable to find user") {
				d.userMu.Lock()
				if d.hasForgeUser == nil {
					d.hasForgeUser = map[string]bool{}
				}
				d.hasForgeUser[id] = false
				d.userMu.Unlock()
				slog.Warn("forge user missing in container, falling back to root", "sandbox", id)
			} else {
				d.userMu.Lock()
				if d.hasForgeUser == nil {
					d.hasForgeUser = map[string]bool{}
				}
				d.hasForgeUser[id] = true
				d.userMu.Unlock()
				return res, nil
			}
		} else {
			slog.Warn("exec as forge failed, falling back to root+sh", "sandbox", id, "err", err)
		}
	}

	// Fallback for custom images that don't have the forge user or bash.
	res, ferr := runExec("", "/root", "sh")
	if ferr != nil {
		return ExecResult{}, fmt.Errorf("exec create: %w", ferr)
	}
	return res, nil
}

// WriteFile writes content to path inside the container via docker cp (tar stream).
func (d *DockerDriver) WriteFile(ctx context.Context, id, path, content string) error {
	if err := d.initClient(); err != nil {
		return err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	data := []byte(content)
	if err := tw.WriteHeader(&tar.Header{
		Name: basename(path),
		Mode: 0644,
		Size: int64(len(data)),
	}); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	dstDir := dirOf(path)
	if err := d.cli.CopyToContainer(ctx, id, dstDir, &buf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}
	return nil
}

// ReadFile reads the contents of path from inside the container.
func (d *DockerDriver) ReadFile(ctx context.Context, id, path string) (string, error) {
	if err := d.initClient(); err != nil {
		return "", err
	}

	rc, _, err := d.cli.CopyFromContainer(ctx, id, path)
	if err != nil {
		return "", fmt.Errorf("copy from container: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar next: %w", err)
		}
		var out bytes.Buffer
		if _, err := io.Copy(&out, tr); err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}
		return out.String(), nil
	}
	return "", fmt.Errorf("file not found in tar: %s", path)
}

// Destroy stops and removes the container identified by id.
// Returns no error if the container is already gone.
func (d *DockerDriver) Destroy(ctx context.Context, id string) error {
	if err := d.initClient(); err != nil {
		return err
	}

	err := d.cli.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil && !client.IsErrNotFound(err) {
		slog.Warn("container stop (non-fatal)", "id", id, "err", err)
	}

	err = d.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("container remove: %w", err)
	}
	d.userMu.Lock()
	delete(d.hasForgeUser, id)
	d.userMu.Unlock()
	slog.Info("container destroyed", "id", id)
	return nil
}

// Status returns the current lifecycle state of the container.
func (d *DockerDriver) Status(ctx context.Context, id string) (Status, error) {
	if err := d.initClient(); err != nil {
		return StatusDead, err
	}

	insp, err := d.cli.ContainerInspect(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) {
			return StatusDead, nil
		}
		return StatusDead, fmt.Errorf("container inspect: %w", err)
	}
	switch insp.State.Status {
	case "running":
		return StatusRunning, nil
	case "exited", "dead", "removing":
		return StatusDead, nil
	default:
		return StatusSleeping, nil
	}
}

// IP returns the IP address of the container in the sandbox-net network.
func (d *DockerDriver) IP(ctx context.Context, id string) (string, error) {
	if err := d.initClient(); err != nil {
		return "", err
	}

	insp, err := d.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("container inspect: %w", err)
	}

	if nets := insp.NetworkSettings.Networks; nets != nil {
		if ep, ok := nets[sandboxNetName]; ok && ep.IPAddress != "" {
			return ep.IPAddress, nil
		}
		for _, ep := range nets {
			if ep.IPAddress != "" {
				return ep.IPAddress, nil
			}
		}
	}
	return "", fmt.Errorf("container %q has no IP in sandbox-net", id)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// basename returns the last path component of p.
func basename(p string) string {
	p = strings.TrimRight(p, "/")
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		return p[idx+1:]
	}
	return p
}

// dirOf returns the directory component of p.
func dirOf(p string) string {
	p = strings.TrimRight(p, "/")
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		return p[:idx+1]
	}
	return "/"
}

// envIntOr reads an env var as int; returns def on absence or parse error.
func envIntOr(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscan(v, &n); err != nil {
		return def
	}
	return n
}

// envFloatOr reads an env var as float64; returns def on absence or parse error.
func envFloatOr(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var f float64
	if _, err := fmt.Sscan(v, &f); err != nil {
		return def
	}
	return f
}
