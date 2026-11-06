# Soporte de Flock para Mac Intel: plan de implementación

> **Para agentes que lo ejecuten:** sub-skill requerida: `superpowers:subagent-driven-development` (recomendada) o `superpowers:executing-plans`. Los pasos usan casillas (`- [ ]`) para el seguimiento.

**Objetivo:** que Flock (app, CLI, daemon y paquetes descargables) funcione al 100 % en un MacBook Pro Intel x86_64, con binarios compilados o espejados en el propio Mac y un manifiesto firmado con una clave del usuario.

**Arquitectura:** el daemon de Flock, en `darwin/amd64`, lee un manifiesto ed25519 propio de `~/.flock-intel/manifest/` por `file://` y descarga de `~/.flock-intel/packages/` los paquetes `macos-x64`. Los paquetes los produce un repo local aparte (`flock_intel_binaries`, clon de `flock-php/binaries`, sin licencia, por eso nunca dentro de `flock_intel`, que es AGPL) con un `build-local.sh` que reutiliza los scripts y las comprobaciones de upstream. Diseño: `intel/specs/2026-11-06-intel-mac-support-design.md`.

**Stack:** Go 1.27 (Flock y `tools/localmanifest`), Swift 6.3 / SwiftPM (app), bash 3.2 (scripts de binarios), static-php-cli 2.8.5, `jq`, `gpgv`, `gh`.

---

## Convenciones (aplican a todas las tareas)

- **Rutas.** `FLOCK=$HOME/Sites/Others/flock_intel` (rama `intel-mac-support`), `BIN=$HOME/Sites/Others/flock_intel_binaries` (rama `macos-x64`), `STORE=$HOME/.flock-intel` (`packages/` y `manifest/`), clave privada `$HOME/.config/flock-intel/manifest.key`. Cada llamada a Bash empieza en una shell nueva: usa rutas absolutas.
- **Commits.** Locales, sin `git push` y **sin línea `Co-Authored-By`** ni ninguna atribución a Claude (petición del usuario). Mensajes en inglés, estilo `tipo(ámbito): resumen`.
- **bash 3.2.** `/bin/bash` de macOS es la 3.2.57. Todo script nuevo debe correr en ella: sin arrays asociativos, sin `mapfile`, sin `${var,,}`, y con `${arr[@]+"${arr[@]}"}` si el array puede estar vacío.
- **El arnés bloquea comandos de Bash que contengan el texto `curl` o `wget`.** Los scripts de upstream sí los usan por dentro (no pasa nada). Para descargar a mano usa `python3 -I`, `gh` o el navegador.
- **Antes de tocar el sistema del usuario hay que pedir confirmación:** `brew install`, `sudo`, `flock install`, parar `httpd`, puertos 80/443. Se marca con 🛑.
- **Línea base (2026-11-06, antes de cambiar nada):** `go test ./...` → 33 paquetes ok. `TestTunnelRunsOnceConnected` (`internal/share`) falla si la suite corre en paralelo y pasa aislado: `go test ./internal/share -run TestTunnel -count=1`. No es un fallo de Intel.
- **Versiones fijadas** (las más recientes de cada línea el 2026-11-06; si una desaparece, usa la siguiente de su línea):

| Paquete | Versión | Cómo |
|---|---|---|
| node | 16.20.2, 18.20.8, 20.20.2, 22.23.4, 24.22.2, 26.13.0 | espejo |
| mongodb | 7.0.45, 8.0.34, 9.0.4 | espejo |
| mysql | 8.0.46, 9.7.4 (8.4.13 si existe para Intel) | espejo |
| meilisearch / typesense / cloudflared | 1.56.1 / 29.1 y 30.2 / 2026.10.7 | espejo |
| valkey / postgres / mariadb | 9.1.3 / 16.15 / 11.8.9 | compilado |
| php + frankenphp | 8.5.12, 8.4.27, 8.3.36 (FrankenPHP 1.13.0) | compilado (static-php-cli 2.8.5) |

## Estructura de archivos

**Repo `flock_intel`** (rama `intel-mac-support`):

| Archivo | Acción | Responsabilidad |
|---|---|---|
| `internal/binaries/binaries.go` | modificar | `platform()`: `darwin/amd64` → `macos-x64` |
| `internal/binaries/binaries_test.go` | modificar | el test de plataformas espera `macos-x64` |
| `internal/binaries/local.go` | crear | `LocalDirName`, `LocalBases(home)`, `RegisterFileProtocol(t)` (probables en cualquier SO) |
| `internal/binaries/local_darwin_amd64.go` | crear | `init()` que activa `file://` y fija `Bases` solo en Mac Intel |
| `internal/binaries/local_test.go`, `local_darwin_amd64_test.go` | crear | tests de lo anterior |
| `internal/binaries/manifest.pub` | modificar | clave pública del usuario |
| `macos/build-app.sh`, `Makefile` | modificar | arquitectura por `ARCH` / `uname -m` |
| `README.md`, `intel/README.md` | modificar / crear | cómo construir y mantener la versión Intel |

**Repo `flock_intel_binaries`** (rama `macos-x64`, solo local):

| Archivo | Acción | Responsabilidad |
|---|---|---|
| `build-local.sh` | crear | orquesta `mirror`, `build`, `php` y `manifest` con las comprobaciones de upstream |
| `tools/localmanifest/{go.mod,main.go,manifest.go,keys.go,manifest_test.go}` | crear | genera, firma y verifica el manifiesto local; `keygen` |
| `scripts/macos/openssl.sh`, `packages/php/macos/build.sh` | modificar | arquitectura por `uname -m` |
| `packages/{node,mysql,mongodb,meilisearch,typesense,cloudflared}/mirror.sh` | modificar | caso `macos-x64` |
| `.gitignore` | modificar | `/work/`, `/tools/localmanifest/localmanifest` |

`scripts/verify-macos.sh` y `versions.json` **no cambian**: `verify-macos.sh` ya usa `otool -L` y rechaza `/usr/local` y `/opt/homebrew`, y `build-local.sh` lee de `versions.json` solo los ajustes (`env`, `.php.macos`) que no dependen de la plataforma.

---

### Tarea 0: Espacio de trabajo

**Archivos:**
- Crear: `$HOME/Sites/Others/flock_intel_binaries` (clon), `$HOME/.flock-intel/{packages,manifest}`, `$HOME/.config/flock-intel`
- Modificar: `$BIN/.gitignore`

- [ ] **Paso 1: Comprobar las herramientas necesarias**

```bash
for t in jq gpgv zip gh go cmake bison autoconf automake pkg-config xcrun codesign otool python3; do
  printf '%-11s %s\n' "$t" "$(command -v $t || echo FALTA)"
done
```

Esperado: ninguna línea con `FALTA`. Antes de esta tarea el equipo del usuario no tenía `re2c` ni se había comprobado `jq`/`gpgv`/`zip`.

- [ ] **Paso 2: 🛑 Si falta `jq` o `gpgv`, pedir permiso e instalarlos**

`jq` lo usan todos los scripts y `gpgv` (paquete `gnupg`) verifica la firma de Node.js. Propón al usuario ejecutar (con el prefijo `!` para que corra en su sesión):

```
! brew install jq gnupg
```

Repite el Paso 1. (`re2c` y el resto de dependencias de PHP se tratan en la Tarea 9.)

- [ ] **Paso 3: Clonar el repo de binarios, aislarlo de upstream y crear la rama**

```bash
gh repo clone flock-php/binaries "$HOME/Sites/Others/flock_intel_binaries"
cd "$HOME/Sites/Others/flock_intel_binaries"
git remote rename origin upstream
git remote set-url --push upstream no-push
git checkout -b macos-x64
git remote -v
```

Esperado: `upstream ... (fetch)` con la URL de `flock-php/binaries` y `upstream no-push (push)`. Así no se puede empujar por error.

- [ ] **Paso 4: Crear el almacén y el directorio de claves**

```bash
mkdir -p "$HOME/.flock-intel/packages" "$HOME/.flock-intel/manifest" "$HOME/.config/flock-intel"
chmod 700 "$HOME/.config/flock-intel"
ls -ld "$HOME/.flock-intel/packages" "$HOME/.flock-intel/manifest" "$HOME/.config/flock-intel"
```

Esperado: `drwxr-xr-x` para las dos primeras y `drwx------` para la tercera.

- [ ] **Paso 5: Ignorar directorios de trabajo en el repo de binarios**

Añade al final de `$BIN/.gitignore` (primero léelo con la herramienta Read):

```
/work/
/tools/localmanifest/localmanifest
```

