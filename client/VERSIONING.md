# Versioning

Current version: **0.5.19**

Version is defined in `wails.json` → `info.productVersion`.  
The build script (`build.ps1` / `build.bat`) reads it from there automatically.

## Bump rules

| Change type                                      | Part to bump | Example          |
|--------------------------------------------------|--------------|------------------|
| Bug fix, typo, minor tweak                       | patch +0.0.1 | 0.1.0 → 0.1.1    |
| New feature, backward-compatible                 | minor +0.1.0 | 0.1.0 → 0.2.0    |
| Breaking change (protocol, API, install layout)  | major +1.0.0 | 0.1.0 → 1.0.0    |

## How to bump

Edit **one** line in `wails.json`:

```json
"productVersion": "0.5.19"
```

Then build and commit:

```
build.bat
git add wails.json
git commit -m "Bump version to 0.5.19"
```

## Changelog

| Version | Type    | Description                                                  |
|---------|---------|--------------------------------------------------------------|
| 0.5.19  | patch   | Refresh public license and installer metadata                |
| 0.2.0   | feature | Windows system tray with project icon and tunnel menu        |
| 0.2.0   | feature | Replace all app icons/logos with Pro Identity shield branding |
| 0.1.0   | feature | Initial Windows build: GUI app + daemon + MSI installer      |
