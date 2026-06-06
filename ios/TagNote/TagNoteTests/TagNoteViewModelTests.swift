import XCTest
@testable import TagNote

@MainActor
final class TagNoteViewModelTests: XCTestCase {
    private var api: TagNoteAPI!
    private var cache: LocalCache!

    override func setUp() async throws {
        try await super.setUp()
        ViewModelURLProtocolStub.reset()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [ViewModelURLProtocolStub.self]
        api = TagNoteAPI(session: URLSession(configuration: configuration))
        api.configure(serverURL: URL(string: "https://notes.example.test"), token: "jwt-token")
        cache = LocalCache()
        await cache.clear()
    }

    override func tearDown() async throws {
        await cache.clear()
        api = nil
        cache = nil
        ViewModelURLProtocolStub.reset()
        try await super.tearDown()
    }

    func testNotesRefreshSearchTagFilterPinDeleteAndCacheFallback() async throws {
        await cache.saveNotes([Self.note(id: "cached", shortID: "cached", content: "Cached", tags: ["offline"])])
        await cache.saveTags([Self.tag("offline")])

        var seenRequests: [(String, String, [URLQueryItem])] = []
        ViewModelURLProtocolStub.handler = { request in
            let components = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)
            seenRequests.append((request.httpMethod ?? "", request.url?.path ?? "", components?.queryItems ?? []))

            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/api/v1/notes"):
                let query = components?.queryItems ?? []
                if query.contains(URLQueryItem(name: "tag", value: "ios")) {
                    return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(Self.noteJSON(id: "note-2", shortID: "short-2", content: "Filtered", tags: ["ios"], pinned: false))]")
                }
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(Self.noteJSON(id: "note-1", shortID: "short-1", content: "Markdown search hit", tags: ["swift"], pinned: false))]")
            case ("GET", "/api/v1/tags/detailed"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: """
                [
                  {"name":"swift","status":"approved","note_count":1,"importance":2,"urgency":3},
                  {"name":"ios","status":"unreviewed","note_count":1,"importance":1,"urgency":1}
                ]
                """)
            case ("PUT", "/api/v1/notes/short-2/pin"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            case ("DELETE", "/api/v1/notes/short-2"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let viewModel = NotesViewModel(api: api, cache: cache)
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.notes.first?.content, "Cached")
        XCTAssertEqual(viewModel.availableTags.first?.name, "offline")

        viewModel.query = "Markdown"
        await viewModel.refresh()
        XCTAssertEqual(viewModel.notes.first?.content, "Markdown search hit")
        XCTAssertEqual(viewModel.availableTags.map(\.name), ["ios", "swift"])

        await viewModel.toggleTagFilter("ios")
        XCTAssertEqual(viewModel.selectedTags, ["ios"])
        XCTAssertEqual(viewModel.notes.first?.content, "Filtered")

        ViewModelURLProtocolStub.handler = { request in
            HTTPURLResponse.vmStub(url: request.url!, statusCode: 503, body: #"{"error":"offline"}"#)
        }

        let fallback = NotesViewModel(api: api, cache: cache)
        await fallback.refresh()
        XCTAssertEqual(fallback.notes.first?.content, "Filtered")
        XCTAssertEqual(fallback.errorMessage, "offline")

        ViewModelURLProtocolStub.handler = { request in
            switch (request.httpMethod, request.url?.path) {
            case ("PUT", "/api/v1/notes/short-2/pin"),
                 ("DELETE", "/api/v1/notes/short-2"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let filtered = try XCTUnwrap(viewModel.notes.first)
        await viewModel.togglePin(filtered)
        XCTAssertEqual(viewModel.notes.first?.pinned, true)

        await viewModel.delete(filtered)
        XCTAssertEqual(viewModel.notes, [])

        XCTAssertTrue(seenRequests.contains { _, path, items in
            path == "/api/v1/notes" && items.contains(URLQueryItem(name: "q", value: "Markdown"))
        })
        XCTAssertTrue(seenRequests.contains { _, path, items in
            path == "/api/v1/notes" && items.contains(URLQueryItem(name: "tag", value: "ios"))
        })
    }

    func testNotesPaginationAppendsAndStopsAtShortPage() async throws {
        var offsets: [String?] = []
        ViewModelURLProtocolStub.handler = { request in
            let items = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)?.queryItems ?? []
            if request.url?.path == "/api/v1/notes" {
                offsets.append(items.first(where: { $0.name == "offset" })?.value)
            }
            let offset = items.first(where: { $0.name == "offset" })?.value ?? "0"
            if offset == "0" {
                let notes = (0..<30).map {
                    Self.noteJSON(id: "note-\($0)", shortID: "short-\($0)", content: "Page 1 note \($0)", tags: ["page"], pinned: false)
                }.joined(separator: ",")
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(notes)]")
            }
            return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(Self.noteJSON(id: "note-31", shortID: "short-31", content: "Page 2 note", tags: ["page"], pinned: false))]")
        }

        let viewModel = NotesViewModel(api: api, cache: cache)
        await viewModel.refresh()
        await viewModel.loadMoreIfNeeded(current: viewModel.notes.last)
        await viewModel.loadMoreIfNeeded(current: viewModel.notes.last)

        XCTAssertEqual(viewModel.notes.count, 31)
        XCTAssertEqual(offsets, ["0", "30"])
    }

    func testNotesRefreshIgnoresCancelledSearchRequests() async throws {
        ViewModelURLProtocolStub.handler = { request in
            throw URLError(.cancelled)
        }

        let viewModel = NotesViewModel(api: api, cache: cache)
        viewModel.notes = [Self.note(id: "existing", shortID: "existing", content: "Existing", tags: ["ios"])]
        await viewModel.refresh()

        XCTAssertNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.notes.map(\.content), ["Existing"])
        XCTAssertFalse(viewModel.isLoading)
    }

    func testNotesRefreshKeepsLatestSearchResultWhenEarlierRequestFinishesLast() async throws {
        ViewModelURLProtocolStub.asyncHandler = { request in
            let components = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)
            let query = components?.queryItems?.first(where: { $0.name == "q" })?.value

            if request.url?.path == "/api/v1/tags/detailed" {
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[]")
            }

            if query == "old" {
                try await Task.sleep(nanoseconds: 200_000_000)
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(Self.noteJSON(id: "old", shortID: "old", content: "Old result", tags: ["ios"], pinned: false))]")
            }

            try await Task.sleep(nanoseconds: 10_000_000)
            return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: "[\(Self.noteJSON(id: "new", shortID: "new", content: "New result", tags: ["ios"], pinned: false))]")
        }

        let viewModel = NotesViewModel(api: api, cache: cache)
        viewModel.query = "old"
        let first = Task { await viewModel.refresh() }

        viewModel.query = "new"
        let second = Task { await viewModel.refresh() }
        _ = await (first.value, second.value)

        XCTAssertNil(viewModel.errorMessage)
        XCTAssertEqual(viewModel.notes.map(\.content), ["New result"])
        XCTAssertFalse(viewModel.isLoading)
    }

    func testEditorCreatesUpdatesAutocompletesUploadsPinsDeletesAndValidatesDrafts() async throws {
        var seenPaths: [String] = []
        ViewModelURLProtocolStub.handler = { request in
            seenPaths.append(request.url?.path ?? "")

            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/api/v1/tags/autocomplete"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: #"["ios","ideas"]"#)
            case ("POST", "/api/v1/notes"):
                let payload = try JSONSerialization.jsonObject(with: try XCTUnwrap(request.vmBodyData)) as? [String: Any]
                XCTAssertEqual(payload?["content"] as? String, "Draft")
                XCTAssertEqual(payload?["tags"] as? [String], ["ios"])
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 201, body: """
                {"id":"note-1","short_id":"short-1","created_at":"2026-05-28T08:00:00Z"}
                """)
            case ("PUT", "/api/v1/notes/short-1"):
                let payload = try JSONSerialization.jsonObject(with: try XCTUnwrap(request.vmBodyData)) as? [String: Any]
                XCTAssertEqual(payload?["content"] as? String, "Updated")
                XCTAssertEqual(payload?["tags"] as? [String], ["ios"])
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: Self.noteJSON(id: "note-1", shortID: "short-1", content: "Updated", tags: ["ios"], pinned: false))
            case ("POST", "/api/v1/images"):
                let body = String(data: try XCTUnwrap(request.vmBodyData), encoding: .utf8)
                XCTAssertTrue(body?.contains("image/jpeg") == true)
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: #"{"data":{"file_path":"/uploads/pic.jpg"}}"#)
            case ("PUT", "/api/v1/notes/short-1/pin"),
                 ("DELETE", "/api/v1/notes/short-1"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let editor = EditorViewModel(note: nil, api: api)
        editor.addTag(" IOS ")
        editor.addTag("ios")
        XCTAssertEqual(editor.tags, ["ios"])

        await editor.autocomplete("io")
        XCTAssertEqual(editor.suggestions, ["ios", "ideas"])

        editor.content = " Draft "
        await editor.saveNow()
        XCTAssertEqual(editor.saveStatus, .saved)

        editor.content = "Updated"
        await editor.saveNow()
        XCTAssertEqual(editor.content, "Updated")
        XCTAssertEqual(editor.saveStatus, .saved)

        await editor.uploadImage(data: Data("jpeg".utf8), mimeType: "image/jpeg")
        XCTAssertTrue(editor.content.contains("![](/uploads/pic.jpg)"))

        await editor.togglePin()
        XCTAssertEqual(editor.isPinned, true)

        try await editor.delete()
        XCTAssertTrue(seenPaths.contains("/api/v1/tags/autocomplete"))
        XCTAssertTrue(seenPaths.contains("/api/v1/images"))
        XCTAssertTrue(seenPaths.contains("/api/v1/notes/short-1/pin"))
        XCTAssertTrue(seenPaths.contains("/api/v1/notes/short-1"))
    }

    func testEmptyDraftStaysIdleAndNeverAutosaves() async {
        ViewModelURLProtocolStub.handler = { request in
            XCTFail("an empty draft must not hit the network: \(request.url?.path ?? "")")
            return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: "")
        }

        let editor = EditorViewModel(note: nil, api: api)
        editor.scheduleAutosave()
        XCTAssertEqual(editor.saveStatus, .idle)

        // Give the (cancelled) autosave window a chance to misfire.
        try? await Task.sleep(nanoseconds: 1_800_000_000)
        XCTAssertEqual(editor.saveStatus, .idle)
    }

    func testContentOnlyDraftSavesWithNoTag() async {
        ViewModelURLProtocolStub.handler = { request in
            switch (request.httpMethod, request.url?.path) {
            case ("POST", "/api/v1/notes"):
                let payload = try? JSONSerialization.jsonObject(with: request.vmBodyData ?? Data()) as? [String: Any]
                XCTAssertEqual(payload?["content"] as? String, "Tagless")
                XCTAssertEqual(payload?["tags"] as? [String], [])
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 201, body: """
                {"id":"note-9","short_id":"short-9","created_at":"2026-05-28T08:00:00Z"}
                """)
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let editor = EditorViewModel(note: nil, api: api)
        editor.content = "Tagless"
        await editor.saveNow()
        XCTAssertEqual(editor.saveStatus, .saved)
        XCTAssertTrue(editor.tags.isEmpty)
    }

    func testAddTagIgnoresReservedDefault() {
        let editor = EditorViewModel(note: nil, api: api)
        editor.addTag("$default")
        editor.addTag(" $DEFAULT ")
        XCTAssertTrue(editor.tags.isEmpty)
    }

    func testRemovingLastTagWithContentStillAutosaves() {
        let note = Self.note(id: "n", shortID: "n", content: "Body", tags: ["work"])
        let editor = EditorViewModel(note: note, api: api)
        editor.removeTag("work")
        XCTAssertTrue(editor.tags.isEmpty)
        // Content remains, so the draft is still eligible to autosave.
        XCTAssertEqual(editor.saveStatus, .unsaved)
    }

    func testTagsViewModelFiltersMutatesAndFallsBackToCache() async throws {
        await cache.saveTags([Self.tag("cached", status: "approved")])
        var detailedCalls = 0
        ViewModelURLProtocolStub.handler = { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/api/v1/tags/detailed"):
                detailedCalls += 1
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: """
                [
                  {"name":"ios","status":"approved","note_count":2,"importance":3,"urgency":4},
                  {"name":"draft","status":"unreviewed","note_count":1,"importance":1,"urgency":1}
                ]
                """)
            case ("PUT", "/api/v1/tags/draft/approve"),
                 ("PUT", "/api/v1/tags/approve-all"),
                 ("PUT", "/api/v1/tags/draft/rename"),
                 ("PUT", "/api/v1/tags/draft/priority"),
                 ("DELETE", "/api/v1/tags/draft"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let viewModel = TagsViewModel(api: api, cache: cache)
        await viewModel.loadCached()
        XCTAssertEqual(viewModel.tags.first?.name, "cached")

        await viewModel.refresh()
        viewModel.filter = .unreviewed
        XCTAssertEqual(viewModel.visibleTags.map(\.name), ["draft"])
        viewModel.query = "io"
        viewModel.filter = .all
        XCTAssertEqual(viewModel.visibleTags.map(\.name), ["ios"])

        let draft = Self.tag("draft", status: "unreviewed")
        await viewModel.approve(draft)
        await viewModel.approveAll()
        await viewModel.rename(draft, to: "idea")
        await viewModel.updatePriority(draft, importance: 4, urgency: 5)
        await viewModel.delete(draft)
        XCTAssertGreaterThanOrEqual(detailedCalls, 6)

        ViewModelURLProtocolStub.handler = { request in
            HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"offline"}"#)
        }
        let fallback = TagsViewModel(api: api, cache: cache)
        await fallback.refresh()
        XCTAssertEqual(fallback.tags.map(\.name), ["ios", "draft"])
        XCTAssertEqual(fallback.errorMessage, "offline")
    }

    func testTrashRestoreAndPurgeRemoveNotes() async throws {
        ViewModelURLProtocolStub.handler = { request in
            switch (request.httpMethod, request.url?.path) {
            case ("GET", "/api/v1/notes/trash"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 200, body: """
                [
                  \(Self.noteJSON(id: "note-1", shortID: "short-1", content: "Deleted 1", tags: ["trash"], pinned: false)),
                  \(Self.noteJSON(id: "note-2", shortID: "short-2", content: "Deleted 2", tags: ["trash"], pinned: false))
                ]
                """)
            case ("PUT", "/api/v1/notes/short-1/restore"),
                 ("DELETE", "/api/v1/notes/short-2/permanent"):
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 204, body: "")
            default:
                return HTTPURLResponse.vmStub(url: request.url!, statusCode: 500, body: #"{"error":"unexpected"}"#)
            }
        }

        let viewModel = TrashViewModel(api: api)
        await viewModel.refresh()
        XCTAssertEqual(viewModel.notes.count, 2)

        await viewModel.restore(viewModel.notes[0])
        XCTAssertEqual(viewModel.notes.map(\.routeID), ["short-2"])

        await viewModel.purge(viewModel.notes[0])
        XCTAssertEqual(viewModel.notes, [])
    }

    func testMergeOptimisticPrependsNewestFirstAndDeduplicates() {
        let viewModel = NotesViewModel(api: api, cache: cache)
        viewModel.notes = [Self.note(id: "a", shortID: "a", content: "A", tags: [])]

        // Inbox order is append order (oldest → newest); merge shows newest on top.
        viewModel.mergeOptimistic([
            Self.note(id: "b", shortID: "b", content: "B", tags: ["x"]),
            Self.note(id: "c", shortID: "c", content: "C", tags: []),
            Self.note(id: "a", shortID: "a", content: "A", tags: [])  // already present
        ])

        XCTAssertEqual(viewModel.notes.map(\.id), ["c", "b", "a"])
    }

    func testSharedNoteInboxAppendAndDrain() {
        _ = SharedNoteInbox.drain()  // start clean
        SharedNoteInbox.append(Self.note(id: "shared-1", shortID: "shared-1", content: "Shared", tags: ["t"]))

        let drained = SharedNoteInbox.drain()
        XCTAssertEqual(drained.map(\.id), ["shared-1"])
        XCTAssertEqual(drained.first?.tags, ["t"])
        XCTAssertTrue(SharedNoteInbox.drain().isEmpty, "drain() must clear the inbox")
    }

    private static func note(id: String, shortID: String, content: String, tags: [String], pinned: Bool = false) -> SubNote {
        SubNote(id: id, shortID: shortID, content: content, snippet: nil, createdAt: Date(timeIntervalSince1970: 0), updatedAt: nil, tags: tags, pinned: pinned)
    }

    private static func tag(_ name: String, status: String = "unreviewed") -> TagInfo {
        TagInfo(name: name, status: status, noteCount: 1, importance: 1, urgency: 1)
    }

    private static func noteJSON(id: String, shortID: String, content: String, tags: [String], pinned: Bool) -> String {
        let tagJSON = tags.map { #""\#($0)""# }.joined(separator: ",")
        return """
        {
          "id": "\(id)",
          "short_id": "\(shortID)",
          "content": "\(content)",
          "created_at": "2026-05-28T08:00:00Z",
          "updated_at": "2026-05-28T09:00:00Z",
          "tags": [\(tagJSON)],
          "pinned": \(pinned)
        }
        """
    }
}

private final class ViewModelURLProtocolStub: URLProtocol {
    typealias Handler = (URLRequest) throws -> (HTTPURLResponse, Data)
    typealias AsyncHandler = (URLRequest) async throws -> (HTTPURLResponse, Data)

    static var handler: Handler?
    static var asyncHandler: AsyncHandler?

    static func reset() {
        handler = nil
        asyncHandler = nil
    }

    override class func canInit(with request: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        if let asyncHandler = Self.asyncHandler {
            Task {
                do {
                    let (response, data) = try await asyncHandler(request)
                    client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
                    client?.urlProtocol(self, didLoad: data)
                    client?.urlProtocolDidFinishLoading(self)
                } catch {
                    client?.urlProtocol(self, didFailWithError: error)
                }
            }
            return
        }

        guard let handler = Self.handler else {
            client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse))
            return
        }
        do {
            let (response, data) = try handler(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}
}

private extension HTTPURLResponse {
    static func vmStub(url: URL, statusCode: Int, body: String) -> (HTTPURLResponse, Data) {
        let response = HTTPURLResponse(url: url, statusCode: statusCode, httpVersion: nil, headerFields: ["Content-Type": "application/json"])!
        return (response, Data(body.utf8))
    }
}

private extension URLRequest {
    var vmBodyData: Data? {
        if let httpBody {
            return httpBody
        }
        guard let httpBodyStream else {
            return nil
        }
        httpBodyStream.open()
        defer { httpBodyStream.close() }

        var data = Data()
        let bufferSize = 1024
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }

        while httpBodyStream.hasBytesAvailable {
            let count = httpBodyStream.read(buffer, maxLength: bufferSize)
            if count <= 0 { break }
            data.append(buffer, count: count)
        }
        return data
    }
}
