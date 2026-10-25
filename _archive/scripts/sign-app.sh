#!/usr/bin/env bash
# Signs build/Givens.app with a Developer ID and the hardened runtime, notarizes it, and
# staples the ticket, so Gatekeeper opens it on any Mac. Run scripts/build-app.sh first.
#
#   GIVENS_SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)" \
#   GIVENS_NOTARY_PROFILE=givens-notary \
#   scripts/sign-app.sh
#
# GIVENS_NOTARY_PROFILE is a keychain profile made once with
#   xcrun notarytool store-credentials givens-notary --apple-id … --team-id …
# so no password is in this repo or in the environment.
# Ref: https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution
# Ref: https://developer.apple.com/documentation/security/hardened-runtime
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

app=build/Givens.app
identity="${GIVENS_SIGN_IDENTITY:?set GIVENS_SIGN_IDENTITY to your Developer ID Application identity}"
profile="${GIVENS_NOTARY_PROFILE:?set GIVENS_NOTARY_PROFILE to a notarytool keychain profile}"
[ -d "$app" ] || { echo "no $app: run scripts/build-app.sh first"; exit 1; }

# Inside out: the helper, then the app. --timestamp is required for notarization.
# Ref: https://developer.apple.com/documentation/security/resolving-common-notarization-issues
codesign --force --options runtime --timestamp --sign "$identity" "$app/Contents/Helpers/givens"
codesign --force --options runtime --timestamp --sign "$identity" "$app"
codesign --verify --strict --verbose=2 "$app"

zip=build/Givens.zip
rm -f "$zip"
ditto -c -k --keepParent "$app" "$zip"
xcrun notarytool submit "$zip" --keychain-profile "$profile" --wait
xcrun stapler staple "$app"
spctl --assess --type execute --verbose "$app"
echo "signed, notarized and stapled $app"