- [ ] **Paso 6: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add .gitignore
git commit -m "chore: ignore the local work directory and the localmanifest binary"
git log --oneline -1
```

---

### Tarea 1: Plataforma `macos-x64` en Flock

**Archivos:**
- Modificar: `$FLOCK/internal/binaries/binaries_test.go` (línea 233)
- Modificar: `$FLOCK/internal/binaries/binaries.go` (líneas 133-152)

Todo en `$FLOCK`, rama `intel-mac-support`.

- [ ] **Paso 1: Cambiar el test (rojo)**

En `internal/binaries/binaries_test.go`, en `TestEmbeddedKeyAndPlatforms`, sustituye `{"darwin", "amd64", ""}` por `{"darwin", "amd64", "macos-x64"}`.

- [ ] **Paso 2: Ver que falla**

```bash
cd "$HOME/Sites/Others/flock_intel" && go test ./internal/binaries/ -run TestEmbeddedKeyAndPlatforms -count=1
```

Esperado: `FAIL` con `darwin/amd64: "", want "macos-x64"`.

- [ ] **Paso 3: Implementar**

En `internal/binaries/binaries.go` sustituye el comentario de `Platform()` por:

```go
// Platform names this machine in the manifest ("macos-arm64", "macos-x64", "windows-x64",
// "linux-x64", "linux-arm64"); "" where Flock has no packages. Upstream publishes no macos-x64
// packages: on an Intel Mac they are the ones built locally (see local.go).
```

y en `platform()`, tras el caso de `darwin`/`arm64`, añade:

```go
	case goos == "darwin" && goarch == "amd64":
		return "macos-x64"
```

- [ ] **Paso 4: Ver que pasa y que nada más se rompe**

```bash
cd "$HOME/Sites/Others/flock_intel"
go test ./internal/binaries/ -count=1
go test ./internal/php/... ./internal/node/... ./internal/catalog/... ./internal/postgres/... ./internal/frankenphp/... ./internal/cli/... ./internal/daemon/... ./internal/api/... -count=1
```

Esperado: todo `ok`.

- [ ] **Paso 5: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel"
git add internal/binaries/binaries.go internal/binaries/binaries_test.go
git commit -m "feat(binaries): name Intel Macs macos-x64"
```

---

### Tarea 2: Leer el almacén local por `file://` en Mac Intel

**Archivos:**
- Crear: `internal/binaries/local_test.go`, `internal/binaries/local_darwin_amd64_test.go`
- Crear: `internal/binaries/local.go`, `internal/binaries/local_darwin_amd64.go`

Verificado con anterioridad en una copia del repo: todos los tests pasan. `archive.Extract` descarta una única carpeta de primer nivel, por eso el paquete de prueba lleva dos entradas.

- [ ] **Paso 1: Escribir los tests (rojo)**

`internal/binaries/local_test.go`:

```go
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
```

`internal/binaries/local_darwin_amd64_test.go`:

```go
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
```

- [ ] **Paso 2: Ver que falla**

```bash
cd "$HOME/Sites/Others/flock_intel" && go test ./internal/binaries/ -count=1 2>&1 | head
```

Esperado: error de compilación `undefined: LocalBases` / `RegisterFileProtocol` / `LocalDirName`.

- [ ] **Paso 3: Implementar**

`internal/binaries/local.go`:

```go
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
```

`internal/binaries/local_darwin_amd64.go`:

```go
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
```

- [ ] **Paso 4: Ver que pasa**

```bash
cd "$HOME/Sites/Others/flock_intel" && gofmt -l internal/binaries; go vet ./internal/binaries/ && go test ./internal/binaries/ -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Esperado: `gofmt` sin salida y todas las líneas `--- PASS` (incluidas `TestLocalBases`, `TestFetchFromLocalFiles*`, `TestInstallFromALocalFile`, `TestIntelMacDefaultsReadTheLocalStore`), `ok flock/internal/binaries`.

- [ ] **Paso 5: Comprobar el resto de paquetes que usan clientes HTTP**

```bash
cd "$HOME/Sites/Others/flock_intel"
go test ./internal/php/... ./internal/node/... ./internal/catalog/... ./internal/postgres/... ./internal/frankenphp/... ./internal/cli/... ./internal/daemon/... ./internal/api/... -count=1
```

Esperado: todo `ok`.

- [ ] **Paso 6: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel"
git add internal/binaries/local.go internal/binaries/local_darwin_amd64.go internal/binaries/local_test.go internal/binaries/local_darwin_amd64_test.go
git commit -m "feat(binaries): read the Intel Mac's own packages from ~/.flock-intel through file://"
```

---

### Tarea 3: Compilar la app para Intel

**Archivos:**
- Modificar: `$FLOCK/macos/build-app.sh` (líneas 35-37 y 59-60)
- Modificar: `$FLOCK/Makefile` (línea 10)

- [ ] **Paso 1: Parametrizar la arquitectura en `macos/build-app.sh`**

Sustituye (la indentación del `case` es un tabulador):

```bash
# Apple silicon only: Flock doesn't support Intel Macs, and an arm64-only app won't open on one.
ARCHS=(--arch arm64)
swift build -c release --product FlockBar "${ARCHS[@]}"
```

por:

```bash
# ARCH=arm64 (Apple silicon) or x86_64 (Intel); default: this Mac's. An app is built for one
# architecture: the Go CLI inside it, and the packages Flock downloads, are not universal.
ARCH="${ARCH:-$(uname -m)}"
case "$ARCH" in
	arm64) GOARCH=arm64 ;;
	x86_64) GOARCH=amd64 ;;
	*) echo "ARCH must be arm64 or x86_64, not $ARCH" >&2; exit 1 ;;
esac
ARCHS=(--arch "$ARCH")
swift build -c release --product FlockBar "${ARCHS[@]}"
```

y en el `go build` del CLI cambia `GOARCH=arm64` por `GOARCH="$GOARCH"`:

```bash
(cd .. && CGO_ENABLED=0 GOOS=darwin GOARCH="$GOARCH" go build -ldflags "-s -w -X flock/internal/cli.Version=$VERSION" \
	-o "build/macos/Flock.app/Contents/MacOS/flock" ./cmd/flock)
```

- [ ] **Paso 2: Probar la protección contra una arquitectura inválida**

```bash
cd "$HOME/Sites/Others/flock_intel" && ARCH=ppc64 macos/build-app.sh; echo "exit: $?"
```

Esperado: `ARCH must be arm64 or x86_64, not ppc64` y `exit: 1`, sin llegar a ejecutar `swift build`.

- [ ] **Paso 3: Añadir el destino Intel a `make build-all`**

En `Makefile`, tras la línea `GOOS=darwin  GOARCH=arm64 ...` añade:

```make
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/flock-darwin-amd64 ./cmd/flock
```

(Es una línea de receta: empieza con un tabulador.)

- [ ] **Paso 4: Construir la app (≈ 2-3 min) y comprobar la arquitectura**

```bash
cd "$HOME/Sites/Others/flock_intel" && SKIP_DMG=1 make app 2>&1 | tail -5
lipo -archs build/macos/Flock.app/Contents/MacOS/FlockBar build/macos/Flock.app/Contents/MacOS/flock
codesign --verify --strict --deep build/macos/Flock.app && echo "firma ok"
```

Esperado: `built …/build/macos/Flock.app`, las dos líneas de `lipo` dicen `x86_64` y `firma ok`. (`build/macos/` y `macos/.build/` están en `.gitignore`.)

- [ ] **Paso 5: `swift test` en Intel**

```bash
cd "$HOME/Sites/Others/flock_intel/macos" && swift test 2>&1 | tail -6
```

Esperado: `Executed N tests, with 0 failures`. Si algún test falla por un motivo que no tiene que ver con la arquitectura, anótalo en `intel/README.md` (Tarea 11) y sigue.

- [ ] **Paso 6: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel"
git add macos/build-app.sh Makefile
git commit -m "build(macos): build the app for the architecture of the Mac it runs on"
```

---

### Tarea 4: Generador y firmador del manifiesto local

**Archivos (en `$BIN`):**
- Crear: `tools/localmanifest/go.mod`, `main.go`, `manifest.go`, `keys.go`, `manifest_test.go`

Verificado con anterioridad en un módulo aparte: los 4 tests pasan, y el código real de Flock aceptó el manifiesto que genera (firma, esquema, bandera LTS de Node, URL `file://`, SHA-256 y extracción).

- [ ] **Paso 1: `go.mod`**

`tools/localmanifest/go.mod`:

```
module localmanifest

go 1.24
```

- [ ] **Paso 2: Escribir los tests (rojo)** — `tools/localmanifest/manifest_test.go`:

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

