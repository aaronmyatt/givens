import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/gherkin-files against the Swift app. Shared steps are in World/.
/// It mirrors acceptance/gherkin-files in Go.
@Suite("Gherkin files") @MainActor struct GherkinFilesTests {
    static let capability = "gherkin-files"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: gherkinFilesSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: gherkinFilesSteps)
    }
}

/// The sample shop's checkout.feature, relative to the shop.
private let gherkinFilesCheckoutPath = "capabilities/checkout/checkout.feature"

/// How long a test runner may run. npx may fetch cucumber-js the first time.
private let gherkinFilesRunnerDeadlineSeconds = 300

/// A cucumber-js step definitions file, which Givens must leave alone.
/// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/support_files/step_definitions.md
private let gherkinFilesStepsSource = """
    import { Given, When, Then } from "@cucumber/cucumber";

    Given("a signed-in shopper with one item in the cart", async function () {});
    When("they place the order", async function () {});
    Then("they see {string}", async function (text: string) {});

    """

/// A valid second .feature file for the checkout folder.
private let gherkinFilesRefundsFeature = """
    Feature: Refunds
      As a shopper who changed their mind
      I want my money back
      So that I'm not out of pocket

      Scenario: Full refund
        Given a shopper with a paid order
        When they ask for a refund
        Then they see "Refund sent"

    """

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class GherkinFilesState {
    let w: World
    /// The scenarios a Given step arranged, in the order it named them.
    var names: [String] = []
    /// The sidebar item a Then step last found, for "selecting it".
    var listed = ""
    /// The comment line a Given step added.
    var comment = ""
    /// The step line an edit replaced, and what replaced it.
    var editFrom = ""
    var editTo = ""

    init(_ w: World) { self.w = w }

    /// Writes checkout.feature and checks it still parses, so a broken arrangement fails here
    /// and not as a confusing app failure later.
    func writeCheckout(_ lines: [String]) throws {
        try w.writeFile(gherkinFilesCheckoutPath, lines.joined(separator: "\n") + "\n")
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        do {
            _ = try Toolchain.pickles(of: project.appendingPathComponent(gherkinFilesCheckoutPath))
        } catch {
            throw StepFailure("the arranged checkout.feature doesn't parse: \(error)")
        }
    }

    /// The scenario names in a sample shop .feature file, read with the same parser the
    /// suites use.
    func namesInFeature(_ folder: String, _ file: String) throws -> [String] {
        let feature = w.capabilities.appendingPathComponent("\(folder)/\(file)")
        return try Toolchain.pickles(of: feature).map(\.name)
    }

    /// Expects a capability on the sidebar with no warning, showing exactly the scenarios its
    /// .feature file holds.
    func loadsNormally(_ name: String) async throws {
        let window = try await w.front()
        let item = try gherkinFilesSidebarCapability(window.sidebar, name)
        let shown = try await window.capability(name)
        guard item.warning.isEmpty, shown.warning.isEmpty else {
            throw StepFailure(
                "\"\(name)\" has a warning: \"\(item.warning)\" / \"\(shown.warning)\"")
        }
        let want = try namesInFeature(slug(name), slug(name) + ".feature")
        let got = shown.scenarios.map(\.name)
        guard got == want else {
            throw StepFailure("\"\(name)\" shows the scenarios \(got), not \(want)")
        }
    }

    /// Runs a test runner's command from the sample shop's root folder.
    func runFromRoot(_ command: String) async throws {
        try w.copySampleShop()
        w.cwd = w.project
        try await gherkinFilesRunBounded(w, command, gherkinFilesRunnerDeadlineSeconds)
    }

    /// Makes the sample shop a git repository with everything committed, so git diff shows
    /// only what later steps change. Ref: https://git-scm.com/docs/git-commit
    func commitShop() throws {
        w.cwd = w.project
        try w.run(
            "git init --quiet && git add -A && "
                + "git -c user.name=Givens -c user.email=givens@example.com commit --quiet -m shop"
        )
        guard w.exitCode == 0 else {
            throw StepFailure("committing the sample shop exited \(w.exitCode):\n\(w.output)")
        }
    }
}

