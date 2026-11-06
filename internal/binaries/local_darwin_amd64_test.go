package binaries

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On an Intel Mac the default bases and the default HTTP transport already read the local store.
func TestIntelMacDefaultsReadTheLocalStore(t *testing.T) {
	if len(Bases) != 1 || !strings.HasPrefix(Bases[0], "file:///") || !strings.HasSuffix(Bases[0], "/"+LocalDirName+"/manifest/") {
		t.Fatalf("Bases = %v", Bases)
	}
	f := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(f, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get((&url.URL{Scheme: "file", Path: f}).String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status %s", resp.Status)
	}
}
