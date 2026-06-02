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
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 10))
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
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 10))

        // If the hamburger appears, we are at compact width — the persistent
        // sidebar does not apply here.
        if app.descendants(matching: .any)["sidebar-open-button"].waitForExistence(timeout: 3) {
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

    private func containsText(_ needle: String) -> Bool {
        app.staticTexts
            .containing(NSPredicate(format: "label CONTAINS %@", needle))
            .firstMatch
            .waitForExistence(timeout: 10)
    }

    private func configureServerIfNeeded() {
        let serverField = app.textFields["server-url-field"]
        guard serverField.waitForExistence(timeout: 2) else { return }

        serverField.tap()
        // The field is pre-filled with the default hosted server; clear it
        // before typing the test server URL.
        if let current = serverField.value as? String, !current.isEmpty {
            serverField.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count))
        }
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

    private func seedNote() async throws -> SeededNote {
        let baseURL = URL(string: ProcessInfo.processInfo.environment["TAGNOTE_E2E_SERVER_URL"] ?? "http://localhost:3777")!
        let email = ProcessInfo.processInfo.environment["TAGNOTE_E2E_EMAIL"] ?? "test@test.com"
        let password = ProcessInfo.processInfo.environment["TAGNOTE_E2E_PASSWORD"] ?? "testpass123"
        let title = "iOS seeded note \(Int(Date().timeIntervalSince1970))"
        let bodyNeedle = "drawer-search-\(UUID().uuidString.prefix(8))"

        let loginURL = baseURL.appending(path: "api/v1/auth/login")
        var loginRequest = URLRequest(url: loginURL)
        loginRequest.httpMethod = "POST"
        loginRequest.setValue("application/json", forHTTPHeaderField: "Content-Type")
        loginRequest.httpBody = try JSONEncoder().encode(["email": email, "password": password])
        let (loginData, loginResponse) = try await URLSession.shared.data(for: loginRequest)
        try assertSuccess(loginResponse)
        let token = try JSONDecoder().decode(LoginPayload.self, from: loginData).token

        let noteURL = baseURL.appending(path: "api/v1/notes")
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
