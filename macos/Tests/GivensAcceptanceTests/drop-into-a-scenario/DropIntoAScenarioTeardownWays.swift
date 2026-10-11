import Foundation
import GherkinKit
import GivensUI

// Teardown scripts: run before every way into setup, and stopping setup when they fail.

extension DropIntoAScenarioState {
    /// Gives the scenario a teardown that prints one line, and a setup that fails while
    /// shop-data/fail-setup exists, so a step can reach "Try again".
    func hasTeardown(_ name: String, _ file: String) async throws {
        scenario = name
        try writeTeardown(name, Self.script(#"ok "cleared orders""#))
        let path = try teardownPath(name)
        guard path.hasSuffix("/" + file) else {
            throw StepFailure("\"\(name)\"'s teardown is \(path), not \(file)")
        }
        try writeSetup(
            name,
            Self.script(
                #"ok "seeded shopper""#,
                "if [ -f " + Self.shopData
                    + #"/fail-setup" ]; then fail "card service is down"; fi"#
            ))
        try await prepare()
    }

    /// Drops in, chooses "Reset state", then makes setup fail once to reach "Try again",
    /// keeping the output after each.
    func everyWayIn() async throws {
        outputs["drop in"] = try await dropInTo(scenario, SessionModel.ready).output
        try await chooseAndKeep("Reset state", SessionModel.ready)
        let failMarker = Self.shopDataFolder + "/fail-setup"
        try w.writeFile(failMarker, "")
        try await chooseAndKeep("Reset state", SessionModel.failed)
        guard let project = w.project else { throw StepFailure("no sample shop") }
        try FileManager.default.removeItem(at: project.appendingPathComponent(failMarker))
        try await chooseAndKeep("Try again", SessionModel.ready)
    }

    /// Takes an action, waits for setup to end in `want`, and keeps its output when ready.
    func chooseAndKeep(_ action: String, _ want: String) async throws {
        try await w.front().choose(action)
        do {
            let session = try await settleTo(want)
            if want == SessionModel.ready { outputs[action] = session.output }
        } catch let failure as StepFailure {
            throw StepFailure("after \"\(action)\": \(failure.description)")
        }
    }

    func teardownFirst() throws {
        guard outputs.count == 3 else { throw StepFailure("only \(outputs.count) ways in tried") }
        for (way, output) in outputs {
            guard let at = dropInIndex(output, "→ teardown"), at + 1 < output.count,
                output[at + 1].contains(Self.teardownLine)
            else {
                throw StepFailure(
                    "after \(way), \"→ teardown\" isn't followed by the teardown's line: \(output)")
            }
        }
    }

    func setupAfterTeardown() throws {
        for (way, output) in outputs {
            guard let teardown = dropInIndex(output, Self.teardownLine),
                dropInIndex(output, Self.firstLine, from: teardown) != nil
            else { throw StepFailure("after \(way), setup didn't run after teardown: \(output)") }
        }
    }

    /// Gives Card declined a teardown that exits 2, and a setup that leaves a marker.
    func teardownFails() async throws {
        try writeTeardown("Card declined", Self.script("exit 2"))
        try writeSetup(
            "Card declined", Self.script(Self.touch("setup-ran"), #"ok "seeded shopper""#))
        try await prepare()
    }

    func setupDidNotRun() async throws {
        let session = try await settleTo(SessionModel.failed)
        if shopFile("setup-ran") != nil {
            throw StepFailure("setup ran after the teardown failed")
        }
        try expectNot(session.output, Self.firstLine, "the output")
    }
}
