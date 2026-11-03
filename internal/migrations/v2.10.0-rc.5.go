package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_10_0_RC5 adds the tag exception to conversation retention: tagged conversations get their own period.
func V2_10_0_RC5(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`INSERT INTO settings ("key", value) VALUES
		('app.conversation_retention_tags', '[]'::jsonb),
		('app.conversation_retention_tagged_days', '0'::jsonb)
		ON CONFLICT ("key") DO NOTHING;`)
	return err
}
