import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/update-the-kit against the Swift app. Each scenario decides which kit
/// versions Givens ships through AppModel.shipKits, and writes the project's own givens.sh, so
/// the versions in the steps are the versions under test. Shared steps are in World/.
@Suite("Update the kit") @MainActor struct UpdateTheKitTests {
    static let capability = "update-the-kit"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: updateTheKitSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: updateTheKitSteps)
    }
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class UpdateTheKitState {
    /// The kit inside the sample shop.
    static let kitPath = "capabilities/_shared/givens.sh"
    /// What a developer added to their kit by hand.
    static let localEdit =
        "\n# Added by hand for this project.\nshop_helper() { ok \"shop helper\"; }\n"

    let w: World
    /// What Givens ships, oldest first; empty until a step says.
    var releases: [KitRelease] = []

    init(_ w: World) { self.w = w }

    /// The kit as Givens shipped it at `version`: a version line, then a body that differs
    /// between versions. The same text as kitText in acceptance/update-the-kit.
    static func kitText(_ version: Int) -> String {
        precondition(version > 0, "kit versions start at 1")
        return "# givens kit \(version)\n"
            + "# The kit, release \(version), as this scenario ships it.\n"
            + "set -euo pipefail\n"
            + "_givens_kit_version=\(version)\n"
            + "ok() { printf '✓ %s\\n' \"$*\"; }\n"
    }

    /// Kit releases 1 to `last`, oldest first.
    static func releases(to last: Int) -> [KitRelease] {
        precondition(last > 0 && last <= 256, "releases fit KitReleasesMax")
        return (1...last).map { KitRelease(version: $0, text: kitText($0)) }
    }

    /// Copies the shop, gives Card declined a setup so "Copy" has something to copy, and
    /// writes the project's kit.
    func writeKit(_ text: String) throws {
        try w.copySampleShop()
        try w.setupStartsWith("Card declined", #"ok "card declined is ready""#)
        try w.writeFile(Self.kitPath, text)
    }

    /// Starts Givens shipping the scenario's kits and opens the sample shop.
    func openShop() async throws {
        guard !releases.isEmpty else {
            throw StepFailure("no step said which kit version Givens ships")
        }
        try w.startApp()
        try await w.app?.shipKits(releases)
        w.window = try await w.app?.open(w.capabilities.path)
    }

    /// Runs one git command in the sample shop, with an identity and no hooks or signing from
    /// this Mac's settings.
    /// Ref: https://git-scm.com/docs/git#Documentation/git.txt--cltnamegtltvaluegt
    func git(_ arguments: String) throws -> String {
        guard let project = w.project else { throw StepFailure("no sample shop copied") }
        let settings =
            "-c user.name=Givens -c user.email=givens@example.com "
            + "-c commit.gpgsign=false -c core.hooksPath=/dev/null"
        let result = try Shell.run("git \(settings) \(arguments)", in: project)
        guard result.status == 0 else {
            throw StepFailure("git \(arguments) failed:\n\(result.output)")
        }
        return result.output
    }

    /// Makes the shop a git repository with everything committed.
    /// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-commit
    func commitShop() throws {
        _ = try git("init --quiet")
        _ = try git("add --all")
        _ = try git("commit --quiet -m 'Sample shop'")
    }
}

@MainActor
func updateTheKitSteps(_ w: World) -> [StepDefinition] {
    let s = UpdateTheKitState(w)
    return updateTheKitArrangeSteps(s) + updateTheKitCheckSteps(s)
}

@MainActor
private func updateTheKitArrangeSteps(_ s: UpdateTheKitState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^the sample shop's givens\.sh is version (\d+)$"#) { call in
            try s.writeKit(UpdateTheKitState.kitText(try updateTheKitNumber(call[0])))
        },
        def(#"^Givens ships version (\d+)$"#) { call in
            s.releases = UpdateTheKitState.releases(to: try updateTheKitNumber(call[0]))
        },
        def(#"^a developer opens the sample shop$"#) { _ in try await s.openShop() },
        def(#"^the sample shop's givens\.sh is version (\d+) and hasn't been edited$"#) { call in
            // Committed first, so "git diff shows only givens.sh changed" sees the update alone.
            let version = try updateTheKitNumber(call[0])
            try s.writeKit(UpdateTheKitState.kitText(version))
            try s.commitShop()
            s.releases = UpdateTheKitState.releases(to: version + 1)
            try await s.openShop()
        },
        def(#"^the sample shop has no givens\.sh$"#) { _ in
            try w.copySampleShop()
            try w.removeFile(UpdateTheKitState.kitPath)
        },
        def(#"^deleting givens\.sh while the sample shop is open puts version (\d+) back$"#) {
            call in
            try w.removeFile(UpdateTheKitState.kitPath)
            let want = UpdateTheKitState.kitText(try updateTheKitNumber(call[0]))
            // Any read of the window looks at the files; ask for Checkout, then check.
            _ = try await w.front().capability("Checkout")
            guard (try? w.readFile(UpdateTheKitState.kitPath)) == want else {
                throw StepFailure("givens.sh didn't come back as kit version \(call[0])")
            }
        },
        def(#"^the sample shop's givens\.sh has local edits$"#) { _ in
            try s.writeKit(UpdateTheKitState.kitText(1) + UpdateTheKitState.localEdit)
            s.releases = UpdateTheKitState.releases(to: 2)
            try await s.openShop()
        },
    ]
}

@MainActor
private func updateTheKitCheckSteps(_ s: UpdateTheKitState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^givens\.sh is (?:replaced with|written at) version (\d+)$"#) { call in
            let got = try w.readFile(UpdateTheKitState.kitPath)
            let want = UpdateTheKitState.kitText(try updateTheKitNumber(call[0]))
            guard got == want else {
                throw StepFailure("givens.sh isn't kit version \(call[0]); it holds:\n\(got)")
            }
        },
        def(#"^git diff shows only givens\.sh changed$"#) { _ in
            // Porcelain status lists changed and new files, so a file written beside the kit
            // shows too. Ref: https://git-scm.com/docs/git-status#_porcelain_format_version_1
            let output = try s.git("status --porcelain=v1 --untracked-files=all")
            let changed = output.split(separator: "\n").map {
                $0.trimmingCharacters(in: .whitespaces)
            }
            guard changed == ["M \(UpdateTheKitState.kitPath)"] else {
                throw StepFailure(
                    "git shows \(changed) changed, not only \(UpdateTheKitState.kitPath)")
            }
        },
        def(#"^reading, searching and copying still work$"#) { _ in
            let window = try await w.front()
            let checkout = try await window.capability("Checkout")
            guard !checkout.scenarios.isEmpty else {
                throw StepFailure("reading Checkout shows no scenarios")
            }
            try await window.select("Card declined")
            try await window.choose("Copy")
            let copied = w.clipboard.readText()
            guard copied.contains("card-declined") else {
                throw StepFailure(
                    "\"Copy\" put \"\(copied)\" on the clipboard, not Card declined's")
            }
            try await w.search("declined")
            try expect(window.visible, "Card declined", "the search results")
        },
    ]
}

/// A version number from a step's capture.
private func updateTheKitNumber(_ text: String) throws -> Int {
    guard let number = Int(text), number > 0 else {
        throw StepFailure("\"\(text)\" isn't a kit version")
    }
    return number
}
