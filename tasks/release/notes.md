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
