# Versioning

Current version: **0.6.1**

Version is defined in `wails.json` Ă˘â€ â€™ `info.productVersion`.  
The build script (`build.ps1` / `build.bat`) reads it from there automatically.

## Bump rules

| Change type                                      | Part to bump | Example          |
|--------------------------------------------------|--------------|------------------|
| Bug fix, typo, minor tweak                       | patch +0.0.1 | 0.1.0 Ă˘â€ â€™ 0.1.1    |
| New feature, backward-compatible                 | minor +0.1.0 | 0.1.0 Ă˘â€ â€™ 0.2.0    |
| Breaking change (protocol, API, install layout)  | major +1.0.0 | 0.1.0 Ă˘â€ â€™ 1.0.0    |

## How to bump

Edit **one** line in `wails.json`:

```json
"productVersion": "0.6.1"
```

Then build and commit:

```
build.bat
git add wails.json
git commit -m "Bump version to 0.6.1"
```

## Changelog

| Version | Type    | Description                                                  |
|---------|---------|--------------------------------------------------------------|
| 0.6.1   | minor   | Harden IPC isolation, auth/session handling, firewall rules, release packaging, and push approval deduplication |
| 0.5.26  | patch   | Fix server endpoint migration on MariaDB production installs |
| 0.5.25  | patch   | Resolve WireGuard DNS endpoints and add endpoint failover    |
| 0.5.24  | patch   | Show cumulative desktop traffic totals instead of live rates |
| 0.5.22  | patch   | Sync local users into ProIdentity Push Auth provisioning     |
| 0.5.20  | patch   | Update free internal and company-use license                 |
| 0.5.19  | patch   | Refresh public license and installer metadata                |
| 0.2.0   | feature | Windows system tray with project icon and tunnel menu        |
| 0.2.0   | feature | Replace all app icons/logos with Pro Identity shield branding |
| 0.1.0   | feature | Initial Windows build: GUI app + daemon + MSI installer      |
