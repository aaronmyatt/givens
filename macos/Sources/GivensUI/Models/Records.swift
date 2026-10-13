// What serve sends about one window. Field names match internal/app's types (Window,
// SidebarItem, Capability, Scenario, Session, CheckItem) with lower-camel JSON keys.
// The views render these fields and nothing else, so WindowModel.visible, built from the same
// fields, is what a person would read off the screen.

/// One window as serve describes it.
public struct WindowSnapshot: Decodable, Sendable, Equatable {
    /// Stable for the window's life; requests name windows by it.
    public let id: String
    /// "welcome" or "project".
    public let kind: String
    /// The capabilities folder's path, for a project window.
    public let title: String
    /// The capabilities folder a project window shows, or "".
    public let path: String
    /// The branch a read-only window shows, or "" for the checked-out folder.
    public let branch: String
    /// Messages across the top: "No capabilities here yet", "This project isn't trusted".
    public let messages: [String]
    /// Capabilities, views, tags and helpers.
    public let sidebar: [SidebarItem]
    /// The name of what's selected, or "".
    public let selected: String
    /// The capability on show, when one is selected.
    public let capability: CapabilityRecord?
    /// The scenario on show, when one is selected.
    public let scenario: ScenarioRecord?
    /// The ⌘K search, when it's open.
    public let search: SearchRecord?
    /// The editor, when a file is open in it.
    public let editor: EditorRecord?
    /// The Drop in page, when one is open.
    public let session: SessionRecord?
    /// The scenarios a selected view, tag or step lists, with their capability and status.
    public let listed: [ScenarioRecord]
    /// The selected outline's Examples rows.
    public let rows: [DropInRowRecord]
    /// The shown scenario's recorded runs, newest first.
    public let runs: [RunRecord]
    /// Every action on offer, by label.
    public let offers: [String]
    /// Plain labels the window shows, like "Recent" or "Open folder…".
    public let labels: [String]
    /// True while the folder picker is up.
    public let folderPickerShown: Bool
}

/// One sidebar line.
public struct SidebarItem: Decodable, Sendable, Equatable {
    /// "Checkout", "Error states", "payments", "seed_shopper".
    public let name: String
    /// "capability", "view", "tag" or "helper".
    public let kind: String
    /// Scenarios in a capability or view.
    public let count: Int
    /// Why the item is marked, or "".
    public let warning: String
}

/// A capability as a window shows it.
public struct CapabilityRecord: Decodable, Sendable, Equatable {
    /// From its Feature: line.
    public let name: String
    /// Its folder's name.
    public let slug: String
    /// The As a, I want and So that lines.
    public let story: [String]
    /// about.md rendered as text, or "".
    public let about: String
    /// Its scenarios, in file order.
    public let scenarios: [ScenarioRecord]
    /// Why it's marked, or "".
    public let warning: String
}

/// A scenario as a window shows it.
public struct ScenarioRecord: Decodable, Sendable, Equatable {
    /// From its Scenario: line.
    public let name: String
    /// Its slug.
    public let slug: String
    /// The capability's name.
    public let capability: String
    /// The Rule: block it's under, or "".
    public let rule: String
    /// Tagged @error.
    public let error: Bool
    /// Every tag, Feature tags included.
    public let tags: [String]
    /// "Given …", "When …", "Then …", Background steps first.
    public let steps: [String]
    /// The setup script's text, or "".
    public let setup: String
    /// "Verified today", "Never run", "Last run failed", "Unverified: …".
    public let status: String
    /// Why it's marked, or "".
    public let warning: String
}

/// One recorded run, as a scenario's page lists it. Mirrors internal/app's RunLine.
public struct RunRecord: Decodable, Sendable, Equatable {
    /// "pass" or "fail".
    public let result: String
    /// "Passed today at 14:05 by Sam", with " · card=expired" for an outline row.
    public let text: String
    /// What was written with the run, or "".
    public let note: String
    /// The paths of the files kept with the run, like a screenshot.
    public let files: [String]
    /// The whole note when it ran to more than one line, as Markdown, or "".
    public let noteMarkdown: String
    /// The run's own folder, which the note's image links are relative to.
    public let folder: String
}

/// The ⌘K search.
public struct SearchRecord: Decodable, Sendable, Equatable {
    /// What's typed.
    public let query: String
    /// Result lines, best first.
    public let results: [String]
}

/// The editor pane.
public struct EditorRecord: Decodable, Sendable, Equatable {
    /// The file open in it.
    public let path: String
    /// Its text as edited.
    public let text: String
    /// Problems shown under the editor, like a Gherkin parse error.
    public let problems: [String]
    /// What "Preview" shows beside the text, once it's chosen.
    public let preview: [String]
    /// The text has changes that aren't saved.
    public let dirty: Bool
}

/// The Drop in page.
public struct SessionRecord: Decodable, Sendable, Equatable {
    /// The scenario dropped in to.
    public let scenario: String
    /// "running", "waiting", "ready", "failed" or "stopped".
    public let status: String
    /// Setup output so far, one line per entry.
    public let output: [String]
    /// Cards setup is paused at: by_hand, ask, or a markdown step.
    public let cards: [String]
    /// The checklist, once setup is done.
    public let checklist: [CheckItem]
    /// The checklist cut into runs of one kind, in the order the steps are written.
    public let parts: [ChecklistPart]
    /// The paths of the files "Record run" will keep with the run.
    public let attachments: [String]
}

/// One checklist line.
public struct CheckItem: Decodable, Sendable, Equatable {
    /// The step as shown, placeholders filled in.
    public let text: String
    /// "given", "when" or "then".
    public let kind: String
    /// "", "done", "pass" or "fail".
    public let mark: String
}

/// One numbered part of the Drop in page: a run of checklist lines of one kind. Mirrors
/// internal/app's ChecklistPart.
public struct ChecklistPart: Decodable, Sendable, Equatable {
    /// "given" (done by hand), "when" or "then".
    public let kind: String
    /// The part's lines, in the order they're written.
    public let items: [CheckItem]
}

/// One kit version, for the `test.shipKits` seam.
public struct KitRelease: Encodable, Sendable, Equatable {
    /// Starts at 1.
    public let version: Int
    /// The whole givens.sh, starting with "# givens kit N".
    public let text: String

    /// Makes a release.
    public init(version: Int, text: String) {
        precondition(version > 0, "kit versions start at 1")
        precondition(text.hasPrefix("# givens kit "), "a kit starts with its version line")
        self.version = version
        self.text = text
    }
}
