import GivensUI
import SwiftUI

/// Shows the front window's model: the welcome window or a project. Each view reads only the
/// model's snapshot fields, the same ones WindowModel.visible lists.
struct RootView: View {
    let app: AppModel

    var body: some View {
        if let window = app.front {
            switch window.kind {
            case "project": ProjectView(window: window)
            default: WelcomeView(app: app, window: window)
            }
        } else {
            ProgressView()
        }
    }
}

/// "Open folder…" and the recent projects.
struct WelcomeView: View {
    let app: AppModel
    let window: WindowModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(window.snapshot?.messages ?? [], id: \.self) { Text($0) }
            ForEach(window.offers, id: \.self) { offer in
                Button(offer) { app.run { _ in try await window.choose(offer) } }
            }
            if !app.recents.isEmpty {
                Text("Recent").font(.headline)
                ForEach(app.recents, id: \.self) { path in
                    Button(path) { app.run { _ in try await window.openRecent(path) } }
                }
            }
        }
        .padding()
    }
}

/// A project: sidebar on the left, the selected capability or scenario on the right.
/// Ref: https://developer.apple.com/documentation/swiftui/navigationsplitview
struct ProjectView: View {
    let window: WindowModel

    var body: some View {
        NavigationSplitView {
            List(window.sidebar, id: \.name) { item in
                Button {
                    window.app.run { _ in try await window.select(item.name) }
                } label: {
                    HStack {
                        Text(item.name)
                        Spacer()
                        if item.count > 0 { Text("\(item.count)").foregroundStyle(.secondary) }
                    }
                }
                .help(item.warning)
            }
        } detail: {
            DetailView(window: window)
        }
        .navigationTitle(window.title)
    }
}

/// Messages, the shown capability or scenario, the Drop in page and the offered actions.
struct DetailView: View {
    let window: WindowModel

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 8) {
                ForEach(window.snapshot?.messages ?? [], id: \.self) { Text($0).bold() }
                if let capability = window.snapshot?.capability {
                    Text(capability.name).font(.title)
                    ForEach(capability.story, id: \.self) { Text($0) }
                    if !capability.about.isEmpty { Text(capability.about) }
                    ForEach(capability.scenarios, id: \.slug) { scenario in
                        Text("\(scenario.name) — \(scenario.status)")
                    }
                }
                if let scenario = window.snapshot?.scenario {
                    Text(scenario.name).font(.title2)
                    Text(scenario.tags.joined(separator: " ")).foregroundStyle(.secondary)
                    ForEach(scenario.steps, id: \.self) { Text($0).monospaced() }
                }
                if let session = window.snapshot?.session {
                    ForEach(session.output, id: \.self) { Text($0).monospaced() }
                    ForEach(session.checklist, id: \.text) { Text("\($0.mark) \($0.text)") }
                }
                HStack {
                    ForEach(window.offers, id: \.self) { offer in
                        Button(offer) { window.app.run { _ in try await window.choose(offer) } }
                    }
                }
            }
            .padding()
        }
    }
}
