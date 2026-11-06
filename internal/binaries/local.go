package binaries

import (
	"net/http"
	"net/url"
	"path/filepath"
)

// LocalDirName is the folder in the user's home where an Intel Mac keeps its own packages
// (packages/) and their signed manifests (manifest/). Upstream publishes no macos-x64 packages,
// so they are built locally and served from there through file:// URLs.
const LocalDirName = ".flock-intel"

// LocalBases is the manifest base for the local store of the user whose home is home:
// file://<home>/.flock-intel/manifest/ (the trailing slash matters: Fetch appends file names).
func LocalBases(home string) []string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(home, LocalDirName, "manifest")) + "/"}
	return []string{u.String()}
}

// RegisterFileProtocol makes t serve file:// URLs from the local file system, so that the HTTP
// clients that read manifests and download packages can use a local store. Only the signed
// manifest and the SHA-256 it lists vouch for what is read, as with any other address.
func RegisterFileProtocol(t *http.Transport) {
	t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
}
