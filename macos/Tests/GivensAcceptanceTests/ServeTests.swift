import Foundation
import GivensUI
import Testing

/// The UI shell's spike: one window listing the sample shop's capabilities, drawn from
/// internal/app through a real `givens serve`. It proves the pipe the views read from, without
/// a screenshot: WindowModel.visible is built from the same snapshot fields the views render.
@Suite("Serve") @MainActor struct ServeTests {
    @Test func oneWindowListsCapabilities() async throws {
        let world = try World()
        defer { world.tearDown() }
        try await world.openSampleShop()
        guard let window = world.window else {
            Issue.record("opening the sample shop left no window")
            return
        }
        #expect(window.kind == "project")
        #expect(window.title == world.capabilities.path)
        #expect(window.sidebar.map(\.name) == ["Checkout", "Invites", "Sign in"])
        #expect(window.visible.contains("Checkout 5"))
    }

    @Test func coreErrorsArriveAsErrors() async throws {
        let world = try World()
        defer { world.tearDown() }
        try await world.openSampleShop()
        guard let window = world.window else {
            Issue.record("opening the sample shop left no window")
            return
        }
        await #expect(throws: GivensError.self) {
            _ = try await window.capability("Payments")
        }
    }
}
