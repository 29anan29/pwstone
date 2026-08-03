package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"1.1.0", "v1.2.0", true},
		{"1.2.0", "v1.2.0", false},
		{"1.2.0", "v1.2.1", true},
		{"1.10.0", "v1.9.0", false},
		{"dev-abc123", "v1.2.0", true},
		{"2.0.0", "v1.2.0", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.cur, c.latest); got != c.want {
			t.Fatalf("IsNewer(%q,%q)=%v want %v", c.cur, c.latest, got, c.want)
		}
	}
}

func makeArchive(t *testing.T, binContent []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "linux-amd64/pw", Mode: 0o755, Size: int64(len(binContent))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binContent); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("binary-data")
	sum := sha256.Sum256(data)
	sums := fmt.Sprintf("deadbeef  other.tar.gz\n%s  pw_1.2.0_linux_amd64.tar.gz\n", hex.EncodeToString(sum[:]))
	if err := verifySHA256(data, []byte(sums), "pw_1.2.0_linux_amd64.tar.gz"); err != nil {
		t.Fatal(err)
	}
	sums = "0000  pw_1.2.0_linux_amd64.tar.gz\n"
	if err := verifySHA256(data, []byte(sums), "pw_1.2.0_linux_amd64.tar.gz"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestExtractBinary(t *testing.T) {
	archive := makeArchive(t, []byte("#!/bin/sh\necho hello"))
	path, err := extractBinary(archive, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "#!/bin/sh\necho hello" {
		t.Fatalf("extracted content mismatch: %q", got)
	}
}

func TestEndToEndUpdate(t *testing.T) {
	binContent := []byte("fake-pw-binary-v2")
	archive := makeArchive(t, binContent)
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  pw_1.2.0_linux_amd64.tar.gz\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		rel := Release{
			Tag:   "v1.2.0",
			Name:  "pw 1.2.0",
			Notes: "- 新增 update 命令\n- 修复若干问题",
			Assets: []Asset{
				{Name: "pw_1.2.0_linux_amd64.tar.gz", URL: "http://" + r.Host + "/archive"},
				{Name: "SHA256SUMS", URL: "http://" + r.Host + "/sums"},
			},
		}
		json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/sums", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("PW_UPDATE_API", srv.URL+"/latest")

	rel, err := Latest("")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v1.2.0" || !strings.Contains(rel.Notes, "update") {
		t.Fatalf("unexpected release: %+v", rel)
	}

	path, err := Download("", "v1.2.0", "pw_1.2.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(path))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binContent) {
		t.Fatalf("downloaded binary mismatch")
	}
}

func TestAssetNameFor(t *testing.T) {
	name := AssetNameFor("1.2.0")
	if !strings.HasPrefix(name, "pw_1.2.0_") {
		t.Fatalf("unexpected asset name: %s", name)
	}
}

func TestReplaceUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix only")
	}
	dir := t.TempDir()
	cur := filepath.Join(dir, "pw")
	if err := os.WriteFile(cur, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "pw.new")
	if err := os.WriteFile(newBin, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExec(cur, newBin); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cur)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary" {
		t.Fatalf("binary not replaced: %q", got)
	}
	if _, err := os.Stat(cur + ".old"); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed, err=%v", err)
	}
}
