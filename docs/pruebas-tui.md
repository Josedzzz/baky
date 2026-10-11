# Registro de Pruebas — TUI de Gestión (magiti-backup-tui)

> **Nota histórica.** Este registro corresponde a la TUI original del proyecto `magiti-backup-tui`. Sus pantallas ahora forman parte de baky (menús *Respaldo de configuraciones* y *Servicio*). Dos cambios respecto a lo descrito aquí: consultar el estado del servicio ya no usa `sudo`, y agregar o quitar una carpeta vigilada ya no reinicia el servicio, porque este recarga la configuración por sí solo. Los nombres de servicio, rutas y archivos que aparecen abajo son los de la versión original.

Proyecto MAGITI - GRID
Componente: `magiti-backup-tui` (interfaz de terminal en Go + Bubble Tea)

## Objetivo

Validar que la interfaz de terminal permita al administrador de infraestructura consultar y gestionar el daemon de respaldo automático (`magiti-config-backup`) sin necesidad de recordar comandos de `systemctl`, `git` o editar archivos de configuración manualmente — cubriendo las cuatro necesidades identificadas como prioritarias: estado del servicio, revisión de logs, historial de respaldos y gestión de carpetas vigiladas.

## Entorno de Pruebas

| Componente | Detalle |
|---|---|
| Sistema operativo | Ubuntu 24.04 LTS (WSL2 sobre Windows) |
| Lenguaje / Framework | Go 1.23, Bubble Tea, Bubbles, Lip Gloss |
| Servicio dependiente | `magiti-backup.service` (systemd), del proyecto `magiti-config-backup` |
| Permisos | Reglas `NOPASSWD` específicas en `/etc/sudoers.d/magiti-backup` para los subcomandos de `systemctl` utilizados |
| Editor de desarrollo | Neovim (LazyVim) |

## Registro de Pruebas

### Prueba 1 — Pantalla de estado del servicio (consulta)

- **Qué se probó:** que la TUI refleje correctamente si el servicio `magiti-backup.service` está activo o inactivo.
- **Resultado esperado:** el estado mostrado en pantalla coincide con el resultado real de `systemctl status`.
- **Resultado obtenido (primer intento):** ❌ Falló. La TUI mostraba siempre "INACTIVO", incluso con el servicio corriendo (confirmado manualmente con `sudo -n systemctl status magiti-backup.service`, que sí mostraba `active (running)`).
- **Causa raíz:** el código ejecutaba `sudo -n systemctl is-active magiti-backup.service`, pero el archivo `/etc/sudoers.d/magiti-backup` únicamente autorizaba sin contraseña los subcomandos `start`, `stop`, `restart` y `status` — **`is-active` no estaba incluido**. Al no tener autorización, `sudo -n` fallaba silenciosamente y el código interpretaba ese fallo como "servicio inactivo".
- **Corrección aplicada:** se agregó `/usr/bin/systemctl is-active magiti-backup.service` a la lista de comandos permitidos sin contraseña en el `sudoers`.
- **Reprueba:** se volvió a consultar el estado desde la TUI tras la corrección.
- **Resultado tras corrección:** ✅ Exitoso. La TUI mostró correctamente "● ACTIVO" en verde, coincidiendo con el estado real del servicio.
- **Lección para el proyecto:** cada vez que el código agregue un nuevo subcomando de `systemctl`, debe sincronizarse manualmente con el archivo `sudoers`. Queda pendiente documentar la línea completa y definitiva en el README de instalación para evitar este desfase en despliegues futuros.

### Prueba 2 — Pantalla de estado del servicio (acciones: iniciar / detener / reiniciar)

- **Qué se probó:** que las acciones `i` (iniciar), `d` (detener) y `r` (reiniciar) ejecuten correctamente el subcomando correspondiente de `systemctl` y reflejen el resultado en pantalla.
- **Procedimiento:** se probaron las tres teclas desde la pantalla de estado, verificando el mensaje de confirmación en la TUI y contrastando con `systemctl status` desde una terminal externa.
- **Resultado obtenido:** ✅ Exitoso. Cada acción mostró el mensaje "Inicio/Detención/Reinicio ejecutado correctamente", y el estado se refrescó automáticamente tras cada acción.

### Prueba 3 — Pantalla de logs en vivo (lectura y auto-refresco)