/// Moves two checkout scenarios into a Rule: block at the end of checkout.feature, adding the
/// second when the sample shop doesn't have it.
@MainActor
private func gherkinFilesUnderRule(
    _ s: GherkinFilesState, _ first: String, _ second: String, _ rule: String
) throws {
    try s.w.copySampleShop()
    try s.w.ensureScenario("checkout", second)
    let text = try s.w.readFile(gherkinFilesCheckoutPath)
    var lines = gherkinFilesTrimTrailingBlanks(text.components(separatedBy: "\n"))
    var moved: [String] = []
    for name in [first, second] {
        let block = try gherkinFilesScenarioBlock(lines, name)
        moved += lines[block].map { gherkinFilesTrimTrailingSpaces("  " + $0) }
        moved.append("")
        lines.removeSubrange(block)
        let start = block.lowerBound
        if start > 0 && start < lines.count && lines[start - 1].isEmpty && lines[start].isEmpty {
            lines.remove(at: start)
        }
    }
    lines = gherkinFilesTrimTrailingBlanks(lines) + ["", "  Rule: \(rule)", ""] + moved
    s.names = [first, second]
    try s.writeCheckout(lines)
}

/// Opens the scenario in Givens's editor, adds words to its last step, and saves.
@MainActor
private func gherkinFilesEditStep(_ s: GherkinFilesState, _ name: String) async throws {
    let window = try await s.w.front()
    let editor = try await window.edit(name)
    var lines = editor.text.components(separatedBy: "\n")
    let block: Range<Int>
    do {
        block = try gherkinFilesScenarioBlock(lines, name)
    } catch {
        throw StepFailure("the editor's text: \(error)")
    }
    s.editFrom = lines[block.upperBound - 1]
    s.editTo = s.editFrom + " for an hour"
    lines[block.upperBound - 1] = s.editTo
    try await window.setEditorText(lines.joined(separator: "\n"))
    try await window.saveEditor()
}

/// Expects neither action offered on either scenario, renames the second in Givens, and then
/// expects both actions offered on both.
@MainActor
private func gherkinFilesDisabledUntilRenamed(
    _ s: GherkinFilesState, _ first: String, _ second: String
) async throws {
    let window = try await s.w.front()
    for name in s.names {
        try await window.select(name)
        for action in [first, second] where window.offers.contains(action) {
            throw StepFailure("\"\(action)\" is offered on \"\(name)\" while its slug clashes")
        }
    }
    guard s.names.count == 2 else { throw StepFailure("no step named two scenarios") }
    let renamed = s.names[1] + " again"
    try await window.rename(s.names[1], to: renamed)
    for name in [s.names[0], renamed] {
        try await window.select(name)
        for action in [first, second] {
            do {
                try expect(window.offers, action, "\(name)'s actions")
            } catch {
                throw StepFailure("after the rename: \(error)")
            }
        }
    }
}

/// Reads git diff with no context lines: an edit that rewrote the file shows more than the one
/// removed and one added line. Ref: https://git-scm.com/docs/git-diff
@MainActor
private func gherkinFilesDiffShowsOnlyStep(_ s: GherkinFilesState) throws {
    guard !s.editFrom.isEmpty else { throw StepFailure("no step edited a step in this scenario") }
    s.w.cwd = s.w.project
    try s.w.run("git diff --no-color --unified=0 -- \(gherkinFilesCheckoutPath)")
    guard s.w.exitCode == 0 else {
        throw StepFailure("git diff exited \(s.w.exitCode):\n\(s.w.output)")
    }
    let got = gherkinFilesChangedLines(s.w.output)
    let want = ["-" + s.editFrom, "+" + s.editTo]
    guard got == want else {
        throw StepFailure("git diff changed \(got), not just \(want):\n\(s.w.output)")
    }
}

@MainActor
func gherkinFilesSteps(_ w: World) -> [StepDefinition] {
    let s = GherkinFilesState(w)
    return gherkinFilesRunnerSteps(s) + gherkinFilesEditingSteps(s) + gherkinFilesLoadingSteps(s)
        + gherkinFilesProblemSteps(s)
}

/// A test runner reading the same files.
@MainActor
private func gherkinFilesRunnerSteps(_ s: GherkinFilesState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they run "([^"]*)" from its root folder$"#) { call in
            try await s.runFromRoot(call[0])
        },
        def(#"^cucumber-js finds all (\d+) scenarios$"#) { call in
            let got = try gherkinFilesScenarioCount(w.output)
            guard String(got) == call[0] else {
                throw StepFailure(
                    "cucumber-js found \(got) scenarios, not \(call[0]):\n\(w.output)")
            }
        },
        def(#"^it reports no parse errors$"#) { _ in
            if w.output.lowercased().contains("parse error") {
                throw StepFailure("cucumber-js reported a parse error:\n\(w.output)")
            }
            _ = try gherkinFilesScenarioCount(w.output)
        },
    ]
}