// asset writes a file into dir and returns its SHA-256.
func asset(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestBuildListsTheLocalPackages(t *testing.T) {
	dir := t.TempDir()
	asset(t, dir, "postgres-16.15-build1-macos-x64.tar.gz", "old build")
	pg2 := asset(t, dir, "postgres-16.15-build2-macos-x64.tar.gz", "new build")
	asset(t, dir, "pgvector-pg16-0.8.0-build1-macos-x64.tar.gz", "ext")
	asset(t, dir, "php-8.6.0RC2-build1-macos-x64.tar.gz", "rc")
	asset(t, dir, "node-22.23.4-build1-macos-x64.tar.gz", "node lts")
	asset(t, dir, "node-22.23.4.json", `{"lts":"Jod"}`)
	asset(t, dir, "node-23.1.0-build1-macos-x64.tar.gz", "node current")
	asset(t, dir, "node-23.1.0.json", `{"lts":false}`)
	asset(t, dir, "mysql-8.0.46-build1-macos-arm64.tar.gz", "another platform")
	asset(t, dir, "notes.txt", "not a package")

	m, err := Build(dir, now, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != 1 || m.Serial != now.Unix() || !m.Expires.Equal(now.Add(24*time.Hour)) {
		t.Errorf("header: %+v", m)
	}
	if len(m.Packages) != 4 {
		t.Errorf("packages: %v", m.Packages)
	}
	pg := m.Packages["postgres"]["16.15"][Platform]
	if pg.Build != 2 || pg.SHA256 != pg2 || pg.Size != int64(len("new build")) {
		t.Errorf("the newest build must win: %+v", pg)
	}
	if want := "file://" + filepath.Join(dir, "postgres-16.15-build2-macos-x64.tar.gz"); pg.URL != want {
		t.Errorf("url %q, want %q", pg.URL, want)
	}
	if _, ok := m.Packages["pgvector-pg16"]["0.8.0"][Platform]; !ok {
		t.Error("a package name with a hyphen (pgvector-pg16) was not read")
	}
	if !m.Packages["php"]["8.6.0RC2"][Platform].Prerelease {
		t.Error("a release candidate must be marked prerelease")
	}
	if pg.Prerelease {
		t.Error("a release is not a prerelease")
	}
	if !m.Packages["node"]["22.23.4"][Platform].LTS || m.Packages["node"]["23.1.0"][Platform].LTS {
		t.Error("Node.js LTS flags come from node-<version>.json")
	}
	if _, ok := m.Packages["mysql"]; ok {
		t.Error("a package of another platform was listed")
	}
}

func TestNodeWithoutItsReleaseInfoIsAnError(t *testing.T) {
	dir := t.TempDir()
	asset(t, dir, "node-22.23.4-build1-macos-x64.tar.gz", "node")
	if _, err := Build(dir, now, time.Hour); err == nil || !strings.Contains(err.Error(), "node-22.23.4.json") {
		t.Fatalf("got %v", err)
	}
}

func TestWriteSignsEveryChannel(t *testing.T) {
	dir, out, keys := t.TempDir(), t.TempDir(), t.TempDir()
	asset(t, dir, "valkey-9.1.1-build1-macos-x64.tar.gz", "valkey")
	keyPath, pubPath := filepath.Join(keys, "manifest.key"), filepath.Join(keys, "manifest.pub")
	if err := GenerateKey(keyPath, pubPath); err != nil {
		t.Fatal(err)
	}
	priv, err := LoadKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := LoadPublicKey(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(out, dir, priv, now, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	for channel, file := range channelFiles {
		data, err := os.ReadFile(filepath.Join(out, file))
		if err != nil {
			t.Fatal(err)
		}
		sig, err := os.ReadFile(filepath.Join(out, file+".sig"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := Verify(pub, data, sig, channel, now)
		if err != nil {
			t.Fatalf("%s: %v", channel, err)
		}
		if _, ok := m.Packages["valkey"]["9.1.1"][Platform]; !ok {
			t.Errorf("%s lacks valkey", channel)
		}
		if _, err := Verify(pub, append(data, ' '), sig, channel, now); err == nil || !strings.Contains(err.Error(), "signature") {
			t.Errorf("%s: a tampered manifest was accepted: %v", channel, err)
		}
		if _, err := Verify(pub, data, sig, "other", now); err == nil || !strings.Contains(err.Error(), "channel") {
			t.Errorf("%s: a manifest taken for another channel: %v", channel, err)
		}
		if _, err := Verify(pub, data, sig, channel, now.Add(48*time.Hour)); err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("%s: an expired manifest was accepted: %v", channel, err)
		}
	}
}

func TestKeygenKeepsAnExistingKey(t *testing.T) {
	keys := t.TempDir()
	keyPath, pubPath := filepath.Join(keys, "sub", "manifest.key"), filepath.Join(keys, "manifest.pub")
	if err := GenerateKey(keyPath, pubPath); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(keyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the private key must be readable only by its owner: %v %v", info, err)
	}
	before, _ := os.ReadFile(keyPath)
	if err := GenerateKey(keyPath, pubPath); err == nil {
		t.Error("a second keygen replaced the key")
	}
	if after, _ := os.ReadFile(keyPath); string(after) != string(before) {
		t.Error("the key changed")
	}
}
```

- [ ] **Paso 3: Ver que falla**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries/tools/localmanifest" && go test ./... 2>&1 | head -5
```

Esperado: error de compilación `undefined: Build` / `Platform` / `GenerateKey`.

- [ ] **Paso 4: Implementar** — `tools/localmanifest/manifest.go`:

```go
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Platform is the only platform the local store has packages for.
const Platform = "macos-x64"

// Manifest is the JSON Flock reads (internal/binaries.Manifest in the Flock repository):
// package → version → platform → entry.
type Manifest struct {
	Schema    int                                    `json:"schema"`
	Channel   string                                 `json:"channel"`
	Serial    int64                                  `json:"serial"`
	Generated time.Time                              `json:"generated"`
	Expires   time.Time                              `json:"expires"`
	Packages  map[string]map[string]map[string]Entry `json:"packages"`
}

// Entry is one package version for one platform (internal/binaries.Entry).
type Entry struct {
	Build      int       `json:"build"`
	URL        string    `json:"url"`
	SHA256     string    `json:"sha256"`
	Size       int64     `json:"size"`
	Published  time.Time `json:"published"`
	Prerelease bool      `json:"prerelease,omitempty"`
	LTS        bool      `json:"lts,omitempty"` // Node.js: a long-term support release
}

// channelFiles are the manifests Flock may ask for. A local store has no promotion between
// channels, so all three list the same packages.
var channelFiles = map[string]string{
	"stable":       "manifest.json",
	"beta":         "manifest-beta.json",
	"experimental": "manifest-experimental.json",
}

// assetName is how the build scripts name a package: <package>-<version>-build<N>-macos-x64.<ext>.
// A package name can hold hyphens (pgvector-pg16); a version starts with a digit (8.6.0RC2).
var assetName = regexp.MustCompile(`^(.+?)-(\d[0-9A-Za-z.]*)-build(\d+)-` + Platform + `\.(?:tar\.gz|zip)$`)

var prereleaseVersion = regexp.MustCompile(`(?i)(rc|alpha|beta)\d*$`)

// Build lists the packages in dir. The newest build of each package version is the one listed.
// URLs are file:// addresses of the files in dir.
func Build(dir string, now time.Time, validFor time.Duration) (*Manifest, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := &Manifest{
		Schema: 1, Serial: now.Unix(), Generated: now.UTC(), Expires: now.Add(validFor).UTC(),
		Packages: map[string]map[string]map[string]Entry{},
	}
	for _, f := range files {
		g := assetName.FindStringSubmatch(f.Name())
		if g == nil || f.IsDir() {
			continue
		}
		pkg, version := g[1], g[2]
		build, _ := strconv.Atoi(g[3])
		if old, ok := m.Packages[pkg][version][Platform]; ok && old.Build >= build {
			continue
		}
		path, err := filepath.Abs(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		sum, err := sha256File(path)
		if err != nil {
			return nil, err
		}
		e := Entry{
			Build: build, URL: (&url.URL{Scheme: "file", Path: path}).String(), SHA256: sum, Size: info.Size(),
			Published: info.ModTime().UTC().Truncate(time.Second), Prerelease: prereleaseVersion.MatchString(version),
		}
		if pkg == "node" {
			if e.LTS, err = nodeLTS(dir, version); err != nil {
				return nil, err
			}
		}
		if m.Packages[pkg] == nil {
			m.Packages[pkg] = map[string]map[string]Entry{}
		}
		if m.Packages[pkg][version] == nil {
			m.Packages[pkg][version] = map[string]Entry{}
		}
		m.Packages[pkg][version][Platform] = e
	}
	return m, nil
}

// nodeLTS reads node-<version>.json, which packages/node/mirror.sh writes next to the archive:
// {"lts": false} or {"lts": "Jod"}. Flock picks the Node.js to install first by it, so a Node.js
// package without that file is an error rather than a silent "not LTS".
func nodeLTS(dir, version string) (bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "node-"+version+".json"))
	if err != nil {
		return false, fmt.Errorf("node %s: %w (packages/node/mirror.sh writes it next to the archive)", version, err)
	}
	var info struct {
		LTS json.RawMessage `json:"lts"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return false, fmt.Errorf("node-%s.json: %w", version, err)
	}
	return strings.HasPrefix(string(info.LTS), `"`), nil
}

// Write builds the manifest of dir and writes, signed with key, one file per channel (and its
// .sig) into out. Each file is replaced atomically.
func Write(out, dir string, key ed25519.PrivateKey, now time.Time, validFor time.Duration) (*Manifest, error) {
	base, err := Build(dir, now, validFor)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	for channel, file := range channelFiles {
		m := *base
		m.Channel = channel
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return nil, err
		}
		data = append(data, '\n')
		sig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, data)) + "\n")
		if err := writeAtomic(filepath.Join(out, file), data); err != nil {
			return nil, err
		}
		if err := writeAtomic(filepath.Join(out, file+".sig"), sig); err != nil {
			return nil, err
		}
	}
	return base, nil
}

