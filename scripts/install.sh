#!/usr/bin/env bash
# Karina - Instalador remoto (macOS)
# Uso:  curl -fsSL https://raw.githubusercontent.com/Kevin25DC/karina-token/main/scripts/install.sh | bash
#
# Descarga el zip de macOS de la última Release, copia Karina.app a
# /Applications (o a ~/Applications si no hay permiso) y la abre.
# En Windows se usa scripts/install.ps1.
set -euo pipefail

repo="Kevin25DC/karina-token"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "Este instalador es para macOS." >&2
  echo "En Windows, desde PowerShell:" >&2
  echo "  irm https://raw.githubusercontent.com/$repo/main/scripts/install.ps1 | iex" >&2
  exit 1
fi

echo
echo "============================================"
echo " Karina - instalando desde GitHub Releases"
echo "============================================"
echo

release="$(curl -fsSL -H 'User-Agent: Karina-Installer' \
  "https://api.github.com/repos/$repo/releases/latest")" || {
  echo "No se pudo consultar la ultima version en GitHub." >&2
  exit 1
}

# Las releases traen un zip por plataforma (Karina-macOS-vX.Y.Z.zip).
url="$(printf '%s' "$release" \
  | grep -o '"browser_download_url": *"[^"]*"' \
  | sed 's/.*"\(https[^"]*\)"/\1/' \
  | grep -i 'macos.*\.zip$' \
  | head -n 1 || true)"
tag="$(printf '%s' "$release" \
  | grep -o '"tag_name": *"[^"]*"' \
  | head -n 1 \
  | sed 's/.*"\([^"]*\)"/\1/' || true)"

if [[ -z "$url" ]]; then
  echo "La ultima release no contiene un zip para macOS." >&2
  exit 1
fi

echo "Version encontrada: ${tag:-desconocida}"
echo "Descargando $(basename "$url") ..."

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fL --progress-bar -o "$tmp/karina.zip" "$url"

# ditto conserva la estructura del bundle .app (unzip puede romperla).
ditto -x -k "$tmp/karina.zip" "$tmp/app"
if [[ ! -d "$tmp/app/Karina.app" ]]; then
  echo "El zip descargado no contiene Karina.app." >&2
  exit 1
fi

dest="/Applications"
if [[ ! -w "$dest" ]]; then
  dest="$HOME/Applications"
  mkdir -p "$dest"
fi

# Cerrar la app si esta abierta, para poder reemplazarla.
osascript -e 'quit app "Karina"' >/dev/null 2>&1 || true

rm -rf "$dest/Karina.app"
ditto "$tmp/app/Karina.app" "$dest/Karina.app"

echo
echo "Karina ${tag:-} instalada en:"
echo "  $dest/Karina.app"
echo "Abriendo Karina..."
open "$dest/Karina.app"
