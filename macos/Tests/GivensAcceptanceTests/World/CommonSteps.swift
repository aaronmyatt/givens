import Foundation
import GherkinKit
import GivensUI

/// Makes a step definition. Capability suites use it so their definitions read as a table.
@MainActor
func def(
    _ pattern: String, _ body: @escaping @MainActor @Sendable (StepCall) async throws -> Void
) -> StepDefinition {
    StepDefinition(pattern, body)
}

/// The kit's source line every setup script starts with.
let kitSource = #"source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh""#

/// The markdown setup the spec uses for "Pay in app".
let payInAppSetup = """
    You'll need the TestFlight build of the shop app.

    1. Install it on your phone: [Shop beta](https://testflight.apple.com/join/abc)
    2. Sign in as `sam@example.com`
    3. Add the test card `4242 4242 4242 4242` under Wallet

       ![Wallet screen](../images/wallet.png)

    """

/// Every step used by more than one capability; the same patterns as Common in
/// acceptance/world/common.go. A capability suite must not define a pattern that also
/// matches one of these; its stepsDefined test reports it if it does.
@MainActor
func commonSteps(_ w: World) -> [StepDefinition] {
    [
        def(#"^a developer has the sample shop project$"#) { _ in try w.copySampleShop() },
        def(#"^a developer has the sample shop project open in Givens$"#) { _ in
            try await w.openSampleShop()
        },
        def(#"^a developer opens the sample shop in Givens$"#) { _ in try await w.openSampleShop()
        },
        def(#"^they see "([^"]*)"$"#) {
            try expect(try await w.front().visible, $0[0], "the window")
        },
        def(#"^they're offered "([^"]*)"(?: and "([^"]*)")?$"#) {
            try await w.offered($0[0], $0[1])
        },
        def(#"^they can choose "([^"]*)" or "([^"]*)"$"#) { try await w.offered($0[0], $0[1]) },
        def(#"^(?:they choose|a developer chooses) "([^"]*)"$"#) {
            try await w.front().choose($0[0])
        },
        def(#"^(?:they select|a developer selects) "([^"]*)"$"#) {
            try await w.front().select($0[0])
        },
        def(#"^"([^"]*)" is selected$"#) { try await w.front().select($0[0]) },
        def(#"^(?:they drop|a developer drops) in to "([^"]*)"$"#) { try await w.dropIn($0[0]) },
        def(#"^(?:they run|a developer runs) "([^"]*)"$"#) { try w.run($0[0]) },
        def(#"^it prints "([^"]*)"$"#) {
            try expect(w.output.components(separatedBy: "\n"), $0[0], "the command's output")
        },
        def(#"^"([^"]*)" shows "([^"]*)"$"#) { try await w.shows($0[0], $0[1]) },
        def(#"^"([^"]*)" has a passing run recorded$"#) { try await w.recordPassingRun($0[0]) },
        def(#"^they press ⌘K and type "([^"]*)"$"#) { try await w.search($0[0]) },
        def(#"^the clipboard holds "([^"]*)"$"#) { call in
            let got = w.clipboard.readText()
            guard got == call[0] else {
                throw StepFailure("the clipboard holds \"\(got)\", not \"\(call[0])\"")
            }
        },
        def(#"^the file is saved anyway$"#) { _ in try w.savedAnyway() },
        def(#"^it opens in the editor$"#) { _ in
            guard try await !w.front().editing.isEmpty else {
                throw StepFailure("nothing is open in the editor")
            }
        },
        def(#"^the "([^"]*)" setup starts with "([^"]*)"$"#) {
            try w.setupStartsWith($0[0], $0[1])
        },
        def(#"^checkout\.feature has the line "([^"]*)" among the steps of "([^"]*)"$"#) {
            try w.lineAmongSteps($0[0], $0[1])
        },
        def(#"^"([^"]*)" has only scripts/([a-z0-9-]+)\.md$"#) {
            try w.onlyMarkdownSetup($0[0], $0[1])
        },
    ]
}

extension World {
    func offered(_ first: String, _ second: String) async throws {
        let offers = try await front().offers
        try expect(offers, first, "the offered actions")
        if !second.isEmpty { try expect(offers, second, "the offered actions") }
    }

    func shows(_ name: String, _ text: String) async throws {
        let scenario = try await front().scenario(name)
        try expect([scenario.status, scenario.warning], text, "\(name)'s status")
    }

    func savedAnyway() throws {
        guard let editedPath else { throw StepFailure("no step edited a file in this scenario") }
        let saved = try String(contentsOf: editedPath, encoding: .utf8)
        guard saved == editedText else { throw StepFailure("\(editedPath.path) wasn't saved") }
    }

    /// Which sample shop capability holds the named scenario.
    func scenarioCapability(_ name: String) throws -> String {
        let folders = try FileManager.default.contentsOfDirectory(atPath: capabilities.path)
        for folder in folders.sorted() where !folder.hasPrefix("_") && !folder.hasPrefix(".") {
            let feature = capabilities.appendingPathComponent("\(folder)/\(folder).feature")
            guard FileManager.default.fileExists(atPath: feature.path) else { continue }
            if try Toolchain.pickles(of: feature).contains(where: { $0.name == name }) {
                return folder
            }
        }
        throw StepFailure("the sample shop has no scenario \"\(name)\"")
    }

    /// The named scenario's setup script, relative to the sample shop.
    func scriptPath(_ name: String) throws -> String {
        "capabilities/\(try scenarioCapability(name))/scripts/\(slug(name)).sh"
    }

    /// Makes `line` the first line after the kit's source line in a setup script.
    func setupStartsWith(_ name: String, _ line: String) throws {
        precondition(!line.contains("\n"), "the step gives one line")
        try copySampleShop()
        let path = try scriptPath(name)
        let existing = (try? readFile(path)) ?? ""
        let body =
            existing.hasPrefix(kitSource + "\n")
            ? String(existing.dropFirst(kitSource.count + 1)) : existing
        try writeFile(path, kitSource + "\n" + line + "\n" + body)
    }

    /// Adds `line` as the last step of a checkout scenario.
    func lineAmongSteps(_ line: String, _ name: String) throws {
        try copySampleShop()
        let path = "capabilities/checkout/checkout.feature"
        var lines = try readFile(path).components(separatedBy: "\n")
        guard let start = lines.firstIndex(of: "  Scenario: \(name)") else {
            throw StepFailure("checkout.feature has no scenario \"\(name)\"")
        }
        var end = start + 1
        while end < lines.count && lines[end].hasPrefix("    ") { end += 1 }  // bound: lines
        lines.insert("    " + line, at: end)
        try writeFile(path, lines.joined(separator: "\n"))
    }

    /// Gives a checkout scenario only a markdown setup.
    func onlyMarkdownSetup(_ name: String, _ scriptSlug: String) throws {
        precondition(slug(name) == scriptSlug, "the script is named after the scenario")
        try copySampleShop()
        try ensureScenario("checkout", name)
        let scripts = capabilities.appendingPathComponent("checkout/scripts")
        try? FileManager.default.removeItem(at: scripts.appendingPathComponent("\(scriptSlug).sh"))
        try writeFile("capabilities/checkout/scripts/\(scriptSlug).md", payInAppSetup)
    }

    /// Adds a three-step scenario named `name` to a sample shop capability, unless it has one.
    func ensureScenario(_ capability: String, _ name: String) throws {
        precondition(!capability.contains("/"), "capability is a slug")
        let path = "capabilities/\(capability)/\(capability).feature"
        let text = try readFile(path)
        if text.contains("  Scenario: \(name)\n") { return }
        let scenario =
            "\n  Scenario: \(name)\n"
            + "    Given a shopper has a saved card on their phone\n"
            + "    When they tap \"Pay\" in the app\n"
            + "    Then they see \"Payment sent\"\n"
        let trimmed = text.replacingOccurrences(
            of: "\\n+$", with: "", options: .regularExpression)
        try writeFile(path, trimmed + "\n" + scenario)
    }
}
