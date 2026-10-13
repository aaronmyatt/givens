import Foundation
import Observation

/// One window: the welcome window or one project. It mirrors internal/app's Window. The views
/// read only `snapshot`'s fields, and `visible` and `offers` are built from those same fields,
/// so a step that checks `visible` checks what the screen shows.
@MainActor
@Observable
public final class WindowModel: Identifiable {
    /// Stable for the window's life.
    public let id: String
    /// What the core last said about this window.
    public internal(set) var snapshot: WindowSnapshot?

    /// The app this window belongs to.
    @ObservationIgnored public unowned let app: AppModel

    init(app: AppModel, id: String) {
        precondition(!id.isEmpty, "a window has an id")
        self.app = app
        self.id = id
    }

    /// "welcome" or "project".
    public var kind: String { snapshot?.kind ?? "" }
    /// The window's title: the capabilities folder's path for a project window.
    public var title: String { snapshot?.title ?? "" }
    /// The capabilities folder shown, or "".
    public var path: String { snapshot?.path ?? "" }
    /// The branch a read-only window shows, or "" for the checked-out folder.
    public var branch: String { snapshot?.branch ?? "" }
    /// The sidebar's lines.
    public var sidebar: [SidebarItem] { snapshot?.sidebar ?? [] }
    /// The name of what's selected, or "".
    public var selected: String { snapshot?.selected ?? "" }
    /// Every action on offer, by label: buttons, menu items and card actions.
    public var offers: [String] { snapshot?.offers ?? [] }
    /// The file open in the editor, or "".
    public var editing: String { snapshot?.editor?.path ?? "" }
    /// True while the folder picker is up.
    public var folderPickerShown: Bool { snapshot?.folderPickerShown ?? false }
    /// The Drop in page, when one is open.
    public var session: SessionModel? {
        snapshot?.session.map { SessionModel(window: self, record: $0) }
    }

    /// Every piece of text the window shows, in the order the views lay it out.
    public var visible: [String] {
        guard let snapshot else { return [] }
        var lines = snapshot.messages + snapshot.labels
        lines += snapshot.sidebar.flatMap { [$0.name, "\($0.name) \($0.count)", $0.warning] }
        if let capability = snapshot.capability { lines += Self.lines(capability) }
        if let scenario = snapshot.scenario { lines += Self.lines(scenario) }
        lines += snapshot.listed.flatMap { ["\($0.name) — \($0.status)", $0.capability] }
        lines += snapshot.runs.flatMap { [$0.text, $0.note] }
        lines += snapshot.rows.map(Self.rowLine)
        if let search = snapshot.search { lines += search.results }
        if let editor = snapshot.editor { lines += editor.problems + editor.preview }
        if let session = snapshot.session {
            lines += session.output + session.cards + session.checklist.map(\.text)
        }
        let shown = lines.filter { !$0.isEmpty }
        precondition(shown.allSatisfy { !$0.isEmpty }, "visible lines have text")
        return shown
    }

    static func lines(_ capability: CapabilityRecord) -> [String] {
        // The views show the story first and about.md below it, so visible keeps that order.
        // A Rule: block's name heads its scenarios, as CapabilityView groups them.
        var lines = [capability.name, capability.warning] + capability.story + [capability.about]
        var rule = ""
        for scenario in capability.scenarios {
            if scenario.rule != rule {
                lines.append(scenario.rule)
                rule = scenario.rule
            }
            lines += [scenario.name, scenario.status, scenario.warning]
        }
        return lines
    }

    static func lines(_ scenario: ScenarioRecord) -> [String] {
        // Tags sit above the steps, as the scenario view lays them out.
        [scenario.name, scenario.status, scenario.warning, scenario.tags.joined(separator: " ")]
            + linedUp(scenario.steps) + [scenario.setup]
    }

    /// An outline row as one line: values in column-name order, then its status, like
    /// "card=expired — Never run". internal/app's rowLine says it the same way.
    public static func rowLine(_ row: DropInRowRecord) -> String {
        let values = row.values.keys.sorted().map { "\($0)=\(row.values[$0] ?? "")" }
        return values.joined(separator: ", ") + " — " + row.status
    }

    /// A scenario as Gherkin text, for "Copy steps": its tags, its Scenario: line, then its
    /// steps indented. An outline, whose steps hold <placeholders>, is a Scenario Outline.
    /// Ref: https://cucumber.io/docs/gherkin/reference/
    public static func gherkin(_ scenario: ScenarioRecord) -> String {
        let outline = scenario.steps.contains { $0.contains("<") && $0.contains(">") }
        var lines = scenario.tags.isEmpty ? [] : [scenario.tags.joined(separator: " ")]
        lines.append((outline ? "Scenario Outline: " : "Scenario: ") + scenario.name)
        lines += scenario.steps.map { "  " + $0 }
        return lines.joined(separator: "\n") + "\n"
    }

