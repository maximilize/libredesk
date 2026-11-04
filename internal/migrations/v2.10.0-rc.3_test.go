package migrations

import (
	"testing"

	"github.com/abhinavxd/libredesk/internal/testutil"
	"github.com/lib/pq"
)

func TestV2_10_0_RC3DeleteConversationPermission(t *testing.T) {
	db := testutil.NewDB(t, "migration_v2_10_0_rc3")
	// Start from a role set that predates the permission, as an upgraded install has.
	if _, err := db.Exec(`UPDATE roles SET permissions = array_remove(permissions, 'conversations:delete')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := V2_10_0_RC3(db, nil, nil); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var perms pq.StringArray
	if err := db.Get(&perms, `SELECT permissions FROM roles WHERE name = 'Admin'`); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, p := range perms {
		if p == "conversations:delete" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("Admin has conversations:delete %d times, want exactly once: %v", count, perms)
	}
	if err := db.Get(&perms, `SELECT permissions FROM roles WHERE name = 'Agent'`); err != nil {
		t.Fatal(err)
	}
	for _, p := range perms {
		if p == "conversations:delete" {
			t.Fatal("Agent role received conversations:delete")
		}
	}
	var hasType bool
	if err := db.Get(&hasType, `SELECT EXISTS (SELECT 1 FROM pg_enum e JOIN pg_type t ON t.oid = e.enumtypid WHERE t.typname = 'activity_log_type' AND e.enumlabel = 'conversation_deleted')`); err != nil {
		t.Fatal(err)
	}
	if !hasType {
		t.Fatal("activity_log_type lacks conversation_deleted")
	}
}
