import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/tags against the Swift app. Shared steps are in World/.
/// It mirrors acceptance/tags in Go.
@Suite("Tags") @MainActor struct TagsTests {
    static let capability = "tags"

    @Test func scenarios() async throws {
        // npx runs only the sample shop's own cucumber-js: an empty cache and "no" to installs
        // stop it fetching the unrelated registry package named cucumber-js. The shared "they
        // run" step inherits this process's environment, so it's set here, as the Go suite does.
        // Ref: https://docs.npmjs.com/cli/commands/npm-exec#yes
        // Ref: https://pubs.opengroup.org/onlinepubs/9799919799/functions/setenv.html
        let cache = FileManager.default.temporaryDirectory
            .appendingPathComponent("givens-npm-cache-\(UUID().uuidString)", isDirectory: true)
        setenv("npm_config_yes", "false", 1)
        setenv("npm_config_cache", cache.path, 1)
        defer { try? FileManager.default.removeItem(at: cache) }
        try await runCapability(Self.capability, steps: tagsSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: tagsSteps)
    }
}

/// The search group scenarios are listed under, matching internal/app's SearchScenarios.
private let tagsScenariosGroup = "Scenarios"

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class TagsState {
    let w: World
    /// The scenarios a Given step tagged, in the order it named them.
    var names: [String] = []
    /// The scenario a step called "it".
    var subject = ""

    init(_ w: World) { self.w = w }

    /// Adds a tag to a scenario in whichever sample shop capability holds it.
    func tagScenario(_ name: String, _ tag: String) throws {
        try w.copySampleShop()
        try addTag(try w.scenarioCapability(name), "Scenario: \(name)", tag)
    }

    /// Adds `tag` to the tag line above `header` ("Feature: Checkout", "Scenario: Card
    /// declined"), or puts a tag line there, at header's indentation. Gherkin tags sit on the
    /// lines before the keyword they apply to. Ref: https://cucumber.io/docs/cucumber/api/#tags
    func addTag(_ folder: String, _ header: String, _ tag: String) throws {
        let relative = "capabilities/\(folder)/\(folder).feature"
        var lines = try w.readFile(relative).components(separatedBy: "\n")
        guard
            let at = lines.firstIndex(where: { $0.trimmingCharacters(in: .whitespaces) == header })
        else { throw StepFailure("\(relative) has no line \"\(header)\"") }
        let above = at > 0 ? lines[at - 1].trimmingCharacters(in: .whitespaces) : ""
        if above.split(separator: " ").contains(Substring(tag)) { return }
        if above.hasPrefix("@") {
            lines[at - 1] += " " + tag
        } else {
            let indent = String(lines[at].prefix { $0 == " " })
            lines.insert(indent + tag, at: at)
        }
        try w.writeFile(relative, lines.joined(separator: "\n"))
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        do {
            _ = try Toolchain.pickles(of: project.appendingPathComponent(relative))
        } catch {
            throw StepFailure("the tagged \(relative) doesn't parse: \(error)")
        }
    }

    /// The sorted scenario names in a sample shop capability.
    func namesIn(_ folder: String) throws -> [String] {
        let feature = w.capabilities.appendingPathComponent("\(folder)/\(folder).feature")
        return try Toolchain.pickles(of: feature).map(\.name).sorted()
    }

    /// The sorted names of the shop's scenarios that have `tag`, on the scenario or on its
    /// Feature: a pickle carries both. Ref: https://github.com/cucumber/messages
    func taggedScenarios(_ tag: String) throws -> [String] {
        let folders = try FileManager.default.contentsOfDirectory(atPath: w.capabilities.path)
        var names: [String] = []
        for folder in folders {
            let directory = w.capabilities.appendingPathComponent(folder, isDirectory: true)
            let files = (try? FileManager.default.contentsOfDirectory(atPath: directory.path)) ?? []
            for file in files where file.hasSuffix(".feature") {
                let pickles = try Toolchain.pickles(of: directory.appendingPathComponent(file))
                names += pickles.filter { $0.tagNames.contains(tag) }.map(\.name)
            }
        }
        return names.sorted()
    }

    /// The sorted names of the scenarios the selected view or tag lists.
    func listedNames() async throws -> [String] {
        try await w.front().listedScenarios().map(\.name).sorted()
    }

    /// The scenarios listed under Scenarios in the search results.
    func foundScenarios() async throws -> [SearchResultRecord] {
        try await w.front().searchResults().results(tagsScenariosGroup)
    }
}

