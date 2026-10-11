import GivensUI
import SwiftUI

/// Where each offered action is drawn. An offer whose page isn't on show, or that no page
/// places, goes in the banner at the top of the detail column with the window's messages, so
/// a question like "Delete Card declined?" always has its answers beside it.
enum Placement {
    /// The toolbar's + menu.
    static let toolbar = ["New story", "New scenario"]
    /// A scenario's page, beside its setup script.
    static let scenario = [
        "Copy", "Drop in", "Create script", "Write setup steps", "Edit script",
        "Edit setup", "Add teardown", "Edit teardown", "Edit",
    ]
    /// The Drop in page.
    static let dropIn = [
        "Reset state", "Record run", "Leave", "Stop", "Keep waiting", "Try again", "Edit script",
        "Edit setup", "Add teardown", "Edit teardown", "Run setup anyway", "Done", "Failed",
        "Continue", "Open", "Copy",
    ]
    /// The editor's header.
    static let editor = ["Preview", "Close", "Keep mine", "Use theirs", "Compare"]
}

/// A project: capabilities and views on the left, the chosen capability's story and
/// scenarios in the middle, the chosen scenario, its Drop in or the editor on the right.
/// Ref: https://developer.apple.com/documentation/swiftui/navigationsplitview
struct ProjectView: View {
    let window: WindowModel
    @State private var dialog: AuthoringDialog?

    var body: some View {
        NavigationSplitView {
            Sidebar(window: window)
                .navigationSplitViewColumnWidth(min: 180, ideal: 220, max: 300)
        } content: {
            ContentColumn(window: window)
                .navigationSplitViewColumnWidth(min: 260, ideal: 340, max: 480)
        } detail: {
            DetailColumn(window: window)
        }
        // The search panel floats over the whole window; a click anywhere outside it closes
        // it, as Esc does.
        .overlay(alignment: .top) {
            if let search = window.snapshot?.search {
                ZStack(alignment: .top) {
                    Color.black.opacity(0.06)
                        .contentShape(Rectangle())
                        .onTapGesture { window.app.run { _ in try await window.press("⎋") } }
                        .accessibilityLabel("Close search")
                    SearchPanel(window: window, search: search).padding(24)
                }
                .transition(.opacity)
            }
        }
        .animation(.easeOut(duration: 0.15), value: window.snapshot?.search == nil)
        .navigationTitle("Givens")
        .navigationSubtitle(window.title)
        .toolbar {
            // ⌘K opens the search panel, whose own field takes the query, as in the mockup.
            // Ref: https://developer.apple.com/documentation/swiftui/view/keyboardshortcut(_:modifiers:)
            ToolbarItem(placement: .primaryAction) {
                Button {
                    window.app.run { _ in try await window.press("⌘K") }
                } label: {
                    Label("Search", systemImage: "magnifyingglass")
                }
                .keyboardShortcut("k")
                .help("Search ⌘K")
            }
            ToolbarItem(placement: .primaryAction) {
                Menu {
                    ForEach(window.offers.filter(Placement.toolbar.contains), id: \.self) {
                        offer in
                        Button(offer) { dialog = offer == "New story" ? .story : .scenario }
                    }
                } label: {
                    Label("New", systemImage: "plus")
                }
                .disabled(window.offers.filter(Placement.toolbar.contains).isEmpty)
            }
        }
        .sheet(item: $dialog) { kind in AuthoringSheet(window: window, kind: kind) }
    }
}

/// The left column: capabilities with their scenario counts, then views and tags. The
/// capability on show stays highlighted while one of its scenarios is open.
/// Ref: https://developer.apple.com/documentation/swiftui/list
struct Sidebar: View {
    let window: WindowModel

    var body: some View {
        List(selection: selection) {
            SidebarSection(window: window, title: "Capabilities", kind: "capability")
            SidebarSection(window: window, title: "Views", kind: "view")
            SidebarSection(window: window, title: "Tags", kind: "tag")
        }
        .listStyle(.sidebar)
    }

