# nvsh (Node Version Shifter) 🚀

Un gestor de versiones de Node.js rápido, nativo y multiplataforma, escrito en **Go**. Diseñado desde cero para ejecutarse en **Linux**, **macOS (Apple Silicon e Intel)** y **Windows**, **sin requerir permisos de administrador** y funcionando en **Shim Mode (sin symlinks)**.

---

## ⚡ ¿Por qué `nvsh` frente a `nvm` / `nvm-windows`?

| Característica | `nvm` (Bash) | `nvm-windows` | `nvsh` (Node Version Shifter) |
| :--- | :--- | :--- | :--- |
| **Lenguaje** | Shell script | Go / C | **Go nativo (ultra-rápido)** |
| **Soporte OS** | Linux, macOS | Solo Windows | **Linux, macOS, Windows** |
| **Permisos de Admin** | No (en Unix) | **Sí requerido** (creación de symlinks en `C:\Program Files`) | **CERO permisos de Administrador / Sudo** |
| **Modo de Operación** | Modifica `$PATH` en shell | Symlinks/Junctions | **Shim Mode (NO symlinks)** |
| **Cambio Automático** | Requiere hooks complejos (`avn`/`direnv`) | Manual | **Automático por directorio (`.nvmrc`, `.node-version`)** |
| **Velocidad de Shim** | N/A | Lento | **~1-2 ms (`syscall.Exec` en Unix, ejecutable directo en Windows)** |

---

## 🛡️ ¿Qué es "Shim Mode - No Symlinks"?

En entornos corporativos o Windows sin "Modo Desarrollador" habilitado, la creación de enlaces simbólicos (*symlinks*) está bloqueada por directivas de seguridad y requiere elevación de Administrador (UAC).

`nvsh` soluciona esto mediante **Shims Ejecutables Reales**:
1. `nvsh` coloca ejecutables ligeros para `node`, `npm`, `npx` y `corepack` en `~/.nvsh/shims/`.
2. Estos ejecutables **no son symlinks**; son creados mediante *hardlinks* (en el mismo sistema de archivos) o copia de binario directa.
3. Cuando ejecutas `node` o `npm`, el shim intercepta la llamada, resuelve qué versión de Node.js corresponde a tu directorio actual y transfiere la ejecución inmediatamente:
   - En **Linux y macOS**: utiliza la llamada al sistema `syscall.Exec`, reemplazando el proceso en memoria con **cero sobrecarga**, propagando señales (SIGINT, SIGTERM) y códigos de salida de forma nativa.
   - En **Windows**: inicia el ejecutable de destino propagando `stdin`, `stdout`, `stderr` y el código de retorno exacto sin requerir elevación.

---

## 📂 Jerarquía de Resolución de Versiones

Cuando ejecutas cualquier comando (`node app.js`, `npm test`, etc.), `nvsh` busca la versión adecuada en el siguiente orden de prioridad:

1. **Variable de entorno**: `NVSH_VERSION` o `NODE_VERSION`.
2. **Directorio actual y superiores (recursivo)**:
   - `.nvsh-version`
   - `.nvmrc` (soporta números como `20`, `v20.18.0`, `lts/*`, `lts/iron`, `node`, `latest`)
   - `.node-version`
   - `package.json` (campo `engines.node`)
3. **Versión global predeterminada**: Configurada en `~/.nvsh/default`.
4. **Última versión instalada**: Si no hay ninguna regla definida.

---

## 📥 Instalación (100% en Espacio de Usuario - Sin Root/Admin)

### En Linux y macOS (Apple Silicon o Intel)

```bash
# 1. Clonar o descargar el repositorio
git clone https://github.com/fongo/node-version-shifter.git
cd node-version-shifter

# 2. Ejecutar el instalador
./install.sh
```

O si compilas directamente:
```bash
go build -o nvsh ./cmd/nvsh
./nvsh setup
```

