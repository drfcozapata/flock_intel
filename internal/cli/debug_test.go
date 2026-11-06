package cli

import (
	"strings"
	"testing"

	"flock/internal/php"
)

// firstValue is what getenv() returns for a key: the first of its occurrences.
func firstValue(env []string, key string) (string, int) {
	value, count := "", 0
	for _, kv := range env {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			if count == 0 {
				value = v
			}
			count++
		}
	}
	return value, count
}

func TestDebugEnvironLoadsXdebug(t *testing.T) {
	root := t.TempDir()
	// An inherited PHP_INI_SCAN_DIR (here Herd Lite's) must not survive either.
	base := []string{"HOME=/home/dev", "PHP_INI_SCAN_DIR=/herd-lite/bin:"}

	env := debugEnviron(base, php.CLIEnv(root, "8.4"), php.DebugEnv(root, "8.4"))

	scan, count := firstValue(env, "PHP_INI_SCAN_DIR")
	if count != 1 {
		t.Errorf("PHP_INI_SCAN_DIR appears %d times, want once: PHP would read the first one", count)
	}
	if !strings.Contains(scan, "conf.d-xdebug") {
		t.Errorf("PHP would scan %q, which doesn't load Xdebug", scan)
	}
	if home, _ := firstValue(env, "HOME"); home != "/home/dev" {
		t.Errorf("HOME = %q, want the inherited one", home)
	}
}
