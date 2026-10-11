# Registro de Pruebas  Daemon de Respaldo Automatico

> **Nota histórica.** Este registro corresponde a la versión original del daemon, escrita en Bash (`backup-daemon.sh`, proyecto `magiti-config-backup`). Esa lógica ahora vive en baky, reescrita en Go (`internal/gitbackup`), y conserva el mismo comportamiento observable: los mismos mensajes de commit y las mismas líneas de log. Los escenarios descritos aquí están cubiertos por las pruebas automáticas de `internal/gitbackup` y `internal/daemon`. Los nombres de servicio, rutas y archivos que aparecen abajo son los de la versión original.

Proyecto MAGITI - GRID
Componente: `scripts/backup-daemon.sh`

## Objetivo

Validar que el daemon de respaldo automático cumple con los requisitos identificados en la entrevista con el administrador de infraestructura (sección 3.3 y 4.1 del Registro de Entrevista): detección de cambios en tiempo real sobre archivos de configuración crítica (Firewall, VPN) y respaldo automático versionado hacia un repositorio Git, sin depender de intervención manual.

## Entorno de Pruebas

| Componente | Detalle |
|---|---|
| Sistema operativo | Ubuntu 24.04 LTS (WSL2 sobre Windows) |
| Gestor de procesos | systemd (habilitado explícitamente en WSL2) |
| Herramientas | git, inotify-tools (inotifywait), jq |
| Repositorio remoto | GitHub (privado) — fase de pruebas |
| Directorio simulado | `~/simulacion-configs/` (sustituye las rutas reales de producción del servidor GRID) |

> Nota: los archivos de configuración utilizados en las pruebas son ficticios. Ninguna configuración real de Firewall o VPN de la infraestructura del GRID fue utilizada durante esta fase.

## Registro de Pruebas

### Prueba 1 — Respaldo automático ante creación/modificación de archivo (MVP inicial)

- **Qué se probó:** que el daemon detecte un cambio en un archivo dentro de una carpeta vigilada y lo respalde automáticamente en el repositorio Git.
- **Procedimiento:**
  ```bash
  echo "regla de prueba $(date)" >> ~/simulacion-configs/firewall/reglas.conf
  ```
- **Resultado esperado:** el daemon detecta el evento `close_write`, copia el archivo al repositorio, hace `git commit` y `git push`.
- **Resultado obtenido:** ✅ Exitoso. Log:
  ```
  [INFO] Cambio detectado: /home/miguel/simulacion-configs/firewall/reglas.conf
  [INFO] Commit realizado para firewall/reglas.conf.
  [INFO] Push exitoso para firewall/reglas.conf.
  ```
  Confirmado también en GitHub (commit `auto-backup: reglas.conf`).

### Prueba 2 — Persistencia del daemon ante caída de sesión

- **Qué se probó:** si el daemon sobrevive al cierre de la terminal o sesión donde fue iniciado.
- **Procedimiento:** se inició el daemon manualmente en una terminal, luego se cerró dicha terminal/sesión.
- **Resultado esperado (antes de la corrección):** el daemon debía seguir corriendo.
- **Resultado obtenido:** ❌ Falló. Al verificar con `ps -ef`, el proceso ya no existía. El daemon dependía de la sesión de terminal donde fue lanzado.
- **Causa raíz:** el script se ejecutaba directamente en foreground, sin un gestor de procesos que lo mantuviera vivo de forma independiente a la sesión del usuario.
- **Corrección aplicada:** se implementó un servicio `systemd` (`magiti-backup.service`) con `Restart=on-failure`, ejecutado como usuario del sistema, con arranque automático (`enable`).
- **Reprueba:** se ejecutó `wsl --shutdown` (equivalente a un reinicio completo del sistema) y se verificó el estado del servicio al reiniciar.
- **Resultado tras corrección:** ✅ Exitoso.
  ```
  Active: active (running) since Fri 2026-07-24 20:57:07 -05; 3min 36s ago
  ```
  El servicio arrancó automáticamente sin intervención manual.

### Prueba 3 — Configuración externalizada (`config.json`)

- **Qué se probó:** que el daemon lea correctamente el directorio vigilado y el mapeo de carpetas desde un archivo de configuración externo (`config.json`), en vez de tener estos valores fijos dentro del script.
- **Procedimiento:**
  ```bash
  jq '.' scripts/config.json
  jq -r '.carpetas | to_entries[] | "\(.key)=\(.value)"' scripts/config.json
  ```
- **Resultado esperado:** el JSON se parsea correctamente y el daemon inicia usando esos valores.
- **Resultado obtenido:** ✅ Exitoso.
  ```
  firewall=firewall
  vpn=vpn
  ```
  El log de inicio del daemon confirmó: `Vigilando: /home/miguel/simulacion-configs (carpetas: vpn firewall)`.
- **Justificación:** esta externalización es un prerrequisito para que una futura interfaz (TUI) pueda modificar qué se vigila sin editar el código del script directamente.

### Prueba 4 — Manejo de eliminación de archivos

- **Qué se probó:** que al eliminar un archivo dentro de una carpeta vigilada, el daemon refleje esa eliminación en el repositorio Git (en vez de dejar un archivo huérfano).
- **Procedimiento:**
  ```bash
  rm ~/simulacion-configs/vpn/config-test.conf
  ```
