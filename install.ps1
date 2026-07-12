# ============================================================
#  Liz v3.0 — Instalador PowerShell
#  Descarga liz.exe + hola.exe nativos y los instala.
#
#  Uso:
#    powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/caos1codex-hash/liz/main/install.ps1 | iex"
# ============================================================

$ErrorActionPreference = "Stop"

$PURPLE = "Magenta"
$PINK   = "Cyan"

Write-Host ""
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host "    Liz v3.0 - Instalador" -ForegroundColor $PURPLE
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host ""

# ============================================================
#  1. Definir rutas
# ============================================================
$INSTALL_DIR = "$env:LOCALAPPDATA\Liz"
Write-Host "  [1/4] Directorio: $INSTALL_DIR" -ForegroundColor Yellow
if (-not (Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
}

# ============================================================
#  2. Descargar binarios
# ============================================================
Write-Host ""
Write-Host "  [2/4] Descargando binarios nativos..." -ForegroundColor Yellow

$RELEASE_URL = "https://github.com/caos1codex-hash/liz/releases/download/v3.0.0"

$ProgressPreference = "SilentlyContinue"

# Descargar liz.exe
$lizUrl = "$RELEASE_URL/liz.exe"
$lizDest = Join-Path $INSTALL_DIR "liz.exe"
Write-Host "    liz.exe  (5.5 MB) ..." -NoNewline
try {
    Invoke-WebRequest -Uri $lizUrl -OutFile $lizDest -UseBasicParsing -TimeoutSec 60
    $size = (Get-Item $lizDest).Length
    Write-Host (" OK ({0:N1} MB)" -f ($size/1MB)) -ForegroundColor Green
} catch {
    Write-Host " FALLO" -ForegroundColor Red
    Write-Host "      Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Descargar hola.exe
$holaUrl = "$RELEASE_URL/hola.exe"
$holaDest = Join-Path $INSTALL_DIR "hola.exe"
Write-Host "    hola.exe (1.7 MB) ..." -NoNewline
try {
    Invoke-WebRequest -Uri $holaUrl -OutFile $holaDest -UseBasicParsing -TimeoutSec 60
    $size = (Get-Item $holaDest).Length
    Write-Host (" OK ({0:N1} MB)" -f ($size/1MB)) -ForegroundColor Green
} catch {
    Write-Host " FALLO" -ForegroundColor Red
    Write-Host "      Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# ============================================================
#  3. Desbloquear archivos (Mark of the Web)
# ============================================================
Write-Host ""
Write-Host "  [3/4] Desbloqueando archivos..." -ForegroundColor Yellow
Unblock-File -Path $lizDest -ErrorAction SilentlyContinue
Unblock-File -Path $holaDest -ErrorAction SilentlyContinue
Write-Host "    OK" -ForegroundColor Green

# ============================================================
#  4. Agregar al PATH
# ============================================================
Write-Host ""
Write-Host "  [4/4] Anadiendo al PATH..." -ForegroundColor Yellow

$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if (-not $userPath) { $userPath = "" }

if ($userPath -split ";" -contains $INSTALL_DIR) {
    Write-Host "    OK ya estaba en PATH" -ForegroundColor Green
} else {
    $newPath = if ($userPath) { "$INSTALL_DIR;$userPath" } else { $INSTALL_DIR }
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    $env:PATH = "$INSTALL_DIR;$env:PATH"
    Write-Host "    OK anadido al PATH" -ForegroundColor Green
}

# ============================================================
#  Verificacion final
# ============================================================
Write-Host ""
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host "    Instalacion completada!" -ForegroundColor Green
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host ""
Write-Host "  Archivos:" -ForegroundColor White
Get-ChildItem $INSTALL_DIR -Filter "*.exe" | ForEach-Object {
    $sizeMB = "{0:N1}" -f ($_.Length / 1MB)
    Write-Host "    $($_.Name)  ($sizeMB MB)" -ForegroundColor Gray
}
Write-Host ""
Write-Host "  Para empezar:" -ForegroundColor White
Write-Host "    1. CIERRA esta ventana de PowerShell" -ForegroundColor $PINK
Write-Host "    2. Abre una NUEVA terminal (PowerShell o CMD)" -ForegroundColor $PINK
Write-Host "    3. Escribe:" -ForegroundColor $PINK
Write-Host ""
Write-Host "       hola liz" -ForegroundColor $PURPLE
Write-Host ""

Set-Content -Path (Join-Path $INSTALL_DIR "install-success.txt") -Value "Installed on $(Get-Date)" -Encoding UTF8
