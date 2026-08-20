package migrations

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed rollback/0031_remote_assistance.sql
var remoteRollbackSQL string

func TestRemoteAssistanceRollbackDependencyOrder(t *testing.T) {
	ordered := []string{
		"DROP TABLE IF EXISTS remote_sessions",
		"DROP TABLE IF EXISTS remote_credential_requests",
		"DROP TABLE IF EXISTS remote_commands",
		"DROP TABLE IF EXISTS remote_devices",
		"DELETE FROM schema_migrations WHERE version='0031_remote_assistance.sql'",
	}
	previous := -1
	for _, statement := range ordered {
		index := strings.Index(remoteRollbackSQL, statement)
		if index < 0 {
			t.Fatalf("rollback is missing %q", statement)
		}
		if index <= previous {
			t.Fatalf("rollback dependency order is invalid at %q", statement)
		}
		previous = index
	}
}
