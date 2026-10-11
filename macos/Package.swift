// swift-tools-version: 6.0
// The Givens macOS app: a SwiftUI shell over the Go core, which it drives through a
// `givens serve` process. See tasks/ui-shell/notes.md for why the shell is Swift.
// Ref: https://docs.swift.org/swiftpm/documentation/packagedescription
import PackageDescription

let package = Package(
    name: "Givens",
    // @Observable needs macOS 14. Ref: https://developer.apple.com/documentation/observation
    platforms: [.macOS(.v14)],
    products: [
        .executable(name: "Givens", targets: ["Givens"])
    ],
    targets: [
        // Runs Gherkin pickles against regex step definitions. No third-party dependency,
        // in keeping with CONSTRAINTS.md's zero-dependency rule.
        .target(name: "GherkinKit"),
        // The models every SwiftUI view renders, the `givens serve` client and the AppKit
        // services (clipboard, Trash, folder picker). Tests drive this, never the views.
        .target(name: "GivensUI"),
        // The app itself: windows, menus and the Dock-drop delegate.
        .executableTarget(name: "Givens", dependencies: ["GivensUI"]),
        .testTarget(name: "GherkinKitTests", dependencies: ["GherkinKit"]),
        // One suite per capability, over the same .feature files as acceptance/ in Go.
        .testTarget(
            name: "GivensAcceptanceTests", dependencies: ["GherkinKit", "GivensUI"]),
    ]
)
