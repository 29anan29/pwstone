package crypto

import (
	"bytes"
	"testing"

	"pwstore/internal/model"
)

func TestSealOpenRoundTrip(t *testing.T) {
	p, err := NewParams()
	if err != nil {
		t.Fatal(err)
	}
	key, err := p.DeriveKey("s3cret-Pass")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(key, []byte("hello 世界"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Open(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "hello 世界" {
		t.Fatalf("round trip mismatch: %q", plain)
	}
}

func TestSealUniqueNonce(t *testing.T) {
	p, _ := NewParams()
	key, _ := p.DeriveKey("s3cret-Pass")
	a, _ := Seal(key, []byte("same"))
	b, _ := Seal(key, []byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two seals of same plaintext must differ (nonce)")
	}
}

func TestOpenWrongKey(t *testing.T) {
	p, _ := NewParams()
	k1, _ := p.DeriveKey("correct-horse")
	k2, _ := p.DeriveKey("wrong-horse!")
	sealed, _ := Seal(k1, []byte("data"))
	if _, err := Open(k2, sealed); err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestVaultEncodeDecode(t *testing.T) {
	p, _ := NewParams()
	key, _ := p.DeriveKey("m@ster-123456")
	entries := []model.Entry{
		{Site: "github.com", Username: "alice", Password: "pw1", Notes: "工作"},
		{Site: "gmail.com", Username: "bob", Password: "pw2"},
	}
	raw, err := EncodeVault(key, p, entries)
	if err != nil {
		t.Fatal(err)
	}
	gotP, got, err := DecodeVault(raw, "m@ster-123456")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Site != "github.com" || got[0].Notes != "工作" {
		t.Fatalf("entry mismatch: %+v", got[0])
	}
	if gotP.Memory != p.Memory || gotP.Iter != p.Iter {
		t.Fatalf("params mismatch: %+v vs %+v", gotP, p)
	}
}

func TestVaultWrongPassword(t *testing.T) {
	p, _ := NewParams()
	key, _ := p.DeriveKey("m@ster-123456")
	raw, _ := EncodeVault(key, p, nil)
	if _, _, err := DecodeVault(raw, "nope-nope-nope"); err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestVaultTamperDetected(t *testing.T) {
	p, _ := NewParams()
	key, _ := p.DeriveKey("m@ster-123456")
	raw, _ := EncodeVault(key, p, []model.Entry{{Site: "x.com", Username: "u", Password: "p"}})
	raw[len(raw)-1] ^= 0xff
	if _, _, err := DecodeVault(raw, "m@ster-123456"); err == nil {
		t.Fatal("expected error on tampered record")
	}
}