- **Qué se probó:** que la pantalla de logs muestre las últimas líneas del archivo `backup-daemon.log`, coloreadas por nivel (INFO/WARN/ERROR), y que se actualice automáticamente sin intervención del usuario.
- **Procedimiento:** se ingresó a la pantalla de logs y, desde otra terminal, se generó un cambio real en un archivo vigilado (`echo "..." >> ~/simulacion-configs/vpn/config-test.conf`).
- **Resultado esperado:** la nueva línea de log debía aparecer en la TUI en un máximo de 3 segundos (intervalo de refresco configurado), sin que el usuario presionara ninguna tecla.
- **Resultado obtenido (primer intento):** ❌ La pantalla de logs se mostraba correctamente al entrar, pero no se podía salir de ella presionando `esc` o `q` — la interfaz quedaba "congelada" en esa vista.
- **Causa raíz:** en la función `Update` del modelo, el `switch` que enruta las teclas (`tea.KeyMsg`) según la vista activa no contemplaba el caso `vistaLogs`. La función `updateLogs` ya existía y manejaba `esc`/`q` correctamente, pero nunca era invocada porque el `switch` no incluía esa rama, por lo que las teclas no producían ningún efecto.
- **Corrección aplicada:** se agregó el caso faltante (`case vistaLogs: return m.updateLogs(msg)`) al `switch` de enrutamiento de teclas.
- **Reprueba:** se repitió el ingreso a la pantalla de logs y la salida con `esc`.
- **Resultado tras corrección:** ✅ Exitoso. Navegación de entrada y salida funcionando correctamente, y el auto-refresco cada 3 segundos mostró en pantalla el evento generado desde la terminal externa sin intervención manual.

### Prueba 4 — Pantalla de historial de commits

- **Qué se probó:** que la TUI liste los últimos respaldos realizados (commits del repositorio `magiti-config-backup`), mostrando fecha y mensaje de forma legible.
- **Procedimiento:** se ingresó a la pantalla de historial y se comparó el listado mostrado contra `git log` ejecutado manualmente sobre el mismo repositorio.
- **Resultado obtenido:** ✅ Exitoso. El listado coincidió con el historial real de commits, mostrando correctamente los mensajes generados automáticamente por el daemon (ej. "actualizado nat.conf", "eliminado config-test.conf"). El refresco manual con la tecla `r` funcionó correctamente.

### Prueba 5 — Gestión de carpetas vigiladas: instalación de dependencia faltante

- **Qué se probó:** compilación del proyecto tras incorporar el componente de entrada de texto (`textinput`) del paquete `bubbles`, necesario para capturar el nombre de una nueva carpeta a vigilar.
- **Resultado obtenido (primer intento):** ❌ Falló la compilación con error `could not import github.com/charmbracelet/bubbles/textinput`.
- **Causa raíz:** el paquete `bubbles/textinput` no se encontraba descargado en el entorno local ni referenciado en `go.mod`/`go.sum`, a pesar de que `bubbles` ya figuraba como dependencia general del proyecto.
- **Corrección aplicada:**
  ```bash
  go get github.com/charmbracelet/bubbles/textinput
  go mod tidy
  ```
- **Resultado tras corrección:** ✅ Exitoso. La compilación finalizó sin errores y el módulo quedó correctamente referenciado en `go.mod` y `go.sum`.

### Prueba 6 — Gestión de carpetas vigiladas: listar carpetas actuales

- **Qué se probó:** que la pantalla muestre correctamente el mapeo de carpetas vigiladas leído desde `config.json` del daemon.
- **Resultado obtenido:** ✅ Exitoso. Se listaron `firewall → firewall` y `vpn → vpn`, con navegación funcional entre ellas mediante las flechas.

### Prueba 7 — Gestión de carpetas vigiladas: agregar una carpeta nueva

- **Qué se probó:** el flujo completo de agregar una carpeta desde la TUI, incluyendo escritura en `config.json`, creación física del directorio, y reinicio automático del servicio (opción de diseño elegida: reinicio inmediato tras guardar, aceptando una breve ventana de inactividad del daemon durante el reinicio).
- **Procedimiento:**
  1. Desde la TUI: tecla `a`, se ingresó el nombre `dns`, `Enter`.
  2. Verificación externa del archivo de configuración:
     ```bash
     cat ~/proyectos/magiti-config-backup/scripts/config.json
     ```
  3. Verificación del reinicio del servicio:
     ```bash
     sudo -n systemctl status magiti-backup.service
     ```
  4. Prueba funcional: creación de un archivo dentro de la nueva carpeta vigilada.
     ```bash
     mkdir -p ~/simulacion-configs/dns
     echo "registro de prueba" > ~/simulacion-configs/dns/registro.conf
     ```
- **Resultado obtenido:** ✅ Exitoso en los cuatro puntos:
  - `config.json` incluyó `"dns": "dns"` tras guardar.
  - El log del daemon mostró un nuevo arranque: `Daemon iniciado. Vigilando: ... (carpetas: vpn dns firewall)`.
  - El archivo `registro.conf` se detectó y respaldó automáticamente:
    ```
    [INFO] Commit realizado: actualizado registro.conf -> dns/registro.conf.
    [INFO] Push exitoso para dns/registro.conf.
    ```

### Prueba 8 — Gestión de carpetas vigiladas: eliminar una carpeta

- **Qué se probó:** que al eliminar una carpeta desde la TUI, el daemon deje de vigilarla tras el reinicio automático, sin borrar los archivos físicos ya existentes (decisión de diseño: eliminar solo detiene la vigilancia, no borra datos).
- **Procedimiento:**
  1. Desde la TUI: se seleccionó `dns`, tecla `d`, confirmación con `y`.
  2. Verificación de que `dns` ya no aparece en `config.json`.
  3. Prueba funcional: creación de un archivo nuevo dentro de la carpeta ya no vigilada.
     ```bash
     echo "esto no debería respaldarse" > ~/simulacion-configs/dns/otro.conf
     ```
