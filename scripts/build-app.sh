#!/usr/bin/env bash
# Builds Givens.app: the SwiftUI app, with the givens command in Contents/Helpers, where the
# app starts `givens serve` from. Not in Contents/MacOS: on a case-insensitive disk "givens"
# would overwrite the app's own "Givens".
# givens is built for arm64 and amd64 and joined with lipo, so one bundle runs on both.
#
#   scripts/build-app.sh [version]     # writes build/Givens.app
#
# Signing and notarizing are scripts/sign-app.sh's.
# Ref: https://developer.apple.com/documentation/apple-silicon/building-a-universal-macos-binary
# Ref: https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

version="${1:-0.1.0}"
app=build/Givens.app
work=build/work
rm -rf "$app" "$work"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Helpers" "$app/Contents/Resources" "$work"

# The Swift app, for both architectures. Ref: https://www.swift.org/documentation/server/guides/building.html
swift build --package-path macos --configuration release --product Givens \
  --arch arm64 --arch x86_64
cp "$(swift build --package-path macos --configuration release --arch arm64 --arch x86_64 \
  --show-bin-path)/Givens" "$app/Contents/MacOS/Givens"

# The givens command. CGO is off so each build is a static Go binary.
# Ref: https://pkg.go.dev/cmd/go#hdr-Environment_variables
for arch in arm64 amd64; do
  CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build -trimpath -o "$work/givens-$arch" \
    ./cmd/givens
done
lipo -create -output "$app/Contents/Helpers/givens" "$work/givens-arm64" "$work/givens-amd64"

sed "s/VERSION/$version/g" macos/Bundle/Info.plist > "$app/Contents/Info.plist"
plutil -lint "$app/Contents/Info.plist" >/dev/null
rm -rf "$work"
echo "built $app ($version)"
