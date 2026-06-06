import Foundation

/// A tiny hand-off channel from the Share Extension to the app, backed by the
/// shared App Group. The extension appends each note it creates; the app drains
/// the inbox on launch / foreground and shows the notes immediately, before the
/// server refresh confirms them. Degrades to a no-op if the App Group isn't
/// available (e.g. unprovisioned), so the app still works via the normal refresh.
enum SharedNoteInbox {
    static let appGroup = "group.com.tag-note.tagnote"
    private static let key = "pendingSharedNotes"

    private static var defaults: UserDefaults? { UserDefaults(suiteName: appGroup) }

    static func append(_ note: SubNote) {
        guard let defaults else { return }
        var notes = decode(defaults.data(forKey: key))
        notes.append(note)
        if let data = try? JSONEncoder().encode(notes) {
            defaults.set(data, forKey: key)
        }
    }

    /// Returns the pending notes and clears the inbox.
    static func drain() -> [SubNote] {
        guard let defaults else { return [] }
        let notes = decode(defaults.data(forKey: key))
        defaults.removeObject(forKey: key)
        return notes
    }

    private static func decode(_ data: Data?) -> [SubNote] {
        guard let data, let notes = try? JSONDecoder().decode([SubNote].self, from: data) else { return [] }
        return notes
    }
}
