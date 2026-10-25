import Foundation
import GherkinKit
import GivensUI

// Editing a capability's about.md from its story, or starting one.

/// "Edit about.md" and "Add about.md".
@MainActor
func writeStoriesAboutSteps(_ s: WriteStoriesAndScenariosState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they press e$"#) { _ in try await w.front().press("e") },
        def(#"^([a-z0-9-]+/[a-z0-9-]+\.feature) opens in the editor at the "([^"]*)" block$"#) {
            call in
            let window = try await w.front()
            let want = w.capabilities.appendingPathComponent(call[0]).path
            let at = window.snapshot?.editor?.scenario ?? ""
            guard samePath(window.editing, want), at == call[1] else {
                throw StepFailure("the editor holds \(window.editing) at \"\(at)\"")
            }
        },
        def(#"^([a-z0-9-]+/[a-z0-9-]+\.(?:md|feature)) opens in the editor$"#) { call in
            let editing = try await w.front().editing
            let want = w.capabilities.appendingPathComponent(call[0]).path
            guard samePath(editing, want) else {
                throw StepFailure("the editor holds \"\(editing)\", not \(want)")
            }
        },
        def(#"^Invites has no about\.md$"#) { _ in
            try await w.openSampleShop()
            let path = w.capabilities.appendingPathComponent("invites/about.md").path
            guard !FileManager.default.fileExists(atPath: path) else {
                throw StepFailure("invites/about.md is there")
            }
        },
        def(#"^([a-z0-9-]+/about\.md) is created, starting with the line "([^"]*)"$"#) { call in
            let text = try w.readFile("capabilities/\(call[0])")
            let first = text.split(separator: "\n", omittingEmptySubsequences: false).first
            guard first.map(String.init) == call[1] else {
                throw StepFailure("\(call[0]) starts with \"\(first ?? "")\", not \"\(call[1])\"")
            }
        },
    ]
}
