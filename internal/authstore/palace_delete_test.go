package authstore

import (
	"path/filepath"
	"testing"
)

func TestRemovePalaceAfterPermanentDelete_DropsTenantAndSubs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	s := &Store{
		path: path,
		Users: []User{
			{Username: "adm", Role: RoleAdmin, Palaces: nil, PasswordBcrypt: "x"},
			{Username: "t1", Role: RoleTenant, Palaces: []string{"gone"}, PasswordBcrypt: "x"},
			{Username: "s1", Role: RoleSubaccount, ParentTenant: "t1", PasswordBcrypt: "x", PalacePerms: map[string][]string{
				"gone": {PermControl},
			}},
		},
	}
	if err := s.RemovePalaceAfterPermanentDelete("gone"); err != nil {
		t.Fatal(err)
	}
	if len(s.Users) != 1 || s.Users[0].Username != "adm" {
		t.Fatalf("want only admin left, got %#v", s.Users)
	}
}

func TestRemovePalaceAfterPermanentDelete_PruneSubKeepTenant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")
	s := &Store{
		path: path,
		Users: []User{
			{Username: "t1", Role: RoleTenant, Palaces: []string{"keep", "gone"}, PasswordBcrypt: "x"},
			{Username: "s1", Role: RoleSubaccount, ParentTenant: "t1", PasswordBcrypt: "x", PalacePerms: map[string][]string{
				"keep": {PermLogs},
				"gone": {PermControl},
			}},
		},
	}
	if err := s.RemovePalaceAfterPermanentDelete("gone"); err != nil {
		t.Fatal(err)
	}
	if len(s.Users) != 2 {
		t.Fatalf("want 2 users, got %d", len(s.Users))
	}
	var t1, s1 *User
	for i := range s.Users {
		switch s.Users[i].Username {
		case "t1":
			t1 = &s.Users[i]
		case "s1":
			s1 = &s.Users[i]
		}
	}
	if t1 == nil || len(t1.Palaces) != 1 || t1.Palaces[0] != "keep" {
		t.Fatalf("tenant palaces: %#v", t1)
	}
	if s1 == nil || len(s1.PalacePerms) != 1 {
		t.Fatalf("sub perms: %#v", s1)
	}
	if _, ok := s1.PalacePerms["gone"]; ok {
		t.Fatal("gone key should be removed")
	}
	if _, ok := s1.PalacePerms["keep"]; !ok {
		t.Fatal("keep key should remain")
	}
}
