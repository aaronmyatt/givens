# Release

## Goal
One signed, notarized Givens.app a developer can download, with the `givens` command inside it.

## Design
- Build: `scripts/build-app.sh` builds `cmd/givens` for arm64 and amd64, joins them with
  `lipo`, and puts the result in `Givens.app/Contents/Resources/givens`. The Swift app starts
  `givens serve` from there. Ref: https://developer.apple.com/documentation/apple-silicon/building-a-universal-macos-binary
- Info.plist declares folders as a document type, so Dock drop and `open -a Givens <folder>`
  reach the app. Ref: https://developer.apple.com/documentation/bundleresources/information-property-list/cfbundledocumenttypes
- Sign with a Developer ID and the hardened runtime, then `notarytool submit --wait` and
  `stapler staple`. Ref: https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution
- Install command: the behaviour is the "Install the givens command" scenario in Open a
  project. This line only checks it works from the signed bundle, not from a dev build.
- Signing secrets come from the environment, never from the repo.

## As built
- Build (2026-10-11): `scripts/build-app.sh [version]` writes `build/Givens.app` (gitignored):
  the Swift app for arm64 and x86_64 in Contents/MacOS/Givens, givens joined with lipo in
  Contents/Helpers/givens, and `macos/Bundle/Info.plist` with folders as a document type.
  - givens isn't in Contents/MacOS: on a case-insensitive disk "givens" overwrote "Givens".
    The app now looks in Contents/Helpers (bundle) or Helpers/ beside its binary (dev build);
    `scripts/run-app.sh` builds that layout too.
  - Checked without opening a window: both binaries universal, `givens check` and `givens
    serve` run from the bundle, an ad-hoc signature verifies.
  - Decided: bundle id `dev.givens.Givens`. Change it in Info.plist if you sign under your
    own domain; nothing else depends on it.
- Sign and notarize: `scripts/sign-app.sh` (hardened runtime, timestamp, notarytool with a
  keychain profile, staple, spctl). Not run: it needs a Developer ID identity and a notary
  profile, which only you have.
- Install command: from the ad-hoc-signed bundle, serve's installCommand links
  Contents/Helpers/givens into a folder and `givens check` works through the link. Re-check
  once the bundle is signed for real.
- Known gap: the app menu installs into /usr/local/bin, which may not exist or be writable on
  Apple silicon Macs. Added to todo.txt.