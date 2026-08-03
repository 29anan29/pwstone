package store

import (
	"os"
	"path/filepath"
	"testing"

	"pwstore/internal/model"
)

func tempVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "vault.dat")
}

func TestCreateOpenRoundTrip(t *testing.T) {
	path := tempVault(t)
	v, err := Create(path, "m@ster-123456")
	if err != nil {
		t.Fatal(err)
	}
	created, err := v.Upsert(model.Entry{Site: "github.com", Username: "alice", Password: "pw1", Notes: "工作"})
	if err != nil || !created {
		t.Fatalf("upsert: created=%v err=%v", created, err)
	}
	v.Close()

	v2, err := Open(path, "m@ster-123456")
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	e, ok := v2.Get("github.com", "alice")
	if !ok || e.Password != "pw1" || e.Notes != "工作" {
		t.Fatalf("get mismatch: %+v ok=%v", e, ok)
	}
}

func TestWrongPasswordFails(t *testing.T) {
	path := tempVault(t)
	v, err := Create(path, "m@ster-123456")
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if _, err := Open(path, "bad-pass-123"); err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestUpsertUpdatesInPlace(t *testing.T) {
	path := tempVault(t)
	v, _ := Create(path, "m@ster-123456")
	defer v.Close()
	v.Upsert(model.Entry{Site: "a.com", Username: "u", Password: "old"})
	created, err := v.Upsert(model.Entry{Site: "a.com", Username: "u", Password: "new", Notes: "x"})
	if err != nil || created {
		t.Fatalf("expected update not create: created=%v err=%v", created, err)
	}
	if len(v.Entries()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(v.Entries()))
	}
	e, _ := v.Get("a.com", "u")
	if e.Password != "new" {
		t.Fatalf("password not updated: %q", e.Password)
	}
}

func TestDeleteBySiteAndUser(t *testing.T) {
	path := tempVault(t)
	v, _ := Create(path, "m@ster-123456")
	defer v.Close()
	v.Upsert(model.Entry{Site: "a.com", Username: "u1", Password: "p1"})
	v.Upsert(model.Entry{Site: "a.com", Username: "u2", Password: "p2"})
	if !v.Delete("a.com", "u1") {
		t.Fatal("delete u1 failed")
	}
	if _, ok := v.Get("a.com", "u2"); !ok {
		t.Fatal("u2 should remain")
	}
	if len(v.Entries()) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(v.Entries()))
	}
}

func TestSearch(t *testing.T) {
	path := tempVault(t)
	v, _ := Create(path, "m@ster-123456")
	defer v.Close()
	v.Upsert(model.Entry{Site: "github.com", Username: "alice", Password: "p1"})
	v.Upsert(model.Entry{Site: "gitlab.com", Username: "bob", Password: "p2", Notes: "开源"})
	if got := v.Search("git"); len(got) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(got))
	}
	if got := v.Search("开源"); len(got) != 1 {
		t.Fatalf("expected 1 note hit, got %d", len(got))
	}
}

func TestRekey(t *testing.T) {
	path := tempVault(t)
	v, _ := Create(path, "m@ster-123456")
	v.Upsert(model.Entry{Site: "a.com", Username: "u", Password: "secret"})
	if err := v.Rekey("new-m@ster-123456"); err != nil {
		t.Fatal(err)
	}
	v.Close()

	if _, err := Open(path, "m@ster-123456"); err != ErrWrongPassword {
		t.Fatalf("old password should fail, got %v", err)
	}
	v2, err := Open(path, "new-m@ster-123456")
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	e, _ := v2.Get("a.com", "u")
	if e.Password != "secret" {
		t.Fatalf("data lost after rekey: %+v", e)
	}
}

func TestFilePermissions(t *testing.T) {
	path := tempVault(t)
	v, _ := Create(path, "m@ster-123456")
	v.Upsert(model.Entry{Site: "a.com", Username: "u", Password: "p"})
	v.Close()
	if os.Getenv("SKIP_PERM") != "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected 0600, got %o", perm)
	}
}