    /// The highlighted line: the capability on show, or the selected view or tag.
    private var selection: Binding<String?> {
        Binding(
            get: {
                let selected = window.selected
                let inSidebar = window.sidebar.contains { $0.name == selected }
                return inSidebar ? selected : window.snapshot?.capability?.name
            },
            set: { name in
                guard let name, name != window.selected else { return }
                window.app.run { _ in try await window.select(name) }
            })
    }
}

/// One part of the sidebar: the items of one kind, each with its icon, count and warning.
struct SidebarSection: View {
    let window: WindowModel
    let title: String
    let kind: String

    var body: some View {
        let items = window.sidebar.filter { $0.kind == kind }
        if !items.isEmpty {
            Section(title) {
                ForEach(items, id: \.name) { item in
                    Label {
                        HStack {
                            Text(item.name).lineLimit(1)
                            if !item.warning.isEmpty {
                                Image(systemName: "exclamationmark.triangle")
                                    .foregroundStyle(.orange)
                                    .accessibilityLabel(item.warning)
                            }
                        }
                    } icon: {
                        Image(systemName: Self.symbol(item))
                    }
                    .badge(item.count)
                    .help(item.warning.isEmpty ? item.name : item.warning)
                    .tag(item.name)
                    .contextMenu {
                        if kind == "capability" {
                            Button("Delete…") {
                                window.app.run { _ in try await window.delete(item.name) }
                            }
                        }
                    }
                }
            }
        }
    }

    /// An SF Symbol for a sidebar line. Ref: https://developer.apple.com/sf-symbols/
    static func symbol(_ item: SidebarItem) -> String {
        switch (item.kind, item.name) {
        case ("view", "Error states"): "exclamationmark.triangle"
        case ("view", "Unverified"): "clock"
        case ("view", "Shared"): "chevron.left.forwardslash.chevron.right"
        case ("view", _): "line.3.horizontal.decrease.circle"
        case ("tag", _): "tag"
        case ("helper", _): "function"
        default: "doc.text"
        }
    }
}

/// The middle column: the capability on show, or the scenarios or helpers a view lists.
struct ContentColumn: View {
    let window: WindowModel

    var body: some View {
        let selected = window.selected
        let view = window.sidebar.first { $0.name == selected && $0.kind != "capability" }
        SharedHelpers(window: window) { helpers in
            let helperChosen = helpers.contains { $0.name == selected }
            if view?.name == "Shared" || helperChosen {
                HelpersColumn(window: window, helpers: helpers)
            } else if view != nil || !(window.snapshot?.listed ?? []).isEmpty {
                ListedColumn(window: window, title: view?.name ?? selected)
            } else if let capability = window.snapshot?.capability {
                CapabilityColumn(window: window, capability: capability)
            } else {
                ContentUnavailableView(
                    "No capability chosen", systemImage: "doc.text",
                    description: Text("Choose one on the left."))
            }
        }
    }
}

/// The right column: the window's messages and loose offers on top, then the Drop in page,
/// the editor or the scenario, and the ⌘K results over them while search is open.
struct DetailColumn: View {
    let window: WindowModel
    /// A Drop in chosen but not yet run; see PreparePage.
    @State private var staged: StagedDropIn?

