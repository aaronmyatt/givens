import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/open-a-project against the Swift app. Shared steps are in World/.
@Suite("Open a project") @MainActor struct OpenAProjectTests {
    static let capability = "open-a-project"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: openAProjectSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: openAProjectSteps)
    }
}

/// What this capability's steps keep between steps of one scenario.
@MainActor
final class OpenAProjectState {
    let w: World
    /// A project folder with no capabilities folder, and what it held when made.
    var emptyProject: URL?
    var emptyProjectListing: [String] = []
    /// A second copy of the sample shop, for the two-window scenarios.
    var otherProject: URL?
    /// The window first opened, for "comes to the front".
    var firstWindowID: String?
    /// Where "Install givens command" put the command, and what Givens said.
    var installFolder: URL?
    var installMessage = ""
    /// A fake `open` on PATH logs here what `givens <folder>` asked macOS to launch.
    var openLog: URL { w.dir.appendingPathComponent("open.log") }

    init(_ w: World) { self.w = w }

    /// Project windows, front first.
    var projectWindows: [WindowModel] { w.app?.windows.filter { $0.kind == "project" } ?? [] }

    func makeEmptyProject() throws {
        let folder = w.dir.appendingPathComponent("empty-project", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try "# Empty\n".write(
            to: folder.appendingPathComponent("README.md"), atomically: true, encoding: .utf8)
        emptyProject = folder
        emptyProjectListing = try listing(folder)
        w.project = folder
    }

    /// Every path under `folder`, sorted.
    func listing(_ folder: URL) throws -> [String] {
        let paths = FileManager.default.subpaths(atPath: folder.path) ?? []
        return paths.sorted()
    }

    func copyOtherProject() throws -> URL {
        let copy = w.dir.appendingPathComponent("other-shop", isDirectory: true)
        if otherProject == nil {
            try FileManager.default.copyItem(
                at: Toolchain.root.appendingPathComponent("testdata/sample-shop"), to: copy)
            otherProject = copy
        }
        return copy.appendingPathComponent("capabilities")
    }

    /// Quits, then starts a new app on the same app data folder, as a relaunch does.
    func relaunch() async throws {
        try await w.app?.quit()
        w.app = nil
        w.window = nil
        try w.startApp()
        try await w.app?.launch()
    }

    /// Installs the command into a folder of its own, and puts a fake `open` next to it on
    /// PATH so `givens <folder>` can't launch the real app during tests.
    func install() async throws {
        let bin = w.dir.appendingPathComponent("bin", isDirectory: true)
        let fakes = w.dir.appendingPathComponent("fake-bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bin, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: fakes, withIntermediateDirectories: true)
        let fakeOpen = fakes.appendingPathComponent("open")
        try "#!/bin/bash\nprintf '%s\\n' \"$@\" >> \(Shell.quote(openLog.path))\n".write(
            to: fakeOpen, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755], ofItemAtPath: fakeOpen.path)
        installMessage = try await w.app?.installCommand(into: bin.path) ?? ""
        installFolder = bin
        w.pathPrepend = [bin, fakes]
    }

    /// Hands folders the fake `open` was asked to launch to the app, as LaunchServices does
    /// through application(_:open:).
    func deliverOpenedFolders() async throws {
        guard let log = try? String(contentsOf: openLog, encoding: .utf8) else { return }
        let folders = log.split(separator: "\n").map(String.init).filter { $0.hasPrefix("/") }
        if !folders.isEmpty { try await w.app?.openDropped(folders) }
    }

    func expectShopOpen() async throws {
        try await deliverOpenedFolders()
        let capabilities = w.capabilities.path
        guard projectWindows.contains(where: { samePath($0.path, capabilities) }) else {
            throw StepFailure(
                "no window shows \(capabilities); open: \(projectWindows.map(\.path))")
        }
    }
}

/// The story lines (As a, I want, So that) of a .feature file.
func storyLines(of feature: URL) throws -> [String] {
    let text = try String(contentsOf: feature, encoding: .utf8)
    let lines = text.components(separatedBy: "\n").map { $0.trimmingCharacters(in: .whitespaces) }
    return lines.filter { ["As a", "I want", "So that"].contains(where: $0.hasPrefix) }
}

@MainActor
func openAProjectSteps(_ w: World) -> [StepDefinition] {
    let s = OpenAProjectState(w)
    return openingSteps(s) + welcomeSteps(s) + terminalSteps(s) + windowSteps(s)
        + openABranchSteps(s)
}

@MainActor
private func openingSteps(_ s: OpenAProjectState) -> [StepDefinition] {
    let w = s.w
    return [
        def(
            #"^a developer has the sample shop project, with ((?:[a-z-]+/[a-z-]+\.feature(?:, | and )?)+)$"#
        ) { call in
            try w.copySampleShop()
            for path in call[0].replacingOccurrences(of: " and ", with: ", ")
                .components(separatedBy: ", ")
            where !FileManager.default.fileExists(atPath: w.capabilities.path + "/" + path) {
                throw StepFailure("the sample shop has no \(path)")
            }
        },
        def(#"^they open its capabilities folder in Givens$"#) { _ in
            try w.startApp()
            w.window = try await w.app?.open(w.capabilities.path)
        },
        def(#"^the sidebar lists "([^"]*)", "([^"]*)" and "([^"]*)", from each Feature: line$"#) {
            call in
            let names = try await w.front().sidebar.filter { $0.kind == "capability" }.map(\.name)
            guard names == call.captures else {
                throw StepFailure("the sidebar lists \(names), not \(call.captures)")
            }
        },
        def(#"^each capability shows how many scenarios it has$"#) { _ in
            let window = try await w.front()
            for item in window.sidebar where item.kind == "capability" {
                let slug = try await window.capability(item.name).slug
                let feature = w.capabilities.appendingPathComponent("\(slug)/\(slug).feature")
                let count = try Toolchain.pickles(of: feature).count
                guard item.count == count else {
                    throw StepFailure("\(item.name) shows \(item.count) scenarios, not \(count)")
                }
                try expect(window.visible, "\(item.name) \(count)", "the window")
            }
        },
        def(#"^the Checkout story is shown$"#) { _ in
            let feature = w.capabilities.appendingPathComponent("checkout/checkout.feature")
            let visible = try await w.front().visible
            for line in try storyLines(of: feature) { try expect(visible, line, "the window") }
        },
        def(#"^they open the project's root folder in Givens$"#) { _ in
            try w.startApp()
            guard let project = w.project else { throw StepFailure("no project copied") }
            w.window = try await w.app?.open(project.path)
        },
        def(#"^Givens finds the capabilities folder inside it$"#) { _ in
            let path = try await w.front().path
            guard samePath(path, w.capabilities.path) else {
                throw StepFailure("the window shows \(path), not \(w.capabilities.path)")
            }
        },
        def(#"^the window title shows the path to that capabilities folder$"#) { _ in
            let title = try await w.front().title
            guard title.hasSuffix("sample-shop/capabilities") else {
                throw StepFailure("the window title is \"\(title)\"")
            }
        },
        def(#"^a developer has a project with no capabilities folder$"#) { _ in
            try s.makeEmptyProject()
        },
        def(#"^a developer has a project with no capabilities folder open in Givens$"#) { _ in
            try s.makeEmptyProject()
            try w.startApp()
            w.window = try await w.app?.open(s.emptyProject?.path ?? "")
        },
        def(#"^they open it in Givens$"#) { _ in
            try w.startApp()
            w.window = try await w.app?.open(s.emptyProject?.path ?? "")
        },
        def(#"^nothing is written to the project$"#) { _ in
            guard let folder = s.emptyProject else { throw StepFailure("no empty project") }
            let now = try s.listing(folder)
            guard now == s.emptyProjectListing else {
                throw StepFailure("the project changed: \(s.emptyProjectListing) → \(now)")
            }
        },
        def(
            #"^the project gets a capabilities folder containing GIVENS\.md and _shared/givens\.sh$"#
        ) {
            _ in
            for file in ["capabilities/GIVENS.md", "capabilities/_shared/givens.sh"] {
                _ = try w.readFile(file)
            }
        },
        def(
            #"^it contains the starter capability "([^"]*)", with ([a-z-]+\.feature), its about\.md and setup scripts$"#
        ) {
            call in
            let folder = "capabilities/" + call[1].replacingOccurrences(of: ".feature", with: "")
            let feature = try w.readFile("\(folder)/\(call[1])")
            try expect([feature], "Feature: \(call[0])", call[1])
            _ = try w.readFile("\(folder)/about.md")
            let scripts = try FileManager.default.contentsOfDirectory(
                atPath: w.capabilities.deletingLastPathComponent().path + "/\(folder)/scripts")
            guard scripts.contains(where: { $0.hasSuffix(".sh") }) else {
                throw StepFailure("\(folder)/scripts has no setup scripts: \(scripts)")
            }
        },
        def(#"^it contains \.gitattributes marking runs/ as merge=union$"#) { _ in
            try expect(
                [try w.readFile("capabilities/.gitattributes")], "runs/** merge=union",
                ".gitattributes")
        },
        def(#"^the sidebar lists "([^"]*)"$"#) { call in
            try expect(try await w.front().sidebar.map(\.name), call[0], "the sidebar")
        },
        def(#"^another tool adds a scenario to checkout\.feature$"#) { _ in
            try w.ensureScenario("checkout", "Pay with a gift card")
        },
        def(#"^"([^"]*)" appears in the Checkout scenarios within two seconds$"#) { call in
            let window = try await w.front()
            for _ in 0..<20 {
                let names = window.snapshot?.capability?.scenarios.map(\.name) ?? []
                if names.contains(call[0]) { return }
                try await Task.sleep(for: .milliseconds(100))
            }
            throw StepFailure("\"\(call[0])\" didn't appear in the shown Checkout in 2 seconds")
        },
        def(#"^"([^"]*)" stays selected$"#) { call in
            let selected = try await w.front().selected
            guard selected == call[0] else { throw StepFailure("\"\(selected)\" is selected") }
        },
    ]
}

@MainActor
private func welcomeSteps(_ s: OpenAProjectState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer has never opened a project in Givens$"#) { _ in try w.startApp() },
        def(#"^they open Givens$"#) { _ in
            try w.startApp()
            try await w.app?.launch()
        },
        def(#"^they see a welcome window with "([^"]*)"$"#) { call in
            let window = try await w.front()
            guard window.kind == "welcome" else { throw StepFailure("the front is \(window.kind)") }
            try expect(window.offers, call[0], "the welcome window's actions")
        },
        def(#"^choosing "([^"]*)" shows a folder picker$"#) { call in
            try await w.front().choose(call[0])
            guard w.folderPicker.presented > 0 else { throw StepFailure("no folder picker shown") }
        },
        def(#"^a developer has opened the sample shop project in Givens before$"#) { _ in
            try await w.openSampleShop()
        },
        def(#"^no project was open when they last quit Givens$"#) { _ in
            try await w.front().close()
            try await w.app?.quit()
            w.app = nil
            w.window = nil
        },
        def(#"^the welcome window lists the sample shop under "([^"]*)"$"#) { call in
            let window = try await w.front()
            try expect(window.visible, call[0], "the welcome window")
            guard w.app?.recents.contains(where: { samePath($0, w.capabilities.path) }) == true
            else { throw StepFailure("Recent is \(w.app?.recents ?? [])") }
        },
        def(#"^choosing it opens the project$"#) { _ in
            let recent = w.app?.recents.first { samePath($0, w.capabilities.path) } ?? ""
            w.window = try await w.front().openRecent(recent)
            try await s.expectShopOpen()
        },
        def(#"^a developer has Givens in their Dock$"#) { _ in
            try w.copySampleShop()
            try w.startApp()
        },
        def(#"^they drag the sample shop's folder onto it$"#) { _ in
            try await w.app?.openDropped([w.project?.path ?? ""])
        },
        def(#"^the sample shop project opens in Givens$"#) { _ in try await s.expectShopOpen() },
    ]
}

@MainActor
private func terminalSteps(_ s: OpenAProjectState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^a developer has Givens open$"#) { _ in
            try w.copySampleShop()
            try w.startApp()
            try await w.app?.launch()
        },
        def(#"^they choose "([^"]*)" from the Givens menu$"#) { call in
            guard call[0] == "Install givens command" else {
                throw StepFailure("the Givens menu has no \"\(call[0])\" in the suite")
            }
            try await s.install()
        },
        def(#"^givens works in new terminal windows$"#) { _ in
            guard let bin = s.installFolder else { throw StepFailure("nothing was installed") }
            let result = try Shell.run(
                "command -v givens", in: w.dir, path: [bin])
            guard
                samePath(
                    result.output.trimmingCharacters(in: .whitespacesAndNewlines),
                    bin.appendingPathComponent("givens").path)
            else { throw StepFailure("a new shell finds givens at \(result.output)") }
        },
        def(#"^Givens says where it installed it$"#) { _ in
            guard let bin = s.installFolder else { throw StepFailure("nothing was installed") }
            try expect([s.installMessage], bin.path, "what Givens said")
        },
        def(#"^a developer has installed the givens command$"#) { _ in
            try w.copySampleShop()
            try w.startApp()
            try await w.app?.launch()
            try await s.install()
        },
        def(#"^they run "([^"]*)" in the sample shop's folder$"#) { call in
            w.cwd = w.project
            try w.run(call[0])
        },
        def(#"^the command prints "([^"]*)"$"#) { call in
            try expect(w.output.components(separatedBy: "\n"), call[0], "the command's output")
        },
        def(#"^Givens doesn't open a window$"#) { _ in
            try await s.deliverOpenedFolders()
            guard s.projectWindows.isEmpty else {
                throw StepFailure("a window opened: \(s.projectWindows.map(\.path))")
            }
        },
    ]
}

