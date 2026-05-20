package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
)

// BackfillCapabilities seeds CAPABILITIES.md template for all agents that don't have it.
// Runs once at startup, idempotent. Returns number of agents backfilled.
func BackfillCapabilities(ctx context.Context, db *sql.DB) (int64, error) {
	if db == nil {
		return 0, nil
	}

	tpl, err := templateFS.ReadFile(filepath.Join("templates", CapabilitiesFile))
	if err != nil {
		return 0, err
	}

	// Try PG first (uuid_generate_v7), fall back to SQLite (hex blob).
	res, err := db.ExecContext(ctx, `
		INSERT INTO agent_context_files (id, agent_id, file_name, content, created_at, updated_at, tenant_id)
		SELECT uuid_generate_v7(), a.id, 'CAPABILITIES.md', $1, NOW(), NOW(), a.tenant_id
		FROM agents a
		WHERE NOT EXISTS (
			SELECT 1 FROM agent_context_files acf
			WHERE acf.agent_id = a.id AND acf.file_name = 'CAPABILITIES.md'
		)`,
		string(tpl),
	)
	if err != nil {
		// SQLite fallback: use hex(randomblob(16)) for UUID generation
		res, err = db.ExecContext(ctx, `
			INSERT INTO agent_context_files (id, agent_id, file_name, content, created_at, updated_at, tenant_id)
			SELECT lower(hex(randomblob(16))), a.id, 'CAPABILITIES.md', $1, datetime('now'), datetime('now'), a.tenant_id
			FROM agents a
			WHERE NOT EXISTS (
				SELECT 1 FROM agent_context_files acf
				WHERE acf.agent_id = a.id AND acf.file_name = 'CAPABILITIES.md'
			)`,
			string(tpl),
		)
		if err != nil {
			return 0, err
		}
	}

	count, _ := res.RowsAffected()
	if count > 0 {
		slog.Info("bootstrap: backfilled CAPABILITIES.md", "agents", count)
	}
	return count, nil
}
