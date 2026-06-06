import Foundation

@MainActor
final class NotesViewModel: ObservableObject {
    @Published var notes: [SubNote] = []
    @Published var availableTags: [TagInfo] = []
    @Published var selectedTags: [String] = []
    @Published var query = ""
    @Published var sort = "newest"
    @Published var isLoading = false
    @Published var isLoadingMore = false
    @Published var errorMessage: String?
    @Published var isCreateRequested = false

    let api: TagNoteAPI
    private let cache: LocalCache
    private let pageSize = 30
    private var offset = 0
    private var canLoadMore = true
    private var refreshGeneration = 0

    init(api: TagNoteAPI, cache: LocalCache) {
        self.api = api
        self.cache = cache
    }

    func loadCached() async {
        notes = await cache.notes()
        availableTags = await cache.tags()
    }

    func refresh() async {
        refreshGeneration += 1
        let generation = refreshGeneration
        offset = 0
        canLoadMore = true
        isLoading = true
        errorMessage = nil
        defer {
            if generation == refreshGeneration {
                isLoading = false
            }
        }
        do {
            let page = try await api.listNotes(tags: selectedTags, query: query, sort: sort, limit: pageSize, offset: 0)
            guard generation == refreshGeneration else { return }
            notes = page
            offset = page.count
            canLoadMore = page.count == pageSize
            await cache.saveNotes(page)
            await refreshTagFilters()
        } catch {
            guard generation == refreshGeneration, !error.isCancellationLike else { return }
            errorMessage = error.localizedDescription
            if notes.isEmpty {
                notes = await cache.notes()
            }
        }
    }

    /// Inserts notes handed over by the Share Extension at the top of the feed so
    /// they appear instantly, ahead of the server refresh that reconciles them.
    /// Deduplicates by id; a following refresh() replaces them with the canonical
    /// server copies (and drops any that don't match the active filter).
    func mergeOptimistic(_ incoming: [SubNote]) {
        guard !incoming.isEmpty else { return }
        let existing = Set(notes.map { $0.id })
        let fresh = incoming.filter { !existing.contains($0.id) }
        guard !fresh.isEmpty else { return }
        notes.insert(contentsOf: fresh.reversed(), at: 0)
    }

    func refreshTagFilters() async {
        do {
            let tags = try await api.listTagsDetailed()
            availableTags = tags.sorted { $0.name < $1.name }
            await cache.saveTags(availableTags)
        } catch {
            if availableTags.isEmpty {
                availableTags = await cache.tags()
            }
        }
    }

    func loadMoreIfNeeded(current note: SubNote?) async {
        guard let note, note.id == notes.last?.id, canLoadMore, !isLoadingMore else { return }
        isLoadingMore = true
        defer { isLoadingMore = false }
        do {
            let page = try await api.listNotes(tags: selectedTags, query: query, sort: sort, limit: pageSize, offset: offset)
            notes.append(contentsOf: page)
            offset += page.count
            canLoadMore = page.count == pageSize
            await cache.saveNotes(notes)
        } catch {
            guard !error.isCancellationLike else { return }
            errorMessage = error.localizedDescription
        }
    }

    func toggleTagFilter(_ tag: String) async {
        if selectedTags.contains(tag) {
            selectedTags.removeAll { $0 == tag }
        } else {
            selectedTags.append(tag)
        }
        await refresh()
    }

    func setTagFilters(_ tags: [String]) async {
        selectedTags = Array(distinctNormalizedTags(from: tags))
        await refresh()
    }

    func clearFilters() async {
        selectedTags = []
        query = ""
        await refresh()
    }

    func presentCreate() {
        isCreateRequested = true
    }

    func consumeCreateRequest() {
        isCreateRequested = false
    }

    func delete(_ note: SubNote) async {
        do {
            try await api.deleteNote(id: note.routeID)
            notes.removeAll { $0.id == note.id }
            await cache.saveNotes(notes)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func togglePin(_ note: SubNote) async {
        do {
            try await api.togglePin(id: note.routeID)
            if let index = notes.firstIndex(where: { $0.id == note.id }) {
                notes[index].pinned.toggle()
                notes.sort { lhs, rhs in
                    if lhs.pinned != rhs.pinned { return lhs.pinned && !rhs.pinned }
                    return lhs.displayDate > rhs.displayDate
                }
                await cache.saveNotes(notes)
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}

private func distinctNormalizedTags(from tags: [String]) -> [String] {
    var seen = Set<String>()
    var normalized: [String] = []
    for tag in tags {
        let value = tag
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "#"))
        guard !value.isEmpty else { continue }
        let key = value.lowercased()
        guard !seen.contains(key) else { continue }
        seen.insert(key)
        normalized.append(value)
    }
    return normalized
}

private extension Error {
    var isCancellationLike: Bool {
        if self is CancellationError {
            return true
        }
        let nsError = self as NSError
        return nsError.domain == NSURLErrorDomain && nsError.code == NSURLErrorCancelled
    }
}
