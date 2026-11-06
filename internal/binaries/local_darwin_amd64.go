package binaries

import (
	"net/http"
	"os"
)

// On an Intel Mac there is no upstream manifest with packages for macos-x64. Flock reads the one
// built and signed locally in ~/.flock-intel/manifest, through file:// URLs, which the default
// HTTP transport (the one every client without a transport of its own uses) is taught here.
// The file name's _darwin_amd64 suffix is the build constraint.
func init() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		RegisterFileProtocol(t)
	}
	if home, err := os.UserHomeDir(); err == nil {
		Bases = LocalBases(home)
	}
}
