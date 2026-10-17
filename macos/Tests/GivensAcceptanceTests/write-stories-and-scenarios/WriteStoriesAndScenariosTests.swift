import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/write-stories-and-scenarios against the Swift app. Shared steps are in
/// World/. It mirrors acceptance/write-stories-and-scenarios in Go, check for check.
@Suite("Write stories and scenarios") @MainActor struct WriteStoriesAndScenariosTests {
    static let capability = "write-stories-and-scenarios"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: writeStoriesAndScenariosSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: writeStoriesAndScenariosSteps)
    }
}

/// Givens's wording of the Gherkin parser's error for a line in a scenario that isn't a step.
private let writeStoriesNotAStep = "Expected a step, a tag, a comment or a new scenario"

/// What "a developer is editing" adds to a scenario's last step without saving, so a step can
/// tell the editor's text from the file's.
private let writeStoriesUnsavedEdit = " until the bank replies"

/// The line that makes 3-D Secure challenge's setup build on Pay with saved card's.
private let writeStoriesFromPayWithSavedCard = "from checkout/pay-with-saved-card"

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class WriteStoriesAndScenariosState {
    let w: World
    /// The scenario a step called "it".
    var subject = ""
    /// The New story dialog's fields: name, As a, I want, So that.
    var story: [String] = []
    /// A .feature file's text before a step acted on it.
    var featureBefore = ""
    /// Every file in the capabilities folder before a delete, and in runs/ before a rename.
    var before: [String: Data] = [:]
    var runsBefore: [String: Data] = [:]
    /// A step line before and after an unsaved edit.
    var changedFrom = ""
    var changedTo = ""
    /// A line a step added among the steps, and its 1-based line number in the editor.
    var addedLine = ""
    var addedLineNumber = 0
    /// What was typed, and the first suggestion checked.
    var typed = ""
    var offered = ""
    /// checkout.feature as another tool saved it.
    var theirs = ""
    var renamedFrom = ""
    var renamedTo = ""
    /// The slug of the capability a rename or delete acts in.
    var actedOn = ""
    /// The renamed scenario's setup script, and a renamed capability's files.
    var scriptBefore = ""
    var filesBefore: [String] = []
    /// What a step asked to delete: its name, and its folder or its setup script.
    var deleting = ""
    var deletingFolder: URL?
    var deletingScript: URL?

    init(_ w: World) { self.w = w }

    /// A capability's .feature file, relative to the sample shop.
    static func featurePath(_ capabilitySlug: String) -> String {
        precondition(!capabilitySlug.contains("/"), "a capability is a slug")
        return "capabilities/\(capabilitySlug)/\(capabilitySlug).feature"
    }

    /// The editor as the front window shows it, failing when nothing is open in it.
    func editor() async throws -> EditorRecord {
        guard let editor = try await w.front().snapshot?.editor, !editor.path.isEmpty else {
            throw StepFailure("nothing is open in the editor")
        }
        return editor
    }

    /// The scenario names of a capability, in file order.
    func scenarioNames(_ capabilitySlug: String) throws -> [String] {
        let feature = w.capabilities.appendingPathComponent(
            "\(capabilitySlug)/\(capabilitySlug).feature")
        return try Toolchain.pickles(of: feature).map(\.name)
    }

    /// Runs git in the sample shop. Ref: https://git-scm.com/docs/git
    func git(_ arguments: String) throws -> String {
        guard let project = w.project else { throw StepFailure("no sample shop for git") }
        let (status, output) = try Shell.run("git " + arguments, in: project)
        guard status == 0 else { throw StepFailure("git \(arguments) failed:\n\(output)") }
        return output
    }

    /// Opens a scenario in the editor, remembering its .feature file's text on disk.
    func open(_ name: String) async throws {
        subject = name
        let window = try await w.front()
        featureBefore = try w.readFile(Self.featurePath(try w.scenarioCapability(name)))
        try await window.edit(name)
    }

    /// Replaces the editor's text, unsaved, remembering it for "the file is saved anyway".
    func setText(_ text: String) async throws {
        let path = try await editor().path
        w.editedPath = URL(fileURLWithPath: path)
        w.editedText = text
        try await w.front().setEditorText(text)
    }
}

// MARK: - File helpers

/// Every file under `root`, by path relative to it, for those `keep` accepts.
/// Ref: https://developer.apple.com/documentation/foundation/filemanager/subpaths(atpath:)
private func writeStoriesFiles(
    _ root: URL, keep: (String) -> Bool = { _ in true }
) throws -> [String: Data] {
    var files: [String: Data] = [:]
    for relative in FileManager.default.subpaths(atPath: root.path) ?? [] where keep(relative) {
        var isDirectory: ObjCBool = false
        let url = root.appendingPathComponent(relative)
        guard FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory),
            !isDirectory.boolValue
        else { continue }
        files[relative] = try Data(contentsOf: url)
    }
    return files
}

/// How two listings from writeStoriesFiles differ, or nil when they don't.
private func writeStoriesDifference(_ before: [String: Data], _ after: [String: Data]) -> String? {
    for (relative, content) in before.sorted(by: { $0.key < $1.key }) {
        guard let now = after[relative] else { return "\(relative) was removed" }
        if now != content { return "\(relative) changed" }
    }
    if let added = after.keys.sorted().first(where: { before[$0] == nil }) {
        return "\(added) was added"
    }
    return nil
}

