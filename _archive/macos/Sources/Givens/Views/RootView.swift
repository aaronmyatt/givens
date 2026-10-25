import GivensUI
import SwiftUI

/// Shows the front window's model: the welcome window or a project. Each view reads only the
/// model's snapshot fields, the same ones WindowModel.visible lists.
struct RootView: View {
    let app: AppModel

    var body: some View {
        Group {
            if let window = app.front {
                switch window.kind {
                case "project": ProjectView(window: window)
                default: WelcomeView(app: app, window: window)
                }
            } else {
                ProgressView()
            }
        }
        .overlay(alignment: .bottom) { FeedbackOverlay() }
    }
}

/// The app icon and name, "Open folder…" and the recent projects.
struct WelcomeView: View {
    let app: AppModel
    let window: WindowModel

    var body: some View {
        HStack(spacing: 0) {
            VStack(spacing: 14) {
                Image(nsImage: NSApplication.shared.applicationIconImage)
                    .resizable()
                    .frame(width: 112, height: 112)
                Text("Givens").font(.largeTitle).bold()
                Text("Set up a scenario. Walk through it.").foregroundStyle(.secondary)
                ForEach(window.snapshot?.messages ?? [], id: \.self) { Text($0) }
                ForEach(window.offers, id: \.self) { offer in
                    Button(offer) { app.run { _ in try await window.choose(offer) } }
                        .buttonStyle(.borderedProminent)
                        .controlSize(.large)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .padding(32)
            if !app.recents.isEmpty {
                Divider()
                List {
                    Section("Recent") {
                        ForEach(app.recents, id: \.self) { path in
                            Button {
                                app.run { _ in try await window.openRecent(path) }
                            } label: {
                                VStack(alignment: .leading, spacing: 2) {
                                    Text(
                                        URL(fileURLWithPath: path).deletingLastPathComponent()
                                            .lastPathComponent)
                                    Text(path).font(.caption).foregroundStyle(.secondary)
                                        .lineLimit(1).truncationMode(.head)
                                }
                            }
                            .buttonStyle(.plain)
                            .help(path)
                        }
                    }
                }
                .frame(width: 280)
            }
        }
        .frame(minWidth: 640, minHeight: 400)
    }
}
