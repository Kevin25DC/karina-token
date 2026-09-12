---
name: release
description: Compila Karina, arma el paquete portable en dist/Karina y genera el zip de release (dist/Karina-Windows-vX.Y.Z.zip). Usar cuando el usuario pida "compilar y empaquetar", "hacer el punto/zip de esta versión", "sacar un release" o similar.
---

# Release de Karina (Windows)

Flujo para publicar una nueva versión local de Karina como paquete portable
+ zip, sin tocar GitHub Releases (eso lo hace el usuario si lo pide aparte).

## 0. Decidir el número de versión

Si el usuario no da un número explícito, **pregúntale** antes de tocar nada
(no asumas). Sigue semver simple del proyecto (ver `CHANGELOG.md` para el
historial): una feature nueva sube el *minor* (0.3.0 → 0.4.0), un fix/ajuste
menor sube el *patch* (0.4.0 → 0.4.1).

## 1. Actualizar la versión en el código

Tres archivos, todos con el mismo número `X.Y.Z`:

- `main.go` → constante `appVersion`.
- `wails.json` → campo `info.productVersion`.
- `CHANGELOG.md` → nueva sección `## [X.Y.Z] - YYYY-MM-DD` arriba de la
  anterior, con un resumen breve de lo añadido/corregido en esta versión
  (mirar `git log` desde el último tag/release si hace falta contexto).

## 2. Compilar

Requiere Go 1.24.x en PATH (ver `AGENTS.md` § *Entorno de esta máquina* si
`go`/`wails` no se encuentran):

```powershell
$env:GOROOT="C:\Users\khernandez\sdk\go"
$env:Path="C:\Users\khernandez\go\bin;C:\Users\khernandez\sdk\go\bin;"+$env:Path
$env:GOTOOLCHAIN="local"
go vet ./...
wails build -skipbindings
```

Genera `build\bin\Karina.exe`. Si `go vet` marca algo, arréglalo antes de
seguir.

## 3. Armar el paquete portable

```powershell
Copy-Item "build\bin\Karina.exe" "dist\Karina\Karina.exe" -Force
```

Actualizar el número de versión en dos archivos dentro de `dist\Karina\`
(buscar el número de versión anterior y reemplazarlo):

- `dist\Karina\LEEME.txt` → línea `Version X.Y.Z (Windows)`.
- `dist\Karina\Manual-Usuario-Karina.html` → `<footer>` con `vX.Y.Z`.

Verificar que no quedó ningún rastro de la versión anterior:

```powershell
Select-String -Path "dist\Karina\*" -Pattern "<versión anterior>"
```

(no debería imprimir nada).

`dist\Karina\Instalar-Karina.cmd`, `Desinstalar-Karina.cmd` y `logo.png` no
cambian entre versiones — no tocarlos salvo que el usuario pida modificar el
instalador.

## 4. Generar el zip

```powershell
Compress-Archive -Path "dist\Karina\*" -DestinationPath "dist\Karina-Windows-vX.Y.Z.zip" -Force
```

El zip final queda en `dist\Karina-Windows-vX.Y.Z.zip`. Los zips de
versiones anteriores en `dist\` se dejan como están (no se borran).

## 5. Qué NO hacer sin que lo pidan explícitamente

- No publicar el Release en GitHub (`gh release create` + subir el zip) a
  menos que el usuario lo pida — eso lo cubre `AGENTS.md` § *Flujo de
  trabajo git / releases* pero es un paso aparte, visible públicamente.
- No hacer `git commit`/`git push` de los cambios de versión salvo que se
  pida explícitamente.
- `dist/` y `build/` están en `.gitignore`: el zip y el exe nunca se
  commitean, solo los cambios de versión en `main.go`, `wails.json` y
  `CHANGELOG.md`.

## 6. Resumen al usuario

Al terminar, decir claramente: la versión compilada, la ruta del zip
generado y su tamaño, y si quedó algo pendiente (p. ej. commitear los
cambios de versión, o publicar el Release en GitHub).