- **Resultado esperado:** el daemon detecta el evento `DELETE`, elimina el archivo correspondiente en el repo y hace commit de la eliminación.
- **Resultado obtenido:** ✅ Exitoso.
  ```
  [INFO] Evento detectado: DELETE en /home/miguel/simulacion-configs/vpn/config-test.conf
  [INFO] Commit realizado: eliminado config-test.conf -> vpn/config-test.conf.
  [INFO] Push exitoso para vpn/config-test.conf.
  ```

### Prueba 5 — Manejo de renombrado de archivos

- **Qué se probó:** que un renombrado/movimiento de archivo (`mv`) se refleje correctamente como una eliminación del nombre anterior y una creación del nuevo.
- **Procedimiento:**
  ```bash
  echo "contenido" > ~/simulacion-configs/firewall/temporal.conf
  mv ~/simulacion-configs/firewall/temporal.conf ~/simulacion-configs/firewall/renombrado.conf
  ```
- **Resultado esperado:** dos eventos (`MOVED_FROM` y `MOVED_TO`), resultando en un commit de eliminación del archivo viejo y un commit de creación del nuevo.
- **Resultado obtenido:** ✅ Exitoso.
  ```
  [INFO] Evento detectado: MOVED_FROM en .../firewall/temporal.conf
  [INFO] El archivo eliminado no existía en el repo, nada que hacer: firewall/temporal.conf
  [INFO] Evento detectado: MOVED_TO en .../firewall/renombrado.conf
  [INFO] Commit realizado: actualizado renombrado.conf -> firewall/renombrado.conf.
  ```
  Nota: el mensaje "no existía en el repo" es correcto en este caso puntual, ya que `temporal.conf` nunca llegó a respaldarse antes del renombrado (bug detectado y corregido en la Prueba 6).

### Prueba 6 — Manejo de subcarpetas anidadas dentro de una carpeta vigilada

- **Qué se probó:** que archivos dentro de subcarpetas (ej. `firewall/reglas_extra/nat.conf`) se respalden preservando su estructura de carpetas, no solo el nombre del archivo.
- **Procedimiento:**
  ```bash
  mkdir -p ~/simulacion-configs/firewall/reglas_extra
  echo "nat rule" > ~/simulacion-configs/firewall/reglas_extra/nat.conf
  ```
- **Resultado esperado (primer intento):** el archivo debía respaldarse en `firewall/reglas_extra/nat.conf` dentro del repo.
- **Resultado obtenido (primer intento):** ❌ Falló silenciosamente.
  ```
  [WARN] Evento no manejado explícitamente: CLOSE_WRITE,CLOSE
  ```
- **Causa raíz:** el `case` del script esperaba el valor literal `CLOSE_WRITE:CLOSE_WRITE`, pero `inotifywait` reporta el evento como `CLOSE_WRITE,CLOSE` (separado por coma, no dos puntos). Esto afectaba **todas** las creaciones/modificaciones de archivo, no solo las de subcarpetas — el bug fue detectado gracias a esta prueba.
- **Corrección aplicada:** se cambió la coincidencia del `case` de una cadena exacta a un patrón (`*CLOSE_WRITE*`), más resistente a variaciones en el formato exacto del evento.
- **Reprueba:**
  ```bash
  echo "nat rule v2" > ~/simulacion-configs/firewall/reglas_extra/nat.conf
  ```
- **Resultado tras corrección:** ✅ Exitoso.
  ```
  [INFO] Commit realizado: actualizado nat.conf -> firewall/reglas_extra/nat.conf.
  [INFO] Push exitoso para firewall/reglas_extra/nat.conf.
  ```
  Verificado también con `ls -la`, confirmando que la subcarpeta `reglas_extra/` se preservó correctamente dentro de `firewall/` en el repositorio.

## Bugs Detectados y Corregidos Durante las Pruebas

| # | Bug | Causa | Corrección |
|---|---|---|---|
| 1 | Daemon no sobrevive al cierre de sesión | Ejecución en foreground sin gestor de procesos | Implementación de servicio systemd con `Restart=on-failure` |
| 2 | Eventos de creación/modificación no se procesaban (`CLOSE_WRITE,CLOSE`) | Coincidencia exacta de string en `case`, no correspondía al formato real de `inotifywait` | Cambio a coincidencia por patrón (`*CLOSE_WRITE*`) |

## Cobertura de Pruebas

| Escenario | Cubierto |
|---|---|
| Creación/modificación de archivo | ✅ |
| Eliminación de archivo | ✅ |
| Renombrado/movimiento de archivo | ✅ |
| Subcarpetas anidadas | ✅ |
| Persistencia ante reinicio del sistema | ✅ |
| Configuración externa (config.json) | ✅ |
| Validación de sintaxis antes de commit | ⏸️ Pendiente (fuera de alcance v1) |
| Cifrado en reposo del repositorio | ⏸️ Pendiente (fuera de alcance v1) |

## Conclusión

El daemon de respaldo automático (`backup-daemon.sh`) cumple, en su versión actual (v1), con los criterios de éxito medibles definidos por el entrevistado en la sección 4.2 del Registro de Entrevista: visibilidad continua sobre el estado de los archivos vigilados, respaldo automático sin intervención manual, y trazabilidad completa de los cambios mediante el historial de commits de Git. Las pruebas realizadas permitieron identificar y corregir dos fallas antes de avanzar a la siguiente fase del proyecto (interfaz TUI de gestión).