Agrega lo siguiente a tu archivo de configuración de shell (`~/.bashrc`, `~/.zshrc` o `~/.config/fish/config.fish`):
```bash
# Bash / Zsh
export NVSH_DIR="$HOME/.nvsh"
export PATH="$NVSH_DIR/shims:$NVSH_DIR/bin:$PATH"
```

```fish
# Fish
set -gx NVSH_DIR "$HOME/.nvsh"
fish_add_path "$NVSH_DIR/shims" "$NVSH_DIR/bin"
```

---

### En Windows (PowerShell)

Abre **PowerShell como usuario normal** (sin permisos de Administrador):

```powershell
# 1. Clonar o descargar el repositorio
cd node-version-shifter

# 2. Ejecutar el instalador
.\install.ps1
```

El script configurará automáticamente la variable de entorno `Path` del **Usuario** en el Registro (`[Environment]::SetEnvironmentVariable("Path", ..., "User")`), sin pedir confirmación de Administrador (UAC).

---

## 📖 Comandos y Uso

### 1. Instalar versiones de Node.js

```bash
# Instalar la versión LTS activa más reciente (ej. Node 22)
nvsh install lts

# Instalar por versión mayor (descarga la última versión de la serie 20)
nvsh install 20

# Instalar la versión más reciente disponible
nvsh install latest

# Instalar una versión exacta
nvsh install 20.18.0
```

### 2. Listar versiones

```bash
# Listar versiones instaladas localmente
nvsh ls

# Listar versiones disponibles en nodejs.org
nvsh ls-remote

# Filtrar versiones remotas
nvsh ls-remote 22
```

### 3. Establecer la versión activa

```bash
# Establecer la versión para el proyecto actual (crea .node-version en el directorio)
nvsh use 20

# Establecer la versión global predeterminada del sistema
nvsh default 22
```

### 4. Consultar el estado

```bash
# Ver qué versión está activa en el directorio actual y de dónde proviene
nvsh current
# Salida: v22.14.0 (resolved from /home/usuario/mi-proyecto/.nvmrc)

# Ver la ruta física del binario que ejecutará el shim
nvsh which node
# Salida: /home/usuario/.nvsh/versions/v22.14.0/bin/node
```

### 5. Ejecutar un comando puntual con otra versión

```bash
# Ejecutar un script con Node 18 sin cambiar la versión de tu proyecto
nvsh exec 18 node -v
nvsh exec 20 npm test
```

### 6. Desinstalar versiones

```bash
nvsh uninstall 18.20.0
```

### 7. Regenerar Shims

Si instalas herramientas globales con `npm install -g pnpm yarn typescript nodemon`:
```bash
nvsh rehash
```
`nvsh` detectará automáticamente los nuevos binarios en todas las versiones instaladas y creará sus correspondientes shims en `~/.nvsh/shims/` **sin usar symlinks**.

---

## 🏗️ Compilación Cruzada (Linux, macOS, Windows)

Para generar los binarios precompilados de todas las arquitecturas soportadas:

```bash
make cross-compile
```

Los binarios se generarán en la carpeta `dist/`:
- `dist/nvsh-linux-amd64`
- `dist/nvsh-linux-arm64`
- `dist/nvsh-darwin-amd64` (macOS Intel)
- `dist/nvsh-darwin-arm64` (macOS Apple Silicon M1/M2/M3/M4)
- `dist/nvsh-windows-amd64.exe` (Windows x64)
- `dist/nvsh-windows-arm64.exe` (Windows ARM64)

---

## 🧪 Pruebas Automatizadas

Para ejecutar la suite de pruebas unitarias:
```bash
go test -v ./...
```
Incluye pruebas específicas que certifican que los shims **nunca** son enlaces simbólicos (`info.Mode() & os.ModeSymlink == 0`), garantizando compatibilidad 100% sin permisos administrativos en todas las plataformas.

---

## 📄 Licencia

MIT License.
