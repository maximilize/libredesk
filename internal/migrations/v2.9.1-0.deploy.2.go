package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_1_0_DEPLOY_2 is a fork-only migration (maximilize/libredesk deploy branch) for the
// not-yet-merged message translations: it adds the team's working language setting, empty
// keeping translations off. Its version sorts after v2.9.1-0.deploy.1 and before any upstream
// v2.9.1/v2.10.0 migration, so those still run later; the insert is idempotent.
func V2_9_1_0_DEPLOY_2(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`INSERT INTO settings ("key", value) VALUES ('ai.translation_language', '""'::jsonb) ON CONFLICT ("key") DO NOTHING;`)
	return err
}