// Verify checks a manifest the way Flock does (internal/binaries.Source.verify): the signature,
// the schema, the channel and the expiry.
func Verify(pub ed25519.PublicKey, data, sig []byte, channel string, now time.Time) (*Manifest, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || len(raw) != ed25519.SignatureSize || !ed25519.Verify(pub, data, raw) {
		return nil, errors.New("the signature doesn't verify")
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse the manifest: %w", err)
	}
	if m.Schema != 1 {
		return nil, fmt.Errorf("the manifest has schema %d, Flock reads 1", m.Schema)
	}
	if m.Channel != channel {
		return nil, fmt.Errorf("the manifest is for the %q channel, not %q", m.Channel, channel)
	}
	if now.After(m.Expires) {
		return nil, fmt.Errorf("the manifest expired on %s", m.Expires.Format(time.DateOnly))
	}
	return &m, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

`tools/localmanifest/keys.go`:

```go
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateKey creates an ed25519 key pair: the private seed (base64) in keyPath, readable only by
// the user, and the public key (base64, the format of Flock's internal/binaries/manifest.pub) in
// pubPath. It refuses to replace a key: a new one means rebuilding the app, which embeds the
// public key.
func GenerateKey(keyPath, pubPath string) error {
	if _, err := os.Stat(keyPath); err == nil {
		return fmt.Errorf("%s exists: not replacing a signing key", keyPath)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(pubPath, []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644)
}

// LoadKey reads the private seed written by GenerateKey. MANIFEST_SIGNING_KEY, when set, holds
// the seed instead of the file (the same variable upstream's flockrel uses).
func LoadKey(path string) (ed25519.PrivateKey, error) {
	raw := os.Getenv("MANIFEST_SIGNING_KEY")
	if raw == "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		raw = string(data)
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s: not a base64 ed25519 seed", path)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// LoadPublicKey reads a base64 public key file.
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%s: not a base64 ed25519 public key", path)
	}
	return pub, nil
}
```

`tools/localmanifest/main.go`:

```go
// Command localmanifest builds and signs the package manifest that Flock reads on an Intel Mac.
//
// Upstream's manifest has no macos-x64 packages and is signed with a key that is not ours, so the
// packages built by build-local.sh are listed in a manifest of our own, signed with our own key
// (the public half is internal/binaries/manifest.pub in the Flock repository).
//
//	localmanifest keygen   [-key FILE] [-pub FILE]
//	localmanifest manifest [-packages DIR] [-out DIR] [-key FILE] [-days N]
//	localmanifest verify   [-pub FILE] [-channel NAME] MANIFEST SIGNATURE
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "manifest":
		err = runManifest(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "localmanifest:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: localmanifest keygen|manifest|verify [flags]")
	os.Exit(2)
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "localmanifest:", err)
		os.Exit(1)
	}
	return h
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	key := fs.String("key", filepath.Join(home(), ".config", "flock-intel", "manifest.key"), "where the private seed goes")
	pub := fs.String("pub", "manifest.pub", "where the public key goes")
	fs.Parse(args)
	if err := GenerateKey(*key, *pub); err != nil {
		return err
	}
	fmt.Printf("private key %s (keep it out of any repository)\npublic key  %s\n", *key, *pub)
	return nil
}

func runManifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	store := filepath.Join(home(), ".flock-intel")
	packages := fs.String("packages", filepath.Join(store, "packages"), "folder with the built packages")
	out := fs.String("out", filepath.Join(store, "manifest"), "folder for the manifests and their signatures")
	key := fs.String("key", filepath.Join(home(), ".config", "flock-intel", "manifest.key"), "private seed")
	days := fs.Int("days", 365, "days until the manifest expires (Flock installs nothing from an expired one)")
	fs.Parse(args)
	priv, err := LoadKey(*key)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	m, err := Write(*out, *packages, priv, now, time.Duration(*days)*24*time.Hour)
	if err != nil {
		return err
	}
	n := 0
	for _, versions := range m.Packages {
		n += len(versions)
	}
	fmt.Printf("%d package versions in %d packages, signed into %s\n", n, len(m.Packages), *out)
	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	pubPath := fs.String("pub", "manifest.pub", "public key")
	channel := fs.String("channel", "stable", "channel the manifest must be for")
	fs.Parse(args)
	if fs.NArg() != 2 {
		return fmt.Errorf("verify needs the manifest and its signature")
	}
	pub, err := LoadPublicKey(*pubPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(fs.Arg(1))
	if err != nil {
		return err
	}
	m, err := Verify(pub, data, sig, *channel, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("ok: %s channel, serial %d, expires %s\n", m.Channel, m.Serial, m.Expires.Format(time.DateOnly))
	return nil
}
```

- [ ] **Paso 5: Ver que pasa**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries/tools/localmanifest" && gofmt -l .; go vet ./... && go test ./... -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Esperado: 4 líneas `--- PASS` (`TestBuildListsTheLocalPackages`, `TestNodeWithoutItsReleaseInfoIsAnError`, `TestWriteSignsEveryChannel`, `TestKeygenKeepsAnExistingKey`) y `ok localmanifest`.

- [ ] **Paso 6: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add tools/localmanifest
git commit -m "feat(tools): localmanifest builds and signs the manifest of a local macos-x64 store"
```

---

### Tarea 5: Clave de firma y clave pública embebida en Flock

**Archivos:**
- Crear (fuera de cualquier repo): `$HOME/.config/flock-intel/manifest.key`
- Modificar: `$FLOCK/internal/binaries/manifest.pub`

- [ ] **Paso 1: Compilar la herramienta y generar el par de claves**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
go -C tools/localmanifest build -o localmanifest .
tools/localmanifest/localmanifest keygen -pub "$HOME/Sites/Others/flock_intel/internal/binaries/manifest.pub"
ls -l "$HOME/.config/flock-intel/manifest.key"
```

Esperado: `private key …/manifest.key`, `public key …/internal/binaries/manifest.pub` y permisos `-rw-------`. Si el comando dice que la clave ya existe, **no la reemplaces**: detente y avisa al usuario.

- [ ] **Paso 2: 🛑 Pedir al usuario que respalde la clave privada**

Dile que copie el contenido de `~/.config/flock-intel/manifest.key` a su gestor de contraseñas. Si se pierde, hay que generar otra, recompilar la app (la pública va embebida) y volver a firmar. No muestres la clave en la conversación.

- [ ] **Paso 3: Comprobar que Flock acepta la clave**

```bash
cd "$HOME/Sites/Others/flock_intel"
git status --short
go test ./internal/binaries/ -count=1
```

Esperado: `M internal/binaries/manifest.pub` como único cambio y `ok`. (`TestEmbeddedKeyAndPlatforms` comprueba que la clave embebida mide 32 bytes.)

- [ ] **Paso 4: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel"
git add internal/binaries/manifest.pub
git commit -m "feat(binaries): verify the local store with this fork's own manifest key"
```

---

### Tarea 6: Extremo a extremo con Node 22 (`build-local.sh` y espejo)

**Archivos (en `$BIN`):**
- Crear: `build-local.sh`
- Modificar: `packages/node/mirror.sh`

- [ ] **Paso 1: Añadir `macos-x64` al espejo de Node**

En `packages/node/mirror.sh`, tras la línea `macos-arm64) name="node-v$V-darwin-arm64.tar.gz" ;;` añade:

```bash
  macos-x64) name="node-v$V-darwin-x64.tar.gz" ;;
