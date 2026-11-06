# Soporte de Flock para Mac Intel (x86_64): diseño

Fecha: 2026-11-06 · Rama: `intel-mac-support` · Estado: pendiente de revisión del usuario

## 1. Objetivo

Que Flock funcione al 100% en un MacBook Pro Intel (i9-9980HK, macOS 26): la app de barra de menú,
el CLI/daemon y la descarga e instalación de PHP, Node.js y servicios, igual que en Apple Silicon.

## 2. Contexto y hallazgos

Verificado el 2026-11-06 sobre el código y sobre el manifiesto publicado:

- El CLI/daemon (Go) y la app (SwiftUI) **compilan sin errores para x86_64** sin tocar código
  (`GOARCH=amd64`; `swift build --arch x86_64`, Sparkle es universal). Los tests de
  `binaries`, `php`, `node`, `catalog` y `paths` pasan en Intel.
- Lo que impide usar Flock en Intel:
  1. `macos/build-app.sh` fija `--arch arm64` y `GOARCH=arm64`.
  2. `internal/binaries/binaries.go` (`platform()`) devuelve `""` para `darwin/amd64`.
  3. El manifiesto firmado de upstream solo tiene entradas `macos-arm64`, `windows-x64` y
     `linux-*`; no existe ningún binario de PHP, bases de datos o Node para macOS Intel.
  4. El manifiesto está firmado con ed25519 y la clave pública está embebida
     (`internal/binaries/manifest.pub`), de modo que no se pueden añadir entradas a mano.
- `flock-php/binaries` (origen de los paquetes) **no declara licencia**; Flock es AGPL-3.0.
- Los scripts de compilación de PostgreSQL, MariaDB y Valkey para macOS no contienen nada
  específico de arm64. Solo PHP (`spc-macos-aarch64`, `--target=arm64-apple-darwin`) y OpenSSL
  (`darwin64-arm64-cc`) lo tienen.
- Proveedores con build oficial para macOS Intel (comprobado con HEAD): Node (6 líneas), MongoDB
  7/8/9, Meilisearch, Typesense 29/30, cloudflared, MySQL 8.0 y 9.7 (`macos15-x86_64`).
  Sin build Intel: **RustFS**. Sin confirmar: **MySQL 8.4** (buscar en los archivos de Oracle).
- Entorno del usuario: Homebrew en `/usr/local` con `httpd` (puerto 80) y MariaDB 13.0.2 (puerto
  3306) en marcha; nvm con Node 16 a 26; PHP 8.1 a 8.5 de Homebrew; PostgreSQL 16.15 (servicio en
  error). 32 GB de RAM, 16 hilos, 157 GB libres.

## 3. Alcance

**Incluido ("núcleo + descargas oficiales"):**

| Tipo | Paquetes |
|---|---|
| Se compilan | PHP 8.3, 8.4 y 8.5; FrankenPHP (mismas versiones); PostgreSQL 16; MariaDB 11.8; Valkey 9.1 |
| Se espejan | Node 16, 18, 20, 22, 24, 26; MongoDB 7.0, 8.0, 9.0; MySQL 8.0, 9.7 (y 8.4 si existe); Meilisearch 1; Typesense 29, 30; cloudflared |

**Fuera de alcance** (la app los mostrará como "no disponibles"; se pueden añadir después con el
mismo mecanismo): RustFS (sin build upstream para Intel), Redis, PHP 8.1/8.2, PostgreSQL 14/15/17/18
y sus extensiones (pgvector, PostGIS, pgRouting, TimescaleDB), el resto de líneas de MariaDB,
canales beta/experimental propios, actualizaciones automáticas (Sparkle) y firma con Developer ID.

## 4. Arquitectura

### 4.1 Repos y carpetas

| Pieza | Ubicación | Contenido |
|---|---|---|
| Código de Flock | `~/Sites/Others/flock_intel`, rama `intel-mac-support` | Cambios pequeños en Go, `build-app.sh`, tests, esta documentación (AGPL) |
| Pipeline de binarios | `~/Sites/Others/flock_intel_binaries` (repo local nuevo, rama `macos-x64`, clon de `flock-php/binaries`) | Scripts adaptados, `build-local.sh`, generador y firmador de manifiesto |
| Artefactos | `~/.flock-intel/` | Paquetes `.tar.gz` y manifiesto firmado, servidos por `file://` |
| Clave privada | `~/.config/flock-intel/manifest.key` (permisos 600) | Semilla ed25519, nunca en un repo |

