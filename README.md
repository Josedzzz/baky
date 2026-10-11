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
      { "source": "/etc/iptables", "dest": "firewall" }
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
│   └── tui/               Interfaz de terminal, un archivo por pantalla
├── deploy/                Unidad de systemd y regla de sudoers
└── docs/                  Instalación y registros de pruebas
```

El código (identificadores y comentarios) está en inglés; la interfaz y la documentación, en español.

## Desarrollo

```bash
go build -o baky ./cmd
go test ./...
```

Las pruebas no usan la red: el respaldo de configuraciones se prueba contra un repositorio Git local que hace de remoto.

## Pendiente

- Política de retención de respaldos (borrar los más antiguos automáticamente).
- Cifrado de los respaldos y del repositorio de configuraciones.
- Alertas ante fallos del servicio.
- Comparación de versiones de respaldos desde la interfaz (`internal/restore/diff.go` ya calcula las diferencias).
- Restauración de una configuración anterior desde el historial.
- Respaldo consistente de máquinas virtuales en ejecución.
- Validación de sintaxis de las reglas de Firewall antes del commit.

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
