#!/usr/bin/env bash
#
# OK Folio iOS — build, sign, install, and launch on an iPhone/iPad.
#
# Run from the repo root or from ios/ (the script locates itself):
#
#   export FOLIO_TEAM_ID=XXXXXXXXXX      # paid Apple Developer team id (required)
#   export FOLIO_DEVICE_ID=<device-uuid> # from: xcrun devicectl list devices
#   ./ios/scripts/m4-build-install.sh
#
# Flags / env:
#   --skip-tests    skip the FolioKit `swift test` gate (not recommended)
#   APP_ONLY=1      build only; skip device install + launch
#   FOLIO_DEVICE_UDID  hardware UDID override (40-hex "Identifier" in Xcode's
#                      Devices window); derived from FOLIO_DEVICE_ID when unset
#
# Idempotent: safe to rerun any time (this is also the annual re-sign ritual).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IOS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

XCODEGEN_VERSION="2.43.0"
XCODEGEN_ZIP_URL="https://github.com/yonaskolb/XcodeGen/releases/download/${XCODEGEN_VERSION}/xcodegen.zip"
BUNDLE_ID="com.befeast.okfolio"

die()  { printf 'error: %s\n' "$*" >&2; exit 1; }
note() { printf '==> %s\n' "$*"; }

SKIP_TESTS=0
for arg in "$@"; do
  case "$arg" in
    --skip-tests) SKIP_TESTS=1 ;;
    -h|--help)
      sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) die "unknown argument: $arg (only --skip-tests is supported; see --help)" ;;
  esac
done

# --- 1. Toolchain sanity -----------------------------------------------------

command -v xcodebuild >/dev/null 2>&1 \
  || die "xcodebuild not found. Install Xcode from the App Store, then run:
  sudo xcode-select -s /Applications/Xcode.app"

if ! xcodebuild -version >/dev/null 2>&1; then
  die "xcodebuild exists but is not usable — usually an unaccepted Xcode license. Run:
  sudo xcodebuild -license accept
and rerun this script."
fi
note "Using $(xcodebuild -version | tr '\n' ' ')"

# --- 2. FolioKit unit tests (pure SwiftPM, no Xcode project needed) ----------

if [[ "$SKIP_TESTS" -eq 1 ]]; then
  note "Skipping FolioKit tests (--skip-tests)"
else
  note "Running FolioKit tests: swift test"
  ( cd "$IOS_DIR/Core" && swift test ) \
    || die "FolioKit tests failed. Fix them before installing, or rerun with --skip-tests to bypass (not recommended)."
fi

# --- 3. Ensure xcodegen ------------------------------------------------------

XCODEGEN_BIN=""
if command -v xcodegen >/dev/null 2>&1; then
  XCODEGEN_BIN="$(command -v xcodegen)"
elif [[ -x "$IOS_DIR/.tools/xcodegen/bin/xcodegen" ]]; then
  XCODEGEN_BIN="$IOS_DIR/.tools/xcodegen/bin/xcodegen"
else
  note "xcodegen not on PATH; downloading XcodeGen ${XCODEGEN_VERSION} into ios/.tools/ (no brew needed)"
  mkdir -p "$IOS_DIR/.tools"
  curl -fL --retry 3 -o "$IOS_DIR/.tools/xcodegen.zip" "$XCODEGEN_ZIP_URL" \
    || die "Could not download XcodeGen from:
  $XCODEGEN_ZIP_URL
Check network access, or install xcodegen yourself and put it on PATH, then rerun."
  unzip -oq "$IOS_DIR/.tools/xcodegen.zip" -d "$IOS_DIR/.tools" \
    || die "Could not unzip $IOS_DIR/.tools/xcodegen.zip — delete ios/.tools/ and rerun."
  rm -f "$IOS_DIR/.tools/xcodegen.zip"
  chmod +x "$IOS_DIR/.tools/xcodegen/bin/xcodegen"
  XCODEGEN_BIN="$IOS_DIR/.tools/xcodegen/bin/xcodegen"
fi
note "xcodegen: $XCODEGEN_BIN ($("$XCODEGEN_BIN" version 2>/dev/null || echo 'version unknown'))"

# --- 4. Generate the Xcode project ------------------------------------------

note "Generating OKFolio.xcodeproj from project.yml"
( cd "$IOS_DIR" && "$XCODEGEN_BIN" generate )

# --- 5. Signed Release build -------------------------------------------------

if [[ -z "${FOLIO_TEAM_ID:-}" ]]; then
  die "FOLIO_TEAM_ID is not set.
Export the ten-character Apple Developer team id of a team with a *paid*
Apple Developer Program membership (1-year provisioning; a free personal
team signs 7-day profiles and the app would die weekly):
  export FOLIO_TEAM_ID=XXXXXXXXXX
