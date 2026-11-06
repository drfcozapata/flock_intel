package binaries

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fileClient is an HTTP client that, like Flock on an Intel Mac, also reads file:// URLs.
func fileClient() *http.Client {
	t := &http.Transport{}
	RegisterFileProtocol(t)
	return &http.Client{Transport: t}
}

// writeManifest puts a manifest signed with priv where LocalBases(home) looks for it.
func writeManifest(t *testing.T, home string, priv ed25519.PrivateKey, body string) {
	t.Helper()
	dir := filepath.Join(home, LocalDirName, "manifest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, sig := signed(priv, body)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json.sig"), sig, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalBases(t *testing.T) {
	cases := map[string]string{
		"/Users/ana": "file:///Users/ana/.flock-intel/manifest/",
		"/Users/a b": "file:///Users/a%20b/.flock-intel/manifest/",
	}
	for home, want := range cases {
		if got := LocalBases(home); len(got) != 1 || got[0] != want {
			t.Errorf("LocalBases(%q) = %v, want [%s]", home, got, want)
		}
	}
}

func TestFetchFromLocalFiles(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	home := t.TempDir()
	writeManifest(t, home, priv, manifestJSON("stable", 42, "2099-01-01T00:00:00Z"))

	s := Source{Bases: LocalBases(home), Key: pub, HTTP: fileClient(), Cache: t.TempDir()}
	m, err := s.Fetch(context.Background(), Stable)
	if err != nil {
		t.Fatal(err)
	}
	if m.Serial != 42 {
		t.Errorf("serial %d, want 42", m.Serial)
	}
}

func TestFetchFromLocalFilesRejectsAnotherKey(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	home := t.TempDir()
	writeManifest(t, home, otherPriv, manifestJSON("stable", 42, "2099-01-01T00:00:00Z"))

	s := Source{Bases: LocalBases(home), Key: pub, HTTP: fileClient(), Cache: t.TempDir()}
	if _, err := s.Fetch(context.Background(), Stable); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("a manifest signed with another key: %v", err)
	}
}

func TestFetchFromLocalFilesWithoutAStore(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	s := Source{Bases: LocalBases(t.TempDir()), Key: pub, HTTP: fileClient(), Cache: t.TempDir()}
	if _, err := s.Fetch(context.Background(), Stable); err == nil {
		t.Fatal("a missing store must be an error")
	}
}

func TestInstallFromALocalFile(t *testing.T) {
	// A package as the build scripts leave it: a .tar.gz with several top-level entries (an
	// archive with a single top-level folder would have it dropped on extraction).
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"bin/hello": "hello", "LICENSE": "license"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(t.TempDir(), "demo-1.0.0-build1-macos-x64.tar.gz")
	if err := os.WriteFile(asset, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	e := Entry{Build: 1, URL: (&url.URL{Scheme: "file", Path: asset}).String(), SHA256: hex.EncodeToString(sum[:]), Size: int64(buf.Len())}

	dir := filepath.Join(t.TempDir(), "demo")
	if err := Install(context.Background(), fileClient(), e, dir, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "bin", "hello")); err != nil || string(got) != "hello" {
		t.Errorf("installed file: %q, %v", got, err)
	}

	// A package that doesn't match the checksum in the manifest is refused.
	e.SHA256 = strings.Repeat("0", 64)
	if err := Install(context.Background(), fileClient(), e, filepath.Join(t.TempDir(), "demo"), nil); err == nil {
		t.Error("a package with the wrong checksum was installed")
	}
}
