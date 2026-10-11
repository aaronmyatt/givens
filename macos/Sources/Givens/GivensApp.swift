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
    /// Where `givens` lives inside the app bundle, or next to this binary in a dev build.
    static var givensExecutable: URL {
        Bundle.main.url(forAuxiliaryExecutable: "givens")
            ?? Bundle.main.bundleURL.appendingPathComponent("givens")
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

    /// Installs the command into /usr/local/bin and shows what Givens says.
    func installCommand() {
        model.run { model in
            let said = try await model.installCommand(into: "/usr/local/bin")
            let alert = NSAlert()
            alert.messageText = said
            alert.runModal()
        }
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
