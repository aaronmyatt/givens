import Foundation
import GivensUI

// Fakes for what only the app can do. The core asks for these as effects; a step reads the
// fake to see the app did it.

/// A clipboard that holds one text.
final class FakeClipboard: Clipboard {
    private(set) var text = ""

    func writeText(_ text: String) {
        precondition(text.utf8.count < 1 << 22, "clipboard text stays bounded")
        self.text = text
    }

    func readText() -> String { text }
}

/// A Trash that's a folder inside the scenario's folder.
final class FakeTrash: Trash {
    let directory: URL
    /// Original paths, in the order they were trashed.
    private(set) var moved: [String] = []

    init(directory: URL) {
        precondition(directory.isFileURL, "the trash is a local folder")
        self.directory = directory
    }

    func move(_ path: String) throws {
        precondition(path.hasPrefix("/"), "only an absolute path is trashed")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        let source = URL(fileURLWithPath: path)
        try FileManager.default.moveItem(
            at: source, to: directory.appendingPathComponent(source.lastPathComponent))
        moved.append(path)
    }
}

/// A folder picker that only records it was shown.
final class FakeFolderPicker: FolderPicker {
    private(set) var presented = 0

    func present() { presented += 1 }
}

/// A browser that only records what it was asked to open, so tests never launch one.
final class FakeURLOpener: URLOpener {
    private(set) var opened: [String] = []

    func open(_ url: String) {
        precondition(!url.isEmpty, "a link has text")
        opened.append(url)
    }
}
