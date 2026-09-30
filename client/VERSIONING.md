# Versioning

Current version: **0.7.11**

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
"productVersion": "0.7.11"
```

Then build and commit:

```
build.bat
git add wails.json
git commit -m "Bump version to 0.7.11"
```

## Changelog

| Version | Type    | Description                                                  |
|---------|---------|--------------------------------------------------------------|
| 0.7.11  | patch   | One-time session configs: the server adds a per-session PresharedKey (memory only) and ends sessions of disabled/deleted users and removed server access; the service removes session tunnels whose session is over (app killed), the app removes leftovers at start and ends sessions before quitting |
| 0.7.10  | patch   | Server address can be entered without https:// (e.g. vpn.example.com); it is added automatically in setup and Settings |
| 0.7.9   | patch   | Password-manager autofill for OpenVPN sign-in: the window title carries the profile's autofill name while its connect form is open (RoboForm matches `exe://ProIdentity Access/*name*`); admin can set a separate autofill name per server profile, imported profiles use their name; standard username/password field attributes; Enter after the password moves to the TOTP field |
| 0.7.8   | patch   | Connected VPNs: the main screen shows OpenVPN connections too, plus a "Connected now" list (IP, routed networks, server, duration, traffic, Disconnect); two connections that use the same network are refused with a message naming the one in the way |
| 0.7.7   | patch   | Fix OpenVPN stuck on Connecting: management commands are sent one at a time (pipelined commands, including the password, were dropped by openvpn) |
| 0.7.6   | patch   | OpenVPN: names the unreachable server and port (shown after 20 s while retrying); management interface protected with a per-session password |
| 0.7.5   | patch   | OpenVPN: clear error instead of endless Connecting (wrong password, unreachable server, certificate, adapter, unsupported option), unanswerable prompts reported, 90 s connect timeout, per-connection logs |
| 0.7.4   | patch   | The service checks for updates every 10 minutes and shows the prompt to signed-in users (opening the app if needed); the app reopens after an update; Later is remembered per user |
| 0.7.3   | patch   | OpenVPN bundled in the installers (official 2.7.7 on Windows with ovpn-dco and TAP drivers; self-contained build on macOS); no separate OpenVPN install needed |
| 0.7.2   | patch   | Version aligned with the 0.7.2 mobile apps (Android/iOS redesign, VPN notification); no desktop changes since 0.7.0 |
| 0.7.0   | minor   | OpenVPN profiles (import or assigned by admin, TUN/TAP), soft re-login after session expiry, longer sessions, security hardening |
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

## Publishing a client update

Installed clients update themselves: the ProIdentity **service** (LocalSystem on
Windows, root on macOS) downloads the installer from the organization's server,
verifies it and installs it, so users without admin rights can update. The service
checks every 10 minutes and shows the prompt to every signed-in user, opening the
app if it isn't running; Later postpones for 24 hours (not for mandatory updates).

Every update is signed with the **release signing key**. The service installs
only packages whose manifest is signed with that key, built for its platform,
and newer than the installed version.

- Private key: `%USERPROFILE%\.proidentity\update-signing.key` (or `$PROIDENTITY_UPDATE_KEY`).
  Keep a backup in the password manager. If it is lost, existing installs can't
  verify new updates and must be updated by hand once.
- Public key: `internal/update/manifest.go` → `ReleasePublicKey`.

**Windows.** `build.ps1` builds the MSI and publishes it automatically.
The build fails if the signing key is missing.

**macOS.** Build the `.pkg` on the Mac (`build.sh`), copy it to this machine, then:

```
.\publish-update.ps1 -File ProIdentity-Access-0.7.3.pkg -Platform darwin-arm64
```

Options: `-Mandatory` (users can't postpone), `-Notes "..."` (shown in the prompt).

The feed lives in `server/internal/api/client_updates/<platform>/` and is embedded
in the server binary: **rebuild and deploy the server** to release the update.
Clients up to 0.7.0 only show a notice; from 0.7.2 on, updates install themselves.
