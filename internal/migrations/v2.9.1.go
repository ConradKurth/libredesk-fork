package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_9_1 adds the assistant's handoff_message, the public reply posted to the customer when the
// assistant hands a conversation to a human. Already present in schema.sql for fresh installs. Idempotent.
func V2_9_1(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`ALTER TABLE ai_assistants ADD COLUMN IF NOT EXISTS handoff_message TEXT NOT NULL DEFAULT '';`)
	return err
}