private func writeStoriesIsRunsFile(_ relative: String) -> Bool {
    relative.split(separator: "/").contains("runs")
}

private func writeStoriesTrimmed(_ text: String) -> [String] {
    text.components(separatedBy: "\n").map { $0.trimmingCharacters(in: .whitespaces) }
}

/// The last indented line of a scenario's block, or nil.
private func writeStoriesLastStep(_ lines: [String], _ name: String) -> Int? {
    let trimmed = lines.map { $0.trimmingCharacters(in: .whitespaces) }
    guard let start = trimmed.firstIndex(of: "Scenario: \(name)") else { return nil }
    var last: Int?
    var index = start + 1
    while index < lines.count && lines[index].hasPrefix("    ") {  // bound: lines.count
        last = index
        index += 1
    }
    return last
}

/// A scenario's steps as written, keyword included: "And the order stays pending".
private func writeStoriesSteps(in text: String, of name: String) -> [String] {
    let keywords = ["Given ", "When ", "Then ", "And ", "But ", "* "]
    let lines = writeStoriesTrimmed(text)
    guard let start = lines.firstIndex(of: "Scenario: \(name)") else { return [] }
    let block = text.components(separatedBy: "\n").dropFirst(start + 1)
        .prefix(while: { $0.hasPrefix("    ") })
    return block.map { $0.trimmingCharacters(in: .whitespaces) }
        .filter { line in keywords.contains(where: line.hasPrefix) }
}

/// How steps read in the library: keywords right-aligned to the widest one, so every step's
/// text starts in the same column.
private func writeStoriesLinedUp(_ steps: [String]) -> [String] {
    let keywords = steps.map { String($0.prefix(while: { $0 != " " })) }
    let width = keywords.map(\.count).max() ?? 0
    return zip(keywords, steps).map { keyword, step in
        String(repeating: " ", count: width - keyword.count) + step
    }
}

/// `text` with the named scenario's block removed: its tags, its Scenario: line, its steps,
/// and the blank line before it.
private func writeStoriesWithoutBlock(_ text: String, _ name: String) throws -> String {
    let lines = text.components(separatedBy: "\n")
    let trimmed = writeStoriesTrimmed(text)
    guard let start = trimmed.firstIndex(of: "Scenario: \(name)") else {
        throw StepFailure("there's no scenario \"\(name)\" to remove")
    }
    var end = start + 1
    while end < lines.count && lines[end].hasPrefix("    ") { end += 1 }  // bound: lines.count
    var first = start
    while first > 0 && trimmed[first - 1].hasPrefix("@") { first -= 1 }  // bound: start
    if first > 0 && trimmed[first - 1].isEmpty { first -= 1 }
    return (lines[..<first] + lines[end...]).joined(separator: "\n")
}

private func writeStoriesTrimEnd(_ text: String) -> String {
    text.replacingOccurrences(of: "\\n+$", with: "", options: .regularExpression)
}

// MARK: - Steps

@MainActor
func writeStoriesAndScenariosSteps(_ w: World) -> [StepDefinition] {
    let s = WriteStoriesAndScenariosState(w)
    return writeStoriesStorySteps(s) + writeStoriesEditorSteps(s) + writeStoriesSaveSteps(s)
        + writeStoriesPreviewSteps(s) + writeStoriesRenameSteps(s) + writeStoriesRunSteps(s)
        + writeStoriesDeleteSteps(s) + writeStoriesAboutSteps(s)
}

