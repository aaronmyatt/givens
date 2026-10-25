import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/share-setup-between-scenarios against the Swift app: helpers in _shared
/// folders, the sidebar's Shared list, and setup fingerprints. Shared steps are in World/.
@Suite("Share setup between scenarios") @MainActor struct ShareSetupBetweenScenariosTests {
    static let capability = "share-setup-between-scenarios"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: shareSetupBetweenScenariosSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: shareSetupBetweenScenariosSteps)
    }
}

/// What this capability's steps keep between steps of one scenario, and the checks that read
/// the sample shop's helper files the way bash would.
@MainActor
final class ShareSetupBetweenScenariosState {
    /// Checkout's own seed_shopper; its ok line tells it apart from the shared one.
    static let checkoutShopper = """
        # shellcheck shell=bash

        # @helper seed_shopper [email]
        # Seeds a shopper with Checkout's own test card.
        seed_shopper() {
          ok "seeded Checkout's own shopper"
        }

        """

    let w: World
    /// The scenarios a "both" step names.
    var pair: [String] = []
    /// The helper a step selected, edited or changed.
    var helper = ""
    /// The install hint on the need line of a helper a scenario calls.
    var hint = ""

    init(_ w: World) { self.w = w }

    /// The sample shop's helper files, relative to its capabilities folder: the project's
    /// _shared/*.sh, then each capability's, leaving out the kit.
    func sharedFiles() throws -> [String] {
        let manager = FileManager.default
        let root = w.capabilities.path
        let shared = (try? manager.contentsOfDirectory(atPath: root + "/_shared")) ?? []
        var files = shared.sorted().map { "_shared/" + $0 }
        let folders = try manager.contentsOfDirectory(atPath: root).sorted()
        for folder in folders where !folder.hasPrefix("_") && !folder.hasPrefix(".") {
            let own = (try? manager.contentsOfDirectory(atPath: "\(root)/\(folder)/_shared")) ?? []
            files += own.sorted().map { "\(folder)/_shared/\($0)" }
        }
        return files.filter { $0.hasSuffix(".sh") && !$0.hasSuffix("/givens.sh") }
    }

    /// True when `line` is the `name() {` that starts a helper.
    static func defines(_ line: String, _ helper: String) -> Bool {
        let pattern = "^" + NSRegularExpression.escapedPattern(for: helper) + #"\(\)\s*\{\s*$"#
        return line.range(of: pattern, options: .regularExpression) != nil
    }

    /// True when a line of shell, outside a comment, uses `word` as a whole word.
    /// Ref: https://developer.apple.com/documentation/foundation/nsregularexpression
    static func callsWord(_ text: String, _ word: String) -> Bool {
        let pattern = #"\b"# + NSRegularExpression.escapedPattern(for: word) + #"\b"#
        return text.components(separatedBy: "\n").contains { line in
            let code = line.split(separator: "#", omittingEmptySubsequences: false).first ?? ""
            return code.range(of: pattern, options: .regularExpression) != nil
        }
    }

    /// The helper files that define `helper`.
    func definers(of helper: String) throws -> [String] {
        try sharedFiles().filter { file in
            let text = try w.readFile("capabilities/" + file)
            return text.components(separatedBy: "\n").contains { Self.defines($0, helper) }
        }
    }

    /// The lines between `helper() {` and its closing `}`, and the file defining it relative
    /// to the sample shop. Exactly one file must define it.
    func body(of helper: String) throws -> (body: String, path: String) {
        let found = try definers(of: helper)
        guard found.count == 1 else {
            throw StepFailure("expected one file to define \(helper), got \(found)")
        }
        let path = "capabilities/" + found[0]
        let lines = try w.readFile(path).components(separatedBy: "\n")
        guard let start = lines.firstIndex(where: { Self.defines($0, helper) }),
            let end = lines[start...].firstIndex(of: "}")
        else { throw StepFailure("\(helper) in \(path) has no closing brace") }
        return (lines[(start + 1)..<end].joined(separator: "\n"), path)
    }