El pipeline va en un repo aparte porque `flock-php/binaries` no tiene licencia: mezclarlo con
Flock (AGPL) contaminaría el fork. Ninguno de los dos repos se publica ni se empuja sin que el
usuario lo pida.

### 4.2 Cambios en `flock_intel`

1. `internal/binaries/binaries.go`: `platform()` mapea `darwin/amd64` a `"macos-x64"`; se
   actualizan el comentario de `Platform()` y sus tests (`binaries_test.go`).
2. `internal/binaries/manifest.pub`: la clave pública del usuario.
3. **Fuente local `file://`** en darwin/amd64: un archivo con build tag que registra el protocolo
   `file` en `http.DefaultTransport` (`http.NewFileTransport`) y fija `Bases` a
   `file://<HOME>/.flock-intel/manifest/`. Verificado en el código: todos los clientes que
   descargan manifiestos o paquetes (`binaries`, `download`, `php`, `frankenphp`, `postgres`,
   `node`, `share`, `catalog`) usan `http.DefaultClient` o `&http.Client{}` con transporte nulo,
   es decir `DefaultTransport`; el único transporte propio (`internal/api/api.go`) es el canal
   con el daemon. Por eso `Source.get` y `download.File` funcionan sin más cambios. La
   verificación (firma ed25519, canal, caducidad, serial, SHA-256) no se toca.
4. `macos/build-app.sh`: arquitectura desde `ARCH` (por defecto `uname -m`); `x86_64` implica
   `--arch x86_64` y `GOARCH=amd64`, `arm64` conserva el comportamiento actual. Se retiran los
   mensajes que dicen que Intel no está soportado.
5. `Makefile` (`build-all`) y `README.md`: plataforma Intel y cómo compilarla.
6. Tests: mapeo de plataforma, lectura de manifiesto y descarga por `file://` con firma válida e
   inválida, y que `Platform()` ya no devuelve `""` en darwin/amd64.

### 4.3 Cambios en `flock_intel_binaries`

- `packages/php/macos/build.sh`: descargar `spc-macos-x86_64`, `--target=x86_64-apple-darwin`.
- `scripts/macos/openssl.sh`: `darwin64-x86_64-cc`.
- `packages/{node,mysql,mongodb,meilisearch,typesense,cloudflared}/mirror.sh`: caso `macos-x64`
  con el asset de cada proveedor (`darwin-x64`, `macos15-x86_64`, `macos-x86_64`, `macos-amd64`,
  `darwin-amd64`); se mantienen las verificaciones de firma o checksum de cada script.
- `versions.json` y `scripts/verify-macos.sh` **no se modifican** (descubierto al escribir el plan):
  `build-local.sh` solo lee de `versions.json` los ajustes que no dependen de la plataforma
  (`env` de cada paquete y `.php.macos`), y `verify-macos.sh` ya usa `otool -L` y rechaza
  cualquier enlace a `/usr/local`, `/opt/homebrew` o el árbol de compilación, que es justo la
  comprobación que hace falta en Intel, donde Homebrew vive en `/usr/local` y no se puede ocultar
  como `/opt/homebrew` en CI. La prueba "sin Homebrew" de CI se sustituye por ejecutar los smoke
  tests con un `PATH` de sistema (`/usr/bin:/bin:/usr/sbin:/sbin`) y un entorno vacío (`env -i`).
- `build-local.sh`: reemplaza a GitHub Actions. Ejecuta, por paquete, build o mirror, verificación,
  smoke test, empaquetado (`<pkg>-<ver>-build<N>-macos-x64.tar.gz`) y copia a `~/.flock-intel/`.
  Los directorios de compilación van en `flock_intel_binaries/work/` (ignorado por git).
- `tools/localmanifest/` (Go): escanea los paquetes, calcula SHA-256 y tamaño, escribe el
  manifiesto (esquema 1) en los tres canales con las mismas entradas, caducidad de 365 días y
  `serial` = hora Unix, y lo firma. Incluye `keygen` y `verify` (reutilizando la verificación de
  Flock). El `flockrel manifest` de upstream no sirve: lee los releases de GitHub.

### 4.4 Flujo de confianza

```
build-local.sh → paquete .tar.gz → tools/localmanifest → manifest.json + .sig (clave privada)
Flock (daemon) → file://~/.flock-intel/manifest/manifest.json(.sig)
   → verifica firma con manifest.pub embebida, canal, caducidad y serial
   → descarga file://…/<paquete>.tar.gz → verifica SHA-256 del manifiesto → extrae
```

