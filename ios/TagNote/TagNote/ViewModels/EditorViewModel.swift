import Foundation

@MainActor
final class EditorViewModel: ObservableObject {
    @Published var content: String
    @Published var tags: [String]
    @Published var suggestions: [String] = []
    @Published var saveStatus: SaveStatus = .idle
    @Published var previewMode: PreviewMode = .write
    @Published var isPinned: Bool
    @Published var errorMessage: String?

    private let api: TagNoteAPI
    private var noteID: String?
    private var lastSavedContent: String
    private var lastSavedTags: [String]
    private var autosaveTask: Task<Void, Never>?
    private var saveTask: Task<Void, Never>?

    var isNewNote: Bool { noteID == nil }
    var canCloseCleanly: Bool { !hasChanges || saveStatus == .saved || saveStatus == .idle }
    var hasChanges: Bool { content.trimmed != lastSavedContent || tags != lastSavedTags }

    init(note: SubNote?, api: TagNoteAPI) {
        self.api = api
        self.noteID = note?.routeID
        self.content = note?.content ?? ""
        self.tags = note?.tags ?? []
        self.lastSavedContent = note?.content.trimmed ?? ""
        self.lastSavedTags = note?.tags ?? []
        self.isPinned = note?.pinned ?? false
        self.saveStatus = note == nil ? .idle : .saved
    }

    func scheduleAutosave() {
        autosaveTask?.cancel()
        guard hasChanges else {
            saveStatus = .saved
            return
        }
        guard isValidDraft else {
            saveStatus = .invalid(invalidReason)
            return
        }
        saveStatus = .unsaved
        autosaveTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: 1_500_000_000)
            guard !Task.isCancelled else { return }
            await self?.save(closeAfterSave: false)
        }
    }

    func saveNow() async {
        autosaveTask?.cancel()
        await save(closeAfterSave: false)
    }

    func autocomplete(_ fragment: String) async {
        let value = fragment.trimmed
        guard !value.isEmpty else {
            suggestions = []
            return
        }
        suggestions = (try? await api.autocompleteTags(query: value)) ?? []
    }

    func addTag(_ rawTag: String) {
        let tag = rawTag.trimmed.lowercased()
        guard !tag.isEmpty, !tags.contains(tag) else { return }
        tags.append(tag)
        scheduleAutosave()
    }

    func removeTag(_ tag: String) {
        tags.removeAll { $0 == tag }
        scheduleAutosave()
    }

    func togglePin() async {
        guard let noteID else { return }
        do {
            try await api.togglePin(id: noteID)
            isPinned.toggle()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func delete() async throws {
        guard let noteID else { return }
        try await api.deleteNote(id: noteID)
    }

    func uploadImage(data: Data, mimeType: String) async {
        do {
            let ext = mimeType == "image/png" ? "png" : "jpg"
            let path = try await api.uploadImage(data: data, fileName: "note-image.\(ext)", mimeType: mimeType)
            content += "\n\n![](\(path))"
            scheduleAutosave()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func save(closeAfterSave: Bool) async {
        guard isValidDraft else {
            saveStatus = .invalid(invalidReason)
            return
        }
        if saveTask != nil {
            await saveTask?.value
            if hasChanges {
                await save(closeAfterSave: closeAfterSave)
            }
            return
        }

        let task = Task { [weak self] in
            guard let self else { return }
            await self.performSave()
        }
        saveTask = task
        await task.value
        saveTask = nil
    }

    private func performSave() async {
        saveStatus = .saving
        do {
            if let noteID {
                let saved = try await api.updateNote(id: noteID, content: content.trimmed, tags: tags)
                self.content = saved.content
                self.tags = saved.tags
                self.lastSavedContent = saved.content.trimmed
                self.lastSavedTags = saved.tags
            } else {
                let created = try await api.createNote(content: content.trimmed, tags: tags)
                self.noteID = created.shortID
                self.lastSavedContent = content.trimmed
                self.lastSavedTags = tags
            }
            saveStatus = .saved
        } catch {
            saveStatus = .failed("Save failed")
            errorMessage = error.localizedDescription
        }
    }

    private var isValidDraft: Bool {
        !content.trimmed.isEmpty && !tags.isEmpty
    }

    // A short, actionable reason the draft isn't autosaving yet. A note is
    // tag-first, so it needs both content and at least one tag.
    private var invalidReason: String {
        if tags.isEmpty {
            return "Add a tag"
        }
        if content.trimmed.isEmpty {
            return "Add content"
        }
        return "Add a tag"
    }
}

private extension String {
    var trimmed: String {
        trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
