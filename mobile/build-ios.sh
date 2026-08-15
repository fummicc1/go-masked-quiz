#!/bin/bash
# Builds and installs the app on a physical iPhone.
#
# gogio is not used for iOS here. It matches provisioning profiles by exact app
# id, so a team wildcard profile ("TEAMID.*") is rejected even though Xcode
# would accept it. Doing the four steps by hand — compile, assemble, sign,
# install — works with the wildcard profile and needs no new App ID registered.
#
# Prerequisites:
#   - the device is registered in the profile and has Developer Mode enabled
#   - an "Apple Development" signing identity is in the keychain
#
# Usage:
#   ./build-ios.sh                 # build, install and launch
#   ./build-ios.sh build           # build only
set -euo pipefail

# Build the module this script lives in, not whatever directory it was invoked
# from.
cd "$(dirname "${BASH_SOURCE[0]}")"

APP_ID="${APP_ID:-dev.fummicc1.go-masked-quiz}"
APP_NAME="${APP_NAME:-GoMaskedQuiz}"
MIN_IOS="${MIN_IOS:-17.0}"
OUT="${OUT:-$(mktemp -d)/${APP_NAME}.app}"

die() { echo "error: $*" >&2; exit 1; }

# A wildcard profile signs any bundle id under its team, which is why no App ID
# has to be registered for this app specifically.
find_profile() {
  local dir="$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles"
  local f appid
  for f in "$dir"/*.mobileprovision; do
    [ -e "$f" ] || continue
    appid=$(security cms -D -i "$f" 2>/dev/null | plutil -extract Entitlements.application-identifier raw - 2>/dev/null) || continue
    case "$appid" in
      *'.*') echo "$f"; return 0 ;;                    # wildcard
      *".$APP_ID") echo "$f"; return 0 ;;              # exact match
    esac
  done
  return 1
}

PROFILE="${PROFILE:-$(find_profile)}" || die "no provisioning profile covers $APP_ID"
echo "profile:  $PROFILE"

IDENTITY="${IDENTITY:-$(security find-identity -v -p codesigning | grep "Apple Development" | head -1 | awk '{print $2}')}"
[ -n "$IDENTITY" ] || die "no Apple Development signing identity in the keychain"
echo "identity: $IDENTITY"

SDK=$(xcrun --sdk iphoneos --show-sdk-path)
CLANG=$(xcrun --sdk iphoneos --find clang)

echo "==> compiling for ios/arm64"
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC="$CLANG" \
  CGO_CFLAGS="-isysroot $SDK -miphoneos-version-min=$MIN_IOS -arch arm64" \
  CGO_LDFLAGS="-isysroot $SDK -miphoneos-version-min=$MIN_IOS -arch arm64" \
  go build -tags ios -o "$OUT.bin" .

echo "==> assembling $OUT"
rm -rf "$OUT" && mkdir -p "$OUT"
mv "$OUT.bin" "$OUT/$APP_NAME"
cp "$PROFILE" "$OUT/embedded.mobileprovision"

cat > "$OUT/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key><string>en</string>
  <key>CFBundleExecutable</key><string>$APP_NAME</string>
  <key>CFBundleIdentifier</key><string>$APP_ID</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key><string>$APP_NAME</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>1.0</string>
  <key>CFBundleVersion</key><string>1</string>
  <key>LSRequiresIPhoneOS</key><true/>
  <key>MinimumOSVersion</key><string>$MIN_IOS</string>
  <key>UIDeviceFamily</key><array><integer>1</integer></array>
  <key>UILaunchScreen</key><dict/>
  <key>UISupportedInterfaceOrientations</key>
  <array><string>UIInterfaceOrientationPortrait</string></array>
  <key>CFBundleSupportedPlatforms</key><array><string>iPhoneOS</string></array>
</dict>
</plist>
PLIST

# The profile's entitlements still carry the wildcard; the signature must carry
# the concrete id, which is the substitution Xcode performs too.
TEAM=$(security cms -D -i "$PROFILE" | plutil -extract Entitlements.com\\.apple\\.developer\\.team-identifier raw -)
cat > "$OUT.entitlements" <<ENT
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>application-identifier</key><string>$TEAM.$APP_ID</string>
  <key>com.apple.developer.team-identifier</key><string>$TEAM</string>
  <key>get-task-allow</key><true/>
  <key>keychain-access-groups</key><array><string>$TEAM.$APP_ID</string></array>
</dict>
</plist>
ENT

echo "==> signing"
codesign --force --sign "$IDENTITY" --entitlements "$OUT.entitlements" --timestamp=none "$OUT"
codesign --verify --verbose=1 "$OUT"

if [ "${1:-run}" = "build" ]; then
  echo "built: $OUT"
  exit 0
fi

# Columns in `devicectl list devices` are variable width, so pick the device by
# the shape of its identifier rather than by position.
DEVICE="${DEVICE:-$(xcrun devicectl list devices 2>/dev/null | grep physical | grep -oE '[0-9A-F]{8}(-[0-9A-F]{4}){3}-[0-9A-F]{12}' | head -1)}"
[ -n "$DEVICE" ] || die "no physical device found; connect one or set DEVICE=<identifier>"
echo "==> installing on $DEVICE"
xcrun devicectl device install app --device "$DEVICE" "$OUT"
xcrun devicectl device process launch --device "$DEVICE" "$APP_ID"
