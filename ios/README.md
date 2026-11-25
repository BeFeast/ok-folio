# OK Folio — native iOS app

Operator runbook for building the OK Folio iOS client from source and
installing it on an iPhone. Everything runs on a Mac with Xcode; no Homebrew,
no CI, no account provisioning beyond a signed-in Apple ID. The build tooling
(XcodeGen) is auto-downloaded into `ios/.tools/` by the build script.

This is a personal, development-signed app for a self-hosted OK Folio
instance on a private network. It is not distributed through the App Store.

Layout:

- `Core/` — SwiftPM package `FolioKit` (typed models + API client for
  `/api/v1`; `swift test` runs anywhere, including Linux — see
  `api/openapi.yaml` for the contract).
- `App/Sources/` — SwiftUI app target `OKFolio`
  (bundle id `com.befeast.okfolio`, iOS 17.0+).
- `Share/Sources/` + `Share/Info.plist` — share-extension target
  `OKFolioShare` (bundle id `com.befeast.okfolio.share`), embedded into the
  app automatically by XcodeGen.
- `project.yml` — XcodeGen spec; `OKFolio.xcodeproj` is generated, never
  committed. The `.entitlements` files for both targets are also written by
  `xcodegen generate` — do not create or edit them by hand.
- `scripts/m4-build-install.sh` — the operator build/install script.

## Prerequisites

- macOS with **Xcode 26.x** installed and the license accepted
  (`sudo xcodebuild -license accept`), iOS platform SDK present
  (`xcodebuild -downloadPlatform iOS` if missing — one-time, ~8.5 GB).
- Apple ID signed in under Xcode > Settings > Accounts, member of a **paid**
  Apple Developer team. Note the ten-character team id
  (developer.apple.com > Account > Membership).
- iPhone running **iOS 17 or newer** (older iOS is invisible to `devicectl`)
  with **Developer Mode** enabled
  (Settings > Privacy & Security > Developer Mode, then reboot).
- One-time trust: connect the iPhone via USB, unlock it, tap
  **Trust This Computer**. After the first install, trust the developer
  profile on the phone (Settings > General > VPN & Device Management).

## Build and install (happy path)

```sh
export FOLIO_TEAM_ID=XXXXXXXXXX      # paid team id
export FOLIO_DEVICE_ID=<device-uuid> # from: xcrun devicectl list devices
./ios/scripts/m4-build-install.sh
```

The script is idempotent and does, in order: FolioKit `swift test` (skip with
`--skip-tests`), ensure/auto-download XcodeGen, `xcodegen generate`, a signed
Release `xcodebuild` for iOS, then `devicectl` install and launch on
`$FOLIO_DEVICE_ID`. Set `APP_ONLY=1` to stop after the build. Without
`FOLIO_DEVICE_ID` it lists known devices and tells you which column to copy.

First build against a new device auto-registers its UDID in the developer
portal (`-allowProvisioningDeviceRegistration`); no manual portal work.

## Configuration

The server base URL is entered in the app's Settings screen (e.g. the LAN
address of your OK Folio instance). Nothing is hardcoded in the repo. An
optional Face ID lock can be enabled in the same screen.

## Share extension

`OKFolioShare` adds OK Folio to the system share sheet: share one or more
images (up to 20) from Photos, Safari, etc. and they upload straight to the
server, with per-image progress and duplicate detection ("Already in Folio").

- The extension reads the server URL from the App Group
  `group.com.befeast.okfolio`, shared with the main app. Open the app and set
  the server address in Settings once before first use; until then the
  extension shows "Set the server address in OK Folio first".
- Automatic signing provisions the app group on first build — the extension
  bundle id (`com.befeast.okfolio.share`) and the group are derived from the
  app's, so no manual developer-portal work is expected.
- After install, the extension appears in the share sheet under **OK Folio**.
  If it is not visible, scroll the share sheet's app row to the end, tap
  **More**, and enable it once.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `xcodebuild` unusable | `sudo xcodebuild -license accept` |
| `iOS ... is not installed` | `xcodebuild -downloadPlatform iOS` |
| `Your team has no devices...` | export `FOLIO_DEVICE_ID` (or `FOLIO_DEVICE_UDID`) so the build runs against the concrete device and registers it |
| Install ok, launch fails | Trust the developer profile on the phone (Settings > General > VPN & Device Management), launch once from the home screen |
| Device not in `devicectl list devices` | USB-connect + unlock + Trust This Computer; check iOS >= 17 |
| Annual profile expiry | Just rerun the script (it re-signs and reinstalls) |