```

- [ ] **Paso 2: Crear `build-local.sh`** (compatible con bash 3.2)

```bash
#!/usr/bin/env bash
# Builds or mirrors packages for macos-x64 on this Mac, checks them and stores them in
# $FLOCK_INTEL_STORE/packages (default ~/.flock-intel/packages). It replaces upstream's GitHub
# Actions workflows (mirror.yml, build-service.yml, build-php-macos.yml) for one machine and one
# platform; the package scripts and the checks are upstream's own.
#
# Usage: build-local.sh mirror <package> <version>   a vendor's binary, checked against the vendor's checksum or signature
#        build-local.sh build  <package> <version>   built here: postgres, mariadb, valkey
#        build-local.sh php    <version>             PHP (php, php-fpm) and FrankenPHP for that PHP release
#        build-local.sh manifest                     lists the stored packages and signs the manifests
#
# BUILD=<n> sets the build number of what is stored (default 1): raise it to replace a package
# Flock already installed. WORK_ROOT moves the compile directories (default ./work).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

PLATFORM=macos-x64
STORE="${FLOCK_INTEL_STORE:-$HOME/.flock-intel}"
PACKAGES="$STORE/packages"
WORK="${WORK_ROOT:-$ROOT/work}"
BUILD="${BUILD:-1}"
# A bare macOS PATH, as on a user's Mac without Homebrew: the checks run with nothing else.
CLEAN_PATH=/usr/bin:/bin:/usr/sbin:/sbin
export COPYFILE_DISABLE=1 # bsdtar: no AppleDouble (._*) entries in the archives

[[ "$(uname -m)" == x86_64 ]] || { echo "build-local.sh builds $PLATFORM: run it on an Intel Mac" >&2; exit 1; }
mkdir -p "$PACKAGES" "$WORK"
if [[ -z "${GITHUB_TOKEN:-}" ]] && command -v gh >/dev/null; then
	GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)" # lifts GitHub's anonymous rate limit
fi
export GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# settings <package>: exports what versions.json lists under the package's env (OPENSSL_VERSION, MIN_OS, SIGNER_MACOS, ...).
settings() {
	local line
	while IFS= read -r line; do
		[[ -n "$line" ]] && export "$line"
	done < <(jq -r --arg p "$1" '.packages[$p].env // {} | to_entries[] | "\(.key)=\(.value)"' versions.json)
	return 0
}

# asset <package> <version>: the package's file name in the store.
asset() { echo "$1-$2-build$BUILD-$PLATFORM.tar.gz"; }

# smoke <package> <archive>: starts the service the way Flock does (scripts/smoke-service.sh).
smoke() {
	env -i HOME="$HOME" PATH="$CLEAN_PATH" TMPDIR="${TMPDIR:-/tmp}" SIGNER_MACOS="${SIGNER_MACOS:-}" \
		bash scripts/smoke-service.sh "$1" "$2"
}

# package_info <package> <version>: the flock-package.json that build-service.yml writes into a package.
package_info() {
	jq -n --arg package "$1" --arg version "$2" --argjson build "$BUILD" --arg platform "$PLATFORM" \
		--arg commit "$(git rev-parse HEAD)" --arg built "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--argjson env "$(jq --arg p "$1" '.packages[$p].env // {}' versions.json)" \
		'{package: $package, version: $version, build: $build, platform: $platform, built: $built, commit: $commit, settings: $env}'
}

mirror() {
	local pkg="$1" version="$2" file
	file="$PACKAGES/$(asset "$pkg" "$version")"
	settings "$pkg"
	bash "packages/$pkg/mirror.sh" "$version" "$PLATFORM" "$file"
	smoke "$pkg" "$file"
	echo "stored $file"
}

build_service() {
	local pkg="$1" version="$2" file out
	file="$PACKAGES/$(asset "$pkg" "$version")"
	out="$WORK/$pkg-$version-out"
	settings "$pkg"
	(cd "$WORK" && MACOSX_DEPLOYMENT_TARGET="${MIN_OS:-13.0}" bash "$ROOT/packages/$pkg/macos/build.sh" "$version" "$out")
	bash scripts/verify-macos.sh "$out" "${MIN_OS:-13.0}"
	package_info "$pkg" "$version" >"$out/flock-package.json"
	tar -C "$out" -czf "$file" .
	smoke "$pkg" "$file"
	echo "stored $file"
}

build_php() {
	local version="$1" minor frankenphp pkg file out line
	minor="${version%.*}"
	while IFS= read -r line; do
		export "$line"
	done < <(jq -r '.php.macos | "SPC_VERSION=\(.spc)", "MIN_OS=\(.min_os)", "STATIC_EXTS=\(.static_extensions)", "SHARED_EXTS=\(.shared_extensions)"' versions.json)
	frankenphp="$(jq -r .frankenphp.version versions.json)"
	for pkg in php frankenphp; do
		file="$PACKAGES/$(asset "$pkg" "$version")"
		out="$WORK/$pkg-$version-out"
		(cd "$WORK" && TARGET="$pkg" FRANKENPHP_VERSION="$frankenphp" WORK_DIR="$WORK/spc-$pkg-$minor" \
			MACOSX_DEPLOYMENT_TARGET="$MIN_OS" bash "$ROOT/packages/php/macos/build.sh" "$version" "$out")
		bash scripts/verify-macos.sh "$out" "$MIN_OS"
		env -i HOME="$HOME" PATH="$CLEAN_PATH" TMPDIR="${TMPDIR:-/tmp}" STATIC_EXTS="$STATIC_EXTS" SHARED_EXTS="$SHARED_EXTS" VERSION="$version" \
			bash scripts/smoke-php-package.sh "$pkg" "$out"
		jq -n --arg package "$pkg" --arg version "$version" --argjson build "$BUILD" --arg platform "$PLATFORM" \
			--arg commit "$(git rev-parse HEAD)" --arg spc "$SPC_VERSION" --arg min_os "$MIN_OS" \
			--arg static "$STATIC_EXTS" --arg shared "$SHARED_EXTS" --arg franken "$([[ $pkg == frankenphp ]] && echo "$frankenphp")" \
			--arg built "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
			'{package: $package, version: $version, build: $build, platform: $platform, min_os: $min_os, built: $built,
			  commit: $commit, static_php_cli: $spc, extensions: {compiled_in: ($static | split(",")), shared: ($shared | split(","))}}
			 + (if $franken != "" then {frankenphp: $franken} else {} end)' >"$out/flock-package.json"
		tar -C "$out" -czf "$file" .
		echo "stored $file"
	done
}

manifest() {
	local pub="${FLOCK_MANIFEST_PUB:-$HOME/Sites/Others/flock_intel/internal/binaries/manifest.pub}"
	go -C tools/localmanifest build -o localmanifest .
	tools/localmanifest/localmanifest manifest -packages "$PACKAGES" -out "$STORE/manifest"
	tools/localmanifest/localmanifest verify -pub "$pub" "$STORE/manifest/manifest.json" "$STORE/manifest/manifest.json.sig"
}

case "${1:-}" in
	mirror) mirror "${2:?package}" "${3:?version}" ;;
	build) build_service "${2:?package}" "${3:?version}" ;;
	php) build_php "${2:?version}" ;;
	manifest) manifest ;;
	*) sed -n '2,13p' "$0" >&2; exit 2 ;;
esac
```

Luego `chmod +x build-local.sh`.

- [ ] **Paso 3: Probar el uso y la protección de arquitectura**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && chmod +x build-local.sh && ./build-local.sh; echo "exit: $?"
```

Esperado: imprime la cabecera de uso y `exit: 2`.

- [ ] **Paso 4: Espejar Node 22.23.4**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh mirror node 22.23.4 2>&1 | tail -15
ls -l "$HOME/.flock-intel/packages/"
```

Esperado: líneas `✓` (`SHASUMS256.txt signed by a Node.js releaser`, SHA-256 del archivo, `node-22.23.4.json: LTS "Jod"` y las del smoke test: `node v22.23.4`, `npm …`, la firma de código del proveedor `Node.js|OpenJS` si procede) y `stored …/node-22.23.4-build1-macos-x64.tar.gz`. En `packages/` deben quedar el `.tar.gz` y `node-22.23.4.json`. Si falla por `gpgv`, vuelve al Paso 2 de la Tarea 0.

- [ ] **Paso 5: Generar y verificar el manifiesto**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh manifest
```

Esperado: `1 package versions in 1 packages, signed into …/.flock-intel/manifest` y `ok: stable channel, serial …, expires …`.

- [ ] **Paso 6: Construir el CLI de Flock con la clave embebida e instalar Node desde el almacén**

