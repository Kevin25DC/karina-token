# Karina - Instalador remoto (Windows)
# Uso:  irm https://raw.githubusercontent.com/Kevin25DC/karina-token/main/scripts/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$repo = 'Kevin25DC/karina-token'

Write-Host ''
Write-Host '============================================'
Write-Host ' Karina - instalando desde GitHub Releases'
Write-Host '============================================'
Write-Host ''

try {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'Karina-Installer' }
} catch {
    Write-Host "No se pudo consultar la ultima version en GitHub: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$asset = $rel.assets | Where-Object { $_.name -like '*.zip' } | Select-Object -First 1
if (-not $asset) {
    Write-Host 'La ultima release no contiene un instalador (.zip).' -ForegroundColor Red
    exit 1
}

Write-Host "Version encontrada: $($rel.tag_name)"
Write-Host "Descargando $($asset.name) ..."

$tmp = Join-Path $env:TEMP ("karina-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null | Out-Null
$zip = Join-Path $tmp $asset.name
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip

$extract = Join-Path $tmp 'app'
Expand-Archive -Path $zip -DestinationPath $extract -Force

$dest = Join-Path $env:LOCALAPPDATA 'Programs\Karina'
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item (Join-Path $extract 'Karina.exe') $dest -Force

# Accesos directos (Escritorio + Menu Inicio)
$shell = New-Object -ComObject WScript.Shell
foreach ($folder in @(([Environment]::GetFolderPath('Desktop')), ([Environment]::GetFolderPath('Programs')))) {
    $lnk = $shell.CreateShortcut((Join-Path $folder 'Karina.lnk'))
    $lnk.TargetPath = (Join-Path $dest 'Karina.exe')
    $lnk.WorkingDirectory = $dest
    $lnk.IconLocation = $lnk.TargetPath
    $lnk.Save()
}

Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue

Write-Host ''
Write-Host "Karina $($rel.tag_name) instalada en:"
Write-Host "  $dest"
Write-Host 'Abriendo Karina...'
Start-Process (Join-Path $dest 'Karina.exe')