    /// The line, counting from 1, of `helper() {` in `file`, relative to the capabilities
    /// folder.
    func definitionLine(_ file: String, _ helper: String) throws -> Int {
        let text = try w.readFile("capabilities/" + file)
        let lines = text.components(separatedBy: "\n")
        guard let at = lines.firstIndex(where: { Self.defines($0, helper) }) else {
            throw StepFailure("expected \(file) to define \(helper), got:\n\(text)")
        }
        return at + 1
    }

    /// The setup scripts that call `helper`, sorted, relative to the capabilities folder.
    func callers(of helper: String) throws -> [String] {
        let manager = FileManager.default
        let root = w.capabilities.path
        var callers: [String] = []
        for folder in try manager.contentsOfDirectory(atPath: root) where !folder.hasPrefix(".") {
            let scripts = (try? manager.contentsOfDirectory(atPath: "\(root)/\(folder)/scripts"))
            for script in scripts ?? [] where script.hasSuffix(".sh") {
                let relative = "\(folder)/scripts/\(script)"
                if Self.callsWord(try w.readFile("capabilities/" + relative), helper) {
                    callers.append(relative)
                }
            }
        }
        return callers.sorted()
    }

    /// The Shared list's entries for a helper; it's a failure when there's none.
    func helpersNamed(_ name: String) async throws -> [SharedHelperRecord] {
        let all = try await w.front().helpers()
        let named = all.filter { $0.name == name }
        guard !named.isEmpty else { throw StepFailure("\(name) isn't in the Shared list: \(all)") }
        return named
    }

    /// The Drop in this scenario started.
    func session() throws -> SessionModel {
        guard let session = w.session else { throw StepFailure("no Drop in has started") }
        return session
    }

    func chooseOnHelper(_ name: String, _ action: String) async throws {
        let window = try await w.front()
        helper = name
        try await window.select(name)
        try await window.choose(action)
    }

    /// The first match of `pattern` in `text`, as its capture groups.
    static func captures(_ pattern: String, in text: String) throws -> [String]? {
        let regex = try NSRegularExpression(pattern: pattern)
        let range = NSRange(text.startIndex..., in: text)
        guard let match = regex.firstMatch(in: text, range: range) else { return nil }
        return (1..<match.numberOfRanges).map { index in
            Range(match.range(at: index), in: text).map { String(text[$0]) } ?? ""
        }
    }
}

@MainActor
func shareSetupBetweenScenariosSteps(_ w: World) -> [StepDefinition] {
    let s = ShareSetupBetweenScenariosState(w)
    return shareSetupHelperGivens(s) + shareSetupRunGivens(s) + shareSetupChanges(s)
        + shareSetupHelperChecks(s) + shareSetupStatusChecks(s) + shareSetupEditorChecks(s)
}

@MainActor
private func shareSetupHelperGivens(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the "([^"]*)" setup calls ([a-z_]+), which only Checkout's helpers define$"#) {
            call in
            try w.copySampleShop()
            let found = try s.definers(of: call[1])
            guard found.count == 1, found[0].hasPrefix("checkout/_shared/") else {
                throw StepFailure("expected only Checkout's helpers to define \(call[1]): \(found)")
            }
            try w.setupStartsWith(call[0], call[1])
        },
        def(#"^Checkout's helpers define a seed_shopper of their own$"#) { _ in
            try w.copySampleShop()
            try w.writeFile(
                "capabilities/checkout/_shared/shopper.sh",
                ShareSetupBetweenScenariosState.checkoutShopper)
        },
        def(#"^two files in _shared define seed_shopper$"#) { _ in
            try w.copySampleShop()
            let found = try s.definers(of: "seed_shopper")
            guard found == ["_shared/users.sh"] else {
                throw StepFailure("expected only _shared/users.sh to define seed_shopper: \(found)")
            }
            let people = ShareSetupBetweenScenariosState.checkoutShopper
                .replacingOccurrences(of: "Checkout's own", with: "people.sh's")
            try w.writeFile("capabilities/_shared/people.sh", people)
        },
        def(#"^([a-z_]+) calls ([a-z_]+)$"#) { call in
            try w.copySampleShop()
            let body = try s.body(of: call[0]).body
            guard ShareSetupBetweenScenariosState.callsWord(body, call[1]) else {
                throw StepFailure("\(call[0]) doesn't call \(call[1]); its body is:\n\(body)")
            }
        },
        def(#"^"([^"]*)" calls ([a-z_]+), which runs (\S+) with ([a-z]+)$"#) { call in
            // Checks the sample shop as written, and keeps the need line's install hint.
            try w.copySampleShop()
            let (scenario, helper, file, command) = (call[0], call[1], call[2], call[3])
            let script = try w.scriptPath(scenario)
            let text = try w.readFile(script)
            guard ShareSetupBetweenScenariosState.callsWord(text, helper) else {
                throw StepFailure("\(script) doesn't call \(helper); it holds:\n\(text)")
            }
            let body = try s.body(of: helper).body
            guard body.contains(#"\#(command) "$GIVENS_ROOT/\#(file)""#) else {
                throw StepFailure("\(helper) doesn't run \(file) with \(command):\n\(body)")
            }
            let need = "need " + NSRegularExpression.escapedPattern(for: command) + #" "([^"]*)""#
            guard let hint = try ShareSetupBetweenScenariosState.captures(need, in: body) else {
                throw StepFailure("\(helper) has no need \(command) line; its body is:\n\(body)")
            }
            s.hint = hint[0]
            _ = try w.readFile("capabilities/" + file)
        },
    ]
}

