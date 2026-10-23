package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_1_0_DEPLOY_1 is a fork-only migration (maximilize/libredesk deploy branch).
// It adds the search indexes from the not-yet-merged upstream changes for subject
// and contact-name search. Its version sorts after v2.9.0 and before any upstream
// v2.9.1/v2.10.0 migration, so those still run later. Both statements are idempotent,
// so an upstream migration creating the same indexes is a no-op afterwards.
func V2_9_1_0_DEPLOY_1(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS pg_trgm`); err != nil {
		return err
	}
	if err := createIndexConcurrently(db, "index_trgm_conversations_on_subject", `CREATE INDEX CONCURRENTLY IF NOT EXISTS index_trgm_conversations_on_subject ON conversations USING GIN (subject gin_trgm_ops)`); err != nil {
		return err
	}
	return createIndexConcurrently(db, "index_tgrm_users_on_full_name", `CREATE INDEX CONCURRENTLY IF NOT EXISTS index_tgrm_users_on_full_name ON users USING GIN ((first_name || ' ' || COALESCE(last_name, '')) gin_trgm_ops)`)
}