    /// The 0-based line where two texts first differ: the line just typed on in the editor.
    /// nil when nothing changed.
    public static func changedLine(from old: String, to new: String) -> Int? {
        let before = old.split(separator: "\n", omittingEmptySubsequences: false)
        let after = new.split(separator: "\n", omittingEmptySubsequences: false)
        guard old != new, !after.isEmpty else { return nil }
        let same = zip(before, after).prefix { $0 == $1 }.count
        return min(same, after.count - 1)
    }

    /// One line of a text, or "" past its end.
    public static func line(_ index: Int, of text: String) -> String {
        let lines = text.split(separator: "\n", omittingEmptySubsequences: false)
        return lines.indices.contains(index) ? String(lines[index]) : ""
    }

    /// The text with one step line's words swapped for an offered step, keeping its indent
    /// and keyword: "    Given a sh" becomes "    Given a shopper has an account".
    /// internal/app's ChooseSuggestion does the same to the line it's typing.
    public static func withSuggestion(_ text: String, line index: Int, step: String) -> String {
        var lines = text.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        guard lines.indices.contains(index) else { return text }
        let indent = lines[index].prefix { $0 == " " || $0 == "\t" }
        let keyword = lines[index].dropFirst(indent.count).split(separator: " ").first ?? ""
        lines[index] = indent + keyword + " " + step
        return lines.joined(separator: "\n")
    }

    /// Pads each step's keyword on the left to the longest keyword's width, so the step texts
    /// start in one column: "Given …", " When …". internal/app's linedUp does the same.
    public static func linedUp(_ steps: [String]) -> [String] {
        let keywords = steps.map { $0.split(separator: " ", maxSplits: 1).first ?? "" }
        let width = keywords.map(\.count).max() ?? 0
        return zip(steps, keywords).map { step, keyword in
            String(repeating: " ", count: width - keyword.count) + step
        }
    }

    /// Selects a sidebar item, scenario, search result or helper by its name.
    public func select(_ name: String) async throws {
        precondition(!name.isEmpty, "select names an item")
        try await call(ServeMethod.select, ["name": name])
    }

    /// Takes an offered action by its label.
    public func choose(_ action: String) async throws {
        precondition(!action.isEmpty, "an action has a label")
        try await call(ServeMethod.choose, ["action": action])
    }

    /// Types into whatever has focus: the search box, the editor, an ask card.
    public func type(_ text: String) async throws {
        try await call(ServeMethod.type, ["text": text])
    }

    /// Presses a key or shortcut: "⌘K", "⌘C", "⇧⌘C", "⌘↵", "⌘E", "↵".
    public func press(_ keys: String) async throws {
        precondition(!keys.isEmpty, "a shortcut names keys")
        try await call(ServeMethod.press, ["keys": keys])
    }

    /// A capability by its name.
    public func capability(_ name: String) async throws -> CapabilityRecord {
        precondition(!name.isEmpty, "a capability has a name")
        return try await value(ServeMethod.capability, name, CapabilityRecord.self)
    }

    /// A scenario by its name, or "Capability/Scenario" when two share it.
    public func scenario(_ name: String) async throws -> ScenarioRecord {
        precondition(!name.isEmpty, "a scenario has a name")
        return try await value(ServeMethod.scenario, name, ScenarioRecord.self)
    }

    /// Drops in to a scenario: runs its setup and opens the Drop in page.
    @discardableResult
    public func dropIn(_ scenario: String) async throws -> SessionModel {
        precondition(!scenario.isEmpty, "drop in names a scenario")
        try await call(ServeMethod.dropIn, ["scenario": scenario])
        guard let session else { throw GivensError.badReply("drop in opened no session") }
        return session
    }

    /// Opens one of the welcome window's recent projects and returns its window.
    @discardableResult
    public func openRecent(_ path: String) async throws -> WindowModel {
        precondition(path.hasPrefix("/"), "a recent project has an absolute path")
        try await call(ServeMethod.openRecent, ["path": path])
        guard let front = app.front else { throw GivensError.badReply("no window opened") }
        return front
    }

    /// Closes this window.
    public func close() async throws {
        try await call(ServeMethod.close)
        precondition(!app.windows.contains { $0.id == id }, "a closed window is gone")
    }

    /// Sends a window call, naming this window, and applies the reply.
    func call(_ method: String, _ params: [String: String] = [:]) async throws {
        precondition(params["window"] == nil, "the window is named by the model")
        try app.apply(try await app.serve.call(method, params.merging(["window": id]) { $1 }))
    }

    private func value<Value: Decodable & Sendable>(
        _ method: String, _ name: String, _ type: Value.Type
    ) async throws -> Value {
        let reply = try await app.serve.call(
            method, ["window": id, "name": name], returning: type)
        try app.apply(reply)
        guard let value = reply.value else { throw GivensError.badReply("\(method) sent nothing") }
        return value
    }
}