@MainActor
private func shareSetupRunGivens(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" and "([^"]*)" both have (?:a passing run|passing runs) recorded$"#) {
            call in
            s.pair = [call[0], call[1]]
            for scenario in s.pair { try await w.recordPassingRun(scenario) }
        },
        def(#"^a developer is editing ([a-z_]+) in Givens$"#) { call in
            try w.copySampleShop()
            try await s.chooseOnHelper(call[0], "Edit")
        },
        def(#"^they select ([a-z_]+) under Shared and choose "([^"]*)"$"#) { call in
            try await s.chooseOnHelper(call[0], call[1])
        },
        def(#"^they choose "New helper" in (\S+) and name it "([a-z_]+)"$"#) { call in
            s.helper = call[1]
            try await w.front().newHelper(capability: call[0], name: call[1])
        },
        def(#"^they save it with its closing brace missing$"#) { _ in
            let window = try await w.front()
            let path = try await window.editingHelper().path
            guard !path.isEmpty else { throw StepFailure("no helper is open in the editor") }
            var lines = try String(contentsOfFile: path, encoding: .utf8)
                .components(separatedBy: "\n")
            let helper = s.helper
            guard
                let start = lines.firstIndex(where: {
                    ShareSetupBetweenScenariosState.defines($0, helper)
                }),
                let end = lines[start...].firstIndex(of: "}")
            else { throw StepFailure("the open file has no \(helper) with a closing brace") }
            lines.remove(at: end)
            let edited = lines.joined(separator: "\n")
            w.editedPath = URL(fileURLWithPath: path)
            w.editedText = edited
            try await window.saveHelper(edited)
        },
    ]
}

@MainActor
private func shareSetupChanges(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^([a-z_]+) changes on disk$"#) { call in
            // Adds a line to the start of the helper's body, in the one file defining it.
            let path = try s.body(of: call[0]).path
            let text = try w.readFile(path)
            let header = call[0] + "() {\n"
            guard text.components(separatedBy: header).count == 2 else {
                throw StepFailure("expected one \"\(header)\" in \(path)")
            }
            let changed = text.replacingOccurrences(
                of: header, with: header + "  export SHOP_SEED_REVISION=2\n")
            try w.writeFile(path, changed)
        },
        def(#"^"([^"]*)" is changed to "([^"]*)"$"#) { call in
            let path = "capabilities/checkout/checkout.feature"
            let text = try w.readFile(path)
            guard text.components(separatedBy: call[0]).count == 2 else {
                throw StepFailure("expected \"\(call[0])\" once in \(path)")
            }
            try w.writeFile(path, text.replacingOccurrences(of: call[0], with: call[1]))
        },
        def(#"^the "([^"]*)" setup script changes$"#) { call in
            let script = try w.scriptPath(call[0])
            try w.writeFile(
                script, try w.readFile(script) + "ok \"set the cart's currency to GBP\"\n")
        },
        def(#"^the sample shop's kit is updated to the next version$"#) { _ in
            // Rewrites the version line and adds a function, as a real update would.
            let path = "capabilities/_shared/givens.sh"
            guard let text = try? w.readFile(path) else {
                throw StepFailure("the sample shop has no kit: internal/kit/givens.sh isn't built")
            }
            var lines = text.components(separatedBy: "\n")
            let prefix = "# givens kit "
            guard lines[0].hasPrefix(prefix), let version = Int(lines[0].dropFirst(prefix.count))
            else { throw StepFailure("expected the kit's version line: \(lines[0])") }
            lines[0] = prefix + String(version + 1)
            try w.writeFile(
                path, lines.joined(separator: "\n") + "\n_givens_next_version() { :; }\n")
        },
    ]
}

