#!/bin/bash
# Intended for ephemeral macOS CI runners, not a user's login keychain.
set -euo pipefail
APP="${1:?app path required}"
STATUS="${2:?status output required}"
if [[ -z "${MACOS_CERTIFICATE_P12:-}${MACOS_CERTIFICATE_PASSWORD:-}${MACOS_SIGNING_IDENTITY:-}${APPLE_API_KEY_P8:-}${APPLE_API_KEY_ID:-}${APPLE_API_ISSUER_ID:-}" ]]; then
  printf '%s\n' 'macOS: unsigned (no Developer ID signing or notarization).' > "$STATUS"
  exit 0
fi
: "${MACOS_CERTIFICATE_P12:?}" "${MACOS_CERTIFICATE_PASSWORD:?}" "${MACOS_SIGNING_IDENTITY:?}" "${APPLE_API_KEY_P8:?}" "${APPLE_API_KEY_ID:?}" "${APPLE_API_ISSUER_ID:?}"
[[ "${CI:-}" == "true" ]] || { echo 'Signing script requires an ephemeral CI runner.' >&2; exit 1; }
SIGN_DIR="$(mktemp -d)"
KEYCHAIN="$SIGN_DIR/signing.keychain-db"
KEYCHAIN_PASSWORD="$(openssl rand -hex 24)"
cleanup() { security delete-keychain "$KEYCHAIN" >/dev/null 2>&1 || true; rm -rf "$SIGN_DIR"; }
trap cleanup EXIT
umask 077
printf '%s' "$MACOS_CERTIFICATE_P12" | base64 --decode > "$SIGN_DIR/certificate.p12"
printf '%s' "$APPLE_API_KEY_P8" > "$SIGN_DIR/api.p8"
security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
security set-keychain-settings -lut 21600 "$KEYCHAIN"
security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN"
security import "$SIGN_DIR/certificate.p12" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign -t cert -f pkcs12 -k "$KEYCHAIN" >/dev/null
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN" >/dev/null
codesign --force --deep --options runtime --timestamp --keychain "$KEYCHAIN" --sign "$MACOS_SIGNING_IDENTITY" "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"
ditto -c -k --keepParent "$APP" "$SIGN_DIR/notarize.zip"
xcrun notarytool submit "$SIGN_DIR/notarize.zip" --key "$SIGN_DIR/api.p8" --key-id "$APPLE_API_KEY_ID" --issuer "$APPLE_API_ISSUER_ID" --wait --timeout 20m
xcrun stapler staple "$APP"
xcrun stapler validate "$APP"
spctl --assess --type execute --verbose=2 "$APP"
printf '%s\n' 'macOS: Developer ID signed, notarized and stapled; signature assessment passed.' > "$STATUS"
