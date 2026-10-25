import Foundation
import GherkinKit
import GivensUI
import Testing

/// Runs capabilities/mix-shell-browser-and-hand-steps against the Swift app, a fake shop from
/// tools/fakeshop and a headless Chrome. It mirrors acceptance/mix-shell-browser-and-hand-steps
/// in Go: the spec's http://localhost:3000 is swapped for the fake shop's address.
@Suite("Mix shell, browser and hand steps") @MainActor struct MixShellBrowserTests {
    static let capability = "mix-shell-browser-and-hand-steps"

    @Test func scenarios() async throws {
        try await runCapability(Self.capability, steps: mixShellBrowserSteps)
    }

    @Test func stepsDefined() throws {
        try checkStepsDefined(Self.capability, steps: mixShellBrowserSteps)
    }
}

@MainActor
func mixShellBrowserSteps(_ w: World) -> [StepDefinition] {
    let s = MixShellState(w)
    return mixScriptSteps(s) + mixBrowserSteps(s) + mixTerminalSteps(s) + mixCardSteps(s)
}

/// The address the spec names, which the steps swap for the fake shop's.
let mixShopURL = "http://localhost:3000"

/// What this capability's steps keep between steps of one scenario. Mirrors Go's state.
@MainActor
final class MixShellState {
    let w: World
    /// Lines the steps put in each setup, after the kit's line.
    var added: [String: Int] = [:]
    /// Setups with a browser_open line.
    var opened: Set<String> = []
    /// pay-in-app.md's steps, as written after "N. ".
    var cardSteps: [String] = []
    /// The project's page before the When step.
    var page = ""

    init(_ w: World) { self.w = w }

    /// `text` with the fake shop's address in place of the spec's.
    func shop(_ text: String) throws -> String {
        text.replacingOccurrences(of: mixShopURL, with: try w.shopURL())
    }

    /// Puts `line` in a setup after the kit's line and the lines earlier steps put there.
    func addLine(_ name: String, _ line: String) throws {
        try w.copySampleShop()
        let path = try w.scriptPath(name)
        let text = (try? w.readFile(path)) ?? ""
        let body =
            text.hasPrefix(kitSource + "\n") ? String(text.dropFirst(kitSource.count + 1)) : text
        var lines = body.components(separatedBy: "\n")
        let at = min(added[name] ?? 0, lines.count)
        lines.insert(try shop(line), at: at)
        added[name] = (added[name] ?? 0) + 1
        if line.hasPrefix("browser_open ") { opened.insert(name) }
        try w.writeFile(path, kitSource + "\n" + lines.joined(separator: "\n"))
    }

