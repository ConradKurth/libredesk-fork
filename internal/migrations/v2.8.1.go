package migrations

import (
	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V2_8_1 adds the processing_at lease column used by claim-outgoing-pending-messages
// to dispatch outgoing messages exactly-once across replicas. The column is already
// present in schema.sql (fresh installs), so this migration brings existing databases
// upgraded via --upgrade in line. Idempotent.
func V2_8_1(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf) error {
	_, err := db.Exec(`ALTER TABLE conversation_messages ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ NULL;`)
	return err
}