```bash
cd "$HOME/Sites/Others/flock_intel"
CGO_ENABLED=0 go build -o bin/flock ./cmd/flock
export FLOCK_HOME="$(mktemp -d)"
bin/flock node install 22
bin/flock node list
find "$FLOCK_HOME" -type f -name node -perm -u+x | head -1 | xargs -I{} {} --version
```

Esperado: `node install` termina sin error, `node list` muestra 22.23.4 y el último comando imprime `v22.23.4`. Esto prueba el camino completo: manifiesto firmado con tu clave, descarga por `file://`, SHA-256 y extracción. (`bin/` está en `.gitignore`.)

- [ ] **Paso 7: Comprobar que un manifiesto manipulado se rechaza**

```bash
printf ' ' >> "$HOME/.flock-intel/manifest/manifest.json"
cd "$HOME/Sites/Others/flock_intel" && FLOCK_HOME="$(mktemp -d)" bin/flock node install 22 2>&1 | tail -3
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh manifest
```

Esperado: el segundo comando falla con un mensaje que menciona la firma (`signature`), porque el `FLOCK_HOME` es nuevo y no hay manifiesto en caché al que recurrir; la regeneración del manifiesto deja el almacén otra vez válido y el último comando termina con `ok: stable channel…`.

- [ ] **Paso 8: Commit** (en `$BIN`)

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add build-local.sh packages/node/mirror.sh
git commit -m "feat: build-local.sh and a macos-x64 case for the Node.js mirror"
```

---

### Tarea 7: Resto de descargas oficiales

**Archivos (en `$BIN`):** `packages/{mysql,mongodb,meilisearch,typesense,cloudflared}/mirror.sh`, y de nuevo `node` para las otras líneas.

- [ ] **Paso 1: Añadir el caso `macos-x64` a cada espejo**

Con la herramienta Edit, tras la línea `macos-arm64)` de cada archivo:

`packages/mysql/mirror.sh` (Oracle nombra el archivo por el macOS mínimo, que cambia con el tiempo):
```bash
  macos-x64) names=("mysql-$V-macos15-x86_64.tar.gz" "mysql-$V-macos14-x86_64.tar.gz" "mysql-$V-macos13-x86_64.tar.gz") ;;
```

`packages/mongodb/mirror.sh`:
```bash
  macos-x64) urls=("https://fastdl.mongodb.org/osx/mongodb-macos-x86_64-$V.tgz") ;;
```

`packages/meilisearch/mirror.sh`:
```bash
  macos-x64) asset=meilisearch-macos-amd64; bin=meilisearch ;;
```

`packages/typesense/mirror.sh` (y actualiza su línea de `Usage:` para incluir `macos-x64`):
```bash
  macos-x64) target=darwin-amd64 ;;
```

`packages/cloudflared/mirror.sh`:
```bash
  macos-x64) asset="cloudflared-darwin-amd64.tgz" ;;
```

- [ ] **Paso 2: Espejar, uno por uno, y anotar el resultado**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
for spec in "node 16.20.2" "node 18.20.8" "node 20.20.2" "node 24.22.2" "node 26.13.0" \
            "cloudflared 2026.10.7" "typesense 29.1" "typesense 30.2" "meilisearch 1.56.1" \
            "mongodb 7.0.45" "mongodb 8.0.34" "mongodb 9.0.4" \
            "mysql 8.0.46" "mysql 9.7.4"; do
  set -- $spec
  if ./build-local.sh mirror "$1" "$2" >"work/mirror-$1-$2.log" 2>&1; then echo "OK    $spec"; else echo "FALLA $spec (work/mirror-$1-$2.log)"; fi
done
```

Esperado: `OK` para todos. Para un `FALLA`, lee la última parte del log (`tail -20`) y corrige la causa (nombre de asset, versión que ya no existe → usa la siguiente de su línea).

- [ ] **Paso 3: MySQL 8.4 (puede no existir para Intel)**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh mirror mysql 8.4.13 2>&1 | tail -4
```

Si termina con `not found on the MySQL CDN or archives`, registra "MySQL 8.4 no disponible para Intel" para `intel/README.md` y continúa. No es un fallo del plan.

- [ ] **Paso 4: Regenerar el manifiesto y revisar su contenido**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh manifest
python3 -I - <<'EOF'
import json, os
m = json.load(open(os.path.expanduser("~/.flock-intel/manifest/manifest.json")))
for pkg, vs in sorted(m["packages"].items()):
    print(f"{pkg:12}", ", ".join(sorted(vs)))
EOF
```

Esperado: node con 6 versiones, mongodb 3, mysql 2 (o 3), meilisearch, typesense 2, cloudflared.

- [ ] **Paso 5: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add packages
git commit -m "feat: macos-x64 cases for the MySQL, MongoDB, Meilisearch, Typesense and cloudflared mirrors"
```

---

### Tarea 8: Compilar Valkey, PostgreSQL 16 y MariaDB 11.8

**Archivos (en `$BIN`):** `scripts/macos/openssl.sh`

Estos scripts ya fijan un `PATH` limpio y el de MariaDB ignora `/usr/local`; `scripts/verify-macos.sh` falla si algún binario enlaza algo de `/usr/local` (en Intel ahí vive Homebrew, y `clang` busca por defecto en `/usr/local/include` y `ld` en `/usr/local/lib`).

- [ ] **Paso 1: Arquitectura de OpenSSL por `uname -m`** (`scripts/macos/openssl.sh`)

Antes de la línea `rm -rf "$PREFIX"` añade:

```bash
case "$(uname -m)" in
  arm64) target=darwin64-arm64-cc ;;
  x86_64) target=darwin64-x86_64-cc ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac
```

y en la línea `&& ./Configure darwin64-arm64-cc no-shared …` cambia `darwin64-arm64-cc` por `"$target"`:

```bash
  && ./Configure "$target" no-shared no-tests no-docs no-apps --prefix="$PREFIX" --libdir=lib \
```

- [ ] **Paso 2: Valkey (el más rápido; mide el tiempo)**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && time ./build-local.sh build valkey 9.1.3 2>&1 | tail -15
```

Esperado: `✓ N Mach-O files are portable`, las líneas `✓` del smoke test de Valkey (`SET/GET`, `SHUTDOWN`) y `stored …/valkey-9.1.3-build1-macos-x64.tar.gz`. Anota el tiempo.

- [ ] **Paso 3: PostgreSQL 16.15 (incluye OpenSSL estático; mide el tiempo)**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && time ./build-local.sh build postgres 16.15 >work/build-postgres.log 2>&1; echo "exit: $?"; tail -15 work/build-postgres.log
```

Esperado: `exit: 0`, `✓ … Mach-O files are portable`, el smoke test (`createdb`, `pg_dump | pg_restore`, contrib) y `stored …`. Si `verify-macos.sh` informa `✗ … links /usr/local/…`, **no sigas**: es contaminación de Homebrew. Identifica la biblioteca (`otool -L` sobre el binario), busca el flag de `configure` que la activó y desactívalo, o pide al usuario permiso para `brew unlink <fórmula>` mientras dura la compilación.

- [ ] **Paso 4: 🛑 Informar al usuario de los tiempos y pedir luz verde para MariaDB y PHP**

Con los tiempos de Valkey y PostgreSQL, da una estimación honesta del resto (MariaDB y los 3 PHP + 3 FrankenPHP) y espera su confirmación antes de lanzar compilaciones largas.

- [ ] **Paso 5: MariaDB 11.8.9 (en segundo plano; es la compilación más larga de las bases de datos)**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh build mariadb 11.8.9 >work/build-mariadb.log 2>&1; echo "exit: $?" >>work/build-mariadb.log
```

Lanza este comando con `run_in_background` y espera la notificación; no hagas sondeos. Después: `tail -15 work/build-mariadb.log`. Esperado: `exit: 0`, `✓ … portable`, smoke test de `mariadb | mysql` (inicializa, arranca en el 33069, `shutdown`) y `stored …`.

- [ ] **Paso 6: Regenerar el manifiesto y comprobar**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh manifest
```

- [ ] **Paso 7: Commit**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add scripts/macos/openssl.sh
git commit -m "build: pick OpenSSL's darwin target from the machine's architecture"
```

---

### Tarea 9: Compilar PHP 8.5, 8.4, 8.3 y FrankenPHP

**Archivos (en `$BIN`):** `packages/php/macos/build.sh`

- [ ] **Paso 1: Arquitectura por `uname -m`** (`packages/php/macos/build.sh`)

Sustituye en la línea de `SPC_CMD_VAR_PHP_MAKE_EXTRA_CFLAGS` el fragmento `--target=arm64-apple-darwin` por `--target=$(uname -m)-apple-darwin`.

Sustituye el bloque:

