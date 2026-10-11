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
            clipboard: SystemClipboard(), trash: FinderTrash(), folderPicker: picker,
            urlOpener: WorkspaceURLOpener())
        let model = AppModel(
            serve: .init(
                executable: Self.givensExecutable, appDataDirectory: Self.appDataDirectory),
            platform: platform)
        picker.onPick = { [weak model] url in model?.run { try await $0.open(url.path) } }
        return model
    }()

    func applicationDidFinishLaunching(_ notification: Notification) {
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
            let alert = NSAlert()
            alert.messageText = said
            alert.runModal()
        }
    }

    /// A path with the home folder shown as ~, as a terminal would say it.
    static func tilde(_ path: String) -> String {
        path.hasPrefix(NSHomeDirectory() + "/")
            ? "~" + path.dropFirst(NSHomeDirectory().count) : path
    }
}

extension AppModel {
    /// Runs an action, showing a failure as an alert rather than dropping it.
    func run(_ action: @escaping @MainActor (AppModel) async throws -> Void) {
        Task { @MainActor in
            do {
                try await action(self)
            } catch {
                NSAlert(error: error).runModal()
            }
        }
    }
}