@MainActor
private func shareSetupHelperChecks(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^setup fails with "([^"]*)"$"#) { call in
            let session = try s.session()
            guard session.status == SessionModel.failed else {
                throw StepFailure("expected setup to fail, but the Drop in is \(session.status)")
            }
            try expect(session.output, call[0], "the setup's output")
        },
        def(#"^Givens points out that ([a-z_]+) is defined in (\S+)$"#) { call in
            // One visible line names both the helper and the file defining it.
            let visible = try await w.front().visible
            guard visible.contains(where: { $0.contains(call[0]) && $0.contains(call[1]) }) else {
                throw StepFailure("no visible line names \(call[0]) and \(call[1]): \(visible)")
            }
        },
        def(#"^Checkout's seed_shopper runs in place of the shared one$"#) { _ in
            let session = try s.session()
            guard session.status == SessionModel.ready else {
                throw StepFailure("expected setup to finish, but the Drop in is \(session.status)")
            }
            try expect(session.output, "✓ seeded Checkout's own shopper", "the output")
            try expectNot(session.output, "with a saved card", "the output")
        },
        def(#"^no warning is shown$"#) { _ in
            let output = try s.session().output
            if let line = output.first(where: {
                $0.trimmingCharacters(in: .whitespaces).hasPrefix("!")
            }) {
                throw StepFailure("expected no warning in the output, got \"\(line)\"")
            }
            try expectNot(try await w.front().visible, "defined in both", "the window")
        },
        def(#"^the Shared list marks it "([^"]*)"$"#) { call in
            let helpers = try await s.helpersNamed("seed_shopper")
            guard
                helpers.contains(where: {
                    $0.definedIn == "checkout/_shared/shopper.sh" && $0.overrides == call[0]
                })
            else {
                throw StepFailure(
                    "checkout/_shared/shopper.sh's seed_shopper isn't marked \"\(call[0])\": "
                        + "\(helpers)")
            }
        },
        def(#"^the output warns "([^"]*)"$"#) { call in
            try expect(try s.session().output, call[0], "the setup's output")
        },
        def(#"^both definitions are flagged in the Shared list$"#) { _ in
            let helpers = try await s.helpersNamed("seed_shopper")
            let flagged = helpers.filter { !$0.warning.isEmpty }.map(\.definedIn).sorted()
            let want = ["_shared/people.sh", "_shared/users.sh"]
            guard flagged == want else {
                throw StepFailure("expected \(want) flagged in the Shared list, got \(flagged)")
            }
        },
    ]
}

@MainActor
private func shareSetupStatusChecks(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" still shows as verified$"#) { call in
            let status = try await w.front().scenario(call[0]).status
            guard status.hasPrefix("Verified") else {
                throw StepFailure("expected \"\(call[0])\" to show as verified, got \"\(status)\"")
            }
        },
        def(#"^both show "([^"]*)"$"#) { call in
            guard s.pair.count == 2 else { throw StepFailure("no step named the two scenarios") }
            for name in s.pair { try await w.shows(name, call[0]) }
        },
        def(#"^if ([a-z]+) isn't installed, it's marked "([^"]*)" with its install hint$"#) {
            call in
            // Checks what Givens says against this Mac: marked with its hint when the command
            // is missing, unmarked when it's on PATH.
            // Ref: https://www.gnu.org/software/bash/manual/bash.html#index-command
            let (command, mark) = (call[0], call[1])
            let window = try await w.front()
            let required = try await window.requiredCommands(window.selected)
            guard let got = required.first(where: { $0.command == command }) else {
                throw StepFailure("\(command) isn't among the commands required: \(required)")
            }
            let lookup = try Shell.run("command -v \(Shell.quote(command))", in: w.dir)
            let installed = lookup.status == 0
            guard got.installed == installed, got.hint == s.hint else {
                throw StepFailure(
                    "expected \(command) installed=\(installed) with hint \"\(s.hint)\": \(got)")
            }
            if installed {
                try expectNot(window.visible, mark, "the window")
            } else {
                try expect(window.visible, mark, "the window")
                try expect(window.visible, s.hint, "the window")
            }
        },
    ]
}

