import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/views against the Swift app. Shared steps are in World/.
@Suite("Views") @MainActor struct ViewsTests {
    static let capability = "views"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: viewsSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: viewsSteps)
    }
}

/// A scenario as the sample shop's files define it, read without Givens, so the steps
/// compare Givens's lists against the files themselves.
private struct ViewsDiskScenario {
    let name: String
    /// The Feature's name.
    let capability: String
    /// Tagged @error, on the scenario or its Feature.
    let error: Bool
}

/// A documented helper as its file defines it, read without Givens.
private struct ViewsDiskHelper {
    let name: String
    let signature: String
    let description: String
    /// Relative to the capabilities folder: "_shared/users.sh".
    let file: String
    var callers = 0
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class ViewsState {
    /// Bounds the lines read from one helper file, so a runaway file fails loudly.
    static let helperLinesMax = 10_000

    let w: World
    /// Scenarios a step named, so "the other N scenarios" can leave them out.
    var named: [String] = []

    init(_ w: World) { self.w = w }

    /// Every capability folder's .feature file.
    func features() throws -> [URL] {
        let root = w.capabilities
        let folders = try FileManager.default.contentsOfDirectory(atPath: root.path).sorted()
        return folders.map { root.appendingPathComponent("\($0)/\($0).feature") }
            .filter { FileManager.default.fileExists(atPath: $0.path) }
    }

    /// Every scenario on disk, compiled by tools/pickles, with its Feature's name.
    fileprivate func projectScenarios() throws -> [ViewsDiskScenario] {
        var found: [ViewsDiskScenario] = []
        for feature in try features() {
            let text = try String(contentsOf: feature, encoding: .utf8)
            let featureLine = text.components(separatedBy: "\n")
                .map { $0.trimmingCharacters(in: .whitespaces) }
                .first { $0.hasPrefix("Feature:") }
            guard let featureLine else { throw StepFailure("\(feature.path) has no Feature:") }
            let capability = featureLine.dropFirst("Feature:".count)
                .trimmingCharacters(in: .whitespaces)
            found += try Toolchain.pickles(of: feature).map { pickle in
                ViewsDiskScenario(
                    name: pickle.name, capability: capability,
                    error: pickle.tagNames.contains("@error"))
            }
        }
        guard !found.isEmpty else { throw StepFailure("the sample shop has no scenarios") }
        return found
    }

    /// What the selected view lists, by name.
    func listed() async throws -> [String: ScenarioRecord] {
        let scenarios = try await w.front().listedScenarios()
        let byName = Dictionary(scenarios.map { ($0.name, $0) }) { first, _ in first }
        guard byName.count == scenarios.count else {
            throw StepFailure("the view lists a scenario twice: \(scenarios.map(\.name))")
        }
        return byName
    }

    func listedWith(_ name: String, _ status: String) async throws {
        named.append(name)
        guard let got = try await listed()[name] else {
            throw StepFailure("\"\(name)\" isn't listed")
        }
        guard got.status == status else {
            throw StepFailure("\"\(name)\" is listed with \"\(got.status)\", not \"\(status)\"")
        }
    }

    /// Drops in, marks the Givens and Whens done and the Thens failed, and records the run,
    /// as a developer would when the app misbehaves.
    func recordFailingRun(_ name: String) async throws {
        named.append(name)
        if w.window == nil { try await w.openSampleShop() }
        try await w.dropIn(name)
        guard let session = w.session, session.status == SessionModel.ready else {
            throw StepFailure("drop in to \(name) ended \(w.session?.status ?? "nil"), not ready")
        }
        for item in session.checklist {
            try await session.mark(item.text, item.kind == "then" ? "fail" : "done")
        }
        try await session.recordRun()
    }
}

extension ViewsState {
    /// The shell files in capabilities/_shared and each capability's _shared folder.
    func helperFiles() throws -> [URL] {
        let root = w.capabilities
        let names = try FileManager.default.contentsOfDirectory(atPath: root.path).sorted()
        let folders =
            [root.appendingPathComponent("_shared")]
            + names.filter { !$0.hasPrefix("_") && !$0.hasPrefix(".") }
            .map { root.appendingPathComponent("\($0)/_shared") }
        return folders.flatMap { folder in
            let found = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
            return found.filter { $0.hasSuffix(".sh") }.sorted().map(folder.appendingPathComponent)
        }
    }

