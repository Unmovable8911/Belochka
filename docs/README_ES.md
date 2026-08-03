<p align="center">
  <img src="../logo.png" width="500">
</p>
<p align="center">
<a href="./README_DE.md">Deutsch</a> / 
<a href="../README.md">English</a> / 
Español / 
<a href="./README_FR.md">Français</a> / 
<a href="./README_IT.md">Italiano</a> / 
<a href="./README_PT.md">Português</a> / 
<a href="./README_RU.md">Русский</a> / 
<a href="./README_CN.md">中文</a> / 
<a href="./README_TW.md">繁體中文</a>
</p>
<hr>

Belochka (белочка, «ardilla») es una herramienta de monitorización de servidores en un solo binario para pequeñas flotas de servidores Linux. Mantiene conexiones SSH persistentes con 5–20 máquinas remotas y transmite métricas en tiempo real de CPU, memoria, disco, red y procesos a un panel de control en el navegador mediante WebSocket. También incluye un terminal interactivo basado en la web para el acceso SSH directo — no se necesita un cliente SSH aparte.

## Características

- **Grupos de servidores** — organice los servidores en grupos planos de nivel raíz en la barra lateral; cree, renombre y elimine grupos, y mueva servidores dentro o fuera de un grupo mediante arrastrar y soltar o el menú «Mover a…»; haga clic en un grupo para filtrar el panel a sus miembros, con navegación por migas de pan
- **Panel en tiempo real** — tarjetas de servidor con métricas en vivo de CPU, memoria, disco y red, codificadas por colores según el uso; clic derecho en una tarjeta para acciones rápidas Editar / Eliminar / Consola
- **Vista detallada del servidor** — medidores de CPU por núcleo, gráficos de anillo de memoria/swap, desglose de particiones de disco, rendimiento de las interfaces de red
- **Gestión de procesos** — pestaña Procesos con una tabla plana ordenable (columnas de ancho ajustable), búsqueda de varias palabras clave, conmutador de actualización automática y finalización con selección de señal SIGTERM/SIGKILL (sshd/init/systemd protegidos)
- **Terminal web** — consola SSH interactiva completa en el navegador mediante xterm.js
- **Icono en la bandeja del sistema** — en equipos de escritorio (Windows, macOS, Linux con GNOME/KDE/XFCE), muestra un icono de bandeja con las opciones **Abrir panel** y **Salir**; cambia automáticamente al modo CLI en servidores sin interfaz gráfica
- **Autenticación** — protección con contraseña y cookie de sesión; la primera visita recorre un asistente de configuración en dos pasos (elegir idioma → establecer contraseña con indicador de seguridad y confirmación); limitación tras 10 intentos de inicio de sesión fallidos (bloqueo de 30 minutos)
- **Un solo binario** — backend Go con frontend React integrado; un solo archivo para desplegar, nada más que instalar
- **Conexiones SSH persistentes** — reconexión automática con retroceso exponencial y keepalive; pruebe la conexión antes de guardar un servidor y verifique la huella de la clave del host al añadir nuevas máquinas (confianza en el primer uso)
- **Subida de claves desde el navegador** — suba archivos de clave privada SSH directamente a través de la interfaz; las claves se validan, se almacenan con nombres UUID y los archivos huérfanos se limpian automáticamente
- **Almacenamiento cifrado de credenciales** — las contraseñas de los servidores se cifran en reposo con AES-256-GCM
- **Gestión de tareas cron** — ver, añadir, editar, habilitar/deshabilitar, eliminar y ejecutar tareas cron directamente desde la página de detalle del servidor
- **Comando por lotes** — escriba un script de varias líneas y envíelo a cualquier conjunto de servidores desde la barra lateral; la salida del terminal de cada servidor se transmite en vivo al diálogo, puede responder a las indicaciones interactivas y cancelar toda la ejecución en cualquier momento
- **Archivo de registro persistente** — toda la salida se escribe en `belochka.log` junto al binario (o en el directorio de trabajo actual cuando se ejecuta con `go run`) con limpieza automática basada en la retención (predeterminado: 3 días)
- **Interfaz multilingüe** — inglés, chino simplificado, francés, ruso, alemán, español, portugués, chino tradicional e italiano; seleccionable durante la configuración inicial y cambiable desde el diálogo de Configuración; se preselecciona el idioma detectado por el navegador
- **Configuración integrada** — configure el puerto, el directorio de datos, el idioma y la retención de registros directamente desde el panel mediante el icono de engranaje; sin necesidad de editar archivos de configuración

## Inicio rápido

Descargue el último binario desde [Releases](https://github.com/Unmovable8911/Belochka/releases) y ejecútelo:

```bash
# Linux (amd64)
chmod +x belochka-linux-amd64
./belochka-linux-amd64

# Windows (64 bits)
belochka-windows-x86-64.exe
```

Abra `http://localhost:53136` en su navegador. En la primera visita se le pedirá elegir idioma y establecer una contraseña — esto protege el panel y todos los endpoints de la API. Tras la configuración, se inicia sesión automáticamente. Añada servidores a través de la interfaz.

## Compilar desde el código fuente

Requiere Go 1.25+ y Node.js 18+.

```bash
git clone https://github.com/Unmovable8911/Belochka.git
cd Belochka
make build
./bin/belochka
```

Compile de forma cruzada los binarios de lanzamiento para todas las plataformas:

```bash
make release
# Salida:
#   bin/belochka-linux-amd64
#   bin/belochka-linux-arm64
#   bin/belochka-windows-x86-64.exe
#   bin/belochka-windows-x86.exe
```

## Configuración

Belochka funciona directamente sin configuración. Todos los ajustes están disponibles a través del **diálogo de Configuración** (icono de engranaje en la cabecera del panel). Un `config.json` con valores predeterminados se crea automáticamente junto al binario en el primer arranque. También puede pasar `--config ruta/a/config.json` para una ubicación personalizada:

```json
{
  "port": 53136,
  "data_dir": "./data",
  "language": "",
  "log_path": "",
  "log_retention_days": 3
}
```

| Campo | Predeterminado | Descripción |
|---|---|---|
| `port` | `53136` | Puerto de escucha HTTP |
| `data_dir` | `./data` | Base de datos, clave de cifrado y archivos de clave SSH subidos (`data/keys/`) |
| `language` | `""` | Idioma de la interfaz (`en`, `zh`, `fr`, `ru`, `de`, `es`, `pt`, `zh-TW`, `it`); se detecta automáticamente en la primera visita si está vacío |
| `log_path` | `""` | Ruta del archivo de registro; usa `belochka.log` junto al binario si está vacío |
| `log_retention_days` | `3` | Número de días que se conservan las entradas de registro |

Los cambios en `port` y `data_dir` requieren un reinicio; `language` y `log_retention_days` se aplican inmediatamente a través del diálogo de Configuración.

### Banderas

| Bandera | Descripción |
|---|---|
| `--config <ruta>` | Ruta al archivo de configuración JSON |
| `--no-tray` | Desactivar el icono de la bandeja; ejecutar como proceso CLI |
| `--version` | Mostrar la versión y salir |

### Variables de entorno

| Variable | Descripción |
|---|---|
| `BELOCHKA_ENCRYPTION_KEY` | Clave AES-256 para las contraseñas almacenadas; se genera automáticamente en el primer arranque si no está definida |

### Clave de cifrado

En el primer arranque sin una clave definida, Belochka genera automáticamente una en `{data_dir}/encryption.key` y registra una advertencia. Para producción, establezca la clave explícitamente mediante la variable de entorno.
