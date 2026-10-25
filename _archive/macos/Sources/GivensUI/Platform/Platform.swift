import AppKit
import Foundation

// What only the app can do, behind protocols so the acceptance suites pass fakes. The core
// asks for these as effects in serve's replies (see ServeProtocol.swift); the app does them.

/// The system clipboard.
@MainActor
public protocol Clipboard: AnyObject {
    /// Replaces the clipboard's text.
    func writeText(_ text: String)
    /// The clipboard's text, or "".
    func readText() -> String
}

/// The Finder Trash, so a delete can be undone in Finder.
@MainActor
public protocol Trash: AnyObject {
    /// Moves the file at `path` to the Trash.
    func move(_ path: String) throws
}

/// The "Open folder…" picker.
@MainActor
public protocol FolderPicker: AnyObject {
    /// Shows the picker; the folder picked, if any, is opened through AppModel.open.
    func present()
}

/// The default browser.
@MainActor
public protocol URLOpener: AnyObject {
    /// Opens `url` in the default browser.
    func open(_ url: String)
}

/// Everything the models need from the system.
@MainActor
public struct Platform {
    /// The clipboard.
    public let clipboard: any Clipboard
    /// The Trash.
    public let trash: any Trash
    /// The folder picker.
    public let folderPicker: any FolderPicker
    /// The default browser, for links on setup cards.
    public let urlOpener: any URLOpener

    /// Bundles the services.
    public init(
        clipboard: any Clipboard, trash: any Trash, folderPicker: any FolderPicker,
        urlOpener: any URLOpener
    ) {
        self.clipboard = clipboard
        self.trash = trash
        self.folderPicker = folderPicker
        self.urlOpener = urlOpener
    }
}

/// The real clipboard. Ref: https://developer.apple.com/documentation/appkit/nspasteboard
public final class SystemClipboard: Clipboard {
    /// Uses the general pasteboard.
    public init() {}

    /// Replaces the general pasteboard's contents with `text`.
    public func writeText(_ text: String) {
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(text, forType: .string)
    }

    /// The general pasteboard's text, or "".
    public func readText() -> String {
        NSPasteboard.general.string(forType: .string) ?? ""
    }
}

/// The real Trash.
/// Ref: https://developer.apple.com/documentation/foundation/filemanager/trashitem(at:resultingitemurl:)
public final class FinderTrash: Trash {
    /// Uses the default file manager.
    public init() {}

    /// Moves `path` to the user's Trash.
    public func move(_ path: String) throws {
        precondition(path.hasPrefix("/"), "only an absolute path is trashed")
        try FileManager.default.trashItem(at: URL(fileURLWithPath: path), resultingItemURL: nil)
    }
}

/// The real folder picker. Ref: https://developer.apple.com/documentation/appkit/nsopenpanel
public final class OpenPanelPicker: FolderPicker {
    /// Called with the folder picked. Set once the app model that opens it exists.
    public var onPick: @MainActor (URL) -> Void = { _ in }

    /// Makes a picker; set `onPick` before it's shown.
    public init() {}

    /// Shows an NSOpenPanel that picks one folder.
    public func present() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url { onPick(url) }
    }
}

/// The real default browser.
/// Ref: https://developer.apple.com/documentation/appkit/nsworkspace/open(_:)
public final class WorkspaceURLOpener: URLOpener {
    /// Uses the shared workspace.
    public init() {}

    /// Opens `url` with whatever app handles its scheme; text that isn't a URL is ignored.
    public func open(_ url: String) {
        guard let parsed = URL(string: url), parsed.scheme != nil else { return }
        NSWorkspace.shared.open(parsed)
    }
}
