package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleSandboxProxy transparently reverse-proxies HTTP traffic from the
// caller's browser to a server running inside a sandbox container.
//
//	GET /sandboxes/{id}/port/{port}/*
//
// The backend can reach sandbox containers directly via sandbox-net (they
// share a Docker bridge network). The agent starts whatever server it likes
// inside the container, and the user browses to:
//
//	http://localhost:7001/sandboxes/{id}/port/3000/
//
// Web-Socket upgrades are forwarded transparently.
func (s *Server) handleSandboxProxy(w http.ResponseWriter, r *http.Request) {
	sandboxID := chi.URLParam(r, "id")
	port := chi.URLParam(r, "port")

	if sandboxID == "" || port == "" {
		writeError(w, http.StatusBadRequest, "sandbox id and port are required")
		return
	}

	// Validate port is numeric to prevent SSRF to arbitrary hosts.
	for _, ch := range port {
		if ch < '0' || ch > '9' {
			writeError(w, http.StatusBadRequest, "port must be numeric")
			return
		}
	}

	// Look up the sandbox IP in the registry.
	rec, err := s.Registry.Get(sandboxID)
	if err != nil {
		writeError(w, http.StatusNotFound, "sandbox not found: "+sandboxID)
		return
	}
	if rec.IP == "" {
		// Fallback: ask the driver.
		ip, ierr := s.Sandbox.IP(r.Context(), sandboxID)
		if ierr != nil || ip == "" {
			writeError(w, http.StatusBadGateway, "sandbox has no IP address yet")
			return
		}
		rec.IP = ip
	}

	target, err := url.Parse(fmt.Sprintf("http://%s:%s", rec.IP, port))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid target url: "+err.Error())
		return
	}

	// Strip the /sandboxes/{id}/port/{port} prefix so the proxied request
	// path starts at / (what the inner server expects).
	prefix := fmt.Sprintf("/sandboxes/%s/port/%s", sandboxID, port)
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	if r2.URL.Path == "" {
		r2.URL.Path = "/"
	}
	r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
	r2.RequestURI = ""

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		writeError(w, http.StatusBadGateway,
			fmt.Sprintf("proxy error (is the server running on port %s?): %v", port, err))
	}
	proxy.ServeHTTP(w, r2)
}