/// Rules, comments and edits made in Givens.
@MainActor
private func gherkinFilesEditingSteps(_ s: GherkinFilesState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^checkout\.feature puts "([^"]*)" and "([^"]*)" under Rule: (.+)$"#) { call in
            try gherkinFilesUnderRule(s, call[0], call[1], call[2])
        },
        def(#"^those two scenarios are listed under "([^"]*)"$"#) { call in
            let groups = try await w.front().ruleGroups("Checkout")
            guard let group = groups.first(where: { $0.name == call[0] }) else {
                throw StepFailure("Checkout has no Rule \"\(call[0])\"; its rules are \(groups)")
            }
            guard group.scenarios == s.names else {
                throw StepFailure("Rule \"\(call[0])\" lists \(group.scenarios), not \(s.names)")
            }
        },
        def(#"^their run keys stay ([a-z0-9/-]+) and ([a-z0-9/-]+)$"#) { call in
            // A run key is the capability's slug, a slash, and the scenario's slug.
            guard s.names.count == 2 else { throw StepFailure("no step named two scenarios") }
            let checkout = try await w.front().capability("Checkout")
            for (name, want) in zip(s.names, call.captures) {
                guard let scenario = checkout.scenarios.first(where: { $0.name == name }) else {
                    throw StepFailure(
                        "Checkout doesn't show \"\(name)\"; it shows \(checkout.scenarios.map(\.name))"
                    )
                }
                let got = checkout.slug + "/" + scenario.slug
                guard got == want else {
                    throw StepFailure("\"\(name)\" has the run key \"\(got)\", not \"\(want)\"")
                }
            }
        },
        def(#"^checkout\.feature has the comment "([^"]*)" above "([^"]*)"$"#) { call in
            try w.copySampleShop()
            var lines = try w.readFile(gherkinFilesCheckoutPath).components(separatedBy: "\n")
            let block = try gherkinFilesScenarioBlock(lines, call[1])
            lines.insert("  " + call[0], at: block.lowerBound)
            s.comment = call[0]
            try s.writeCheckout(lines)
            try s.commitShop()
        },
        def(#"^a developer edits a step of "([^"]*)" in Givens and saves$"#) { call in
            try await gherkinFilesEditStep(s, call[0])
        },
        def(#"^the comment is still there$"#) { _ in
            let lines = try w.readFile(gherkinFilesCheckoutPath).components(separatedBy: "\n")
            let start = try gherkinFilesScenarioBlock(lines, "Card declined").lowerBound
            let above = start > 0 ? lines[start - 1] : ""
            guard above == "  " + s.comment else {
                throw StepFailure(
                    "the line above \"Card declined\" and its tags is \"\(above)\", "
                        + "not \"  \(s.comment)\"")
            }
        },
        def(#"^git diff shows only the step that changed$"#) { _ in
            try gherkinFilesDiffShowsOnlyStep(s)
        },
    ]
}

/// Files in the capabilities folder that aren't one capability's .feature file.
@MainActor
private func gherkinFilesLoadingSteps(_ s: GherkinFilesState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the sample shop has ([a-z0-9/._-]+\.ts)$"#) { call in
            try w.copySampleShop()
            try w.writeFile("capabilities/\(call[0])", gherkinFilesStepsSource)
        },
        def(#"^a folder capabilities/steps/ with no \.feature file$"#) { _ in
            try w.copySampleShop()
            try w.writeFile("capabilities/steps/shared.steps.ts", gherkinFilesStepsSource)
        },
        def(#"^Checkout loads as normal, with no warning about the steps file$"#) { _ in
            try await s.loadsNormally("Checkout")
            try expectNot(try await w.front().visible, "steps.ts", "the window")
        },
        def(#"^"([^"]*)" isn't listed as a capability$"#) { call in
            let names = try await w.front().sidebar.filter { $0.kind == "capability" }.map(\.name)
            guard !names.isEmpty else { throw StepFailure("the sidebar lists no capabilities") }
            if names.contains(where: { $0.caseInsensitiveCompare(call[0]) == .orderedSame }) {
                throw StepFailure("the sidebar lists \"\(call[0])\" as a capability: \(names)")
            }
        },
        def(#"^the checkout folder holds checkout\.feature and ([a-z-]+\.feature)$"#) { call in
            try w.copySampleShop()
            try w.writeFile("capabilities/checkout/\(call[0])", gherkinFilesRefundsFeature)
        },
        def(#"^Checkout loads from checkout\.feature$"#) { _ in
            // Checkout shows checkout.feature's scenarios and none of refunds.feature's.
            let shown = try await w.front().capability("Checkout")
            let want = try s.namesInFeature("checkout", "checkout.feature")
            let got = shown.scenarios.map(\.name)
            guard got == want else {
                throw StepFailure("Checkout shows the scenarios \(got), not \(want)")
            }
            s.listed = "Checkout"
        },
        def(#"^it shows the warning "([^"]*)"$"#) { call in
            guard !s.listed.isEmpty else { throw StepFailure("no earlier step found a capability") }
            let shown = try await w.front().capability(s.listed)
            guard shown.warning == call[0] else {
                throw StepFailure(
                    "\"\(s.listed)\" has the warning \"\(shown.warning)\", not \"\(call[0])\"")
            }
        },
    ]
}