    /// Every documented helper on disk, with how many setup scripts call it.
    fileprivate func projectHelpers() throws -> [ViewsDiskHelper] {
        var helpers: [ViewsDiskHelper] = []
        for file in try helperFiles() { helpers += try helpersIn(file) }
        guard !helpers.isEmpty else {
            throw StepFailure("the sample shop has no documented helpers")
        }
        let scripts = try features().map {
            $0.deletingLastPathComponent().appendingPathComponent("scripts")
        }.flatMap { folder in
            let found = (try? FileManager.default.contentsOfDirectory(atPath: folder.path)) ?? []
            return found.filter { $0.hasSuffix(".sh") }.map(folder.appendingPathComponent)
        }
        let sources = try scripts.map { try String(contentsOf: $0, encoding: .utf8) }
        for index in helpers.indices {
            helpers[index].callers = try sources.filter { try viewsCalls($0, helpers[index].name) }
                .count
        }
        return helpers
    }

    /// One helper file's "# @helper" blocks: the signature, then the description on the
    /// comment lines below, then the function.
    fileprivate func helpersIn(_ file: URL) throws -> [ViewsDiskHelper] {
        let lines = try String(contentsOf: file, encoding: .utf8).components(separatedBy: "\n")
        let rel = file.path.replacingOccurrences(of: w.capabilities.path + "/", with: "")
        guard lines.count <= Self.helperLinesMax else {
            throw StepFailure("\(rel) has more than \(Self.helperLinesMax) lines")
        }
        let marker = "# @helper "
        return lines.enumerated().filter { $0.element.hasPrefix(marker) }.map { index, line in
            let signature = line.dropFirst(marker.count).trimmingCharacters(in: .whitespaces)
            let description = lines[(index + 1)...].prefix { $0.hasPrefix("#") }
                .map { $0.dropFirst().trimmingCharacters(in: .whitespaces) }
            return ViewsDiskHelper(
                name: String(signature.split(separator: " ").first ?? ""), signature: signature,
                description: description.joined(separator: " "), file: rel)
        }
    }

    /// Pairs each helper on disk with the one the Shared view lists for it.
    fileprivate func shownHelpers() async throws -> [(ViewsDiskHelper, ViewHelperRecord)] {
        let onDisk = try projectHelpers()
        let shown = try await w.front().viewHelpers()
        guard shown.count == onDisk.count else {
            throw StepFailure(
                "Shared lists \(shown.count) helpers, not the \(onDisk.count) documented: "
                    + "\(shown.map(\.name))")
        }
        return try onDisk.map { want in
            guard let got = shown.first(where: { $0.name == want.name && $0.file == want.file })
            else {
                throw StepFailure(
                    "Shared doesn't list \(want.name) from \(want.file); it lists "
                        + "\(shown.map { "\($0.name) (\($0.file))" })")
            }
            return (want, got)
        }
    }

