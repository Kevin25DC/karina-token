#!/usr/bin/env bash
# Karina - compila y empaqueta los binarios de distribución (desde un Mac).
# Uso:  scripts/build.sh [mac|windows|all]      (por defecto: all)
#
# Salida (dist/ está en .gitignore; los binarios no se commitean):
#   dist/macos/Karina.app      + dist/Karina-macOS-vX.Y.Z.zip
#   dist/windows/Karina.exe    + dist/Karina-Windows-vX.Y.Z.zip
set -euo pipefail

cd "$(dirname "$0")/.."

target="${1:-all}"
case "$target" in
  mac|windows|all) ;;
  *) echo "Uso: scripts/build.sh [mac|windows|all]" >&2; exit 1 ;;
esac

# Wails v2 falla con Go >= 1.25 (ver AGENTS.md): fijar 1.24.x, que `go`
# descarga solo si no es el instalado.
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.24.4}"

# La CLI de Wails vive en GOPATH/bin, que puede no estar en el PATH.
PATH="$(go env GOPATH)/bin:$PATH"
if ! command -v wails >/dev/null; then
  echo "Falta la CLI de Wails: go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2" >&2
  exit 1
fi

version="$(sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)"
echo "==> Karina v$version ($target)"

build_mac() {
  echo "==> macOS (universal: Intel + Apple Silicon)"
  wails build -clean -skipbindings -platform darwin/universal
  rm -rf dist/macos
  mkdir -p dist/macos
  cp -R build/bin/Karina.app dist/macos/
  # Firma ad-hoc: sin ninguna firma, Apple Silicon se niega a ejecutar la app.
  codesign --force --deep -s - dist/macos/Karina.app
  # ditto conserva la estructura del bundle .app (zip normal la rompe).
  rm -f "dist/Karina-macOS-v$version.zip"
  ditto -c -k --keepParent dist/macos/Karina.app "dist/Karina-macOS-v$version.zip"
}

build_windows() {
  echo "==> Windows (amd64)"
  wails build -clean -skipbindings -platform windows/amd64
  rm -rf dist/windows
  mkdir -p dist/windows
  cp build/bin/Karina.exe dist/windows/
  # Karina.exe en la raíz del zip: es lo que espera scripts/install.ps1.
  rm -f "dist/Karina-Windows-v$version.zip"
  (cd dist/windows && zip -q "../Karina-Windows-v$version.zip" Karina.exe)
}

[[ "$target" == "mac" || "$target" == "all" ]] && build_mac
[[ "$target" == "windows" || "$target" == "all" ]] && build_windows

echo "==> Listo:"
ls -lh dist/*.zip