```bash
echo "::group::static-php-cli $SPC_VERSION"
if [[ ! -x ./spc ]]; then
  curl -fsSL "https://github.com/crazywhalecc/static-php-cli/releases/download/$SPC_VERSION/spc-macos-aarch64.tar.gz" | tar xz
  chmod +x spc
fi
```

por:

```bash
echo "::group::static-php-cli $SPC_VERSION"
case "$(uname -m)" in
  arm64) spc_arch=aarch64 ;;
  x86_64) spc_arch=x86_64 ;;
  *) echo "::error::unsupported architecture $(uname -m)"; exit 1 ;;
esac
if [[ ! -x ./spc ]]; then
  curl -fsSL "https://github.com/crazywhalecc/static-php-cli/releases/download/$SPC_VERSION/spc-macos-$spc_arch.tar.gz" | tar xz
  chmod +x spc
fi
```

- [ ] **Paso 2: Comprobar la sintaxis**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && bash -n packages/php/macos/build.sh && bash -n scripts/macos/openssl.sh && echo "sintaxis ok"
```

- [ ] **Paso 3: 🛑 Ver qué instalaría `spc doctor` y pedir permiso**

`build.sh` ejecuta `./spc doctor --auto-fix`, que instala con Homebrew lo que falte. Descarga el binario de `spc` y mira, sin arreglar nada, qué falta:

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && mkdir -p work/spc-php-8.5
python3 -I - <<'EOF'
import urllib.request, tarfile, io, os
url = "https://github.com/crazywhalecc/static-php-cli/releases/download/2.8.5/spc-macos-x86_64.tar.gz"
data = urllib.request.urlopen(url, timeout=60).read()
tarfile.open(fileobj=io.BytesIO(data)).extractall("work/spc-php-8.5", filter="data")
os.chmod("work/spc-php-8.5/spc", 0o755)
print("spc descargado")
EOF
cd work/spc-php-8.5 && ./spc --version && ./spc doctor </dev/null 2>&1 | tail -25
```

Muestra al usuario la lista de paquetes de Homebrew que `doctor` quiere instalar (por ejemplo `re2c`) y pide permiso. Si acepta, que ejecute `! cd ~/Sites/Others/flock_intel_binaries/work/spc-php-8.5 && ./spc doctor --auto-fix` (es interactivo). Después, el `--auto-fix` de `build.sh` no tendrá nada que hacer.

- [ ] **Paso 4: Compilar PHP 8.5.12 y FrankenPHP (en segundo plano; el más largo)**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh php 8.5.12 >work/build-php-8.5.log 2>&1; echo "exit: $?" >>work/build-php-8.5.log
```

Lanza con `run_in_background` y espera la notificación. Después: `tail -30 work/build-php-8.5.log`. Esperado: `exit: 0`; para `php` y para `frankenphp`: `✓ … portable`, el smoke test (todas las extensiones cargan, las compartidas de `ext/`) y `stored …/php-8.5.12-build1-macos-x64.tar.gz` y `…/frankenphp-8.5.12-build1-macos-x64.tar.gz`.

Si una extensión **estática** falla (`print_spc_errors` muestra el error), para y trátalo como el riesgo anotado en el diseño: aísla cuál es, propón al usuario quitarla o parchearla solo en esta build (editando una copia de `STATIC_EXTS`, nunca `versions.json` de upstream) y registra la decisión en `intel/README.md`. Las extensiones **compartidas** que fallen no detienen la compilación: `build.sh` reintenta sin ellas y avisa con `::warning::`.

- [ ] **Paso 5: Compilar 8.4.27 y 8.3.36**

Igual que el Paso 4, una versión cada vez:

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh php 8.4.27 >work/build-php-8.4.log 2>&1; echo "exit: $?" >>work/build-php-8.4.log
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh php 8.3.36 >work/build-php-8.3.log 2>&1; echo "exit: $?" >>work/build-php-8.3.log
```

- [ ] **Paso 6: Regenerar el manifiesto y revisarlo**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries" && ./build-local.sh manifest
python3 -I - <<'EOF'
import json, os
m = json.load(open(os.path.expanduser("~/.flock-intel/manifest/manifest.json")))["packages"]
for pkg in ("php", "frankenphp", "postgres", "mariadb", "valkey"):
    print(f"{pkg:11}", ", ".join(sorted(m.get(pkg, {}))) or "—")
EOF
```

Esperado: php y frankenphp con 8.3.36, 8.4.27 y 8.5.12; postgres 16.15; mariadb 11.8.9; valkey 9.1.3.

- [ ] **Paso 7: Commit y limpieza opcional de espacio**

```bash
cd "$HOME/Sites/Others/flock_intel_binaries"
git add packages/php/macos/build.sh
git commit -m "build(php): pick the static-php-cli binary and the compiler target from the machine's architecture"
du -sh work 2>/dev/null
```

Ofrece al usuario borrar `work/spc-*` (decenas de GB) cuando todo esté estable; el almacén (`~/.flock-intel`) no depende de ellos.

---

### Tarea 10: Verificación integral aislada (sin tocar el sistema)

**Archivos:** ninguno nuevo; reconstruye el CLI y la app con la clave real.

- [ ] **Paso 1: Suite completa de Go**

```bash
cd "$HOME/Sites/Others/flock_intel" && go vet ./... && test -z "$(gofmt -l . | grep -v '/vendor/')" && go test ./... -count=1 2>&1 | grep -vE '^ok|no test files'
```

Esperado: sin salida (todo `ok`). Si solo falla `TestTunnelRunsOnceConnected`, repítelo aislado (`go test ./internal/share -run TestTunnel -count=1`): debe pasar.

- [ ] **Paso 2: Reconstruir CLI y app con la clave embebida**

```bash
cd "$HOME/Sites/Others/flock_intel"
CGO_ENABLED=0 go build -o bin/flock ./cmd/flock
SKIP_DMG=1 make app 2>&1 | tail -3
lipo -archs build/macos/Flock.app/Contents/MacOS/FlockBar build/macos/Flock.app/Contents/MacOS/flock
```

Esperado: `x86_64` dos veces.

- [ ] **Paso 3: Daemon aislado con puertos propios, para probar PHP, Node y servicios por la ruta real**

```bash
export FLOCK_HOME="$(mktemp -d)"
cat > "$FLOCK_HOME/config.json" <<'EOF'
{"ports": {"http": 18080, "https": 18443, "dns": 53545, "smtp": 12525, "postgres": 55432, "dumps": 19912, "agent": 19913, "api": 53541}}
EOF
echo "$FLOCK_HOME"
```

`Load` mezcla ese JSON parcial con los valores por defecto. Arranca el daemon con `run_in_background` (`cd $HOME/Sites/Others/flock_intel && FLOCK_HOME=<el directorio> bin/flock daemon`), y desde otra llamada, con el mismo `FLOCK_HOME`:

```bash
cd "$HOME/Sites/Others/flock_intel"
bin/flock status
bin/flock php install 8.5 && bin/flock php list
bin/flock node install 22 && bin/flock node list
```

Esperado: PHP 8.5 y Node 22 instalados desde tu manifiesto. Si el daemon no arranca de forma aislada (por ejemplo porque necesita algo que solo da `flock install`), anota el motivo y deja estas comprobaciones para la Tarea 11, que corre sobre el sistema real. Detén el daemon al terminar (TaskStop o Ctrl+C) y borra el directorio temporal.

- [ ] **Paso 4: Servicios con el daemon aislado**

Con el daemon aún en marcha y el mismo `FLOCK_HOME`:

```bash
bin/flock services install postgres 16 --port 55433
bin/flock services install mariadb 11.8 --port 33307
bin/flock services list
```

Esperado: ambos instalados y en ejecución (puertos de prueba que no chocan con tu MariaDB de Homebrew). Al terminar, detén el daemon y borra el directorio de `FLOCK_HOME`, que contiene todo lo instalado en la prueba.

- [ ] **Paso 5: `swift test`**

```bash
cd "$HOME/Sites/Others/flock_intel/macos" && swift test 2>&1 | tail -4
```

---

### Tarea 11: Instalación real, documentación y cierre

**Archivos:**
- Crear: `$FLOCK/intel/README.md`
- Modificar: `$FLOCK/README.md` (fila *System* de la tabla de instalación)

- [ ] **Paso 1: 🛑 Elegir con el usuario el proyecto Laravel de prueba**

Pregúntale cuál de sus proyectos usar y con qué versión de PHP (8.3, 8.4 u 8.5) y base de datos (PostgreSQL o MariaDB). No continúes sin la respuesta.

- [ ] **Paso 2: 🛑 Preparar el sistema (con confirmación de cada punto)**

1. Parar el Apache de Homebrew, que ocupa el puerto 80: `! brew services stop httpd`.
2. Decidir con el usuario qué hacer con la MariaDB de Homebrew (3306): lo más simple es instalar la de Flock en otro puerto (`--port 3307`) hasta migrar los datos con `mysqldump`.
3. Avisarle de que Flock antepone sus versiones de `php`, `composer` y `node` en el `PATH` y que hoy el `php` y `composer` en uso salen de Herd Lite (`~/.config/herd-lite/bin`): después de `flock install`, el orden de las rutas decide cuál gana.

- [ ] **Paso 3: 🛑 Instalar y abrir la app**

```bash
cp -R "$HOME/Sites/Others/flock_intel/build/macos/Flock.app" /Applications/
open /Applications/Flock.app
```

La app, firmada ad-hoc y compilada en este Mac, no lleva cuarentena. En el asistente *Set up Flock* el usuario introduce su contraseña (`sudo`: `/etc/resolver/test`, CA local en el Llavero, servicio en segundo plano, puertos 80/443). No ejecutes `flock install` tú mismo.

- [ ] **Paso 4: Comprobaciones con el proyecto de prueba**

Haz con el usuario cada punto y anota el resultado:

1. El asistente instala PHP y Node por defecto desde `~/.flock-intel`.
2. `flock park` (o `link`) sobre la carpeta del proyecto y `flock secure <sitio>`; `https://<sitio>.test` abre sin aviso del navegador.
3. `flock pin php 8.3`, 8.4 y 8.5: el sitio responde con cada versión (`phpinfo()` o `php -v` en el proyecto).
4. `flock services install postgres 16` y `flock services create-db postgres <nombre>`; `php artisan migrate` contra ella. Lo mismo con MariaDB en su puerto.
5. Un correo enviado por Laravel llega a la ventana *Mail*.
6. `dump()` aparece en la ventana *Dumps*.
7. Xdebug: activarlo y comprobar que el IDE recibe la conexión en el 9003.
8. SQLite: una migración con `DB_CONNECTION=sqlite` funciona (`pdo_sqlite` viene compilado).
9. `flock share` (cloudflared) abre un túnel.

