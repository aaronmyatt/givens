import Foundation
import GherkinKit
import GivensUI

// Reading another branch's capabilities without checking that branch out.

/// A branch that adds a scenario, the read-only window that shows it, and a branch that
/// doesn't exist.
@MainActor
func openABranchSteps(_ s: OpenAProjectState) -> [StepDefinition] {
    let w = s.w
    return [
        def(
            #"^the sample shop is a git repository with a branch "([^"]*)" that adds the scenario "([^"]*)" to Checkout$"#
        ) { call in try s.branchAddsScenario(call[0], call[1]) },
        def(#"^they open the branch "([^"]*)"$"#) { call in
            let window = try await w.front()
            s.firstWindowID = window.id
            try await window.openBranch(call[0])
            // The branch's window, when one opened, is the window under test from here on.
            w.window = w.app?.front ?? window
        },
        def(#"^a second window opens, its title ending "([^"]*)"$"#) { call in
            let front = try await w.front()
            guard s.projectWindows.count == 2, front.id != s.firstWindowID else {
                throw StepFailure("\(s.projectWindows.count) project windows are open")
            }
            guard front.title.hasSuffix(call[0]) else {
                throw StepFailure("the front window's title is \"\(front.title)\"")
            }
        },
        def(#"^it lists "([^"]*)" under "([^"]*)"$"#) { call in
            let shown = try await w.front().capability(call[1])
            try expect(shown.scenarios.map(\.name), call[0], "\(call[1])'s scenarios")
        },
        def(#"^the sample shop's own checkout\.feature doesn't have "([^"]*)"$"#) { call in
            let text = try w.readFile(OpenAProjectState.checkoutFeature)
            guard !text.contains(call[0]) else {
                throw StepFailure("the working tree's checkout.feature has \"\(call[0])\"")
            }
        },
        def(#"^"Drop in" isn't offered for "([^"]*)"$"#) { call in
            let front = try await w.front()
            try await front.select(call[0])
            try expectNot(front.offers, "Drop in", "the branch's window's offers")
        },
    ]
}

extension OpenAProjectState {
    /// The sample shop's Checkout feature, relative to its folder.
    static let checkoutFeature = "capabilities/checkout/checkout.feature"

    /// Runs git in the sample shop, away from this Mac's own git settings.
    /// Ref: https://git-scm.com/docs/git#Documentation/git.txt-codeGITCONFIGGLOBALcode
    func git(_ args: [String]) throws {
        guard let project = w.project else { throw StepFailure("no sample shop for git") }
        let command =
            "GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 git "
            + args.map(Shell.quote).joined(separator: " ")
        let result = try Shell.run(command, in: project)
        guard result.status == 0 else {
            throw StepFailure("git \(args.joined(separator: " ")) failed:\n\(result.output)")
        }
    }

    /// Commits the sample shop on main, adds a scenario to Checkout on `branch`, and goes
    /// back to main, so the working tree doesn't have it.
    /// Ref: https://git-scm.com/docs/git-init and https://git-scm.com/docs/git-checkout
    func branchAddsScenario(_ branch: String, _ name: String) throws {
        try w.copySampleShop()
        for args in [
            ["init", "--quiet", "--initial-branch=main"], ["config", "user.name", "Sam"],
            ["config", "user.email", "sam@example.com"], ["config", "commit.gpgsign", "false"],
            ["add", "--all"], ["commit", "--quiet", "--message", "Sample shop"],
            ["checkout", "--quiet", "-b", branch],
        ] {
            try git(args)
        }
        let scenario =
            "\n  Scenario: \(name)\n    Given a shopper with a gift card\n"
            + "    When they pay with it\n    Then the order is paid\n"
        try w.writeFile(Self.checkoutFeature, try w.readFile(Self.checkoutFeature) + scenario)
        try git(["commit", "--quiet", "--all", "--message", name])
        try git(["checkout", "--quiet", "main"])
    }
}
