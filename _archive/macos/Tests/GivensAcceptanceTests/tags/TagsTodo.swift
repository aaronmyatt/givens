import Foundation
import GherkinKit
import GivensUI

/// The to do flag's steps: "A scenario flagged to do", "Flag a scenario to do" and "Remove a
/// to do flag". They mirror acceptance/tags/todo_test.go in Go.
@MainActor
func tagsTodoSteps(_ s: TagsState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^"([^"]*)" is listed under Tags in the sidebar$"#) { call in
            let tags = try await w.front().sidebar.filter { $0.kind == "tag" }.map(\.name)
            guard tags.contains(call[0]) else {
                throw StepFailure("Tags in the sidebar lists \(tags), not \"\(call[0])\"")
            }
        },
        def(#"^"([^"]*)" is tagged (@\S+) in checkout\.feature, and nothing else changes$"#) {
            call in try tagsOnlyFlagChanged(s, call[0], call[1], tagged: true)
        },
        def(
            #"^"([^"]*)" isn't tagged (@\S+) in checkout\.feature, and nothing else changes$"#
        ) { call in try tagsOnlyFlagChanged(s, call[0], call[1], tagged: false) },
        def(#"^it's listed when they select "([^"]*)" under Tags$"#) { call in
            let window = try await w.front()
            let tags = window.sidebar.filter { $0.kind == "tag" }.map(\.name)
            guard tags.contains(call[0]) else {
                throw StepFailure("Tags in the sidebar lists \(tags), not \"\(call[0])\"")
            }
            try await window.select(call[0])
            let listed = try await s.listedNames()
            guard listed.contains(s.subject) else {
                throw StepFailure("\"\(call[0])\" lists \(listed), not \"\(s.subject)\"")
            }
        },
        def(#"^it isn't shown with a red flag$"#) { _ in
            guard try await !w.shownFlagged(s.subject) else {
                throw StepFailure("\"\(s.subject)\" is still shown with a red flag")
            }
        },
    ]
}

/// Expects the scenario's own tags to have `tag`, or not, and checkout.feature to be the
/// sample shop's as checked in apart from that tag: no other line and no other tag.
@MainActor
private func tagsOnlyFlagChanged(
    _ s: TagsState, _ name: String, _ tag: String, tagged: Bool
) throws {
    s.subject = name
    let path = s.w.capabilities.appendingPathComponent("checkout/checkout.feature")
    let original = Toolchain.root.appendingPathComponent(
        "testdata/sample-shop/capabilities/checkout/checkout.feature")
    // tools/pickles fails on a file that isn't valid Gherkin.
    _ = try Toolchain.pickles(of: path)
    let now = try tagsOwnLines(try String(contentsOf: path, encoding: .utf8), name)
    let before = try tagsOwnLines(try String(contentsOf: original, encoding: .utf8), name)
    guard now.tags.contains(tag) == tagged else {
        throw StepFailure("\"\(name)\"'s own tags are \(now.tags)")
    }
    guard now.tags.filter({ $0 != tag }) == before.tags.filter({ $0 != tag }) else {
        throw StepFailure("\"\(name)\"'s other tags became \(now.tags), from \(before.tags)")
    }
    guard now.rest == before.rest else {
        throw StepFailure(
            "checkout.feature changed beyond \"\(name)\"'s tag lines:\n"
                + now.rest.joined(separator: "\n"))
    }
}

/// A scenario's own tags, from the tag lines just above its Scenario: line, and the file's
/// other lines. Gherkin tags sit on the lines before the keyword they apply to.
/// Ref: https://cucumber.io/docs/cucumber/api/#tags
private func tagsOwnLines(_ text: String, _ name: String) throws -> (
    tags: [String], rest: [String]
) {
    var lines = text.components(separatedBy: "\n")
    let header = "Scenario: \(name)"
    guard let at = lines.firstIndex(where: { $0.trimmingCharacters(in: .whitespaces) == header })
    else { throw StepFailure("checkout.feature has no line \"\(header)\"") }
    var top = at
    while top > 0 && lines[top - 1].trimmingCharacters(in: .whitespaces).hasPrefix("@") {
        top -= 1
    }
    let tags = lines[top..<at].flatMap { $0.split(separator: " ").map(String.init) }
    lines.removeSubrange(top..<at)
    return (tags, lines)
}
