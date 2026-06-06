import AppIntents
import Foundation

/// "Save to TagNote" App Shortcut. Creates a note from supplied text using the
/// signed-in session, so it can be run from Spotlight, the Shortcuts app, the
/// Action button, or Control Center — a legitimate, Apple-sanctioned quick-capture
/// entry point. (Sharing a web page / image stays with the Share Extension; App
/// Intents don't receive share-sheet payloads.)
struct SaveToTagNoteIntent: AppIntent {
    static var title: LocalizedStringResource = "Save to TagNote"
    static var description = IntentDescription("Save text as a new note in TagNote.")
    static var openAppWhenRun = false

    @Parameter(
        title: "Note",
        description: "The text to save as a note.",
        requestValueDialog: "What do you want to save to TagNote?"
    )
    var content: String

    static var parameterSummary: some ParameterSummary {
        Summary("Save \(\.$content) to TagNote")
    }

    func perform() async throws -> some IntentResult & ProvidesDialog {
        let text = content.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else {
            throw TagNoteIntentError(message: "There was nothing to save.")
        }
        guard let token = KeychainStore.read("token"), !token.isEmpty else {
            throw TagNoteIntentError(message: "Open TagNote and sign in before saving.")
        }
        let serverURL = URL(string: KeychainStore.read("serverURL") ?? "https://tag-note.com")

        let api = TagNoteAPI()
        api.configure(serverURL: serverURL, token: token)
        let created = try await api.createNote(content: text, tags: [])

        // Hand the note to the app so it appears immediately on next foreground.
        SharedNoteInbox.append(SubNote(
            id: created.id,
            shortID: created.shortID,
            content: text,
            snippet: nil,
            createdAt: created.createdAt,
            updatedAt: nil,
            tags: [],
            pinned: false
        ))
        return .result(dialog: "Saved to TagNote.")
    }
}

struct TagNoteIntentError: LocalizedError {
    let message: String
    var errorDescription: String? { message }
}

/// Registers the App Shortcut so it appears automatically (no user setup) in
/// Spotlight, the Shortcuts app, and the Action button / Control Center pickers.
struct TagNoteShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(
            intent: SaveToTagNoteIntent(),
            phrases: [
                "Save to \(.applicationName)",
                "Save this to \(.applicationName)",
                "New \(.applicationName) note"
            ],
            shortTitle: "Save to TagNote",
            systemImageName: "tag.fill"
        )
    }
}
