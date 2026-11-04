package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_10_0_RC3 adds the permission to delete conversations (granted to Admin) and its activity log type.
func V2_10_0_RC3(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`
		UPDATE roles
		SET permissions = array_append(permissions, 'conversations:delete')
		WHERE name = 'Admin' AND NOT ('conversations:delete' = ANY(permissions));
	`); err != nil {
		return err
	}
	if _, err := db.Exec(`ALTER TYPE activity_log_type ADD VALUE IF NOT EXISTS 'conversation_deleted';`); err != nil {
		return err
	}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS deleted_message_source_ids (
			source_id TEXT PRIMARY KEY,
			deleted_at TIMESTAMPTZ DEFAULT NOW() NOT NULL
		);
		CREATE INDEX IF NOT EXISTS index_deleted_message_source_ids_on_deleted_at ON deleted_message_source_ids (deleted_at);
	`)
	return err
}
