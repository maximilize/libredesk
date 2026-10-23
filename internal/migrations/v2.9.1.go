package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_1 adds a trigram index on conversations.subject for the subject filter on conversation lists and views.
func V2_9_1(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	return createIndexConcurrently(db, "index_trgm_conversations_on_subject", `CREATE INDEX CONCURRENTLY IF NOT EXISTS index_trgm_conversations_on_subject ON conversations USING GIN (subject gin_trgm_ops)`)
}