/// Compares two lists of names in any order.
private func tagsSameNames(_ got: [String], _ want: [String], _ what: String) throws {
    let gotSorted = got.sorted()
    let wantSorted = want.sorted()
    guard !wantSorted.isEmpty, gotSorted == wantSorted else {
        throw StepFailure("\(what) lists \(gotSorted), not \(wantSorted)")
    }
}

/// Expects the view to list exactly the shop's @error scenarios, Feature tags included, with
/// the named one among them marked as an error state.
@MainActor
private func tagsListedAsError(_ s: TagsState, _ name: String) async throws {
    let listed = try await s.w.front().listedScenarios()
    let want = try s.taggedScenarios("@error")
    let got = listed.map(\.name).sorted()
    guard got == want else {
        throw StepFailure("the view lists \(got), not the @error scenarios \(want)")
    }
    guard listed.contains(where: { $0.name == name && $0.error }) else {
        throw StepFailure("\"\(name)\" isn't marked as an error state in \(listed.map(\.name))")
    }
}

/// Tags another capability's Feature: line, opens the shop afresh so Givens reads it, searches
/// for the tag again, and expects that capability's scenarios among the results alongside the
/// ones tagged one by one.
@MainActor
private func tagsFeatureTagFound(_ s: TagsState, _ tag: String) async throws {
    let feature = "Sign in"
    try s.addTag(slug(feature), "Feature: \(feature)", tag)
    s.w.app?.stop()
    s.w.app = nil
    s.w.window = nil
    try await s.w.openSampleShop()
    try await s.w.search(tag)
    let want = s.names + (try s.namesIn(slug(feature)))
    try tagsSameNames(try await s.foundScenarios().map(\.name), want, "the search's Scenarios")
}

/// Compares cucumber-js's dry run with the Error states view: the scenario count from its
/// summary, and the names it reports for each scenario it couldn't run.
/// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/filtering.md#tags
@MainActor
private func tagsCucumberMatchesErrorStates(_ s: TagsState) async throws {
    let ran = try tagsCucumberScenarios(s.w.output)
    let window = try await s.w.front()
    try await window.select("Error states")
    let view = try await s.listedNames()
    guard !view.isEmpty else { throw StepFailure("the Error states view lists nothing") }
    guard ran.count == view.count else {
        throw StepFailure(
            "cucumber-js found \(ran.count) scenarios; the Error states view lists \(view)")
    }
    if !ran.names.isEmpty && ran.names != view {
        throw StepFailure("cucumber-js found \(ran.names); the Error states view lists \(view)")
    }
}

/// Expects no results while the expression is broken, then completes it and expects results
/// and no error.
@MainActor
private func tagsNoResultsUntilComplete(_ s: TagsState) async throws {
    let window = try await s.w.front()
    let broken = try await window.searchResults()
    guard broken.groups.isEmpty else {
        throw StepFailure("results are listed for a broken expression: \(broken.groups)")
    }
    try await window.type(" not @slow")
    let completed = try await window.searchResults()
    guard !completed.results(tagsScenariosGroup).isEmpty else {
        throw StepFailure("the completed expression \"\(completed.query)\" lists no scenarios")
    }
    try expectNot(window.visible, "Not a tag expression", "the window")
}

@MainActor
func tagsSteps(_ w: World) -> [StepDefinition] {
    let s = TagsState(w)
    return tagsTaggingSteps(s) + tagsViewSteps(s) + tagsSearchSteps(s) + tagsTodoSteps(s)
}

/// Tags written into the sample shop's .feature files.
@MainActor
private func tagsTaggingSteps(_ s: TagsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" is tagged (@\S+) in checkout\.feature$"#) { call in
            try w.copySampleShop()
            let folder = try w.scenarioCapability(call[0])
            guard folder == "checkout" else {
                throw StepFailure("\"\(call[0])\" is in \(folder), not checkout")
            }
            s.subject = call[0]
            try s.addTag(folder, "Scenario: \(call[0])", call[1])
        },
        def(#"^"([^"]*)" and "([^"]*)" are both tagged (@\S+)$"#) { call in
            for name in [call[0], call[1]] { try s.tagScenario(name, call[2]) }
            s.names = [call[0], call[1]]
        },
        def(#"^the Feature: (.+) line is tagged (@\S+)$"#) { call in
            try w.copySampleShop()
            try s.addTag(slug(call[0]), "Feature: \(call[0])", call[1])
        },
        def(#"^"([^"]*)" is tagged (@\S+) and "([^"]*)" is tagged (@\S+) and (@\S+)$"#) { call in
            let taggings = [(call[0], call[1]), (call[2], call[3]), (call[2], call[4])]
            for (name, tag) in taggings { try s.tagScenario(name, tag) }
        },
    ]
}

