package sandbox

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SandboxRecord tracks a live sandbox instance in the SQLite registry.
// Schema matches platform-plan.jsx exactly.
type SandboxRecord struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	Status     Status    `json:"status"`
	ProjectID  string    `json:"project_id,omitempty"`
	AgentRole  string    `json:"agent_role,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastActive time.Time `json:"last_active"`
	MemUsage   int64     `json:"mem_usage"`
	Transport  string    `json:"transport"`
}

// ListFilter optionally constrains the result set of Registry.List.
type ListFilter struct {
	Status    string // "running" | "sleeping" | "dead" | "" (all)
	ProjectID string // "" means all projects
}

// Registry persists sandbox records in a SQLite database.
type Registry struct {
	db *sql.DB
}

// NewRegistry opens (and migrates) the SQLite DB at path.
// Path defaults to ./data/forge.db if empty.
func NewRegistry(path string) (*Registry, error) {
	if path == "" {
		path = os.Getenv("FORGE_DB_PATH")
	}
	if path == "" {
		path = "./data/forge.db"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("registry: mkdir %s: %w", filepath.Dir(path), err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("registry: open %s: %w", path, err)
	}

	// Single writer; WAL mode for concurrent reads.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("registry: WAL pragma: %w", err)
	}

	r := &Registry{db: db}
	if err := r.migrate(); err != nil {
		return nil, err
	}

	slog.Info("registry opened", "path", path)
	return r, nil
}

// migrate creates or upgrades the sandboxes table.
func (r *Registry) migrate() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS sandboxes (
		id          TEXT PRIMARY KEY,
		ip          TEXT NOT NULL,
		status      TEXT NOT NULL,
		project_id  TEXT,
		agent_role  TEXT,
		created_at  TIMESTAMP NOT NULL,
		last_active TIMESTAMP NOT NULL,
		mem_usage   INTEGER DEFAULT 0,
		transport   TEXT DEFAULT 'none'
	)`)
	if err != nil {
		return fmt.Errorf("registry: migrate: %w", err)
	}
	return nil
}

// Insert adds a new sandbox record.
func (r *Registry) Insert(sb SandboxRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO sandboxes (id, ip, status, project_id, agent_role, created_at, last_active, mem_usage, transport)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sb.ID, sb.IP, string(sb.Status), sb.ProjectID, sb.AgentRole,
		sb.CreatedAt.UTC(), sb.LastActive.UTC(), sb.MemUsage, sb.Transport,
	)
	if err != nil {
		return fmt.Errorf("registry insert: %w", err)
	}
	return nil
}

// Update replaces a sandbox record by ID.
func (r *Registry) Update(sb SandboxRecord) error {
	_, err := r.db.Exec(
		`UPDATE sandboxes SET ip=?, status=?, project_id=?, agent_role=?, last_active=?, mem_usage=?, transport=?
		 WHERE id=?`,
		sb.IP, string(sb.Status), sb.ProjectID, sb.AgentRole,
		sb.LastActive.UTC(), sb.MemUsage, sb.Transport, sb.ID,
	)
	if err != nil {
		return fmt.Errorf("registry update: %w", err)
	}
	return nil
}

// Get retrieves a single sandbox record by ID. Returns sql.ErrNoRows if not found.
func (r *Registry) Get(id string) (SandboxRecord, error) {
	row := r.db.QueryRow(
		`SELECT id, ip, status, COALESCE(project_id,''), COALESCE(agent_role,''),
		        created_at, last_active, mem_usage, transport
		 FROM sandboxes WHERE id=?`, id,
	)
	return scanRecord(row)
}

// List returns all sandbox records, optionally filtered.
func (r *Registry) List(f ListFilter) ([]SandboxRecord, error) {
	query := `SELECT id, ip, status, COALESCE(project_id,''), COALESCE(agent_role,''),
	                 created_at, last_active, mem_usage, transport
	          FROM sandboxes WHERE 1=1`
	var args []any
	if f.Status != "" {
		query += " AND status=?"
		args = append(args, f.Status)
	}
	if f.ProjectID != "" {
		query += " AND project_id=?"
		args = append(args, f.ProjectID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("registry list: %w", err)
	}
	defer rows.Close()

	var out []SandboxRecord
	for rows.Next() {
		sb, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sb)
	}
	return out, rows.Err()
}

// Delete removes a sandbox record by ID.
func (r *Registry) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM sandboxes WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("registry delete: %w", err)
	}
	return nil
}

// TouchLastActive updates last_active to now for the given sandbox ID.
func (r *Registry) TouchLastActive(id string) error {
	_, err := r.db.Exec(`UPDATE sandboxes SET last_active=? WHERE id=?`, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("registry touch: %w", err)
	}
	return nil
}

// ListOlderThan returns sandboxes where last_active < cutoff, optionally
// filtered by status.
func (r *Registry) ListOlderThan(cutoff time.Time, status string) ([]SandboxRecord, error) {
	query := `SELECT id, ip, status, COALESCE(project_id,''), COALESCE(agent_role,''),
	                 created_at, last_active, mem_usage, transport
	          FROM sandboxes WHERE last_active < ?`
	var args []any
	args = append(args, cutoff.UTC())
	if status != "" {
		query += " AND status=?"
		args = append(args, status)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("registry list older than: %w", err)
	}
	defer rows.Close()

	var out []SandboxRecord
	for rows.Next() {
		sb, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sb)
	}
	return out, rows.Err()
}

// SetStatus sets the status field for a sandbox record.
func (r *Registry) SetStatus(id string, status Status) error {
	_, err := r.db.Exec(`UPDATE sandboxes SET status=? WHERE id=?`, string(status), id)
	if err != nil {
		return fmt.Errorf("registry set status: %w", err)
	}
	return nil
}

// ─── scanner ──────────────────────────────────────────────────────────────────

// scanner matches both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(s scanner) (SandboxRecord, error) {
	var sb SandboxRecord
	var statusStr string
	var createdStr, lastStr string

	if err := s.Scan(
		&sb.ID, &sb.IP, &statusStr, &sb.ProjectID, &sb.AgentRole,
		&createdStr, &lastStr, &sb.MemUsage, &sb.Transport,
	); err != nil {
		return sb, fmt.Errorf("registry scan: %w", err)
	}

	sb.Status = Status(statusStr)

	// Parse timestamps — SQLite stores them as text.
	var err error
	sb.CreatedAt, err = parseTS(createdStr)
	if err != nil {
		return sb, fmt.Errorf("registry parse created_at: %w", err)
	}
	sb.LastActive, err = parseTS(lastStr)
	if err != nil {
		return sb, fmt.Errorf("registry parse last_active: %w", err)
	}
	return sb, nil
}

// parseTS tries several layouts used by SQLite for timestamp strings.
func parseTS(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z",
		"2006-01-02 15:04:05",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown timestamp format: %q", s)
}

// Record is the legacy alias kept for any external reference from the scaffold.
// New code should use SandboxRecord.
type Record = SandboxRecord
