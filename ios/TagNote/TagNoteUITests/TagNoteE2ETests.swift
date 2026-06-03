import XCTest

final class TagNoteE2ETests: XCTestCase {
    private var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false

        app = XCUIApplication()
        app.launchEnvironment["TAGNOTE_E2E_SERVER_URL"] = ProcessInfo.processInfo.environment["TAGNOTE_E2E_SERVER_URL"] ?? "http://localhost:3777"
        app.launchEnvironment["TAGNOTE_E2E_EMAIL"] = ProcessInfo.processInfo.environment["TAGNOTE_E2E_EMAIL"] ?? "test@test.com"
        app.launchEnvironment["TAGNOTE_E2E_PASSWORD"] = ProcessInfo.processInfo.environment["TAGNOTE_E2E_PASSWORD"] ?? "testpass123"
        app.launchArguments.append("-ui-testing")
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // Compact width (iPhone, or any device forced compact): the seeded note is
    // reached through the hamburger-triggered slide-over drawer, and search lives
    // inside that drawer.
    @MainActor
    func testCompactDrawerShowsSeededNoteAndSearchesContent() async throws {
        // Force the compact layout so this exercises the drawer regardless of the
        // device the suite happens to run on (iPhone or iPad).
        app.launchEnvironment["TAGNOTE_UI_FORCE_COMPACT"] = "1"

        let seeded = try await seedNote()
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))
        XCTAssertTrue(containsText(seeded.title))

        // The drawer (and its search field) is hidden until the hamburger opens it.
        let menuButton = app.descendants(matching: .any)["sidebar-open-button"]
        XCTAssertTrue(menuButton.waitForExistence(timeout: 5), "Compact layout must show the hamburger menu button")
        menuButton.tap()

        let searchField = app.textFields["note-search-field"]
        XCTAssertTrue(searchField.waitForExistence(timeout: 5))
        searchField.tap()
        searchField.typeText(seeded.bodyNeedle)
        XCTAssertTrue(containsText(seeded.title))
    }

    // Regular width (iPad full screen / wide Stage Manager window): the sidebar is
    // persistent — search and navigation are visible at launch with no hamburger.
    // Skips on compact-width devices so the same suite stays green on iPhone.
    @MainActor
    func testRegularWidthShowsPersistentSidebarAndSearches() async throws {
        let seeded = try await seedNote()
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))

        // If the hamburger appears, we are at compact width — the persistent
        // sidebar does not apply here. Use a generous wait so a slow login (the
        // shell, and thus the hamburger, renders only after auth) is not
        // misread as a regular-width layout.
        if app.descendants(matching: .any)["sidebar-open-button"].waitForExistence(timeout: 6) {
            throw XCTSkip("Persistent sidebar only renders at regular width (iPad); skipping on compact width.")
        }

        // Persistent sidebar: navigation and search are already on screen.
        XCTAssertTrue(app.descendants(matching: .any)["sidebar-notes-button"].exists, "Regular layout must show the persistent sidebar navigation")
        XCTAssertTrue(containsText(seeded.title))

        let searchField = app.textFields["note-search-field"]
        XCTAssertTrue(searchField.waitForExistence(timeout: 5), "Search field must be visible without opening a drawer at regular width")
        searchField.tap()
        searchField.typeText(seeded.bodyNeedle)
        XCTAssertTrue(containsText(seeded.title))
    }

    // Captures a set of full-screen screenshots of the key surfaces, attached to
    // the test result (lifetime .keepAlways) so they can be exported from the
    // .xcresult for the App Store Connect listing. Runs on whatever device the
    // suite targets, so running it on an iPhone and an iPad simulator produces
    // both required screenshot sets. It does not assert layout (the other two
    // tests do that); it just drives the UI and snapshots it.
    //
    // Named to sort last so it runs after the lightweight functional tests: it is
    // the heaviest case (seeds several notes, opens the editor) and running it
    // first left enough residual app/account state to destabilize the others.
    @MainActor
    func testScreenshotsForAppStoreListing() async throws {
        // Seed a few notes (single login) so the list and search look populated.
        try await seedNotes(count: 3)

        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))
        capture("01-Notes")

        // Sidebar: tag-first search & filtering. On compact width it lives behind
        // the hamburger; on regular width it is always on screen.
        let wasCompact = openSidebarIfCompact()
        XCTAssertTrue(app.textFields["note-search-field"].waitForExistence(timeout: 5))
        capture("02-SearchAndTags")

        // Tags management surface.
        let tagsButton = app.descendants(matching: .any)["sidebar-tags-button"]
        if tagsButton.waitForExistence(timeout: 5) {
            tagsButton.tap()
            capture("03-Tags")
        }

        // Authoring surface: open a new note in the editor.
        _ = openSidebarIfCompact()
        let newNote = app.descendants(matching: .any)["sidebar-new-note"]
        if newNote.waitForExistence(timeout: 5) {
            newNote.tap()
            if app.descendants(matching: .any)["editor-screen"].waitForExistence(timeout: 5) {
                let editor = app.textViews["note-content-editor"]
                if editor.waitForExistence(timeout: 3) {
                    editor.tap()
                    editor.typeText("# Project kickoff\n\nDraft the launch checklist and assign owners.")
                }
                capture("04-Editor")
                // Dismiss the editor so the test does not leave a modal open.
                let close = app.buttons["Close"]
                if close.exists { close.tap() }
            }
        }

        // Keep the compiler aware the value is intentionally observed.
        _ = wasCompact
    }

    /// Opens the slide-over drawer when the layout is compact. Returns true if it
    /// opened a drawer (compact width), false if the sidebar is already persistent
    /// (regular width / iPad).
    @discardableResult
    private func openSidebarIfCompact() -> Bool {
        let menuButton = app.descendants(matching: .any)["sidebar-open-button"]
        guard menuButton.waitForExistence(timeout: 3) else { return false }
        menuButton.tap()
        _ = app.textFields["note-search-field"].waitForExistence(timeout: 5)
        return true
    }

    private func capture(_ name: String) {
        let screenshot = XCUIScreen.main.screenshot()
        let attachment = XCTAttachment(screenshot: screenshot)
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    private func containsText(_ needle: String) -> Bool {
        app.staticTexts
            .containing(NSPredicate(format: "label CONTAINS %@", needle))
            .firstMatch
            .waitForExistence(timeout: 10)
    }

    private func configureServerIfNeeded() {
        let serverField = app.textFields["server-url-field"]
        guard serverField.waitForExistence(timeout: 2) else { return }

        // Under -ui-testing the field starts empty, so type the test server URL
        // directly. Select-all + delete first as a defensive clear in case any
        // text is present, so the app never falls back to the production server.
        serverField.tap()
        serverField.typeKey("a", modifierFlags: .command)
        serverField.typeText(XCUIKeyboardKey.delete.rawValue)
        serverField.typeText(app.launchEnvironment["TAGNOTE_E2E_SERVER_URL"] ?? "http://localhost:3777")
        app.descendants(matching: .any)["server-continue-button"].tap()
    }

    private func loginIfNeeded() {
        let emailField = app.textFields["login-email-field"]
        guard emailField.waitForExistence(timeout: 8) else { return }

        emailField.tap()
        emailField.typeText(app.launchEnvironment["TAGNOTE_E2E_EMAIL"] ?? "test@test.com")

        let passwordField = app.secureTextFields["login-password-field"]
        XCTAssertTrue(passwordField.waitForExistence(timeout: 3))
        passwordField.tap()
        passwordField.typeText(app.launchEnvironment["TAGNOTE_E2E_PASSWORD"] ?? "testpass123")

        app.descendants(matching: .any)["login-submit-button"].tap()
    }

    @discardableResult
    private func seedNote() async throws -> SeededNote {
        try await seedNotes(count: 1)
    }

    /// Creates `count` notes using a single login, and returns the last seeded
    /// note. Reusing one token avoids repeated `/auth/login` calls, which trip
    /// the server's auth rate limiter (HTTP 429) when seeding several notes.
    @discardableResult
    private func seedNotes(count: Int) async throws -> SeededNote {
        let token = try await authToken()
        var last: SeededNote?
        for index in 0..<max(count, 1) {
            last = try await createNote(token: token, index: index)
        }
        return last!
    }

    private func authToken() async throws -> String {
        let email = ProcessInfo.processInfo.environment["TAGNOTE_E2E_EMAIL"] ?? "test@test.com"
        let password = ProcessInfo.processInfo.environment["TAGNOTE_E2E_PASSWORD"] ?? "testpass123"

        let loginURL = serverBaseURL.appending(path: "api/v1/auth/login")
        var loginRequest = URLRequest(url: loginURL)
        loginRequest.httpMethod = "POST"
        loginRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        loginRequest.httpBody = try JSONEncoder().encode(["email": email, "password": password])
        let (loginData, loginResponse) = try await URLSession.shared.data(for: loginRequest)
        try assertSuccess(loginResponse)
        return try JSONDecoder().decode(LoginPayload.self, from: loginData).token
    }

    private func createNote(token: String, index: Int) async throws -> SeededNote {
        let title = "iOS seeded note \(Int(Date().timeIntervalSince1970))-\(index)"
        let bodyNeedle = "drawer-search-\(UUID().uuidString.prefix(8))"

        let noteURL = serverBaseURL.appending(path: "api/v1/notes")
        var noteRequest = URLRequest(url: noteURL)
        noteRequest.httpMethod = "POST"
        noteRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        noteRequest.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        noteRequest.httpBody = try JSONEncoder().encode(NotePayload(
            content: "### \(title)\nCreated for native iOS E2E \(bodyNeedle)",
            tags: ["ios-e2e"]
        ))
        let (_, noteResponse) = try await URLSession.shared.data(for: noteRequest)
        try assertSuccess(noteResponse)

        return SeededNote(title: title, bodyNeedle: bodyNeedle)
    }

    private var serverBaseURL: URL {
        URL(string: ProcessInfo.processInfo.environment["TAGNOTE_E2E_SERVER_URL"] ?? "http://localhost:3777")!
    }

    private func assertSuccess(_ response: URLResponse) throws {
        let statusCode = (response as? HTTPURLResponse)?.statusCode ?? 0
        XCTAssertTrue((200..<300).contains(statusCode), "Unexpected HTTP status \(statusCode)")
    }
}

private struct LoginPayload: Decodable {
    let token: String
}

private struct SeededNote {
    let title: String
    let bodyNeedle: String
}

private struct NotePayload: Encodable {
    let content: String
    let tags: [String]
}
