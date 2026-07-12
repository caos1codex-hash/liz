# ============================================================
#  Liz v3.1 — Instalador PowerShell
#  Solo descarga hola.exe (5.1 MB). Claude Code y el proxy se
#  instalan automáticamente en la primera ejecución de `hola liz`.
# ============================================================

$ErrorActionPreference = "Stop"

$PURPLE = "Magenta"
$PINK   = "Cyan"

Write-Host ""
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host "    Liz v3.1 - Instalador" -ForegroundColor $PURPLE
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host ""

# ============================================================
#  1. Definir rutas
# ============================================================
$INSTALL_DIR = "$env:LOCALAPPDATA\Liz"
Write-Host "  [1/3] Directorio: $INSTALL_DIR" -ForegroundColor Yellow
if (-not (Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
}

# ============================================================
#  2. Descargar hola.exe con reintentos
# ============================================================
Write-Host ""
Write-Host "  [2/3] Descargando hola.exe..." -ForegroundColor Yellow

# URLs de descarga (intentamos varias en orden)
$urls = @(
    "https://github.com/caos1codex-hash/liz/releases/download/v3.1.0/hola.exe",
    "https://raw.githubusercontent.com/caos1codex-hash/liz/main/bin/hola.exe"
)

$holaDest = Join-Path $INSTALL_DIR "hola.exe"
$ProgressPreference = "SilentlyContinue"

$downloaded = $false
foreach ($url in $urls) {
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        Write-Host "    Intento $attempt de 3 - $url" -NoNewline
        try {
            # Usar .NET WebClient que es más estable que Invoke-WebRequest
            $wc = New-Object System.Net.WebClient
            $wc.Headers.Add("User-Agent", "LizInstaller/3.0")
            $wc.DownloadFile($url, $holaDest)
            $size = (Get-Item $holaDest).Length
            if ($size -gt 1000000) {
                Write-Host (" OK ({0:N1} MB)" -f ($size/1MB)) -ForegroundColor Green
                $downloaded = $true
                break
            } else {
                Write-Host " FALLO (archivo demasiado pequeno)" -ForegroundColor Red
                Remove-Item $holaDest -ErrorAction SilentlyContinue
            }
        } catch {
            Write-Host " FALLO" -ForegroundColor Red
            Write-Host "      $($_.Exception.Message)" -ForegroundColor Gray
        }
        Start-Sleep -Seconds 2
    }
    if ($downloaded) { break }
}

if (-not $downloaded) {
    Write-Host ""
    Write-Host "  No se pudo descargar automaticamente." -ForegroundColor Red
    Write-Host "  Descarga manual desde:" -ForegroundColor Yellow
    Write-Host "    https://github.com/caos1codex-hash/liz/releases/tag/v3.1.0" -ForegroundColor White
    Write-Host "  Y copia hola.exe a: $INSTALL_DIR" -ForegroundColor White
    Write-Host ""
    Write-Host "  Presiona una tecla para salir..." -ForegroundColor Gray
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit 1
}

# ============================================================
#  3. Desbloquear + PATH
# ============================================================
Write-Host ""
Write-Host "  [3/3] Configurando..." -ForegroundColor Yellow

# Desbloquear (quitar Mark of the Web)
Unblock-File -Path $holaDest -ErrorAction SilentlyContinue
Write-Host "    OK desbloqueado" -ForegroundColor Green

# Añadir al PATH
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if (-not $userPath) { $userPath = "" }

if ($userPath -split ";" -contains $INSTALL_DIR) {
    Write-Host "    OK PATH (ya estaba)" -ForegroundColor Green
} else {
    $newPath = if ($userPath) { "$INSTALL_DIR;$userPath" } else { $INSTALL_DIR }
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    $env:PATH = "$INSTALL_DIR;$env:PATH"
    Write-Host "    OK PATH anadido" -ForegroundColor Green
}

# ============================================================
#  Resumen final
# ============================================================
Write-Host ""
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host "    Instalacion completada!" -ForegroundColor Green
Write-Host "  ============================================" -ForegroundColor $PURPLE
Write-Host ""
Write-Host "  Archivo instalado:" -ForegroundColor White
Write-Host "    $holaDest" -ForegroundColor Gray
Write-Host ""
Write-Host "  Para empezar:" -ForegroundColor White
Write-Host "    1. CIERRA esta ventana de PowerShell" -ForegroundColor $PINK
Write-Host "    2. Abre una NUEVA terminal (PowerShell o CMD)" -ForegroundColor $PINK
Write-Host "    3. Escribe:" -ForegroundColor $PINK
Write-Host ""
Write-Host "       hola liz" -ForegroundColor $PURPLE
Write-Host ""
Write-Host "  La primera vez tardara un poco porque instalara:" -ForegroundColor Gray
Write-Host "    - claude-code-proxy (pip)" -ForegroundColor Gray
Write-Host "    - Claude Code (npm)" -ForegroundColor Gray
Write-Host ""