    /// Copies the sample shop and takes @error off every scenario and Feature, dropping a
    /// tag line left with no tags, so no capability has an error state.
    func removeErrorTags() throws {
        try w.copySampleShop()
        for feature in try features() {
            let lines = try String(contentsOf: feature, encoding: .utf8)
                .components(separatedBy: "\n")
            let kept = lines.compactMap { line -> String? in
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                guard trimmed.hasPrefix("@") else { return line }
                let tags = trimmed.split(separator: " ").filter { $0 != "@error" }
                guard !tags.isEmpty else { return nil }
                return String(line.prefix { $0 == " " || $0 == "\t" }) + tags.joined(separator: " ")
            }
            try kept.joined(separator: "\n").write(to: feature, atomically: true, encoding: .utf8)
        }
    }
}

/// True when a line of `source`, before any comment, calls `name` as a word.
/// Ref: https://developer.apple.com/documentation/swift/regex
private func viewsCalls(_ source: String, _ name: String) throws -> Bool {
    precondition(name.allSatisfy { $0.isLetter || $0 == "_" }, "a helper name is a word")
    // \b keeps seed_shopper from matching seed_shopper_admin.
    let call = try Regex("\\b\(name)\\b")
    return source.components(separatedBy: "\n").contains { line in
        String(line.prefix { $0 != "#" }).contains(call)
    }
}

@MainActor
func viewsSteps(_ w: World) -> [StepDefinition] {
    let s = ViewsState(w)
    return viewsErrorSteps(s) + viewsUnverifiedSteps(s) + viewsHelperSteps(s)
}

@MainActor
private func viewsErrorSteps(_ s: ViewsState) -> [StepDefinition] {
    [
        def(#"^they select the "([^"]*)" view$"#) { call in
            try await s.w.front().select(call[0])
        },
        // Compares the listed scenarios with every @error scenario on disk, which spans more
        // than one capability in the sample shop.
        def(#"^only error states are listed, from every capability$"#) { _ in
            let onDisk = try s.projectScenarios()
            let listed = try await s.listed()
            for scenario in onDisk where scenario.error != (listed[scenario.name] != nil) {
                throw StepFailure(
                    scenario.error
                        ? "error state \"\(scenario.name)\" (\(scenario.capability)) isn't listed"
                        : "happy path \"\(scenario.name)\" is listed among error states")
            }
            let errors = onDisk.filter(\.error)
            guard listed.count == errors.count else {
                throw StepFailure(
                    "\(listed.count) scenarios are listed, not the \(errors.count) error states")
            }
            let capabilities = Set(errors.map(\.capability))
            guard capabilities.count >= 2 else {
                throw StepFailure(
                    "the sample shop's error states come from \(capabilities.count) capability")
            }
        },
        def(#"^each one shows which capability it belongs to$"#) { _ in
            let listed = try await s.listed()
            guard !listed.isEmpty else { throw StepFailure("nothing is listed") }
            for scenario in try s.projectScenarios() {
                if let got = listed[scenario.name], got.capability != scenario.capability {
                    throw StepFailure(
                        "\"\(scenario.name)\" shows capability \"\(got.capability)\", not "
                            + "\"\(scenario.capability)\"")
                }
            }
        },
        def(#"^a developer has a project open in Givens whose scenarios are all happy paths$"#) {
            _ in
            try s.removeErrorTags()
            if let left = try s.projectScenarios().first(where: \.error) {
                throw StepFailure("\"\(left.name)\" is still an error state")
            }
            try await s.w.openSampleShop()
        },
    ]
}

@MainActor
private func viewsUnverifiedSteps(_ s: ViewsState) -> [StepDefinition] {
    [
        def(#"^"([^"]*)" has a failing run recorded$"#) { call in
            try await s.recordFailingRun(call[0])
        },
        def(#"^"([^"]*)" isn't listed$"#) { call in
            s.named.append(call[0])
            let listed = try await s.listed()
            guard !listed.isEmpty else {
                throw StepFailure("nothing is listed, so the view wasn't worked out")
            }
            guard listed[call[0]] == nil else {
                throw StepFailure("\"\(call[0])\" is listed, and shouldn't be")
            }
        },
        def(#"^"([^"]*)" is listed with "([^"]*)"$"#) { call in
            try await s.listedWith(call[0], call[1])
        },
        // Checks every scenario on disk that no earlier step named.
        def(#"^the other (\d+) scenarios are listed with "([^"]*)"$"#) { call in
            let named = s.named
            let others = try s.projectScenarios().map(\.name).filter { !named.contains($0) }
            guard others.count == Int(call[0]) else {
                throw StepFailure(
                    "the sample shop has \(others.count) other scenarios, not \(call[0])")
            }
            let listed = try await s.listed()
            for name in others {
                guard let got = listed[name] else { throw StepFailure("\"\(name)\" isn't listed") }
                guard got.status == call[1] else {
                    throw StepFailure(
                        "\"\(name)\" is listed with \"\(got.status)\", not \"\(call[1])\"")
                }
            }
            let namedListed = listed.keys.filter { named.contains($0) }.count
            guard listed.count == others.count + namedListed else {
                throw StepFailure(
                    "the view lists \(listed.count) scenarios, not \(others.count) others and "
                        + "\(namedListed) named")
            }
        },
    ]
}

@MainActor
private func viewsHelperSteps(_ s: ViewsState) -> [StepDefinition] {
    [
        def(#"^they select "Shared" in the sidebar$"#) { _ in
            try await s.w.front().select("Shared")
        },
        def(#"^every documented helper is listed with its signature and description$"#) { _ in
            for (want, got) in try await s.shownHelpers() {
                guard got.signature == want.signature else {
                    throw StepFailure(
                        "\(want.name) shows the signature \"\(got.signature)\", not "
                            + "\"\(want.signature)\"")
                }
                guard got.description == want.description else {
                    throw StepFailure(
                        "\(want.name) shows the description \"\(got.description)\", not "
                            + "\"\(want.description)\"")
                }
            }
        },
        def(#"^each one shows the file it's defined in and how many scripts call it$"#) { _ in
            for (want, got) in try await s.shownHelpers() where got.callers != want.callers {
                throw StepFailure(
                    "\(want.name) (\(want.file)) shows \(got.callers) calling scripts, not "
                        + "\(want.callers)")
            }
        },
    ]
}