    /// Opens the shop's page first, unless a line did, then adds the lines.
    func withPage(_ name: String, _ lines: [String] = []) throws {
        if !opened.contains(name) { try addLine(name, #"browser_open "\#(mixShopURL)/""#) }
        for line in lines { try addLine(name, line) }
    }

    /// The Drop in under test.
    func session() throws -> SessionModel {
        guard let session = w.session else { throw StepFailure("no step dropped in") }
        return session
    }

    /// Runs `givens browser <arguments>` against the sample shop's Givens browser.
    func browser(_ arguments: String) throws -> String {
        let givens = try Toolchain.givens()
        let result = try Shell.run(
            "\(Shell.quote(givens.path)) browser \(arguments)", in: w.dir,
            environment: w.environment.merging(["GIVENS_ROOT": w.capabilities.path]) { $1 })
        guard result.status == 0 else { throw StepFailure("givens browser: \(result.output)") }
        return result.output.trimmingCharacters(in: .newlines)
    }

    /// The ids of the Givens browser's tabs, from its DevTools /json/list.
    /// Ref: https://chromedevtools.github.io/devtools-protocol/#endpoints
    func pages() async throws -> [String] {
        let browsers = w.dir.appendingPathComponent("browsers")
        let profiles = try FileManager.default.contentsOfDirectory(atPath: browsers.path)
        guard profiles.count == 1 else { throw StepFailure("\(profiles.count) profiles") }
        let port = try String(
            contentsOf: browsers.appendingPathComponent("\(profiles[0])/DevToolsActivePort"),
            encoding: .utf8
        ).components(separatedBy: "\n")[0]
        guard let url = URL(string: "http://127.0.0.1:\(port)/json/list") else {
            throw StepFailure("no port in DevToolsActivePort")
        }
        let (data, _) = try await URLSession.shared.data(from: url)
        struct Tab: Decodable {
            let id: String
            let type: String
        }
        return try JSONDecoder().decode([Tab].self, from: data).filter { $0.type == "page" }
            .map(\.id)
    }

    /// The project's page, as the profile's givens-page file names it.
    func pageFile() throws -> String {
        let browsers = w.dir.appendingPathComponent("browsers")
        let profiles = try FileManager.default.contentsOfDirectory(atPath: browsers.path)
        guard profiles.count == 1 else { throw StepFailure("\(profiles.count) profiles") }
        return try String(
            contentsOf: browsers.appendingPathComponent("\(profiles[0])/givens-page"),
            encoding: .utf8
        ).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// Expects a line of setup's output to hold `text`, the shop's address swapped in.
    func outputShows(_ text: String) throws {
        try expect(try session().output, try shop(text), "setup's output")
    }
}

/// The Given steps that write a setup script, line by line.
@MainActor
private func mixScriptSteps(_ s: MixShellState) -> [StepDefinition] {
    [
        def(#"^the "([^"]*)" setup has browser_open "([^"]*)"$"#) { call in
            try s.addLine(call[0], #"browser_open "\#(call[1])""#)
        },
        def(#"^the "([^"]*)" setup seeds a shopper with a shell command$"#) { call in
            try s.addLine(call[0], #"ok "seeded $(echo sam)""#)
        },
        def(#"^then signs in with app_action signIn '([^']*)'$"#) { call in
            try s.withPage("Card declined", ["app_action signIn '\(call[0])'"])
        },
        def(#"^then has a by_hand step for "([^"]*)" with the text "([^"]*)"$"#) { call in
            try s.addLine("Card declined", #"by_hand "\#(call[0])" "\#(call[1])""#)
        },
        def(#"^the "([^"]*)" setup has app_action addToCart '([^']*)', then ok "([^"]*)"$"#) {
            call in
            try s.withPage(call[0], ["app_action addToCart '\(call[1])'", #"ok "\#(call[2])""#])
        },
        def(#"^the "([^"]*)" setup has ORDER="\$\(in_browser '([^']*)'\)"$"#) { call in
            try s.withPage(
                call[0], [#"ORDER="$(in_browser '\#(call[1])')""#, #"ok "ORDER is $ORDER""#])
        },
        def(#"^the "([^"]*)" setup has in_browser '([^']*)'$"#) { call in
            try s.withPage(call[0], ["in_browser '\(call[1])'"])
        },
        def(#"^the "([^"]*)" setup has app_action refund '([^']*)'$"#) { call in
            try s.withPage(call[0], ["app_action refund '\(call[1])'"])
        },
        def(#"^the "([^"]*)" setup has in_browser with a promise that never settles$"#) { call in
            try s.withPage(call[0], ["in_browser 'new Promise(() => {})'"])
        },
        def(#"^the "([^"]*)" setup signs in as "([^"]*)" with app_action$"#) { call in
            try s.withPage(call[0], [#"app_action signIn '{"email":"\#(call[1])"}'"#])
        },
        def(#"^the "([^"]*)" setup opens the Givens browser$"#) { call in try s.withPage(call[0]) },
        def(#"^then has by_hand --url "([^"]*)" for "([^"]*)"$"#) { call in
            try s.addLine(
                "Card declined",
                #"by_hand --url "\#(call[0])" "\#(call[1])" "Look at the admin page""#)
        },
        def(#"^the "([^"]*)" setup has browser_open and app_action lines$"#) { call in
            try s.withPage(call[0], [#"app_action signIn '{"email":"sam@example.com"}'"#])
        },
        def(#"^the shop's window\.app\.addToCart returns a promise that settles after 2 seconds$"#)
        {
            _ in _ = try s.w.shopURL()
        },
        def(#"^the shop's page returns "ord_42" from window\.app\.lastOrderId\(\)$"#) { _ in
            _ = try s.w.shopURL()
        },
        def(#"^the shop's page has no window\.app\.refund$"#) { _ in _ = try s.w.shopURL() },
        def(#"^Chrome isn't installed$"#) { _ in
            guard s.w.app == nil else { throw StepFailure("serve already started") }
            s.w.environment["GIVENS_CHROME"] = s.w.dir.appendingPathComponent("no-chrome-here").path
        },
    ]
}
