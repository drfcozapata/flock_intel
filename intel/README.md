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
- `ONLY=frankenphp ./build-local.sh php 8.4.27` (o `ONLY=php`) compila solo uno de los dos
  paquetes de PHP; sirve para repetir FrankenPHP si falla la red (ver las notas).

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

Hechos observados en Mac Intel (i9-9980HK, macOS 26) el 2026-11-06.

**Homebrew puede romper la compilación de PHP.** El primer intento de PHP 8.5.12 falló al
enlazar con `Undefined symbols … _ASN1_STRING_get_length` en `libcurl.a`. Sin un *sysroot*,
clang busca en `/usr/local/include`, que en un Mac Intel es donde Homebrew enlaza sus cabeceras
(aquí OpenSSL 4.1.0); libcurl se compiló contra ellas y no enlazaba con el OpenSSL 3.6.2 que
construye static-php-cli. `packages/php/macos/build.sh` ahora exporta `SDKROOT` (con él, la
lista de búsqueda de clang ya no incluye `/usr/local/include`); los scripts de PostgreSQL y
MariaDB ya lo hacían. No hizo falta desvincular ni desinstalar nada de Homebrew, y
`spc doctor` no pidió instalar paquetes.

**Extensiones de PHP.** No se quitó ni se parcheó ninguna extensión estática. Las seis
compartidas (`xdebug`, `imagick`, `pcov`, `mongodb`, `memcached`, `opentelemetry`) se
compilaron en PHP 8.3, 8.4 y 8.5 y cargan en las pruebas de humo.

**MySQL 8.4 no existe para Intel.** `mysql-8.4.13-macos{13,14,15}-x86_64.tar.gz` no están en
el CDN de MySQL ni en sus archivos; solo se espejaron 8.0.46 y 9.7.4. El resto de espejos (Node, MongoDB,
Meilisearch, Typesense, cloudflared) se descargó y verificó sin problemas.

**FrankenPHP depende de la red.** Su paso `xcaddy build` descarga módulos de Go y falló en 8.4 y
en 8.3 por cortes de red (`connection reset by peer` al consultar `sum.golang.org`;
`stream error … INTERNAL_ERROR` en `proxy.golang.org`), después de terminada la compilación de
PHP. Repetirlo con `ONLY=frankenphp` funcionó a la primera.

**Tiempos medidos** (Valkey, PostgreSQL y MariaDB compilan solos; PHP incluye FrankenPHP):

| Paquete | Tiempo |
| --- | --- |
| Valkey 9.1.3 | 52 s |
| PostgreSQL 16.15 | 354 s |
| MariaDB 11.8.9 | 1567 s |
| PHP 8.5.12 + FrankenPHP | 3498 s (tras corregir `SDKROOT`; el intento fallido tardó 1215 s) |
| PHP 8.4.27 + FrankenPHP | 4607 s, incluido el intento fallido de FrankenPHP; el reintento de FrankenPHP, 1035 s |
| PHP 8.3.36 + FrankenPHP | 4044 s, incluido el intento fallido de FrankenPHP; el reintento de FrankenPHP, 979 s |

Los directorios de compilación (`work/spc-*`) ocupan unos 25 GB y se pueden borrar: el almacén
`~/.flock-intel` no depende de ellos.

**Verificaciones.** `go vet`, `gofmt` y `go test ./...` pasan completos; `swift test` pasa
(73 pruebas, con Swift Testing). La app y el CLI compilados son `x86_64` y llevan embebida la
clave pública propia, no la de upstream. `verify-macos.sh` confirma que los Mach-O de Valkey (3),
PostgreSQL (119) y MariaDB (77) solo enlazan con bibliotecas del sistema.

**Prueba aislada** (daemon con `FLOCK_HOME` temporal y puertos propios, sin tocar el sistema): el
daemon instala por su cuenta PHP 8.5 y Node 26 como predeterminados desde `~/.flock-intel`;
`flock php install 8.4` y `8.3`, `flock node install 22` y la instalación de PostgreSQL 16, MariaDB
11.8, MySQL 8.0, Valkey 9.1, MongoDB 8.0, Meilisearch 1 y Typesense 30 funcionan. PostgreSQL y MariaDB
respondieron consultas desde `pdo_pgsql` y `pdo_mysql` del PHP de Flock; Valkey respondió `PING`;
Meilisearch y Typesense, `/health`. Dos detalles de uso: `flock services install postgres` no admite
`--port` (el puerto de PostgreSQL sale de `ports.postgres` en la configuración, por defecto 5432) y
`flock services` sin argumentos es el listado (`flock services list` pide el nombre de un servicio).

**Instalación real** (app copiada a `/Applications`, asistente *Set up Flock* con contraseña, y un
proyecto Laravel 13 enlazado como `https://example-app.test`):

- El asistente deja el daemon en marcha, `/etc/resolver/test` apuntando al DNS de Flock, la CA local
  de confianza y los atajos al principio del `PATH` (una línea al final de `~/.zshrc`, así que gana
  a Herd Lite y a nvm en shells nuevos). Instala PHP 8.5 y Node 26 como predeterminados desde
  `~/.flock-intel`. El `PHP_INI_SCAN_DIR` que Herd Lite exporta no interfiere: Flock fija el suyo.
- Había que liberar el puerto 80 (`brew services stop httpd`). PostgreSQL de Homebrew ocupaba el
  5432 y el puerto de PostgreSQL de Flock no se desplaza solo, así que se fijó en
  `~/Library/Application Support/Flock/config.json` (`{"ports": {"postgres": 5433}}`) antes de
  instalarlo; MariaDB de Flock va en el 3307 (`--port 3307`). Conviven con las de Homebrew.
- Comprobado en esa instalación: DNS de `*.test`, certificado aceptado por el Llavero
  (`security verify-cert`), `flock pin php` con 8.3, 8.4 y 8.5 (cada una responde con su versión),
  `php artisan migrate` contra PostgreSQL y MariaDB de Flock y contra SQLite, correo de Laravel en
  el SMTP de Flock (`flock mail`), `dump()` y eventos (consultas, vistas, peticiones) capturados por
  el agente, Xdebug en una petición web con la cookie `XDEBUG_SESSION` (conecta al 9003) y
  `flock share` (la URL pública de Cloudflare sirvió la app con 200; cloudflared 2026.10.7 es
  `x86_64` y conserva la firma de Cloudflare). La ventana *Mail*, la de *Dumps* y el candado HTTPS
  del navegador se revisaron a ojo.
- El proyecto de prueba exige PHP >= 8.4.1 en `vendor/` aunque su `composer.json` diga `^8.3`: con
  PHP 8.3 responde 500 porque `platform_check.php` de Composer se niega, no por Flock.
- `flock share` ejecuta `npm run build` en el proyecto y copia la URL al portapapeles.
- `flock debug` no cargaba Xdebug (fallo de Flock, no de Intel): su entorno llevaba dos
  `PHP_INI_SCAN_DIR` y `getenv()` devuelve el primero. Corregido en `internal/cli/services.go`, con
  prueba en `internal/cli/debug_test.go`. La app ya instalada conserva el CLI anterior hasta que
  se recompile con `make app` y se vuelva a copiar a `/Applications`.