/// Files that don't parse, and names that clash.
@MainActor
private func gherkinFilesProblemSteps(_ s: GherkinFilesState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" and "([^"]*)" load as normal$"#) { call in
            try await s.loadsNormally(call[0])
            try await s.loadsNormally(call[1])
        },
        def(#"^"([^"]*)" is listed with a warning$"#) { call in
            let item = try gherkinFilesSidebarCapability(try await w.front().sidebar, call[0])
            guard !item.warning.isEmpty else {
                throw StepFailure("\"\(call[0])\" is listed with no warning")
            }
            s.listed = call[0]
        },
        def(#"^selecting it shows "([^"]*)"$"#) { call in
            guard !s.listed.isEmpty else {
                throw StepFailure("no earlier step found a listed item to select")
            }
            let window = try await w.front()
            try await window.select(s.listed)
            try expect(window.visible, call[0], "the window")
        },
        def(#"^checkout\.feature has scenarios named "([^"]*)" and "([^"]*)"$"#) { call in
            try w.copySampleShop()
            for name in call.captures { try w.ensureScenario("checkout", name) }
            s.names = call.captures
        },
        def(#"^both are listed with the warning "([^"]*)"$"#) { call in
            let window = try await w.front()
            for name in s.names {
                let warning = try await window.scenario(name).warning
                guard warning == call[0] else {
                    throw StepFailure(
                        "\"\(name)\" has the warning \"\(warning)\", not \"\(call[0])\"")
                }
            }
        },
        def(#"^"([^"]*)" and "([^"]*)" are disabled on both until one is renamed$"#) { call in
            try await gherkinFilesDisabledUntilRenamed(s, call[0], call[1])
        },
    ]
}

/// A capability's line in the sidebar.
private func gherkinFilesSidebarCapability(_ items: [SidebarItem], _ name: String) throws
    -> SidebarItem
{
    let capabilities = items.filter { $0.kind == "capability" }
    guard let item = capabilities.first(where: { $0.name == name }) else {
        throw StepFailure(
            "the sidebar has no capability \"\(name)\"; it lists \(capabilities.map(\.name))")
    }
    return item
}

/// A scenario's lines in a feature file: from its first tag line to its last non-blank line.
/// The next line indented two spaces (a tag, scenario, Rule or comment) starts the next block.
private func gherkinFilesScenarioBlock(_ lines: [String], _ name: String) throws -> Range<Int> {
    let header = "Scenario: \(name)"
    guard var start = lines.firstIndex(where: { $0.trimmingCharacters(in: .whitespaces) == header })
    else { throw StepFailure("checkout.feature has no scenario \"\(name)\"") }
    var end = start + 1
    while end < lines.count && !gherkinFilesStartsBlock(lines[end]) { end += 1 }  // bound: lines
    while end > start && lines[end - 1].trimmingCharacters(in: .whitespaces).isEmpty {
        end -= 1  // bound: end only falls, to start
    }
    while start > 0 && lines[start - 1].trimmingCharacters(in: .whitespaces).hasPrefix("@") {
        start -= 1  // bound: start only falls, to 0
    }
    precondition(start < end, "a block holds its Scenario: line")
    return start..<end
}

/// True for a line indented exactly two spaces.
private func gherkinFilesStartsBlock(_ line: String) -> Bool {
    line.count > 2 && line.hasPrefix("  ") && line.dropFirst(2).first != " "
}

