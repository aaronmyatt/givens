import GivensUI
import SwiftUI

/// What the app says after an action: a small notification at the foot of the window that
/// fades after a moment, red and longer for a failure; and "Working…" while an action takes
/// more than a moment. One for the app, since every window's actions go through
/// AppModel.run.
/// Ref: https://developer.apple.com/documentation/observation/observable()
@MainActor
@Observable
final class Feedback {
    /// The app's one Feedback.
    static let shared = Feedback()

    /// One notification.
    struct Note: Identifiable, Equatable {
        let id = UUID()
        let text: String
        let symbol: String
        let failure: Bool
    }

    /// The notification on show, or nil.
    private(set) var note: Note?
    /// Actions still running.
    private(set) var running = 0
    /// True once an action has run longer than `slowAfter`, until none are running.
    private(set) var slow = false

    /// How long an action runs before "Working…" shows, so quick ones don't flicker.
    static let slowAfter: Duration = .milliseconds(400)

    /// Shows a notification: a tick for a success, a warning for a failure.
    func say(_ text: String, symbol: String = "checkmark.circle.fill", failure: Bool = false) {
        precondition(!text.isEmpty, "a notification has text")
        let shown = Note(text: text, symbol: symbol, failure: failure)
        note = shown
        Task {
            try? await Task.sleep(for: failure ? .seconds(6) : .seconds(2.2))
            if note?.id == shown.id { note = nil }
        }
    }

    /// Shows a failure.
    func fail(_ error: any Error) {
        say(error.localizedDescription, symbol: "exclamationmark.triangle.fill", failure: true)
    }

    /// Hides the notification at once, as a click on it does.
    func dismiss() { note = nil }

    /// Counts an action in; "Working…" shows if it's still running after `slowAfter`.
    func begin() {
        running += 1
        Task {
            try? await Task.sleep(for: Self.slowAfter)
            if running > 0 { slow = true }
        }
    }

    /// Counts an action out.
    func end() {
        precondition(running > 0, "an action ends after it begins")
        running -= 1
        if running == 0 { slow = false }
    }

    /// What to say after choosing an offer, for offers whose result isn't plain on screen.
    /// Copies and moves to the Trash say themselves, through the clipboard and Trash.
    static func after(_ offer: String) -> String? {
        switch offer {
        case "Trust", "Trust this project": "Project trusted: its scripts can run"
        case "Don't trust": "Scripts won't run for this project"
        case "Create script", "Write setup steps": "Setup created"
        case "Add teardown": "Teardown created"
        case "Reset state", "Try again", "Run setup anyway": "Running setup again"
        case "Stop": "Setup stopped"
        case "Keep waiting": "Waiting a while longer"
        case "Keep mine": "Kept your version"
        case "Use theirs": "Took the version on disk"
        case "Replace anyway": "Kit replaced"
        case "Move to its own capability": "Moved to its own capability"
        case "Create capabilities folder": "Capabilities folder created"
        case "Leave": "Left the Drop in; its marks weren't recorded"
        case "Delete", "Delete anyway": "Deleted"
        case "Discard changes": "Changes discarded"
        default: nil
        }
    }
}

/// The notification and "Working…", floating at the foot of a window.
struct FeedbackOverlay: View {
    @State private var feedback = Feedback.shared

    var body: some View {
        VStack(spacing: 8) {
            if feedback.slow {
                HStack(spacing: 8) {
                    ProgressView().controlSize(.small)
                    Text("Working…")
                }
                .modifier(Pill(tint: .secondary))
                .transition(.opacity)
            }
            if let note = feedback.note {
                let tint = note.failure ? Theme.error : Theme.happy
                HStack(alignment: .firstTextBaseline, spacing: 8) {
                    Image(systemName: note.symbol).foregroundStyle(tint)
                    Text(note.text).lineLimit(3).multilineTextAlignment(.leading)
                }
                .modifier(Pill(tint: tint))
                .onTapGesture { feedback.dismiss() }
                .help("Click to dismiss")
                .id(note.id)
                .transition(.move(edge: .bottom).combined(with: .opacity))
                .accessibilityAddTraits(.isStaticText)
            }
        }
        .frame(maxWidth: 520)
        .padding(.bottom, 20)
        .animation(.spring(duration: 0.3), value: feedback.note)
        .animation(.easeInOut(duration: 0.2), value: feedback.slow)
    }
}

/// A rounded, raised capsule with its symbol in the tint colour.
private struct Pill: ViewModifier {
    let tint: Color

    func body(content: Content) -> some View {
        content
            .padding(.horizontal, 16)
            .padding(.vertical, 10)
            .background(.regularMaterial, in: Capsule())
            .overlay(Capsule().stroke(tint.opacity(0.45)))
            .shadow(color: .black.opacity(0.15), radius: 12, y: 4)
    }
}

/// The system clipboard, saying what was copied.
@MainActor
final class SayingClipboard: Clipboard {
    let wrapped: any Clipboard

    init(_ wrapped: any Clipboard) { self.wrapped = wrapped }

    func writeText(_ text: String) {
        wrapped.writeText(text)
        let first = text.split(separator: "\n").first.map(String.init) ?? ""
        let shown = first.count > 60 ? String(first.prefix(60)) + "…" : first
        Feedback.shared.say(
            "Copied: \(shown)" + (text.contains("\n") ? " …" : ""), symbol: "doc.on.doc.fill")
    }

    func readText() -> String { wrapped.readText() }
}

/// The Finder Trash, saying what went there.
@MainActor
final class SayingTrash: Trash {
    let wrapped: any Trash

    init(_ wrapped: any Trash) { self.wrapped = wrapped }

    func move(_ path: String) throws {
        try wrapped.move(path)
        let name = URL(fileURLWithPath: path).lastPathComponent
        Feedback.shared.say("Moved \(name) to the Trash", symbol: "trash.fill")
    }
}