@MainActor
private func shareSetupEditorChecks(_ s: ShareSetupBetweenScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^(\S+) opens in the editor at ([a-z_]+)$"#) { call in
            let window = try await w.front()
            let want = w.capabilities.appendingPathComponent(call[0]).path
            let edit = try await window.editingHelper()
            guard !edit.path.isEmpty, samePath(edit.path, want),
                !window.editing.isEmpty, samePath(window.editing, want)
            else {
                throw StepFailure(
                    "expected \(want) in the editor, got \"\(window.editing)\" "
                        + "(helper \"\(edit.path)\")")
            }
            let line = try s.definitionLine(call[0], call[1])
            guard edit.line == line else {
                throw StepFailure("expected the editor at line \(line), got line \(edit.line)")
            }
        },
        def(#"^the (\d+) scripts that call it are listed beside the editor$"#) { call in
            let want = try s.callers(of: s.helper)
            guard String(want.count) == call[0] else {
                throw StepFailure(
                    "the sample shop has \(want.count) callers, not \(call[0]): \(want)")
            }
            let got = try await w.front().editingHelper().callers.sorted()
            guard got == want else {
                throw StepFailure("expected \(want) beside the editor, got \(got)")
            }
        },
        def(#"^(\S+) is created with a # @helper comment and an empty ([a-z_]+) function$"#) {
            call in
            let text = try w.readFile("capabilities/" + call[0])
            guard text.components(separatedBy: "\n").contains(where: { $0.hasPrefix("# @helper") })
            else { throw StepFailure("no # @helper comment in \(call[0]):\n\(text)") }
            _ = try s.definitionLine(call[0], call[1])
            // Bash must load the file, and calling the function must print nothing.
            let path = w.capabilities.appendingPathComponent(call[0]).path
            let result = try Shell.run("source \(Shell.quote(path)) && \(call[1])", in: w.dir)
            guard result.status == 0, result.output.isEmpty else {
                throw StepFailure("\(call[1]) didn't load and do nothing: \(result)")
            }
        },
        def(#"^([a-z_]+) is listed under Shared as one of (\S+)'s helpers$"#) { call in
            let helpers = try await s.helpersNamed(call[0])
            let want = "\(call[1].lowercased())/_shared/\(call[0]).sh"
            guard helpers.contains(where: { $0.capability == call[1] && $0.definedIn == want })
            else {
                throw StepFailure("expected \(call[0]) as \(call[1])'s, in \(want): \(helpers)")
            }
        },
        def(#"^the line bash reports is marked with its syntax error$"#) { _ in
            try await shareSetupSyntaxMarked(s)
        },
    ]
}

/// Runs `bash -n` on the edited text itself, so the expected line is bash's own.
/// Ref: https://www.gnu.org/software/bash/manual/bash.html#The-Set-Builtin
@MainActor
private func shareSetupSyntaxMarked(_ s: ShareSetupBetweenScenariosState) async throws {
    let copy = s.w.dir.appendingPathComponent("edited-helper.sh")
    try s.w.editedText.write(to: copy, atomically: true, encoding: .utf8)
    let report = try Shell.run("bash -n \(Shell.quote(copy.path))", in: s.w.dir)
    let pattern = #"line (\d+): (syntax error.*)"#
    guard report.status != 0,
        let found = try ShareSetupBetweenScenariosState.captures(pattern, in: report.output),
        let line = Int(found[0])
    else { throw StepFailure("bash reports no syntax error in the edit: \(report.output)") }
    let problems = try await s.w.front().editingHelper().problems
    guard problems.contains(where: { $0.line == line && $0.message.contains("syntax error") })
    else { throw StepFailure("expected line \(line) marked \"\(found[1])\", got \(problems)") }
}
