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
git clone https://github.com/caco123/node-version-shifter.git
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

### En Windows (PowerShell / CMD)

Abre **PowerShell o CMD como usuario normal** (sin permisos de Administrador):

**Instalación rápida en una sola línea (One-Liner):**
```powershell
powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/caco123/node-version-shifter/master/install.ps1 | iex"
```

**O si descargas / clonas el repositorio:**
```powershell
# 1. Entrar en la carpeta
cd node-version-shifter

# 2. Ejecutar el instalador (Opción recomendada, evita bloqueos de directiva):
.\install.cmd

# O alternativamente con PowerShell explícito:
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

> [!TIP]
> **Nota sobre políticas de PowerShell en Windows (`PSSecurityException`)**:
> Si ejecutas directamente scripts `.ps1`, Windows puede bloquearlos por su política de ejecución predeterminada (`Restricted`). Los scripts `.cmd` (`install.cmd` y `uninstall.cmd`) resuelven esto de forma nativa sin requerir elevación de administrador ni alterar la seguridad global del sistema.

El instalador detectará el binario local, o lo compilará con Go si está disponible, o lo descargará automáticamente. Además, configurará la variable de entorno `Path` de usuario sin requerir elevación de Administrador (UAC).

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

# Establecer la versión global predeterminada del sistema (actualiza automáticamente la versión en el Path)
nvsh default 22
```

> [!NOTE]
> **Gestión automática de la versión de Node.js en el `Path`**:
> - Al instalar tu primera versión de Node.js o al cambiar la versión default (`nvsh default <version>`), `nvsh` agrega o actualiza automáticamente el directorio de dicha versión directamente en la variable `Path` (en las variables de usuario del Registro en Windows sin permisos de administrador y en tu archivo de perfil `.bashrc`, `.zshrc` o `config.fish` en Unix/macOS).
> - **Solo existe una entrada de versión en el `Path`** a la vez y siempre corresponde a la versión global `default` (ej. `%USERPROFILE%\.nvsh\versions\v26.0.0`).
> - **Al inicio no existe ninguna versión instalada**, por lo que el `Path` solo contendrá `shims` y `bin`, sin rutas de versiones.
> - **Si se eliminan todas las versiones de Node.js** mediante `nvsh uninstall`, la ruta de la versión se elimina automáticamente del `Path`.

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

### 6. Desinstalar versiones de Node.js

```bash
# Eliminar una versión específica de Node.js instalada:
nvsh uninstall 18.20.0
```

*(Para desinstalar la herramienta `nvsh` por completo de tu sistema, ejecuta `nvsh self-uninstall` o consulta la sección [Desinstalación Completa](#-desinstalación-completa-100-limpia)).*

### 7. Comprobar firma y suma de comprobación (SHA-256 / GPG)

`nvsh` verifica **automáticamente** la suma SHA-256 contra el archivo oficial `SHASUMS256.txt` de nodejs.org antes de descomprimir cualquier binario. Si el hash no coincide o el archivo está corrupto, la instalación se aborta inmediatamente.

Además, puedes auditar manualmente el hash y la firma GPG desprendida (`SHASUMS256.txt.sig`) con:

```bash
nvsh verify 20.18.0
```
- Comprueba que el archivo descargado coincide bit a bit con el hash oficial del release team de Node.js.
- Si tienes `gpg` instalado, verifica la firma criptográfica PGP desprendida del release de Node.js.

### 8. Regenerar Shims

Si instalas herramientas globales con `npm install -g pnpm yarn typescript nodemon`:
```bash
nvsh rehash
```
`nvsh` detectará automáticamente los nuevos binarios en todas las versiones instaladas y creará sus correspondientes shims en `~/.nvsh/shims/` **sin usar symlinks**.

---

## 🗑️ Desinstalación Completa (100% Limpia)

Si deseas desinstalar `nvsh`, eliminar sus shims, versiones descargadas y restaurar la configuración original de tu terminal y variables de entorno:

### En Linux y macOS (Bash / Zsh / Fish):

**Desinstalación rápida en una sola línea (One-Liner):**
```bash
curl -fsSL https://raw.githubusercontent.com/caco123/node-version-shifter/master/uninstall.sh | bash
```

**Desde la carpeta del repositorio:**
```bash
./uninstall.sh

# O forzar desinstalación directa sin confirmación:
./uninstall.sh -y

# Conservar versiones descargadas de Node.js eliminando únicamente nvsh y shims:
./uninstall.sh --keep-versions
```
*(O también mediante `make uninstall`).*

---

### En Windows (PowerShell / CMD):

**Desinstalación rápida en una sola línea (One-Liner):**
```powershell
powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/caco123/node-version-shifter/master/uninstall.ps1 | iex"
```

**Desde la carpeta del repositorio:**
```cmd
# Opción más rápida y sencilla (evita bloqueos de directiva de ejecución):
.\uninstall.cmd
```

O invocando PowerShell con Bypass:
```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1

# O forzar desinstalación directa sin confirmación:
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1 -Force

# Conservar versiones descargadas de Node.js:
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1 -KeepVersions
```

---

### Directamente desde la CLI de `nvsh` (Cualquier SO):
```bash
nvsh self-uninstall
# o también:
nvsh uninstall --self
```

**¿Qué hace el desinstalador?**
- Elimina `~/.nvsh` (binarios, shims, cache y archivos de configuración).
- Limpia las líneas añadidas en tu perfil de shell (`~/.bashrc`, `~/.bash_profile`, `~/.zshrc`, `~/.profile` o `~/.config/fish/config.fish`).
- En Windows, elimina las rutas añadidas a la variable de entorno `Path` de usuario en el Registro y las variables `NVSH_DIR` y `NODE_HOME` sin requerir elevación ni permisos de administrador.

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
