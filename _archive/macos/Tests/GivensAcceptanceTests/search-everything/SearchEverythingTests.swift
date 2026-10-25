import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/search-everything against the Swift app. Shared steps are in World/.
@Suite("Search everything") @MainActor struct SearchEverythingTests {
    static let capability = "search-everything"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: searchEverythingSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: searchEverythingSteps)
    }
}

/// A helper as its `# @helper` comment documents it, read straight from the sample shop.
private struct SearchEverythingHelperDoc {
    /// The text after "# @helper": "seed_shopper [email]".
    let signature: String
    /// The comment lines under it, joined with spaces.
    let description: String
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class SearchEverythingState {
    let w: World
    /// The result a Then step last found, for "selecting it".
    var selected: SearchResultRecord?

    init(_ w: World) { self.w = w }

    /// The results of one kind, or none when the search found none of that kind.
    func group(_ kind: String) async throws -> [SearchResultRecord] {
        let results = try await w.front().searchResults()
        return results.groups.first { $0.kind == kind }?.results ?? []
    }

    /// Finds a result by its title under a kind, and keeps it for "selecting it".
    @discardableResult
    func listedUnder(_ name: String, _ kind: String) async throws -> SearchResultRecord {
        let results = try await group(kind)
        guard let found = results.first(where: { $0.name == name }) else {
            throw StepFailure(
                "\"\(name)\" isn't listed under \(kind); it lists \(results.map(\.name))")
        }
        selected = found
        return found
    }

    /// The files in every capability's `folder` (and capabilities/`folder` when `shared`)
    /// with the extension `fileExtension`, sorted.
    func files(in folder: String, fileExtension: String, shared: Bool) throws -> [URL] {
        let root = w.capabilities
        let names = try FileManager.default.contentsOfDirectory(atPath: root.path)
        var folders = names.filter { !$0.hasPrefix(".") && !$0.hasPrefix("_") }
            .map { root.appendingPathComponent("\($0)/\(folder)") }
        if shared { folders.insert(root.appendingPathComponent(folder), at: 0) }
        return folders.flatMap { dir in
            let found = (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []
            return found.filter { $0.hasSuffix(fileExtension) }.sorted()
                .map(dir.appendingPathComponent)
        }
    }

    /// Finds `# @helper <name>` in the shop's _shared folders and reads the comment lines
    /// under it, up to the function itself.
    fileprivate func helperDoc(_ name: String) throws -> SearchEverythingHelperDoc {
        for file in try files(in: "_shared", fileExtension: ".sh", shared: true) {
            let lines = try String(contentsOf: file, encoding: .utf8).components(separatedBy: "\n")
            for (index, line) in lines.enumerated() where line.hasPrefix("# @helper ") {
                let signature = String(line.dropFirst("# @helper ".count))
                guard signature.split(separator: " ").first.map(String.init) == name else {
                    continue
                }
                let comment = lines[(index + 1)...].prefix { $0.hasPrefix("# ") }
                    .map { String($0.dropFirst(2)) }
                return SearchEverythingHelperDoc(
                    signature: signature, description: comment.joined(separator: " "))
            }
        }
        throw StepFailure("the sample shop documents no helper \(name)")
    }

    /// The setup scripts whose text names the helper as a word, relative to the capabilities
    /// folder: "checkout/scripts/card-declined.sh".
    /// Ref: https://developer.apple.com/documentation/swift/regex
    func scriptsCalling(_ name: String) throws -> [String] {
        precondition(name.allSatisfy { $0.isLetter || $0 == "_" }, "a helper name is a word")
        let call = try Regex("\\b\(name)\\b")
        let prefix = w.capabilities.path + "/"
        return try files(in: "scripts", fileExtension: ".sh", shared: false).filter { script in
            try String(contentsOf: script, encoding: .utf8).contains(call)
        }.map { $0.path.replacingOccurrences(of: prefix, with: "") }
    }
}

@MainActor
func searchEverythingSteps(_ w: World) -> [StepDefinition] {
    let s = SearchEverythingState(w)
    return searchEverythingListSteps(s) + searchEverythingStepSteps(s)
        + searchEverythingFilterSteps(s) + searchEverythingHelperSteps(s)
}

@MainActor
private func searchEverythingListSteps(_ s: SearchEverythingState) -> [StepDefinition] {
    [
        def(#"^the results are grouped under (.+) and ([A-Z][a-z]+)$"#) { call in
            let want = call[0].components(separatedBy: ", ") + [call[1]]
            let results = try await s.w.front().searchResults()
            let got = results.groups.filter { !$0.results.isEmpty }.map(\.kind)
            guard got == want else {
                throw StepFailure("the results are grouped under \(got), not \(want)")
            }
        },
        def(#"^"([^"]*)" and "([^"]*)" are listed under ([A-Z][a-z]+)$"#) { call in
            try await s.listedUnder(call[0], call[2])
            try await s.listedUnder(call[1], call[2])
        },
        def(#"^every match is highlighted$"#) { _ in
            let results = try await s.w.front().searchResults()
            guard !results.query.isEmpty && !results.groups.isEmpty else {
                throw StepFailure("no search results to check: \(results)")
            }
            let query = results.query.lowercased()
            for group in results.groups {
                for result in group.results {
                    let shown = (result.name + "\n" + result.detail).lowercased()
                    let want = shown.components(separatedBy: query).count - 1
                    guard want > 0 && result.matches.count == want else {
                        throw StepFailure(
                            "\(group.kind) \"\(result.name)\" shows \"\(results.query)\" "
                                + "\(want) times and highlights \(result.matches)")
                    }
                    for match in result.matches where match.lowercased() != query {
                        throw StepFailure(
                            "\"\(result.name)\" highlights \"\(match)\", not \"\(query)\"")
                    }
                }
            }
        },
        def(#"^"([^"]*)" is listed under ([A-Z][a-z]+), matching "([^"]*)"$"#) { call in
            let found = try await s.listedUnder(call[0], call[1])
            guard found.detail == call[2] else {
                throw StepFailure(
                    "\"\(call[0])\" under \(call[1]) matches \"\(found.detail)\", not "
                        + "\"\(call[2])\"")
            }
        },
        def(#"^"([^"]*)" is listed under ([A-Z][a-z]+)$"#) { call in
            try await s.listedUnder(call[0], call[1])
        },
        def(#"^they see "Nothing matches "([^"]*)""$"#) { call in
            let window = try await s.w.front()
            let message = "Nothing matches \"\(call[0])\""
            try expect(window.visible, message, "the window")
            let groups = try await window.searchResults().groups
            guard groups.isEmpty else {
                throw StepFailure("\"\(message)\" is shown beside results: \(groups)")
            }
        },
    ]
}

@MainActor
private func searchEverythingStepSteps(_ s: SearchEverythingState) -> [StepDefinition] {
    [
        def(#"^the step "(.+)" is listed with "(\d+) uses?"$"#) { call in
            let steps = try await s.group("Steps")
            let found = steps.filter { $0.name == call[0] }
            guard found.count == 1, let step = found.first else {
                throw StepFailure(
                    "Steps lists \"\(call[0])\" \(found.count) times: \(steps.map(\.name))")
            }
            s.selected = step
            guard step.uses == Int(call[1]) else {
                throw StepFailure("\"\(call[0])\" is listed with \(step.uses) uses, not \(call[1])")
            }
        },
        def(#"^selecting it lists "([^"]*)" and "([^"]*)"$"#) { call in
            guard let selected = s.selected else {
                throw StepFailure("no earlier step found a result to select")
            }
            let window = try await s.w.front()
            try await window.select(selected.name)
            let got = try await window.listedScenarios().map(\.name).sorted()
            let want = call.captures.sorted()
            guard got == want else {
                throw StepFailure("selecting \"\(selected.name)\" lists \(got), not \(want)")
            }
        },
    ]
}

@MainActor
private func searchEverythingFilterSteps(_ s: SearchEverythingState) -> [StepDefinition] {
    [
        def(#"^a developer has searched for "([^"]*)"$"#) { call in
            try await s.w.openSampleShop()
            try await s.w.search(call[0])
        },
        def(#"^they choose the "([^"]*)" filter$"#) { call in
            try await s.w.front().filterSearch(call[0])
        },
        def(#"^only (stories|scenarios|steps|scripts|helpers) are listed$"#) { call in
            let results = try await s.w.front().searchResults()
            let heading = call[0].prefix(1).uppercased() + call[0].dropFirst()
            guard results.filter == heading else {
                throw StepFailure("the chosen filter is \"\(results.filter)\", not \(heading)")
            }
            guard results.groups.count == 1, let only = results.groups.first,
                only.kind == heading
            else {
                throw StepFailure("the results are \(results.groups), not only \(call[0])")
            }
            guard !only.results.isEmpty else { throw StepFailure("no \(call[0]) are listed") }
        },
        def(
            #"^scripts with "([^"]*)" in their name come before scripts that only mention it$"#
        ) { call in
            let scripts = try await s.group("Scripts").map(\.name)
            let named = scripts.map { $0.lowercased().contains(call[0].lowercased()) }
            guard let lastNamed = named.lastIndex(of: true),
                let firstMention = named.firstIndex(of: false)
            else {
                throw StepFailure("the scripts \(scripts) don't include both kinds to order")
            }
            guard lastNamed < firstMention else {
                throw StepFailure("the scripts are in the order \(scripts)")
            }
        },
    ]
}

@MainActor
private func searchEverythingHelperSteps(_ s: SearchEverythingState) -> [StepDefinition] {
    [
        def(#"^([a-z_]+) is listed under Helpers with its description$"#) { call in
            let doc = try s.helperDoc(call[0])
            let found = try await s.listedUnder(call[0], "Helpers")
            guard found.detail == doc.description else {
                throw StepFailure(
                    "\(call[0]) is listed with \"\(found.detail)\", not its description "
                        + "\"\(doc.description)\"")
            }
        },
        def(#"^selecting it shows its signature and the (\d+) scripts that call it$"#) { call in
            guard let helper = s.selected?.name else {
                throw StepFailure("no earlier step found a helper to select")
            }
            let doc = try s.helperDoc(helper)
            let scripts = try s.scriptsCalling(helper)
            guard scripts.count == Int(call[0]) else {
                throw StepFailure(
                    "the sample shop has \(scripts.count) scripts calling \(helper), not "
                        + "\(call[0]): \(scripts)")
            }
            let window = try await s.w.front()
            try await window.select(helper)
            for want in [doc.signature] + scripts {
                try expect(window.visible, want, "the window")
            }
        },
    ]
}
