# baky

Herramienta de terminal para respaldar la infraestructura del Grupo de Investigación GRID desde un solo lugar:

- **Respaldo de archivos**: comprime archivos y carpetas en `tar.gz` hacia un destino de respaldo (por ejemplo un NAS), de forma manual, programada o al detectar cambios, y permite restaurarlos.
- **Respaldo de configuraciones**: vigila carpetas de configuración (Firewall, VPN, DNS...) y registra cada cambio como un commit en un repositorio Git, con historial y `diff` línea por línea.

Está escrita en Go con [Bubble Tea](https://github.com/charmbracelet/bubbletea). Es un único programa con dos modos: una interfaz de terminal (TUI) para el administrador y un servicio en segundo plano que ejecuta los respaldos automáticos.

Proyecto desarrollado en el marco del trabajo de grado **MAGITI**: *Modelo de Automatización para la Gestión de la Infraestructura de TI del Grupo de Investigación GRID*.

## Origen

baky unifica tres herramientas que antes se instalaban y mantenían por separado:

| Herramienta original | Qué hacía | Dónde quedó en baky |
|---|---|---|
| `baky` | Respaldo de archivos en `tar.gz` y restauración | `internal/backup`, `internal/restore` |
| [`magiti-config-backup`](https://github.com/MiguelFVasquez/magiti-config-backup) | Daemon en Bash que versionaba configuraciones en Git | `internal/gitbackup`, reescrito en Go |
| [`magiti-backup-tui`](https://github.com/MiguelFVasquez/magiti-backup-tui) | TUI para controlar ese daemon | Menús *Respaldo de configuraciones* y *Servicio* |

El administrador ahora instala un solo binario, un solo servicio y edita un solo archivo de configuración.

## Uso

```
baky                               Abre la interfaz de terminal
baky daemon                        Ejecuta los respaldos automáticos (lo usa systemd)
baky import-magiti <config.json>   Importa las carpetas vigiladas del daemon magiti-config-backup
baky help                          Muestra la ayuda
```

### Menús de la interfaz

```
BAKY
  Respaldo de archivos         Gestionar rutas · Respaldar ahora · Ver respaldos / Restaurar · Destino de respaldo
  Respaldo de configuraciones  Carpetas vigiladas · Historial de cambios · Repositorio
  Servicio                     Estado del servicio · Logs en vivo
  Ayuda
  Salir
```

Teclas comunes: `↑/↓` o `k/j` para navegar, `enter` para seleccionar, `esc` para volver, `?` para la ayuda y `q` para salir.

Los respaldos automáticos los ejecuta el servicio, no la interfaz: se puede cerrar la TUI sin interrumpirlos. Los cambios hechos desde la TUI (agregar una ruta, vigilar una carpeta nueva) los aplica el servicio por sí solo, sin reiniciarlo.

## Instalación rápida

```bash
git clone https://github.com/Josedzzz/baky.git
cd baky
go build -o baky ./cmd
sudo install -m 0755 baky /usr/local/bin/baky
```

Requisitos: Go 1.24 o superior para compilar, `git` para el respaldo de configuraciones y `systemd` para el servicio.

La instalación completa (servicio, permisos y repositorio de configuraciones) está en [docs/instalacion.md](docs/instalacion.md).

## Cómo funciona

### Respaldo de archivos

Cada ruta configurada tiene una frecuencia:

| Frecuencia | Cuándo se respalda |
|---|---|
| Diario | Todos los días, durante la hora de las 3 AM |
| Semanal | Los domingos, durante la hora de las 3 AM |
| Al cambiar | Unos segundos después de que cambia un archivo de la ruta |

Los respaldos se guardan en el destino como `[nombre]_AAAAMMDD_HHMMSS.tar.gz`. Al restaurar se puede elegir la ubicación original u otra ruta, y qué hacer si ya hay archivos:

- **Sobrescribir**: reemplaza los archivos que están en el respaldo y conserva los demás.
- **Renombrar**: guarda lo existente con la fecha en el nombre y restaura el respaldo.
- **Omitir**: no restaura si ya hay archivos.

Antes de restaurar, baky muestra qué cambiaría en el destino elegido:

```
CAMBIOS DE LA RESTAURACIÓN
M se modifican: 1   + se crean: 1   = quedan igual: 1

M informe.txt  (se modifica)
+ borrado.txt  (se crea)
= igual.txt  (queda igual)
```

Con `enter` sobre un archivo se ven sus diferencias línea por línea contra el archivo actual (en rojo lo que se quita, en verde lo que vuelve), y con `r` se continúa con la restauración. Esta vista previa no modifica nada. La comparación línea por línea usa `git`, y solo aplica a archivos de texto.

El respaldo siempre se extrae primero a una carpeta temporal: un respaldo dañado nunca modifica lo que ya existe en el destino.

### Respaldo de configuraciones

Cada carpeta vigilada se asocia a una carpeta dentro del repositorio de configuraciones (por ejemplo `/etc/openvpn` → `vpn`). Cuando un archivo cambia, el servicio lo copia al repositorio, crea un commit por archivo y lo envía al remoto:

```
auto-backup: actualizado nat.conf (2026-07-24 21:46:14)
auto-backup: eliminado config-test.conf (2026-07-24 21:39:09)
```

Además:

- Al iniciar, sincroniza todo: los cambios hechos mientras el servicio estaba detenido no se pierden.
- Ignora los archivos temporales de los editores (`*.swp`, `*~`, `4913`).
- Si el envío al remoto falla, los commits quedan en el repositorio local y se reintenta.
- Si una carpeta vigilada deja de estar accesible, conserva lo ya respaldado.

#### Validar cada cambio

Cada carpeta vigilada puede tener dos comandos opcionales, que se definen con `c` en *Carpetas vigiladas*. `{dir}` se reemplaza por la carpeta que contiene la configuración:

| Comando | Ejemplo con Shorewall | Cuándo se ejecuta |
|---|---|---|
| Validación | `sudo -n shorewall check {dir}` | Después de cada cambio detectado, y antes de restaurar |
| Aplicación | `sudo -n shorewall try {dir}` | Solo al restaurar un archivo desde el historial |

Cuando un cambio no pasa la validación, baky lo respalda igual (el respaldo refleja lo que hay en el disco), pero lo marca: el commit termina en `[validación fallida]`, guarda el error del validador, aparece en rojo en el historial y queda una advertencia en el log. Así un error de sintaxis se conoce a los pocos segundos de guardarlo, antes de que alguien reinicie el servicio.

baky nunca aplica por su cuenta un cambio que detecta: solo lo valida.

Los comandos se ejecutan tal como están escritos, sin intérprete de comandos. Si necesitan privilegios deben empezar con `sudo -n`, y el usuario del servicio debe tener ese permiso sin contraseña (ver [docs/instalacion.md](docs/instalacion.md)).

#### Restaurar una configuración anterior

En *Respaldo de configuraciones → Historial de cambios* cada línea es un cambio respaldado. Al elegir uno con `enter`, baky compara el archivo actual con la versión guardada en ese cambio y muestra qué pasaría al restaurarla:

```
── Cambios desde la línea 1 del archivo actual ──
  -A INPUT -p tcp --dport 22 -j ACCEPT
- -A INPUT -p tcp --dport 8080 -j ACCEPT      línea actual que se quita
+ -A INPUT -p tcp --dport 80 -j ACCEPT        línea que vuelve
  -A INPUT -p tcp --dport 443 -j ACCEPT
```

Con `r` se restaura esa versión, previa confirmación. Si la carpeta tiene comandos de validación y aplicación, la restauración no toca la configuración real hasta saber que es buena:

1. Copia la carpeta a un directorio temporal y pone ahí la versión a restaurar.
2. Ejecuta la validación sobre esa copia. Si falla, se detiene.
3. Ejecuta la aplicación sobre esa copia. Si falla, se detiene.
4. Solo entonces escribe el archivo en la carpeta vigilada, conservando su dueño y sus permisos. Si el usuario no puede escribir ahí, lo instala con `sudo`.

El servicio detecta la restauración y la registra como un cambio más, de modo que también se puede deshacer. Si el cambio elegido eliminó el archivo, no contiene una versión que restaurar: se elige uno anterior.

El repositorio de configuraciones es un repositorio dedicado, separado del código. En el GRID es [`grid-config-data`](https://github.com/MiguelFVasquez/grid-config-data).

## Archivos

Todos viven en `~/.config/baky/` del usuario que ejecuta baky. La variable `BAKY_CONFIG` permite usar otra ubicación para `config.json`; los demás archivos lo acompañan.

| Archivo | Contenido |
|---|---|
| `config.json` | Ajustes: destino, rutas, repositorio y carpetas vigiladas |
| `state.json` | Historial de respaldos y restauraciones, último respaldo de cada ruta |
| `baky.log` | Registro de actividad del servicio |

Ejemplo de `config.json`:

```json
{
  "nas_path": "/mnt/nas/respaldos",
  "backup_paths": [
    { "path": "/home/grid/documentos", "frequency": "daily" },
    { "path": "/etc/nginx/nginx.conf", "frequency": "on_change" }
  ],
  "config_backup": {
    "repo_dir": "/home/grid/grid-config-data",
    "branch": "main",
    "watches": [
      { "source": "/etc/openvpn", "dest": "vpn" },
      {
        "source": "/etc/shorewall",
        "dest": "firewall",
        "check": "sudo -n shorewall check {dir}",
        "apply": "sudo -n shorewall try {dir}"
      }
    ]
  }
}
```

No es necesario editarlo a mano: todo se configura desde la interfaz.

## Estructura del proyecto

```
baky/
├── cmd/main.go            Punto de entrada: TUI, daemon e importación
├── internal/
│   ├── backup/            Respaldo de archivos (tar.gz) y su programación
│   ├── restore/           Listado, comparación y restauración de respaldos
│   ├── gitbackup/         Respaldo versionado de configuraciones en Git
│   ├── daemon/            Servicio: ejecuta ambos motores y recarga la configuración
│   ├── service/           Control del servicio systemd
│   ├── config/            Configuración y estado compartidos entre TUI y servicio
│   ├── logging/           Escritura y lectura del log del servicio
│   ├── watch/             Vigilancia recursiva de carpetas (fsnotify)
│   ├── textdiff/          Comparación línea por línea de dos versiones de un archivo
│   └── tui/               Interfaz de terminal, un archivo por pantalla
├── deploy/                Unidad de systemd y regla de sudoers
└── docs/                  Instalación y registros de pruebas
```

El código (identificadores y comentarios) está en inglés; la interfaz y la documentación, en español.

## Pruebas

### Pruebas automáticas

```bash
go test ./...          # todas las pruebas
go test -race ./...    # además detecta condiciones de carrera
go vet ./...           # análisis estático
```

No usan la red ni tocan la configuración real: cada prueba trabaja en carpetas temporales, y el respaldo de configuraciones se prueba contra un repositorio Git local que hace de remoto.

| Paquete | Qué cubren sus pruebas |
|---|---|
| `internal/backup` | Creación del `tar.gz` (carpetas, archivo único, enlaces simbólicos) y cuándo toca un respaldo programado |
| `internal/restore` | Restauración con sobrescribir, renombrar y omitir; respaldo dañado; archivo único; vista previa de los cambios |
| `internal/gitbackup` | Crear, modificar, eliminar y renombrar archivos; subcarpetas; carpetas movidas; archivos temporales; fallo y reintento de push; validación de cada cambio; comparar y restaurar una versión anterior con validación previa |
| `internal/daemon` | El servicio completo con ambos motores y un cambio de configuración en caliente |
| `internal/config` | Guardado, historial en `state.json`, migración de configuraciones antiguas, archivo dañado, carpetas vigiladas |
| `internal/tui` | Navegación por los menús y los flujos principales, enviando teclas al modelo |
| `internal/logging`, `internal/watch`, `internal/textdiff` | Formato y lectura del log; vigilancia recursiva; comparación línea por línea |

### Probar a mano sin instalar nada

Se puede probar todo en una carpeta temporal, sin systemd, sin `sudo` y sin tocar `~/.config/baky`. La variable `BAKY_CONFIG` hace que baky use una configuración aparte.

**1. Preparar el entorno de prueba**

```bash
go build -o baky ./cmd

export PRUEBA=/tmp/baky-prueba
export BAKY_CONFIG=$PRUEBA/cfg/config.json
mkdir -p $PRUEBA/cfg $PRUEBA/etc/firewall $PRUEBA/docs $PRUEBA/nas

# Un repositorio "remoto" local y su clon, que hace de repositorio de configuraciones
git init --bare --initial-branch=main $PRUEBA/remoto.git
git init --initial-branch=main $PRUEBA/repo
git -C $PRUEBA/repo remote add origin $PRUEBA/remoto.git

echo "regla 1" > $PRUEBA/etc/firewall/reglas.conf
echo "hola" > $PRUEBA/docs/informe.txt
```

**2. Iniciar el servicio en una terminal**

```bash
./baky daemon
```

Queda en primer plano mostrando el log. Se detiene con `Ctrl+C`.

**3. Abrir la interfaz en otra terminal**

```bash
export PRUEBA=/tmp/baky-prueba
export BAKY_CONFIG=$PRUEBA/cfg/config.json
./baky
```

El menú principal mostrará el servicio como inactivo: es lo esperado, porque en esta prueba el daemon no corre bajo systemd. Por lo mismo, las teclas de iniciar y detener de *Servicio → Estado del servicio* no aplican aquí.

**4. Probar el respaldo de configuraciones**

1. *Respaldo de configuraciones → Repositorio*: `e`, escribir `/tmp/baky-prueba/repo`, `enter`.
2. *Respaldo de configuraciones → Carpetas vigiladas*: `a`, escribir `/tmp/baky-prueba/etc/firewall`, `enter`, aceptar el nombre `firewall` con `enter`.
3. En una tercera terminal, modificar, crear y borrar archivos:

   ```bash
   echo "regla 2" >> /tmp/baky-prueba/etc/firewall/reglas.conf
   mkdir /tmp/baky-prueba/etc/firewall/extra
   echo "nat" > /tmp/baky-prueba/etc/firewall/extra/nat.conf
   rm /tmp/baky-prueba/etc/firewall/reglas.conf
   ```

4. Resultado esperado, a los pocos segundos de cada cambio:
   - En *Historial de cambios* (`r` para refrescar): un commit por archivo, `auto-backup: actualizado ...` o `auto-backup: eliminado ...`.
   - En *Historial de cambios*, `enter` sobre un cambio anterior muestra las diferencias con el archivo actual, y `r` lo restaura.
   - En *Servicio → Logs en vivo*: `Commit realizado` y `Push exitoso`.
   - En el remoto: `git -C /tmp/baky-prueba/remoto.git log --oneline main` muestra los mismos commits.

Para probar la sincronización al iniciar: detener el daemon con `Ctrl+C`, cambiar un archivo de la carpeta vigilada y volver a iniciarlo. El cambio debe quedar respaldado al arrancar.

**5. Probar el respaldo de archivos**

1. *Respaldo de archivos → Destino de respaldo*: `e`, escribir `/tmp/baky-prueba/nas`, `enter`; `t` debe decir que es accesible.
2. *Respaldo de archivos → Gestionar rutas*: `a`, escribir `/tmp/baky-prueba/docs`, `enter`.
3. *Respaldo de archivos → Respaldar ahora*: `enter`. Debe aparecer un `docs_AAAAMMDD_HHMMSS.tar.gz` en `/tmp/baky-prueba/nas`.
4. *Ver respaldos / Restaurar*: `enter` sobre el respaldo, `c`, escribir `/tmp/baky-prueba/restaurado`, `enter`. Aparece la lista de cambios de la restauración (aquí, `informe.txt` se crea); `r` para continuar y `enter` para confirmar. El archivo `informe.txt` debe aparecer en esa carpeta.
5. Vista previa con diferencias: modificar el archivo original (`echo "otra linea" >> /tmp/baky-prueba/docs/informe.txt`), volver a *Ver respaldos / Restaurar*, `enter` sobre el respaldo y `o` (ubicación original). El archivo aparece como "se modifica"; `enter` muestra la línea que se quitaría.
6. Respaldo al cambiar: en *Gestionar rutas* presionar `f` hasta ver `[al cambiar]`, luego modificar un archivo:

   ```bash
   echo "cambio" >> /tmp/baky-prueba/docs/informe.txt
   ```

   Unos 5 segundos después el daemon crea un nuevo `tar.gz`, y el log muestra `Respaldo por cambio iniciado` y `Respaldo completado`.

**6. Limpiar**

```bash
rm -rf /tmp/baky-prueba
unset PRUEBA BAKY_CONFIG
```

La prueba sobre una instalación real (servicio de systemd y repositorio remoto) está en el paso 7 de [docs/instalacion.md](docs/instalacion.md).

## Pendiente

- Política de retención de respaldos (borrar los más antiguos automáticamente).
- Cifrado de los respaldos y del repositorio de configuraciones.
- Alertas ante fallos del servicio.
- Comparación de versiones de respaldos desde la interfaz (`internal/restore/diff.go` ya calcula las diferencias).
- Respaldo consistente de máquinas virtuales en ejecución.

## Documentación

- [Instalación y puesta en marcha](docs/instalacion.md)
- [Registro de pruebas del daemon de configuraciones](docs/pruebas-daemon.md)
- [Registro de pruebas de la TUI](docs/pruebas-tui.md)

## Autores

- Jose D. Amaya — Investigador del Proyecto MAGITI
- Juan M. Florez — Investigador del Proyecto MAGITI

Basado en entrevista con Luis E. Sepúlveda, Administrador de Sistemas GRID (22 de junio de 2026).

## Licencia

MIT. Ver [LICENSE](LICENSE).
