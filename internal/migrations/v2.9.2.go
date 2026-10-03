package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_2 re-runs V2_9_0. Upstream re-keyed that migration from v2.9.0 to v2.9.0-rc.8 and appended
// DDL to it (whatsapp, reopen window, last_inbound_at/last_resolved_at, ai_tools approval flags,
// widget campaigns, help-center translations). Databases that already recorded v2.9.0 or our v2.9.1
// sort above v2.9.0-rc.8, so --upgrade would skip the new DDL. V2_9_0 is idempotent.
func V2_9_2(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	return V2_9_0(db, fs, ko)
}
