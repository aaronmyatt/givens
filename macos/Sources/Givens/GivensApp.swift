import AppKit
import GivensUI
import SwiftUI

/// The Givens app. Every window renders one WindowModel; nothing here holds state of its own.
/// Ref: https://developer.apple.com/documentation/swiftui/app
@main
struct GivensApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var delegate

    var body: some Scene {
        WindowGroup("Givens") {
            RootView(app: delegate.model)
        }
        .commands {
            // "Install givens command" lives in the app menu, after About.
            // Ref: https://developer.apple.com/documentation/swiftui/commandgroupplacement
            CommandGroup(after: .appInfo) {
                Button("Install givens command") { delegate.installCommand() }
            }
        }
    }
}

/// Owns the AppModel and takes folders dropped on the Dock icon.
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    /// Where `givens` lives: Contents/Helpers in the app bundle, or Helpers/ beside this binary
    /// in a dev build. Not beside the app binary itself: on a case-insensitive disk, "givens"
    /// and "Givens" are the same file.
    /// Ref: https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle
    static var givensExecutable: URL {
        let bundled = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers/givens")
        if FileManager.default.isExecutableFile(atPath: bundled.path) { return bundled }
        let binary = Bundle.main.executableURL ?? Bundle.main.bundleURL
        return binary.deletingLastPathComponent().appendingPathComponent("Helpers/givens")
    }

    /// Givens's own data, outside every project.
    /// Ref: https://developer.apple.com/documentation/foundation/url/applicationsupportdirectory
    static var appDataDirectory: URL {
        URL.applicationSupportDirectory.appendingPathComponent("Givens", isDirectory: true)
    }

    lazy var model: AppModel = {
        let picker = OpenPanelPicker()
        let platform = Platform(
            clipboard: SayingClipboard(SystemClipboard()), trash: SayingTrash(FinderTrash()),
            folderPicker: picker,
            urlOpener: WorkspaceURLOpener())
        let model = AppModel(
            serve: .init(
                executable: Self.givensExecutable, appDataDirectory: Self.appDataDirectory),
            platform: platform)
        picker.onPick = { [weak model] url in model?.run { try await $0.open(url.path) } }
        return model
    }()

    /// `Givens --write-icon <file.png>` writes the icon and quits, for scripts/build-app.sh.
    /// Otherwise the icon drawn in AppIcon is the Dock and ⌘Tab icon, also in a dev build,
    /// which has no bundle to carry a .icns.
    /// Ref: https://developer.apple.com/documentation/appkit/nsapplication/applicationiconimage
    func applicationWillFinishLaunching(_ notification: Notification) {
        let arguments = CommandLine.arguments
        if let flag = arguments.firstIndex(of: "--write-icon"), flag + 1 < arguments.count {
            do {
                try AppIcon.writePNG(to: arguments[flag + 1])
                exit(0)
            } catch {
                FileHandle.standardError.write(Data("can't write the icon: \(error)\n".utf8))
                exit(1)
            }
        }
        NSApplication.shared.applicationIconImage = AppIcon.image()
        // A dev build is a bare executable with no bundle, so AppKit doesn't make it a regular
        // app: no Dock icon, and keystrokes never reach its text fields. Say so, then come to
        // the front. Ref: https://developer.apple.com/documentation/appkit/nsapplication/setactivationpolicy(_:)
        NSApplication.shared.setActivationPolicy(.regular)
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Ref: https://developer.apple.com/documentation/appkit/nsapplication/activate()
        NSApplication.shared.activate()
        model.run { try await $0.launch() }
    }

    /// Folders dropped on the Dock icon, or passed by `open -a Givens <folder>`.
    func application(_ application: NSApplication, open urls: [URL]) {
        let paths = Array(urls.prefix(AppModel.droppedPathsMax).map(\.path))
        guard !paths.isEmpty else { return }
        model.run { try await $0.openDropped(paths) }
    }

    func applicationWillTerminate(_ notification: Notification) {
        model.run { try await $0.quit() }
    }

    /// Asks where to install the command, starting at the first folder on the developer's
    /// PATH they can write to, then installs it there and shows what Givens says.
    /// Ref: https://developer.apple.com/documentation/appkit/nsopenpanel
    func installCommand() {
        model.run { model in
            let folders = try await model.installFolders()
            let panel = NSOpenPanel()
            panel.canChooseFiles = false
            panel.canChooseDirectories = true
            panel.canCreateDirectories = true
            panel.prompt = "Install"
            panel.message =
                folders.isEmpty
                ? "None of the folders on your PATH can be written to. Pick one, like "
                    + "~/.local/bin, then add it to your PATH."
                : "Pick a folder on your PATH. You can write to: "
                    + folders.prefix(4).map(Self.tilde).joined(separator: ", ")
            panel.directoryURL = URL(
                fileURLWithPath: folders.first ?? NSHomeDirectory() + "/.local/bin")
            guard panel.runModal() == .OK, let folder = panel.url?.path else { return }
            let said = try await model.installCommand(into: folder)
            Feedback.shared.say(said, symbol: "terminal.fill")
        }
    }

    /// A path with the home folder shown as ~, as a terminal would say it.
    static func tilde(_ path: String) -> String {
        path.hasPrefix(NSHomeDirectory() + "/")
            ? "~" + path.dropFirst(NSHomeDirectory().count) : path
    }
}

extension AppModel {
    /// Runs an action. While it runs, "Working…" shows if it's slow; after, `done` shows as a
    /// notification, and a failure shows as a red one rather than being dropped.
    func run(_ done: String? = nil, _ action: @escaping @MainActor (AppModel) async throws -> Void)
    {
        Task { @MainActor in
            Feedback.shared.begin()
            defer { Feedback.shared.end() }
            do {
                try await action(self)
                if let done { Feedback.shared.say(done) }
            } catch {
                Feedback.shared.fail(error)
            }
        }
    }
}