Cualquier fallo se trata con `superpowers:systematic-debugging` antes de seguir.

- [ ] **Paso 5: Documentar** — crear `intel/README.md`

Escribe `intel/README.md` con este contenido (en español, como el diseño):

````markdown
# Flock en un Mac Intel (x86_64)

Esta rama (`intel-mac-support`) hace que Flock funcione en Macs Intel. Upstream solo publica
paquetes para Apple Silicon, así que aquí los paquetes se compilan, o se descargan del proveedor,
en el propio Mac y se listan en un manifiesto firmado con una clave propia. Diseño:
[specs/2026-11-06-intel-mac-support-design.md](specs/2026-11-06-intel-mac-support-design.md).

## Cómo funciona

| Pieza | Dónde |
| --- | --- |
| Código de Flock | este repo, rama `intel-mac-support` |
| Scripts de compilación de paquetes | `~/Sites/Others/flock_intel_binaries`: clon local de `flock-php/binaries`, rama `macos-x64`. Ese repo no tiene licencia, por eso no está dentro de este (AGPL) |
| Paquetes y manifiesto firmado | `~/.flock-intel/packages` y `~/.flock-intel/manifest` |
| Clave privada de firma | `~/.config/flock-intel/manifest.key` (nunca en un repo; guarda una copia) |
| Clave pública | `internal/binaries/manifest.pub`, embebida en el CLI y en la app |

En un Mac Intel el daemon lee el manifiesto de `~/.flock-intel/manifest/` por `file://` y descarga
de ahí los paquetes `macos-x64`. La firma ed25519 del manifiesto y el SHA-256 de cada paquete se
comprueban igual que con el manifiesto oficial.

## Construir la app

```sh
make app    # build/macos/Flock.app, x86_64 en un Mac Intel; SKIP_DMG=1 evita crear el .dmg
```

`ARCH=arm64` o `ARCH=x86_64` fuerza la arquitectura; por defecto es la del Mac.

## Mantener los paquetes

Desde `~/Sites/Others/flock_intel_binaries`:

```sh
./build-local.sh mirror node 22.23.4     # descarga oficial, verificada contra el proveedor
./build-local.sh build postgres 16.15    # compila (postgres, mariadb, valkey)
./build-local.sh php 8.5.12              # PHP y FrankenPHP de esa versión
./build-local.sh manifest                # lista los paquetes y firma los manifiestos
```

- Para reemplazar un paquete que Flock ya instaló, sube el número de build
  (`BUILD=2 ./build-local.sh …`): Flock sustituye un paquete cuando el manifiesto tiene un build
  mayor de la misma versión.
- El manifiesto caduca a los 365 días y Flock no instala nada de uno caducado:
  `./build-local.sh manifest` lo renueva.
- Si se pierde la clave privada: `tools/localmanifest/localmanifest keygen`, sustituir
  `internal/binaries/manifest.pub`, recompilar la app y volver a firmar.

## Qué hay y qué no

Disponible: PHP 8.3, 8.4 y 8.5 con FrankenPHP; Node.js 16, 18, 20, 22, 24 y 26; PostgreSQL 16;
MariaDB 11.8; Valkey 9.1; MongoDB 7.0, 8.0 y 9.0; MySQL 8.0 y 9.7; Meilisearch; Typesense;
cloudflared (`flock share`).

No disponible en Intel (la app los muestra como no disponibles): RustFS (upstream no publica
macOS Intel), Redis, PHP 8.1 y 8.2, PostgreSQL 14, 15, 17 y 18 con sus extensiones (pgvector,
PostGIS, pgRouting, TimescaleDB) y las demás líneas de MariaDB. Se añaden con el mismo mecanismo:
compilar con `build-local.sh` y regenerar el manifiesto.

## Integrar cambios de upstream

```sh
git remote add upstream https://github.com/flock-php/flock.git   # una sola vez
git fetch upstream && git merge upstream/main
```

Los conflictos esperables están en `internal/binaries/manifest.pub`, `internal/binaries/binaries.go`
y `macos/build-app.sh`; los cambios de esta rama son pequeños para que sean fáciles de resolver.

## Notas de esta compilación

(Se rellena al terminar la tarea 11; ver la nota debajo.)
````

Después de crear el archivo, **sustituye** la sección «Notas de esta compilación» por los hechos que realmente se observaron en las tareas 3, 7, 8, 9 y 10: extensiones de PHP que hubo que quitar o parchear, si MySQL 8.4 existe para Intel, tests que no pasaron y por qué, y los tiempos de compilación medidos. Solo hechos observados; si no hubo nada que anotar, escribe «Sin incidencias».

En `README.md`, en la fila *System* de la tabla, cambia `macOS 15 Sequoia or newer, Apple silicon` por `macOS 15 Sequoia or newer, Apple silicon (Intel: build from source, see [intel/README.md](intel/README.md))`.

- [ ] **Paso 6: Verificación final y commit**

```bash
cd "$HOME/Sites/Others/flock_intel"
go run ./tools/docslint
git status --short
git add intel/README.md README.md
git commit -m "docs(intel): how to build, run and maintain Flock on an Intel Mac"
git log --oneline | head -10
```

Esperado: `docslint` sin errores, y la rama `intel-mac-support` con los commits de las tareas 1 a 5 y 11 (más el del diseño y el de este plan). Muestra al usuario `git log` y dile que no se ha empujado nada. Pregúntale si quiere hacer `git push origin intel-mac-support` a su fork; no lo hagas sin su confirmación.

---

## Autorrevisión

**Cobertura del diseño:** 4.2.1 → T1; 4.2.2 → T5; 4.2.3 → T2; 4.2.4 → T3; 4.2.5 → T11; 4.2.6 → T1, T2; 4.3 (PHP, OpenSSL, mirrors) → T6-T9; `build-local.sh` y `localmanifest` → T4, T6; fases 0-5 → T0, T1-T5, T6, T7, T8-T9, T10-T11; confirmaciones de la sección 6 → marcadas con 🛑 en T0, T8, T9, T11.

**Desviaciones respecto al diseño** (el diseño se actualiza en el mismo commit que este plan): `versions.json` no se modifica y `verify-macos.sh` tampoco (ya funciona en Intel); los directorios de compilación están en `$BIN/work/`.

**Coherencia de nombres:** `LocalBases`, `RegisterFileProtocol`, `LocalDirName` (T2) coinciden con los usados en sus tests; `Build`, `Write`, `Verify`, `GenerateKey`, `LoadKey`, `LoadPublicKey`, `Platform`, `channelFiles` (T4) coinciden entre `manifest.go`, `keys.go`, `main.go` y los tests; `build-local.sh` usa los subcomandos `mirror | build | php | manifest` que documentan T6-T9.