- **Resultado obtenido:** ✅ Exitoso.
  - `config.json` ya no incluyó la entrada `dns`.
  - El log del daemon mostró el reinicio con el mapeo actualizado: `Daemon iniciado. Vigilando: ... (carpetas: vpn firewall)`.
  - El archivo `otro.conf` fue correctamente ignorado:
    ```
    [WARN] Carpeta no mapeada, se ignora: dns
    ```

### Prueba 9 — Gestión de carpetas vigiladas: validación del nombre al agregar

- **Qué se probó:** que el campo de texto para agregar una carpeta rechace nombres inválidos (espacios, mayúsculas, barras, secuencias `..`) tanto mientras se escribe como al confirmar con Enter, evitando así nombres que pudieran romper el mapeo 1:1 o crear rutas fuera del directorio vigilado.
- **Procedimiento:**
  1. Se intentó escribir un espacio en blanco dentro del campo — carácter bloqueado, no apareció en el input.
  2. Se intentó escribir una letra mayúscula (`D`) — carácter bloqueado.
  3. Se intentó escribir una secuencia con barras (`../../etc`) — los caracteres `/` y `.` fueron bloqueados individualmente por el validador de `textinput`.
  4. Se dejó el campo vacío y se presionó Enter — se mostró el mensaje "el nombre no puede estar vacío" sin intentar guardar.
  5. Se escribió un nombre válido (`dns2`) y se confirmó con Enter.
- **Resultado esperado:** los casos 1–4 debían impedir el guardado; el caso 5 debía completarse con éxito.
- **Resultado obtenido:** ✅ Exitoso en los cinco casos. El nombre válido `dns2` se guardó correctamente en `config.json` y el servicio se reinició reflejando la nueva carpeta:
  ```
  Daemon iniciado. Vigilando: /home/miguel/simulacion-configs (carpetas: vpn dns2 firewall)
  ```
- **Observación adicional (no es un bug):** tras agregar `dns2`, no apareció ningún commit nuevo en el historial de GitHub hasta que se creó un archivo real dentro de la carpeta. Esto es el comportamiento esperado: la TUI únicamente actualiza `config.json`, crea el directorio físico y reinicia el servicio — no ejecuta operaciones de Git por sí misma. El respaldo (commit/push) sigue siendo responsabilidad exclusiva del daemon, que solo actúa ante eventos reales de archivo (`close_write`, `delete`, `moved_to`, etc.); una carpeta vacía no genera ningún evento de `inotifywait`, por lo que no hay nada que respaldar hasta que se agregue contenido. Se confirmó el flujo completo creando un archivo de prueba dentro de `dns2/`, el cual sí generó el commit y push correspondientes.

## Bugs Detectados y Corregidos Durante las Pruebas

| # | Bug | Causa | Corrección |
|---|---|---|---|
| 1 | Estado del servicio mostraba siempre "INACTIVO" | Subcomando `is-active` de `systemctl` no autorizado en `sudoers` | Se agregó `is-active` a la lista de comandos `NOPASSWD` |
| 2 | Pantalla de logs no permitía volver al menú | Caso `vistaLogs` faltante en el `switch` de enrutamiento de teclas dentro de `Update` | Se agregó el caso faltante para invocar `updateLogs` |
| 3 | Fallo de compilación al usar `textinput` | Dependencia `bubbles/textinput` no descargada/registrada en `go.mod` | `go get` del paquete específico + `go mod tidy` |

## Cobertura de Pruebas

| Funcionalidad | Cubierta |
|---|---|
| Consultar estado del servicio | ✅ |
| Iniciar / detener / reiniciar el servicio desde la TUI | ✅ |
| Visualización de logs con auto-refresco | ✅ |
| Coloreado de logs por nivel (INFO/WARN/ERROR) | ✅ |
| Historial de commits (respaldos realizados) | ✅ |
| Listar carpetas vigiladas | ✅ |
| Agregar carpeta vigilada (con reinicio automático) | ✅ |
| Eliminar carpeta vigilada (con reinicio automático) | ✅ |
| Validación de formato del nombre de carpeta (caracteres especiales, espacios) | ✅ |
| Scroll en listas largas (logs/historial con muchas entradas) | ⏸️ Pendiente |

## Conclusión

La TUI (`magiti-backup-tui`) cubre, en su versión actual, las cuatro necesidades priorizadas por el equipo del proyecto para la gestión operativa del daemon de respaldo: visibilidad del estado del servicio, revisión de actividad, trazabilidad de respaldos realizados, y administración de las carpetas vigiladas sin edición manual de archivos de configuración. Las pruebas realizadas permitieron detectar y corregir tres fallas — dos de configuración/permisos y una de lógica de enrutamiento de eventos — antes de considerar esta fase del proyecto como estable. Adicionalmente, se incorporó y validó una capa de saneamiento de entrada (nombres de carpetas) para prevenir configuraciones inválidas o rutas fuera del directorio vigilado.
