package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_3 re-runs upstream's V2_9_0_RC9 and V2_9_0_RC10 (assistant citations, media uploaded_by,
// notification_email_queue.message_id, inbox_email_addresses, macro content limit). Their keys
// v2.9.0-rc.9 and v2.9.0-rc.10 sort below our recorded v2.9.2, so --upgrade would skip them.
// Both are idempotent.
func V2_9_3(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	if err := V2_9_0_RC9(db, fs, ko); err != nil {
		return err
	}
	return V2_9_0_RC10(db, fs, ko)
}