@MainActor
private func windowSteps(_ s: OpenAProjectState) -> [StepDefinition] {
    let w = s.w
    return [
        def(#"^they open another project's capabilities folder$"#) { _ in
            s.firstWindowID = w.window?.id
            w.window = try await w.app?.open(try s.copyOtherProject().path)
        },
        def(#"^it opens in a second window$"#) { _ in
            guard s.projectWindows.count == 2 else {
                throw StepFailure("\(s.projectWindows.count) project windows are open")
            }
            guard w.app?.front?.id != s.firstWindowID else {
                throw StepFailure("the new window isn't in front")
            }
        },
        def(#"^each window's title shows its own project's path$"#) { _ in
            for window in s.projectWindows where !window.title.contains(window.path) {
                throw StepFailure("\"\(window.title)\" doesn't show \(window.path)")
            }
        },
        def(#"^they open the sample shop's capabilities folder again$"#) { _ in
            s.firstWindowID = w.window?.id
            try await w.app?.open(w.capabilities.path)
        },
        def(#"^its existing window comes to the front$"#) { _ in
            guard let first = s.firstWindowID, w.app?.front?.id == first else {
                throw StepFailure("window \(w.app?.front?.id ?? "none") is in front")
            }
        },
        def(#"^no second window opens$"#) { _ in
            guard s.projectWindows.count == 1 else {
                throw StepFailure("\(s.projectWindows.count) project windows are open")
            }
        },
        def(#"^a developer has two projects open in Givens$"#) { _ in
            try await w.openSampleShop()
            try await w.app?.open(try s.copyOtherProject().path)
        },
        def(#"^they quit Givens and open it again$"#) { _ in try await s.relaunch() },
        def(#"^both projects reopen in their own windows$"#) { _ in
            let paths = s.projectWindows.map(\.path)
            let wanted = [w.capabilities.path, try s.copyOtherProject().path]
            for path in wanted where !paths.contains(where: { samePath($0, path) }) {
                throw StepFailure("\(path) didn't reopen; open: \(paths)")
            }
        },
    ]
}