    var body: some View {
        VStack(spacing: 0) {
            Banner(window: window, placed: placed)
            Group {
                if let session = window.session {
                    DropInPage(window: window, session: session)
                } else if let editor = window.snapshot?.editor {
                    // A new identity per file, so one file's typing never carries to the next.
                    EditorPage(window: window, editor: editor).id(editor.path)
                } else if let scenario = window.snapshot?.scenario {
                    if let staged, staged.scenario == scenario.name {
                        PreparePage(
                            window: window, scenario: scenario, staged: staged,
                            onBack: { self.staged = nil })
                    } else {
                        ScenarioPage(window: window, scenario: scenario) { row in
                            staged = StagedDropIn(scenario: scenario.name, row: row)
                        }
                    }
                } else {
                    SharedHelpers(window: window) { helpers in
                        if let helper = helpers.first(where: { $0.name == window.selected }) {
                            HelperPage(window: window, helper: helper)
                        } else {
                            ContentUnavailableView(
                                "No scenario chosen", systemImage: "checklist",
                                description: Text("Choose a scenario to read it or drop in."))
                        }
                    }
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        // Once setup runs, the Drop in page takes over; another scenario forgets the stage.
        .onChange(of: window.snapshot?.session == nil) { _, closed in
            if !closed { staged = nil }
        }
        .onChange(of: window.snapshot?.scenario?.name) { _, _ in staged = nil }
    }

    /// The offers a page on show draws itself; the banner draws the rest.
    private var placed: Set<String> {
        var labels = Set(Placement.toolbar)
        if window.session != nil {
            labels.formUnion(Placement.dropIn)
        } else if window.snapshot?.editor != nil {
            labels.formUnion(Placement.editor)
        } else if window.snapshot?.scenario != nil {
            labels.formUnion(Placement.scenario)
        } else {
            labels.insert("Edit")
        }
        return labels
    }
}

/// Messages and labels across the top, like "This project isn't trusted", with the offers
/// no page places: the answers to a question the window is asking.
struct Banner: View {
    let window: WindowModel
    let placed: Set<String>

    var body: some View {
        let messages = window.snapshot?.messages ?? []
        // "No setup yet" sits where the script would, on the scenario page; Shared's helper
        // lines are in its own column.
        let shared = window.selected == "Shared"
        let labels = (window.snapshot?.labels ?? []).filter {
            $0 != ScenarioPage.noSetup && !shared
        }
        let loose = window.offers.filter { !placed.contains($0) }
        if !messages.isEmpty || !labels.isEmpty || !loose.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                ForEach(messages, id: \.self) { Text($0).bold() }
                ForEach(labels, id: \.self) { Text($0).foregroundStyle(.secondary) }
                if !loose.isEmpty {
                    HStack { OfferButtons(window: window, labels: loose) }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(14)
            .background(.yellow.opacity(0.12))
            .overlay(alignment: .bottom) { Divider() }
        }
    }
}

/// Which authoring dialog is up.
enum AuthoringDialog: String, Identifiable {
    case story, scenario
    var id: String { rawValue }
}

/// The New story and New scenario dialogs: their fields, then one call into the core.
/// Ref: https://developer.apple.com/documentation/swiftui/view/sheet(item:ondismiss:content:)
struct AuthoringSheet: View {
    let window: WindowModel
    let kind: AuthoringDialog
    @Environment(\.dismiss) private var dismiss
    @State private var name = ""
    @State private var asA = ""
    @State private var iWant = ""
    @State private var soThat = ""
    @State private var errorState = false

    var body: some View {
        Form {
            Text(kind == .story ? "New story" : "New scenario").font(.title3).bold()
            TextField("Name", text: $name)
            if kind == .story {
                TextField("As a", text: $asA)
                TextField("I want", text: $iWant)
                TextField("So that", text: $soThat)
            } else {
                Toggle("Error state", isOn: $errorState)
            }
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }.keyboardShortcut(.cancelAction)
                Button("Create") {
                    let done = kind == .story ? "Story \(name) created" : "Scenario \(name) created"
                    window.app.run(done) { _ in
                        if kind == .story {
                            try await window.newStory(
                                name: name, asA: asA, iWant: iWant, soThat: soThat)
                        } else {
                            try await window.newScenario(name, error: errorState)
                        }
                    }
                    dismiss()
                }
                .keyboardShortcut(.defaultAction)
                .disabled(name.isEmpty)
            }
        }
        .padding(20)
        .frame(minWidth: 400)
    }
}