Find it at developer.apple.com > Account > Membership details, or in the
'listTeams' section of a previous xcodebuild -allowProvisioningUpdates log."
fi

# A generic destination cannot auto-register the iPhone in the developer
# portal. Building against the concrete device lets
# -allowProvisioningDeviceRegistration register its UDID on first build.
# FOLIO_DEVICE_UDID is the hardware UDID (the "Identifier" in Xcode's Devices
# window — NOT the dashed devicectl CoreDevice UUID); when unset we try to
# derive it from FOLIO_DEVICE_ID.
if [[ -z "${FOLIO_DEVICE_UDID:-}" && -n "${FOLIO_DEVICE_ID:-}" ]]; then
  FOLIO_DEVICE_UDID="$(xcrun devicectl device info details --device "$FOLIO_DEVICE_ID" 2>/dev/null \
    | awk -F': *' 'tolower($1) ~ /udid/ { print $2; exit }' || true)"
fi
if [[ -n "${FOLIO_DEVICE_UDID:-}" ]]; then
  DESTINATION="platform=iOS,id=$FOLIO_DEVICE_UDID"
  note "Building against device $FOLIO_DEVICE_UDID (registers it in the portal on first build)"
else
  DESTINATION="generic/platform=iOS"
  note "FOLIO_DEVICE_UDID/FOLIO_DEVICE_ID not set — generic destination; this FAILS on a team with no registered iOS devices"
fi

note "Building Release for iOS (team $FOLIO_TEAM_ID)"
( cd "$IOS_DIR" && xcodebuild \
    -project OKFolio.xcodeproj \
    -scheme OKFolio \
    -configuration Release \
    -destination "$DESTINATION" \
    -derivedDataPath .build-xcode \
    -allowProvisioningUpdates \
    -allowProvisioningDeviceRegistration \
    DEVELOPMENT_TEAM="$FOLIO_TEAM_ID" \
    FOLIO_TEAM_ID="$FOLIO_TEAM_ID" \
    build ) \
  || die "xcodebuild failed. Common causes:
  - Not signed in: Xcode > Settings > Accounts > add the Apple ID for team $FOLIO_TEAM_ID.
  - iOS SDK missing: xcodebuild -downloadPlatform iOS
  - Signing/provisioning: open ios/OKFolio.xcodeproj in Xcode once and
    check Signing & Capabilities for the concrete error."

# --- 6. Locate the built .app ------------------------------------------------

APP_PATH="$IOS_DIR/.build-xcode/Build/Products/Release-iphoneos/OKFolio.app"
if [[ ! -d "$APP_PATH" ]]; then
  die "Build succeeded but the app bundle is missing at:
  $APP_PATH
Look under $IOS_DIR/.build-xcode/Build/Products/ for where Xcode actually put it."
fi
note "Built app: $APP_PATH"

# --- 7. Install + launch on the iPhone ---------------------------------------

if [[ "${APP_ONLY:-0}" == "1" ]]; then
  note "APP_ONLY=1 — skipping device install/launch. Done."
  exit 0
fi

if [[ -z "${FOLIO_DEVICE_ID:-}" ]]; then
  echo "FOLIO_DEVICE_ID is not set. Known devices:" >&2
  xcrun devicectl list devices >&2 || true
  die "Pick the iPhone row above, copy its Identifier (UUID), then:
  export FOLIO_DEVICE_ID=<identifier>
and rerun. If no iPhone is listed: connect it via USB, unlock it, tap
'Trust This Computer', and enable Developer Mode
(Settings > Privacy & Security > Developer Mode, then reboot the phone).
To build without a device, rerun with APP_ONLY=1."
fi

note "Installing to device $FOLIO_DEVICE_ID"
xcrun devicectl device install app --device "$FOLIO_DEVICE_ID" "$APP_PATH" \
  || die "Install failed. Check that the iPhone is connected (USB or same-LAN
Wi-Fi debugging), unlocked, trusted, and running iOS 17+
(devicectl cannot talk to older iOS). Then rerun this script."

note "Launching $BUNDLE_ID"
xcrun devicectl device process launch --device "$FOLIO_DEVICE_ID" "$BUNDLE_ID" \
  || die "Install succeeded but launch failed. If this is the first install,
trust the developer profile on the phone (Settings > General >
VPN & Device Management), launch OK Folio once from the home screen,
then rerun. Otherwise retry:
  xcrun devicectl device process launch --device $FOLIO_DEVICE_ID $BUNDLE_ID"

note "Done: $BUNDLE_ID is installed and running on $FOLIO_DEVICE_ID."
