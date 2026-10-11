# Instalación y puesta en marcha

Esta guía deja baky funcionando en un servidor Linux con systemd: el binario, el servicio que ejecuta los respaldos automáticos y el repositorio de configuraciones.

## 1. Requisitos

| Requisito | Para qué |
|---|---|
| Go 1.24 o superior | Compilar baky (solo en la máquina donde se compila) |
| `git` | Respaldo de configuraciones |
| `systemd` | Ejecutar el servicio en segundo plano |
| Clave SSH con acceso al repositorio de configuraciones | Enviar los commits al remoto sin pedir contraseña |

## 2. Compilar e instalar el binario

```bash
git clone https://github.com/Josedzzz/baky.git
cd baky
go build -o baky ./cmd
sudo install -m 0755 baky /usr/local/bin/baky
```

## 3. Elegir el usuario

La interfaz y el servicio deben ejecutarse con **el mismo usuario**, porque comparten los archivos de `~/.config/baky/`. Ese usuario debe poder:

- leer las carpetas de configuración que se van a vigilar,
- leer las rutas que se van a respaldar y escribir en el destino de respaldo,
- hacer `git push` al repositorio de configuraciones con su clave SSH.

En los pasos siguientes ese usuario se llama `grid`; reemplácelo por el real.

## 4. Clonar el repositorio de configuraciones

Las configuraciones se guardan en un repositorio dedicado, que no contiene código. En el GRID es `grid-config-data`:

```bash
git clone git@github.com:MiguelFVasquez/grid-config-data.git ~/grid-config-data
```

Use la dirección SSH (`git@github.com:...`), no la HTTPS: el servicio no puede escribir una contraseña.

Compruebe que el usuario puede enviar cambios sin que se le pida nada:

```bash
ssh -T git@github.com
```

Si el usuario no tiene nombre ni correo configurados en Git, baky firma los commits como `baky@<nombre del equipo>`.

## 5. Instalar el servicio

```bash
sed 's/CAMBIAR_USUARIO/grid/' deploy/baky.service | sudo tee /etc/systemd/system/baky.service > /dev/null
sudo systemctl daemon-reload
sudo systemctl enable --now baky.service
systemctl status baky.service
```

Para que la interfaz pueda iniciar, detener y reiniciar el servicio sin pedir contraseña:

```bash
sed 's/CAMBIAR_USUARIO/grid/' deploy/baky.sudoers | sudo tee /etc/sudoers.d/baky > /dev/null
sudo chmod 0440 /etc/sudoers.d/baky
sudo visudo -cf /etc/sudoers.d/baky
```

Consultar el estado no necesita `sudo`.

## 6. Configurar desde la interfaz

Ejecute `baky` con el usuario elegido:

1. **Respaldo de configuraciones → Repositorio**: `e`, escriba la ruta del clon (`/home/grid/grid-config-data`) y confirme. Con `t` se comprueba que es un repositorio válido.
2. **Respaldo de configuraciones → Carpetas vigiladas**: `a`, escriba la carpeta a vigilar (`/etc/openvpn`) y luego el nombre que tendrá dentro del repositorio (`vpn`).
3. **Respaldo de archivos → Destino de respaldo**: `e`, escriba la ruta del destino (`/mnt/nas/respaldos`). Con `t` se comprueba que es accesible.
4. **Respaldo de archivos → Gestionar rutas**: `a` para agregar una ruta y `f` para elegir su frecuencia.

El servicio aplica cada cambio por sí solo. En **Servicio → Logs en vivo** se ve lo que hace.

## 7. Comprobar que funciona

```bash
echo "# prueba" | sudo tee -a /etc/openvpn/prueba.conf
```

En pocos segundos debe aparecer en **Respaldo de configuraciones → Historial de cambios** un commit `auto-backup: actualizado prueba.conf (...)`, y en el log las líneas `Commit realizado` y `Push exitoso`.

## Migrar desde magiti-config-backup

Si la máquina tenía el daemon en Bash (`magiti-backup.service`):

```bash
# 1. Detener y deshabilitar el servicio anterior
sudo systemctl disable --now magiti-backup.service

# 2. Importar las carpetas que vigilaba
baky import-magiti ~/proyectos/magiti-config-backup/scripts/config.json

# 3. Configurar el repositorio nuevo desde la interfaz (paso 6.1) e instalar el servicio (paso 5)
```

El repositorio anterior no se importa: mezclaba el código del daemon con los respaldos. Las configuraciones pasan al repositorio dedicado, y en su primer arranque el servicio las respalda todas.

Después de comprobar que baky funciona, se pueden retirar la unidad `/etc/systemd/system/magiti-backup.service` y la regla `/etc/sudoers.d/magiti-backup`.

## Solución de problemas

| Síntoma | Causa probable |
|---|---|
| El log muestra `Fallo al hacer push` | El usuario del servicio no tiene acceso SSH al remoto, o el clon usa una dirección HTTPS. Los commits quedan guardados y se reintenta. |
| `Respaldo de configuraciones desactivado: ... no es un repositorio Git válido` | La ruta configurada no es un clon de Git. Corríjala en *Repositorio*. |
| `Carpeta vigilada no accesible, se omite` | La carpeta no existe o el usuario del servicio no puede leerla. Lo ya respaldado se conserva. |
| La interfaz muestra el servicio como inactivo | `systemctl status baky.service` y `journalctl -u baky.service` muestran el motivo. |
| Iniciar o detener desde la interfaz falla | Falta la regla de sudoers del paso 5. |
| La interfaz no muestra lo que hace el servicio | La interfaz y el servicio se ejecutan con usuarios distintos y usan archivos distintos. |