Un cambio de clave obliga a recompilar la app (la pública va embebida). Una carpeta
`~/.flock-intel/` manipulada no puede inyectar contenido: sin la clave privada no hay firma válida.

## 5. Fases y verificación

| # | Fase | Verificación |
|---|---|---|
| 0 | Rama, clon de binarios, par de claves | Repos listos, clave con permisos 600 |
| 1 | Cambios en Flock (TDD) | `go test ./...` y `swift test` en x86_64; CLI y app Intel compilan |
| 2 | Generador y firmador de manifiesto | `flock node install 22` extremo a extremo desde el manifiesto local, con `FLOCK_HOME` aislado |
| 3 | Mirrors oficiales (~1,6 GB) | Checksum o firma del proveedor |
| 4 | Compilación: Valkey, PostgreSQL 16, MariaDB 11.8, PHP 8.3/8.4/8.5 + FrankenPHP | Smoke tests de upstream y `otool` limpio |
| 5 | Integración en el equipo del usuario | Criterio de éxito (sección 7) |

Hasta la fase 4 todo se prueba con un `FLOCK_HOME` aislado: no se modifica el sistema del usuario.
Tras medir Valkey y PostgreSQL se da una estimación real de tiempo antes de lanzar PHP.

## 6. Cambios sobre el sistema del usuario (requieren confirmación explícita)

- **Fase 4:** `static-php-cli` instala dependencias con Homebrew (p. ej. `re2c`). Se muestra la lista
  exacta antes de ejecutarla. Espacio temporal: decenas de GB en `flock_intel_binaries`.
- **Fase 5 (`flock install`):** `sudo`, `/etc/resolver/test`, CA local en el Llavero, servicio en
  segundo plano y puertos 80 y 443. Antes hay que parar el `httpd` de Homebrew.
- **Puertos de bases de datos:** la MariaDB de Homebrew (3306) y la de Flock no pueden coincidir;
  se decide en la fase 5 (sugerencia: mantener la de Homebrew hasta migrar con `mysqldump`).
- Homebrew, nvm y las bases de datos existentes no se modifican. Ningún `git push`.

## 7. Criterio de éxito ("al 100%")

1. `go test ./...` y `swift test` pasan en el Intel del usuario.
2. La app Intel abre y su asistente instala PHP y Node desde el manifiesto local.
3. Un proyecto Laravel del usuario se sirve en `https://<carpeta>.test` con PHP 8.3, 8.4 u 8.5
   según el proyecto; conecta a PostgreSQL y MariaDB de Flock; el correo llega al buzón; `dump()`
   aparece en la ventana de Dumps; Xdebug funciona al activarlo. *(Proyecto de prueba: lo elige el
   usuario antes de la fase 5.)*
4. Los paquetes compilados pasan los smoke tests de upstream y la comprobación `otool`.

## 8. Riesgos

- Alguna de las 50 extensiones estáticas de PHP (gd, ldap, intl, xsl, gmp…) o de las 6 compartidas
  (xdebug, imagick, pcov, mongodb, memcached, opentelemetry) puede fallar en x86_64. Respuesta:
  aislar cuál, y quitarla o parchearla solo en la build Intel, avisando al usuario. `pdo_sqlite` y
  `sqlite3` están entre las estáticas, así que SQLite queda cubierto.
- FrankenPHP se compila con Go/xcaddy (los instala `spc`); puede requerir ajustes en Intel.
- MySQL 8.4 puede no existir para Intel: quedaría sin esa línea.
- El manifiesto caduca en 365 días; `tools/localmanifest` lo renueva.
- Integrar cambios futuros de upstream puede dar conflictos en `manifest.pub`, `binaries.go` y
  `build-app.sh`; por eso los cambios se mantienen pequeños y localizados.
- Las versiones del manifiesto de upstream avanzan a diario; los paquetes locales quedan fijados
  en las versiones que se compilen, y se actualizan relanzando `build-local.sh`.

## 9. Decisiones registradas

- Compilación local en el Mac del usuario, no en GitHub Actions (elegido por el usuario).
- Alcance "núcleo + descargas oficiales" (elegido por el usuario).
- MariaDB 11.8: la 13.0.2 de Homebrew no existe en el catálogo de Flock (10.11, 11.4, 11.8, 12.3).
  Se elige 11.8 por ser una línea intermedia del catálogo; se cambia en el `versions.json` local si se
  prefiere otra. Los datos de Flock son independientes de los de Homebrew, así que no hay
  incompatibilidad.
- Commits locales en `intel-mac-support`, sin línea de co-autoría.