@MainActor
private func writeStoriesStorySteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(
            #"^they choose "New story" and fill in "([^"]*)", "([^"]*)", "([^"]*)" and "#
                + #""([^"]*)"$"#
        ) { call in
            let window = try await w.front()
            s.story = call.captures
            try await window.choose("New story")
            try await window.newStory(
                name: call[0], asA: call[1], iWant: call[2], soThat: call[3])
        },
        def(
            #"^([a-z0-9-]+)/([a-z0-9-]+\.feature) is created, with "([^"]*)" and those "#
                + #"lines as its description$"#
        ) { call in try s.storyCreated(call[0], call[1], call[2]) },
        def(#"^"([^"]*)" appears in the sidebar$"#) { call in
            let names = try await w.front().sidebar.filter { $0.kind == "capability" }.map(\.name)
            guard names.contains(call[0]) else {
                throw StepFailure("the sidebar has no capability \"\(call[0])\": \(names)")
            }
        },
        def(
            #"^they choose "New scenario" in ([A-Za-z ]+), name it "([^"]*)" and mark it "#
                + #"as an error state$"#
        ) { call in
            let window = try await w.front()
            s.subject = call[1]
            s.featureBefore = try w.readFile(
                WriteStoriesAndScenariosState.featurePath(slug(call[0])))
            try await window.select(call[0])
            try await window.choose("New scenario")
            try await window.newScenario(call[1], error: true)
        },
        def(#"^a "Scenario: ([^"]*)" block tagged (@\S+) is added to checkout\.feature$"#) {
            call in try s.blockAdded(call[0], call[1])
        },
        def(
            #"^checkout/scripts/([a-z0-9-]+\.sh) is created, starting with the line that "#
                + #"sources givens\.sh$"#
        ) { call in
            guard call[0] == slug(s.subject) + ".sh" else {
                throw StepFailure("\(call[0]) isn't named after \"\(s.subject)\"")
            }
            let text = try w.readFile("capabilities/checkout/scripts/\(call[0])")
            let first = text.components(separatedBy: "\n").first ?? ""
            guard first == kitSource else {
                throw StepFailure("\(call[0]) starts with \"\(first)\", not \"\(kitSource)\"")
            }
        },
        def(#"^the new scenario opens in the editor$"#) { _ in
            let editor = try await s.editor()
            let feature = w.capabilities.appendingPathComponent("checkout/checkout.feature")
            guard samePath(editor.path, feature.path) else {
                throw StepFailure("the editor holds \(editor.path), not \(feature.path)")
            }
            guard editor.text.contains("Scenario: \(s.subject)") else {
                throw StepFailure("the editor doesn't hold \"\(s.subject)\": \(editor.text)")
            }
        },
        def(#"^"([^"]*)" has no setup yet$"#) { call in try await s.noSetupYet(call[0]) },
        def(#"^scripts/([a-z0-9-]+\.md) is created with an empty numbered list$"#) { call in
            try s.markdownSetupCreated(call[0])
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Checks the new .feature file line by line, and that it compiles as strict Gherkin.
    func storyCreated(_ folder: String, _ file: String, _ featureLine: String) throws {
        guard story.count == 4, folder == slug(story[0]), file == folder + ".feature" else {
            throw StepFailure("\(folder)/\(file) isn't named after \(story.first ?? "a story")")
        }
        let path = "capabilities/\(folder)/\(file)"
        let lines = writeStoriesTrimmed(try w.readFile(path)).filter { !$0.isEmpty }
        let want = [
            featureLine, "As a " + story[1], "I want " + story[2], "So that " + story[3],
        ]
        guard lines == want else { throw StepFailure("\(path) holds \(lines), not \(want)") }
        // tools/pickles fails on a file that isn't valid Gherkin.
        _ = try Toolchain.pickles(of: w.capabilities.appendingPathComponent("\(folder)/\(file)"))
    }

    /// Checks the new block, and that every other line of checkout.feature is as it was.
    func blockAdded(_ name: String, _ tag: String) throws {
        let feature = w.capabilities.appendingPathComponent("checkout/checkout.feature")
        let pickles = try Toolchain.pickles(of: feature)
        guard let added = pickles.first(where: { $0.name == name }) else {
            throw StepFailure("checkout.feature has no scenario \"\(name)\"")
        }
        guard added.tagNames.contains(tag) else {
            throw StepFailure("\"\(name)\" is tagged \(added.tagNames), not \(tag)")
        }
        let text = try w.readFile(Self.featurePath("checkout"))
        for line in featureBefore.components(separatedBy: "\n") where !text.contains(line + "\n") {
            throw StepFailure("checkout.feature lost the line \"\(line)\"")
        }
        let wanted = featureBefore.components(separatedBy: "Scenario:").count
        guard pickles.count == wanted else {
            throw StepFailure("checkout.feature has \(pickles.count) scenarios, not \(wanted)")
        }
    }

    /// Adds the scenario to Checkout if needed, removes its setup files, and selects it, so
    /// "Write setup steps" has a scenario to act on.
    func noSetupYet(_ name: String) async throws {
        subject = name
        try w.copySampleShop()
        try w.ensureScenario("checkout", name)
        let scripts = w.capabilities.appendingPathComponent("checkout/scripts")
        for suffix in [".sh", ".md", ".teardown.sh"] {
            let url = scripts.appendingPathComponent(slug(name) + suffix)
            if FileManager.default.fileExists(atPath: url.path) {
                try FileManager.default.removeItem(at: url)
            }
        }
        try await w.front().select(name)
    }

    func markdownSetupCreated(_ file: String) throws {
        guard file == slug(subject) + ".md" else {
            throw StepFailure("\(file) isn't named after \"\(subject)\"")
        }
        let text = try w.readFile("capabilities/checkout/scripts/\(file)")
        let lines = writeStoriesTrimmed(text).filter { !$0.isEmpty }
        // An empty ordered list item is "1." alone. Ref: https://spec.commonmark.org/0.31.2/#lists
        guard lines == ["1."] else {
            throw StepFailure("\(file) holds \"\(text)\", not an empty numbered list")
        }
    }
}

@MainActor
private func writeStoriesEditorSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer is writing a new scenario in Checkout$"#) { _ in
            try w.copySampleShop()
            try s.ensureStepUses("a signed-in shopper with one item in the cart", 4)
            let window = try await w.front()
            try await window.select("Checkout")
            try await window.choose("New scenario")
            try await window.newScenario("Pay with store credit", error: false)
        },
        def(#"^they type "([^"]*)"$"#) { call in
            s.typed = call[0]
            try await w.front().type(call[0])
        },
        def(#"^(?:they're offered )?"([^"]*)" with "([^"]*)"$"#) { call in
            let suggestions = try await w.front().stepSuggestions()
            guard suggestions.contains(where: { $0.text == call[0] && $0.label == call[1] })
            else {
                throw StepFailure(
                    "the editor doesn't offer \"\(call[0])\" with \"\(call[1])\": \(suggestions)")
            }
            if s.offered.isEmpty { s.offered = call[0] }
        },
        def(#"^choosing one inserts it word for word$"#) { _ in try await s.choosingInserts() },
    ]
}