/// Drops blank lines from the end.
private func gherkinFilesTrimTrailingBlanks(_ lines: [String]) -> [String] {
    var trimmed = lines
    while trimmed.last?.isEmpty == true { trimmed.removeLast() }  // bound: trimmed only shrinks
    return trimmed
}

/// Drops spaces from the end of a line.
private func gherkinFilesTrimTrailingSpaces(_ line: String) -> String {
    var trimmed = Substring(line)
    while trimmed.last == " " { trimmed = trimmed.dropLast() }  // bound: trimmed only shrinks
    return String(trimmed)
}

/// A unified diff's removed and added lines, without the file headers.
/// Ref: https://www.gnu.org/software/diffutils/manual/html_node/Unified-Format.html
private func gherkinFilesChangedLines(_ diff: String) -> [String] {
    diff.components(separatedBy: "\n").filter { line in
        let header = line.hasPrefix("--- ") || line.hasPrefix("+++ ")
        return !header && (line.hasPrefix("-") || line.hasPrefix("+"))
    }
}

/// The number of scenarios in cucumber-js's summary line, like "9 scenarios (9 undefined)".
/// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/formatters.md
/// Ref: https://developer.apple.com/documentation/swift/regex
private func gherkinFilesScenarioCount(_ output: String) throws -> Int {
    if output.contains("npx canceled due to missing packages") {
        throw StepFailure(
            "the sample shop has no cucumber-js of its own (its package.json needs "
                + "@cucumber/cucumber, installed):\n\(output)")
    }
    let summary = try Regex(#"(?m)^(\d+) scenarios? \("#)
    guard let match = output.firstMatch(of: summary),
        let digits = match.output[1].substring, let count = Int(digits)
    else { throw StepFailure("cucumber-js printed no scenario count:\n\(output)") }
    return count
}

/// Runs `command` like World.run, but stops it at the deadline. A runner that isn't installed
/// fails with a message naming it rather than an unexplained exit code. Output goes to a file,
/// so a chatty runner can't fill a pipe and stall while the step polls.
/// Ref: https://developer.apple.com/documentation/foundation/process
@MainActor
private func gherkinFilesRunBounded(_ w: World, _ command: String, _ deadlineSeconds: Int)
    async throws
{
    precondition(deadlineSeconds > 0, "a run has a deadline")
    let directory = w.cwd ?? w.project ?? w.dir
    let path = w.pathPrepend + [try Toolchain.binaries()]
    if command.hasPrefix("npx ") {
        let (status, _) = try Shell.run("command -v npx", in: directory, path: path)
        guard status == 0 else {
            throw StepFailure("npx isn't on PATH, so cucumber-js can't run; install Node.js")
        }
    }
    let log = w.dir.appendingPathComponent("runner-\(UUID().uuidString).log")
    FileManager.default.createFile(atPath: log.path, contents: nil)
    let handle = try FileHandle(forWritingTo: log)
    defer { try? handle.close() }
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/bash")
    process.arguments = ["-c", command]
    process.currentDirectoryURL = directory
    process.environment = gherkinFilesRunnerEnvironment(w, path)
    process.standardOutput = handle
    process.standardError = handle
    try process.run()
    let deadline = Date().addingTimeInterval(TimeInterval(deadlineSeconds))
    while process.isRunning && Date() < deadline {  // bound: the deadline
        try await Task.sleep(for: .milliseconds(100))
    }
    let timedOut = process.isRunning
    if timedOut { process.terminate() }
    process.waitUntilExit()
    w.output = (try? String(contentsOf: log, encoding: .utf8)) ?? ""
    w.exitCode = process.terminationStatus
    if timedOut {
        throw StepFailure("\"\(command)\" didn't finish within \(deadlineSeconds)s:\n\(w.output)")
    }
}

/// The runner's environment: PATH as World.run sets it, and npx limited to the sample shop's
/// own cucumber-js. An empty cache and "no" to installs stop it fetching the unrelated registry
/// package named cucumber-js. Ref: https://docs.npmjs.com/cli/commands/npm-exec#yes
@MainActor
private func gherkinFilesRunnerEnvironment(_ w: World, _ path: [URL]) -> [String: String] {
    var environment = ProcessInfo.processInfo.environment
    let inherited = environment["PATH"] ?? "/usr/bin:/bin"
    environment["PATH"] = (path.map(\.path) + [inherited]).joined(separator: ":")
    environment["npm_config_yes"] = "false"
    environment["npm_config_cache"] = w.dir.appendingPathComponent("npm-cache").path
    return environment
}
