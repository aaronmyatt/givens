import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/copy-a-setup-script against the Swift app. Shared steps are in World/.
/// It mirrors acceptance/copy-a-setup-script/copy_test.go step for step.
@Suite("Copy a setup script") @MainActor struct CopyASetupScriptTests {
    static let capability = "copy-a-setup-script"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: copyASetupScriptSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: copyASetupScriptSteps)
    }
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class CopyASetupScriptState {
    let w: World
    /// Where the sample shop's capabilities folder is, once a step has moved it.
    var capabilitiesDirectory: URL?

    init(_ w: World) { self.w = w }

    /// Pastes the clipboard into a shell at the project's root, as a developer would, and
    /// expects setup to succeed.
    func runClipboardFromRoot() throws {
        let command = w.clipboard.readText()
        guard !command.isEmpty else { throw StepFailure("the clipboard is empty") }
        w.cwd = w.project
        try w.run(command)
        guard w.exitCode == 0 else {
            throw StepFailure("\"\(command)\" exited \(w.exitCode):\n\(w.output)")
        }
    }

    /// Moves the sample shop's capabilities folder to docs/capabilities and makes the shop a
    /// git repo, so the repo root and the capabilities folder's parent differ.
    func capabilitiesInDocs() throws {
        try w.copySampleShop()
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        let docs = project.appendingPathComponent("docs", isDirectory: true)
        try FileManager.default.createDirectory(at: docs, withIntermediateDirectories: true)
        let moved = docs.appendingPathComponent("capabilities", isDirectory: true)
        // Ref: https://developer.apple.com/documentation/foundation/filemanager/moveitem(at:to:)
        try FileManager.default.moveItem(at: w.capabilities, to: moved)
        capabilitiesDirectory = moved
        // git init makes the shop a repository. Ref: https://git-scm.com/docs/git-init
        let (status, output) = try Shell.run("git init --quiet", in: project)
        guard status == 0 else { throw StepFailure("git init exited \(status):\n\(output)") }
    }

    /// Selects a scenario, then takes an action on it, opening the shop first if needed.
    func chooseOn(_ action: String, _ scenario: String) async throws {
        if w.window == nil {
            try w.startApp()
            let folder = capabilitiesDirectory ?? w.capabilities
            w.window = try await w.app?.open(folder.path)
        }
        guard let window = w.window else { throw StepFailure("no Givens window is open") }
        try await window.select(scenario)
        try await window.choose(action)
    }

    func terminalInSrc() throws {
        try w.copySampleShop()
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        let source = project.appendingPathComponent("src", isDirectory: true)
        try FileManager.default.createDirectory(at: source, withIntermediateDirectories: true)
        w.cwd = source
    }

    /// Runs the last command again from the root and compares the two runs.
    func sameAsFromRoot() throws {
        let fromSubfolder = w.output
        guard w.exitCode == 0 else {
            throw StepFailure("setup exited \(w.exitCode) from the subfolder:\n\(fromSubfolder)")
        }
        w.cwd = w.project
        try w.run("bash capabilities/checkout/scripts/card-declined.sh")
        guard w.exitCode == 0, w.output == fromSubfolder else {
            throw StepFailure(
                "from the root it exited \(w.exitCode) with:\n\(w.output)\n"
                    + "from src:\n\(fromSubfolder)")
        }
    }

    func clipboardHoldsMarkdown(_ name: String) throws {
        let want = try w.readFile("capabilities/checkout/scripts/\(name)")
        let got = w.clipboard.readText()
        guard got == want else {
            throw StepFailure("the clipboard holds \"\(got)\", not the markdown of \(name)")
        }
    }
}

@MainActor
func copyASetupScriptSteps(_ w: World) -> [StepDefinition] {
    let s = CopyASetupScriptState(w)
    return copyActionSteps(s) + copyPlaceSteps(s)
}

/// Copying from the window and from search, and what ends up on the clipboard.
@MainActor
private func copyActionSteps(_ s: CopyASetupScriptState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^running it from the (?:project's|repo's) root folder sets up the scenario$"#) {
            _ in try s.runClipboardFromRoot()
        },
        def(#"^they press ⌘K, type "([^"]*)" and press ⇧⌘C$"#) { call in
            try await w.search(call[0])
            try await w.front().press("⇧⌘C")
        },
        def(#"^"([^"]*)" is still selected$"#) { call in
            let selected = try await w.front().selected
            guard selected == call[0] else {
                throw StepFailure("\"\(selected)\" is selected, not \"\(call[0])\"")
            }
        },
        def(#"^a developer chooses "([^"]*)" on "([^"]*)"$"#) { call in
            try await s.chooseOn(call[0], call[1])
        },
        def(#"^the clipboard holds the markdown of ([a-z0-9-]+\.md)$"#) { call in
            try s.clipboardHoldsMarkdown(call[0])
        },
    ]
}

/// Where the capabilities folder is, and where the terminal runs setup from.
@MainActor
private func copyPlaceSteps(_ s: CopyASetupScriptState) -> [StepDefinition] {
    [
        def(#"^the sample shop's capabilities folder is at docs/capabilities in its git repo$"#) {
            _ in try s.capabilitiesInDocs()
        },
        def(#"^a developer's terminal is in the sample shop's src folder$"#) { _ in
            try s.terminalInSrc()
        },
        def(#"^setup runs the same as it would from the root folder$"#) { _ in
            try s.sameAsFromRoot()
        },
    ]
}