@MainActor
private func writeStoriesSaveSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer has the sample shop project in git and open in Givens$"#) { _ in
            try w.copySampleShop()
            _ = try s.git("init --quiet")
            _ = try s.git("add --all")
            _ = try s.git(
                "-c user.name=Givens -c user.email=givens@example.com -c commit.gpgsign=false "
                    + "commit --quiet -m 'Sample shop'")
            _ = try await w.front()
        },
        def(#"^they change "([^"]*)" to "([^"]*)" in "([^"]*)"$"#) { call in
            try await s.open(call[2])
            let text = try await s.editor().text
            let count = text.components(separatedBy: call[0]).count - 1
            guard count == 1 else {
                throw StepFailure("the editor holds \"\(call[0])\" \(count) times, not once")
            }
            s.changedFrom = call[0]
            s.changedTo = call[1]
            try await s.setText(text.replacingOccurrences(of: call[0], with: call[1]))
        },
        def(#"^they save$"#) { _ in
            _ = try await s.editor()
            try await w.front().saveEditor()
        },
        def(#"^git diff shows that one line changed and nothing else$"#) { _ in
            try s.diffOneLine()
        },
        def(#"^a developer (?:has unsaved changes to|is editing) "([^"]*)"$"#) { call in
            try await s.editing(call[0])
        },
        def(#"^another tool saves a different change to checkout\.feature$"#) { _ in
            let path = WriteStoriesAndScenariosState.featurePath("checkout")
            let text = try w.readFile(path)
            let from = #"Then they see "Order placed""#
            guard text.contains(from) else {
                throw StepFailure("checkout.feature has no \(from) to change")
            }
            s.theirs = text.replacingOccurrences(
                of: from, with: #"Then they see "Thanks for your order""#)
            try w.writeFile(path, s.theirs)
        },
        def(#"^they can choose "([^"]*)", "([^"]*)" or "([^"]*)"$"#) { call in
            let offers = try await w.front().offers
            for action in call.captures where !offers.contains(action) {
                throw StepFailure("the window doesn't offer \"\(action)\"; it offers \(offers)")
            }
            let onDisk = try w.readFile(WriteStoriesAndScenariosState.featurePath("checkout"))
            guard onDisk == s.theirs else {
                throw StepFailure("checkout.feature no longer holds the other tool's change")
            }
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Adds Checkout scenarios using `step` until `want` scenarios in the project use it.
    func ensureStepUses(_ step: String, _ want: Int) throws {
        let folders = try FileManager.default.contentsOfDirectory(atPath: w.capabilities.path)
        var uses = 0
        for folder in folders.sorted() {
            let feature = w.capabilities.appendingPathComponent("\(folder)/\(folder).feature")
            guard FileManager.default.fileExists(atPath: feature.path) else { continue }
            uses += try Toolchain.pickles(of: feature).filter { pickle in
                pickle.steps.contains { $0.text == step }
            }.count
        }
        guard uses <= want else {
            throw StepFailure("the sample shop uses \"\(step)\" in \(uses) scenarios, over \(want)")
        }
        var text = try w.readFile(Self.featurePath("checkout"))
        for added in 0..<(want - uses) {
            text =
                writeStoriesTrimEnd(text) + "\n\n  Scenario: Reorder \(added + 1)\n"
                + "    Given \(step)\n    When they place the order\n"
                + "    Then they see \"Order placed\"\n"
        }
        try w.writeFile(Self.featurePath("checkout"), text)
    }

    /// Chooses the first suggestion checked and expects the typed line replaced by the whole
    /// step, keyword kept.
    func choosingInserts() async throws {
        guard !offered.isEmpty else { throw StepFailure("no step offered a suggestion") }
        try await w.front().chooseStepSuggestion(offered)
        let keyword = typed.prefix(while: { $0 != " " })
        let lines = writeStoriesTrimmed(try await editor().text)
        guard lines.contains("\(keyword) \(offered)") else {
            throw StepFailure("the editor has no line \"\(keyword) \(offered)\": \(lines)")
        }
        guard !lines.contains(typed) else {
            throw StepFailure("the editor still has the typed line \"\(typed)\"")
        }
    }

    /// Expects git to see one line of checkout.feature replaced, and no other change.
    func diffOneLine() throws {
        let feature = Self.featurePath("checkout")
        let status = try git("status --porcelain")
        guard writeStoriesTrimEnd(status) == " M \(feature)" else {
            throw StepFailure("git status shows \"\(status)\", not only \(feature) modified")
        }
        let diff = try git("diff --unified=0 --no-color")
        let changed = diff.components(separatedBy: "\n").filter { line in
            !line.hasPrefix("---") && !line.hasPrefix("+++")
                && (line.hasPrefix("-") || line.hasPrefix("+"))
        }
        let old = featureBefore.components(separatedBy: "\n").first { $0.contains(changedFrom) }
        let oldLine = old ?? ""
        let newLine = oldLine.replacingOccurrences(of: changedFrom, with: changedTo)
        guard changed == ["-" + oldLine, "+" + newLine] else {
            throw StepFailure("git diff changes \(changed), not one line:\n\(diff)")
        }
    }

    /// Opens a scenario in the editor and changes its last step, unsaved: a developer part-way
    /// through editing it.
    func editing(_ name: String) async throws {
        try w.copySampleShop()
        try await open(name)
        var lines = try await editor().text.components(separatedBy: "\n")
        guard let last = writeStoriesLastStep(lines, name) else {
            throw StepFailure("the editor doesn't hold \"\(name)\"'s steps: \(lines)")
        }
        changedFrom = lines[last].trimmingCharacters(in: .whitespaces)
        lines[last] += writeStoriesUnsavedEdit
        changedTo = lines[last].trimmingCharacters(in: .whitespaces)
        try await setText(lines.joined(separator: "\n"))
    }

    /// Adds `line` after the subject's last step in the editor, unsaved.
    func addLine(_ line: String) async throws {
        var lines = try await editor().text.components(separatedBy: "\n")
        guard let last = writeStoriesLastStep(lines, subject) else {
            throw StepFailure("the editor doesn't hold \"\(subject)\"'s steps: \(lines)")
        }
        lines.insert("    " + line, at: last + 1)
        addedLine = line
        addedLineNumber = last + 2
        try await setText(lines.joined(separator: "\n"))
    }
}

@MainActor
private func writeStoriesPreviewSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they add the line "([^"]*)" among its steps$"#) { call in
            try await s.addLine(call[0])
        },
        def(#"^one line among its steps doesn't start with Given, When, Then, And, But or \*$"#) {
            _ in try await s.addLine("Check the order is pending")
        },
        def(#"^that line is marked "([^"]*)"$"#) { call in
            let marks = try await w.front().editorMarks()
            guard marks.contains(where: { $0.line == s.addedLineNumber && $0.message == call[0] })
            else {
                throw StepFailure(
                    "line \(s.addedLineNumber) isn't marked \"\(call[0])\"; marks: \(marks)")
            }
        },
        def(
            #"^(?:the scenario is shown as it reads in the library, with its keywords "#
                + #"lined up|the steps around it are shown as normal)$"#
        ) { _ in try await s.previewLinedUp() },
        def(#"^their unsaved changes are included$"#) { _ in
            let shown = try await w.front().editorPreview().map {
                $0.text.trimmingCharacters(in: .whitespaces)
            }
            guard shown.contains(s.changedTo) else {
                throw StepFailure("the preview doesn't show the unsaved \"\(s.changedTo)\"")
            }
            guard !shown.contains(s.changedFrom) else {
                throw StepFailure("the preview shows the saved \"\(s.changedFrom)\"")
            }
        },
        def(#"^that line is shown marked with the parser's message$"#) { _ in
            let preview = try await w.front().editorPreview()
            guard
                preview.contains(where: {
                    $0.text.contains(s.addedLine) && $0.mark == writeStoriesNotAStep
                })
            else {
                throw StepFailure(
                    "the preview doesn't mark \"\(s.addedLine)\" with the parser's message: "
                        + "\(preview)")
            }
        },
        def(#"^a developer is editing Checkout's about\.md$"#) { _ in
            try w.copySampleShop()
            _ = try s.aboutImages()
            try await w.front().edit("checkout/about.md")
        },
        def(#"^the checkout flow diagram is shown as it will look in the library$"#) { _ in
            try await s.previewAsLibrary()
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Expects the scenario's name, then its steps lined up and unmarked, in order: the file's
    /// steps with the unsaved edit made.
    func previewLinedUp() async throws {
        let preview = try await w.front().editorPreview()
        guard preview.contains(where: { $0.text.contains(subject) }) else {
            throw StepFailure("the preview doesn't name \"\(subject)\": \(preview)")
        }
        let feature = try w.readFile(Self.featurePath(try w.scenarioCapability(subject)))
        let steps = writeStoriesSteps(in: feature, of: subject).map {
            $0 == changedFrom ? changedTo : $0
        }
        guard !steps.isEmpty else { throw StepFailure("\"\(subject)\" has no steps on disk") }
        var from = 0
        for want in writeStoriesLinedUp(steps) {
            let rest = preview[from...]
            guard let found = rest.firstIndex(where: { $0.text == want && $0.mark.isEmpty })
            else {
                throw StepFailure(
                    "the preview doesn't show \"\(want)\" unmarked after line \(from): \(preview)")
            }
            from = found + 1
        }
    }

    /// The images Checkout's about.md shows, each checked to exist.
    /// Ref: https://spec.commonmark.org/0.31.2/#images
    func aboutImages() throws -> [String] {
        let text = try w.readFile("capabilities/checkout/about.md")
        let pattern = try Regex(#"!\[[^\]]*\]\(([^)\s]+)\)"#)
        let images = text.matches(of: pattern).compactMap { match in
            match.output[1].substring.map(String.init)
        }
        guard !images.isEmpty else {
            throw StepFailure("checkout/about.md shows no image, so no flow diagram")
        }
        for image in images {
            _ = try Data(contentsOf: w.capabilities.appendingPathComponent("checkout/\(image)"))
        }
        return images
    }

    /// Expects the preview to be exactly what the library shows as Checkout's about.md, with
    /// its images.
    func previewAsLibrary() async throws {
        let images = try aboutImages()
        let window = try await w.front()
        let library = try await window.capability("Checkout").about
        let preview = try await window.editorPreview().map(\.text).joined(separator: "\n")
        guard !library.isEmpty, preview == library else {
            throw StepFailure("the preview is \"\(preview)\", not the library's \"\(library)\"")
        }
        for image in images where !preview.contains(image) {
            throw StepFailure("the preview doesn't show \(image): \(preview)")
        }
    }
}

@MainActor
private func writeStoriesRenameSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they rename "([^"]*)" to "([^"]*)"$"#) { call in
            try await s.rename(call[0], call[1])
        },
        def(#"^a developer renames "([^"]*)" to "([^"]*)" and confirms$"#) { call in
            try await s.rename(call[0], call[1])
            try await s.confirmRename()
        },
        def(#"^a developer renames it to "([^"]*)"( and confirms)?$"#) { call in
            guard !s.subject.isEmpty else { throw StepFailure("no step named what to rename") }
            try await s.rename(s.subject, call[0])
            if !call[1].isEmpty { try await s.confirmRename() }
        },
        def(
            #"^its Scenario: line is updated, and its setup script is renamed to "#
                + #"([a-z0-9-]+\.sh)$"#
        ) { call in try s.scenarioLineUpdated(call[0]) },
        def(#"^the from line in 3-D Secure challenge's setup is updated(?: to match)?$"#) { _ in
            let lines = try s.fromLines("3-D Secure challenge")
            let want = "from checkout/\(slug(s.renamedTo))"
            guard lines.contains(want) else {
                throw StepFailure("3-D Secure challenge's from lines are \(lines), not \(want)")
            }
            let old = "from checkout/\(slug(s.renamedFrom))"
            guard !lines.contains(old) else {
                throw StepFailure("3-D Secure challenge's setup still has \"\(old)\"")
            }
        },
        def(#"^it shows "([^"]*)", since its runs were recorded under the old name$"#) {
            call in try await s.statusIs(s.renamedTo, call[0])
        },
        def(#"^the sidebar lists "([^"]*)" in place of "([^"]*)"$"#) { call in
            let names = try await w.front().sidebar.filter { $0.kind == "capability" }.map(\.name)
            guard names.contains(call[0]), !names.contains(call[1]) else {
                throw StepFailure("the sidebar lists \(names)")
            }
        },
        def(
            #"^its folder and \.feature file are renamed to ([a-z0-9-]+) and "#
                + #"([a-z0-9-]+\.feature), with its scripts and helpers inside$"#
        ) { call in try s.folderRenamed(call[0], call[1]) },
        def(#"^its scenarios show "([^"]*)"$"#) { call in
            let names = try s.scenarioNames(slug(s.renamedTo))
            guard !names.isEmpty else { throw StepFailure("\"\(s.renamedTo)\" has no scenarios") }
            for name in names { try await s.statusIs(name, call[0]) }
        },
        def(#"^that line becomes "([^"]*)"$"#) { call in try s.lineBecomes(call[0]) },
        def(#"^"([^"]*)" and "([^"]*)" show "([^"]*)"$"#) { call in
            try await s.statusIs(call[0], call[2])
            try await s.statusIs(call[1], call[2])
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Gives every sample shop scenario without one a setup script, and makes 3-D Secure
    /// challenge's setup build on Pay with saved card's, as the spec assumes.
    func arrangeSetups() throws {
        try w.copySampleShop()
        let folders = try FileManager.default.contentsOfDirectory(atPath: w.capabilities.path)
        for folder in folders.sorted() {
            let feature = w.capabilities.appendingPathComponent("\(folder)/\(folder).feature")
            guard FileManager.default.fileExists(atPath: feature.path) else { continue }
            for name in try Toolchain.pickles(of: feature).map(\.name) {
                let script = try w.scriptPath(name)
                let markdown = String(script.dropLast(3)) + ".md"
                let exists = [script, markdown].contains { (try? w.readFile($0)) != nil }
                if !exists { try w.writeFile(script, kitSource + "\nok \"set up \(name)\"\n") }
            }
        }
        let script = try w.readFile(try w.scriptPath("3-D Secure challenge"))
        if !script.contains(writeStoriesFromPayWithSavedCard + "\n") {
            try w.setupStartsWith("3-D Secure challenge", writeStoriesFromPayWithSavedCard)
        }
    }

    /// Gives a capability a helper file of its own when it has none.
    func ensureHelper(_ capabilitySlug: String) throws {
        let shared = w.capabilities.appendingPathComponent("\(capabilitySlug)/_shared")
        let found = (try? FileManager.default.contentsOfDirectory(atPath: shared.path)) ?? []
        if found.contains(where: { $0.hasSuffix(".sh") }) { return }
        let function = "seed_" + capabilitySlug.replacingOccurrences(of: "-", with: "_")
        let helper =
            "# shellcheck shell=bash\n\n# @helper \(function)\n"
            + "# Seeds what \(capabilitySlug)'s scenarios start from.\n"
            + "\(function)() {\n  ok \"seeded \(capabilitySlug)\"\n}\n"
        try w.writeFile("capabilities/\(capabilitySlug)/_shared/\(capabilitySlug).sh", helper)
    }

    /// The slug of the capability called `name`, or nil when it names a scenario.
    func capabilityNamed(_ name: String) -> String? {
        let capabilitySlug = slug(name)
        let feature = w.capabilities.appendingPathComponent(
            "\(capabilitySlug)/\(capabilitySlug).feature")
        return FileManager.default.fileExists(atPath: feature.path) ? capabilitySlug : nil
    }

    /// Renames a scenario or capability, remembering what it had before.
    func rename(_ name: String, _ newName: String) async throws {
        try arrangeSetups()
        let window = try await w.front()
        renamedFrom = name
        renamedTo = newName
        subject = newName
        runsBefore = try writeStoriesFiles(w.capabilities, keep: writeStoriesIsRunsFile)
        if let capabilitySlug = capabilityNamed(name) {
            try ensureHelper(capabilitySlug)
            actedOn = capabilitySlug
            let folder = w.capabilities.appendingPathComponent(capabilitySlug)
            filesBefore = try writeStoriesFiles(folder).keys.sorted()
        } else {
            actedOn = try w.scenarioCapability(name)
            scriptBefore = try w.readFile(try w.scriptPath(name))
        }
        try await window.rename(name, to: newName)
    }

    /// Chooses "Rename" when Givens asks first; a rename that loses no runs doesn't ask.
    func confirmRename() async throws {
        let window = try await w.front()
        if window.offers.contains("Rename") { try await window.choose("Rename") }
    }

    func statusIs(_ name: String, _ want: String) async throws {
        let status = try await w.front().scenario(name).status
        guard status == want else { throw StepFailure("\"\(name)\" shows \"\(status)\"") }
    }
}

extension WriteStoriesAndScenariosState {
    func scenarioLineUpdated(_ file: String) throws {
        guard file == slug(renamedTo) + ".sh" else {
            throw StepFailure("\(file) isn't named after \"\(renamedTo)\"")
        }
        let lines = writeStoriesTrimmed(try w.readFile(Self.featurePath(actedOn)))
        guard lines.contains("Scenario: \(renamedTo)") else {
            throw StepFailure("\(actedOn).feature has no line \"Scenario: \(renamedTo)\"")
        }
        guard !lines.contains("Scenario: \(renamedFrom)") else {
            throw StepFailure("\(actedOn).feature still has \"Scenario: \(renamedFrom)\"")
        }
        let scripts = "capabilities/\(actedOn)/scripts"
        let renamed = try w.readFile("\(scripts)/\(file)")
        guard renamed == scriptBefore else {
            throw StepFailure("\(file) holds \"\(renamed)\", not the old script")
        }
        guard (try? w.readFile("\(scripts)/\(slug(renamedFrom)).sh")) == nil else {
            throw StepFailure("\(slug(renamedFrom)).sh is still there")
        }
    }

    /// A setup script's from lines, by their first two words.
    func fromLines(_ scenario: String) throws -> [String] {
        let text = try w.readFile(try w.scriptPath(scenario))
        return text.components(separatedBy: "\n").compactMap { line in
            let fields = line.split(separator: " ")
            return fields.count >= 2 && fields[0] == "from" ? "from \(fields[1])" : nil
        }
    }

    /// Expects every file the capability had, under its new folder and name.
    func folderRenamed(_ folder: String, _ file: String) throws {
        guard folder == slug(renamedTo), file == folder + ".feature" else {
            throw StepFailure("\(folder)/\(file) isn't named after \"\(renamedTo)\"")
        }
        let old = w.capabilities.appendingPathComponent(actedOn)
        guard !FileManager.default.fileExists(atPath: old.path) else {
            throw StepFailure("\(actedOn) is still there")
        }
        let first = try w.readFile(Self.featurePath(folder)).components(separatedBy: "\n")[0]
        guard first == "Feature: \(renamedTo)" else {
            throw StepFailure("\(file) starts \"\(first)\", not \"Feature: \(renamedTo)\"")
        }
        var kinds: [String: Int] = [:]
        for relative in filesBefore {
            let moved = relative == actedOn + ".feature" ? file : relative
            let url = w.capabilities.appendingPathComponent("\(folder)/\(moved)")
            guard FileManager.default.fileExists(atPath: url.path) else {
                throw StepFailure("\(folder)/\(moved) is missing")
            }
            kinds[String(relative.prefix(while: { $0 != "/" })), default: 0] += 1
        }
        guard kinds["scripts", default: 0] > 0, kinds["_shared", default: 0] > 0 else {
            throw StepFailure("\(renamedFrom) had no scripts or helpers to move: \(filesBefore)")
        }
    }

    /// Expects Accept invite's setup to start with `line`, and no from line naming the
    /// capability's old slug.
    func lineBecomes(_ line: String) throws {
        let text = try w.readFile(try w.scriptPath("Accept invite"))
        let lines = text.components(separatedBy: "\n")
        guard lines.count >= 2, lines[1] == line else {
            throw StepFailure("Accept invite's setup doesn't start with \"\(line)\": \(text)")
        }
        let old = "from \(slug(renamedFrom))/"
        guard !text.contains(old) else {
            throw StepFailure("Accept invite's setup still has \"\(old)\": \(text)")
        }
    }
}

@MainActor
private func writeStoriesRunSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" has (\d+) runs recorded$"#) { call in
            try await s.recordRuns(call[0], Int(call[1]) ?? 0)
        },
        def(#"^"([^"]*)" has a passing run recorded today$"#) { call in
            try await s.recordRuns(call[0], 1)
        },
        def(#"^it's listed in the Unverified view$"#) { _ in
            let window = try await w.front()
            try await window.select("Unverified")
            let listed = try await window.listedScenarios().map(\.name)
            guard listed.contains(s.subject) else {
                throw StepFailure("the Unverified view doesn't list \"\(s.subject)\": \(listed)")
            }
        },
        def(#"^nothing in runs/ changes$"#) { _ in
            guard !s.runsBefore.isEmpty else {
                throw StepFailure("no runs were recorded before the rename")
            }
            let after = try writeStoriesFiles(w.capabilities, keep: writeStoriesIsRunsFile)
            if let difference = writeStoriesDifference(s.runsBefore, after) {
                throw StepFailure("runs/ changed: \(difference)")
            }
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Records passing runs a minute apart, as a developer dropping in again would.
    func recordRuns(_ name: String, _ runs: Int) async throws {
        guard runs > 0 && runs <= 16 else { throw StepFailure("\(runs) runs is out of range") }
        subject = name
        try arrangeSetups()
        for _ in 0..<runs {
            try await w.recordPassingRun(name)
            try await w.app?.advanceClock(seconds: 60)
        }
    }
}

@MainActor
private func writeStoriesDeleteSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they delete "([^"]*)"$"#) { call in try await s.delete(call[0]) },
        def(#"^they confirm$"#) { _ in try await w.front().choose("Delete") },
        def(
            #"^its block is removed from checkout\.feature, and its setup script is moved "#
                + #"to the Trash$"#
        ) { _ in try s.scenarioDeleted() },
        def(#"^Checkout shows (\d+) scenarios$"#) { call in
            let want = Int(call[0]) ?? -1
            let names = try s.scenarioNames("checkout")
            guard names.count == want else {
                throw StepFailure("checkout.feature has \(names.count) scenarios, not \(want)")
            }
            let sidebar = try await w.front().sidebar
            guard
                sidebar.contains(where: {
                    $0.name == "Checkout" && $0.kind == "capability" && $0.count == want
                })
            else { throw StepFailure("the sidebar doesn't show Checkout \(want): \(sidebar)") }
        },
        def(#"^nothing is deleted unless they choose "([^"]*)"$"#) { call in
            try await s.deletedOnlyWhen(call[0])
        },
        def(#"^the invites folder is moved to the Trash$"#) { _ in
            try s.trashed(w.capabilities.appendingPathComponent("invites"))
        },
        def(#"^the sidebar lists "([^"]*)" and "([^"]*)"$"#) { call in
            let names = try await w.front().sidebar.filter { $0.kind == "capability" }.map(\.name)
            guard names.sorted() == call.captures.sorted() else {
                throw StepFailure("the sidebar lists \(names), not \(call.captures)")
            }
        },
        def(#"^a developer has chosen "([^"]*)" on "([^"]*)"$"#) { call in
            let window = try await w.front()
            try await window.select(call[1])
            try await window.choose(call[0])
            _ = try await s.editor()
        },
        def(#"^([a-z0-9-]+\.(?:md|sh)) is moved to the Trash$"#) { call in
            try s.trashed(w.capabilities.appendingPathComponent("checkout/scripts/\(call[0])"))
        },
        def(#"^the editor closes$"#) { _ in
            if let editor = try await w.front().snapshot?.editor, !editor.path.isEmpty {
                throw StepFailure("the editor still holds \(editor.path)")
            }
        },
    ]
}