/// The Error states view and tag views in the sidebar, and a runner's matching filter.
@MainActor
private func tagsViewSteps(_ s: TagsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer selects the "([^"]*)" view$"#) { call in
            try await w.front().select(call[0])
        },
        def(#"^"([^"]*)" is listed, marked as an error state$"#) { call in
            try await tagsListedAsError(s, call[0])
        },
        def(#"^a developer selects "([^"]*)" under Tags(?: in the sidebar)?$"#) { call in
            // The tag must be under Tags in the sidebar before it can be selected.
            let window = try await w.front()
            let tags = window.sidebar.filter { $0.kind == "tag" }.map(\.name)
            guard tags.contains(call[0]) else {
                throw StepFailure("Tags in the sidebar lists \(tags), not \"\(call[0])\"")
            }
            try await window.select(call[0])
        },
        def(#"^only those two scenarios are listed$"#) { _ in
            try tagsSameNames(try await s.listedNames(), s.names, "the tag view")
        },
        def(#"^every (.+) scenario is listed$"#) { call in
            let want = try s.namesIn(slug(call[0]))
            try tagsSameNames(try await s.listedNames(), want, "the tag view")
        },
        def(#"^cucumber-js finds the same scenarios the Error states view lists$"#) { _ in
            try await tagsCucumberMatchesErrorStates(s)
        },
    ]
}

/// ⌘K search by tag expression.
@MainActor
private func tagsSearchSteps(_ s: TagsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer presses ⌘K and types "([^"]*)"$"#) { call in
            try await w.search(call[0])
        },
        def(#"^only those two scenarios are listed under Scenarios$"#) { _ in
            let found = try await s.foundScenarios().map(\.name)
            try tagsSameNames(found, s.names, "the search's Scenarios")
        },
        def(#"^a scenario that gets (@\S+) from its Feature line is listed too$"#) { call in
            try await tagsFeatureTagFound(s, call[0])
        },
        def(#"^"([^"]*)" is listed$"#) { call in
            let found = try await s.foundScenarios().map(\.name)
            guard found.contains(call[0]) else {
                throw StepFailure("the search's Scenarios are \(found), without \"\(call[0])\"")
            }
        },
        def(#"^"([^"]*)" isn't$"#) { call in
            let found = try await s.foundScenarios().map(\.name)
            guard !found.isEmpty else { throw StepFailure("the search lists no scenarios at all") }
            guard !found.contains(call[0]) else {
                throw StepFailure("the search lists \"\(call[0])\" among \(found)")
            }
        },
        def(#"^only "([^"]*)" is listed, with "([^"]*)" highlighted$"#) { call in
            let found = try await s.foundScenarios()
            guard found.count == 1, let only = found.first, only.name == call[0] else {
                throw StepFailure(
                    "the search's Scenarios are \(found.map(\.name)), not just \"\(call[0])\"")
            }
            guard only.matches.contains(call[1]) else {
                throw StepFailure("\"\(call[0])\" highlights \(only.matches), not \"\(call[1])\"")
            }
        },
        def(#"^no results are listed until the expression is complete$"#) { _ in
            try await tagsNoResultsUntilComplete(s)
        },
    ]
}

/// What a cucumber-js dry run reported: its scenario count, and the sorted names it reported,
/// which are empty when its formatter printed none.
private struct TagsCucumberRun {
    let count: Int
    let names: [String]
}

/// Reads the summary line ("4 scenarios (4 undefined)") and each reported scenario
/// ("1) Scenario: Card declined # capabilities/checkout/checkout.feature:12").
/// Ref: https://github.com/cucumber/cucumber-js/blob/main/docs/formatters.md
/// Ref: https://developer.apple.com/documentation/swift/regex
private func tagsCucumberScenarios(_ output: String) throws -> TagsCucumberRun {
    if output.contains("command not found") && output.contains("npx") {
        throw StepFailure("npx isn't on PATH; install Node.js:\n\(output)")
    }
    if output.contains("npx canceled due to missing packages") {
        throw StepFailure(
            "the sample shop has no cucumber-js of its own (its package.json needs "
                + "@cucumber/cucumber, installed):\n\(output)")
    }
    let summary = try Regex(#"(?m)^(\d+) scenarios? \("#)
    guard let match = output.firstMatch(of: summary),
        let digits = match.output[1].substring, let count = Int(digits)
    else { throw StepFailure("cucumber-js printed no scenario count:\n\(output)") }
    let reported = try Regex(#"(?m)^\d+\) Scenario: (.+?) # \S+$"#)
    let names = output.matches(of: reported).compactMap { $0.output[1].substring.map(String.init) }
    return TagsCucumberRun(count: count, names: names.sorted())
}