extension WriteStoriesAndScenariosState {
    /// Asks to delete a scenario or capability, remembering every file before it.
    func delete(_ name: String) async throws {
        try arrangeSetups()
        let window = try await w.front()
        deleting = name
        deletingFolder = nil
        deletingScript = nil
        if let capabilitySlug = capabilityNamed(name) {
            deletingFolder = w.capabilities.appendingPathComponent(capabilitySlug)
        } else {
            actedOn = try w.scenarioCapability(name)
            deletingScript = w.project?.appendingPathComponent(try w.scriptPath(name))
            featureBefore = try w.readFile(Self.featurePath(actedOn))
        }
        before = try writeStoriesFiles(w.capabilities)
        try await window.delete(name)
    }

    /// Expects `url` gone and moved to the FakeTrash, matched by folder and name since macOS
    /// reaches its temporary folder through a symlink.
    func trashed(_ url: URL) throws {
        guard !FileManager.default.fileExists(atPath: url.path) else {
            throw StepFailure("\(url.path) is still there")
        }
        let folder = url.deletingLastPathComponent().path
        let moved = w.trash.moved.contains { path in
            let original = URL(fileURLWithPath: path)
            return original.lastPathComponent == url.lastPathComponent
                && samePath(original.deletingLastPathComponent().path, folder)
        }
        guard moved else {
            throw StepFailure("\(url.path) wasn't moved to the Trash; it got \(w.trash.moved)")
        }
    }

    func scenarioDeleted() throws {
        guard let script = deletingScript else { throw StepFailure("no scenario was deleted") }
        let want = try writeStoriesWithoutBlock(featureBefore, deleting)
        let got = try w.readFile(Self.featurePath(actedOn))
        guard writeStoriesTrimEnd(got) == writeStoriesTrimEnd(want) else {
            throw StepFailure("\(actedOn).feature is\n\(got)\nnot\n\(want)")
        }
        try trashed(script)
    }

    /// Expects nothing deleted yet, then chooses `action` and expects the delete.
    func deletedOnlyWhen(_ action: String) async throws {
        let now = try writeStoriesFiles(w.capabilities)
        if let difference = writeStoriesDifference(before, now) {
            throw StepFailure("something was deleted before \"\(action)\": \(difference)")
        }
        guard w.trash.moved.isEmpty else {
            throw StepFailure("the Trash got \(w.trash.moved) before \"\(action)\"")
        }
        let window = try await w.front()
        guard window.offers.contains(action) else {
            throw StepFailure("the window doesn't offer \"\(action)\": \(window.offers)")
        }
        try await window.choose(action)
        if let folder = deletingFolder { try trashed(folder) } else { try scenarioDeleted() }
    }
}
