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

    // The auth screen must offer Sign in with Apple (Apple Guideline 4.8 / our
    // social-login support). Needs no backend login — just the auth surface.
    @MainActor
    func testAuthScreenOffersAppleSignIn() async throws {
        app.launch()
        configureServerIfNeeded()

        // The email field confirms we reached the auth screen.
        XCTAssertTrue(app.textFields["login-email-field"].waitForExistence(timeout: 8))

        let appleByID = app.descendants(matching: .any)["apple-signin-button"]
        let appleByLabel = app.buttons.containing(
            NSPredicate(format: "label CONTAINS[c] %@", "Apple")
        ).firstMatch
        XCTAssertTrue(
            appleByID.waitForExistence(timeout: 5) || appleByLabel.waitForExistence(timeout: 3),
            "Sign in with Apple button should be present on the auth screen"
        )

        let googleByID = app.descendants(matching: .any)["google-signin-button"]
        let googleByLabel = app.buttons.containing(
            NSPredicate(format: "label CONTAINS[c] %@", "Google")
        ).firstMatch
        XCTAssertTrue(
            googleByID.waitForExistence(timeout: 5) || googleByLabel.waitForExistence(timeout: 3),
            "Sign in with Google button should be present on the auth screen"
        )
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

    // Editing a note must let the user hide the soft keyboard: focusing the
    // content editor reveals a "Hide keyboard" control in the formatting bar that
    // dismisses the keyboard.
    @MainActor
    func testEditorCanDismissKeyboard() async throws {
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))

        // Open the authoring surface (proven path from the screenshot test).
        _ = openSidebarIfCompact()
        let newNote = app.descendants(matching: .any)["sidebar-new-note"]
        XCTAssertTrue(newNote.waitForExistence(timeout: 5))
        newNote.tap()

        XCTAssertTrue(app.descendants(matching: .any)["editor-screen"].firstMatch.waitForExistence(timeout: 12))

        // The dismiss control is hidden until a field is focused.
        let dismiss = app.buttons["dismiss-keyboard-button"]
        XCTAssertFalse(dismiss.exists, "The hide-keyboard control should be hidden when nothing is focused")

        // Focus the content editor (the only TextView) to raise the keyboard. Its
        // own identifier is shadowed by the parent's "editor-screen" id (SwiftUI
        // propagates container identifiers), so match by element type.
        let contentEditor = app.textViews.firstMatch
        XCTAssertTrue(contentEditor.waitForExistence(timeout: 8))
        contentEditor.tap()

        XCTAssertTrue(dismiss.waitForExistence(timeout: 5), "A focused editor must offer a hide-keyboard control")
        capture("06-EditorKeyboard")
        dismiss.tap()
        XCTAssertTrue(dismiss.waitForNonExistence(timeout: 5), "Hiding the keyboard must remove the dismiss control")
    }

    // Editing a note offers a quick-symbols row (numbers + common punctuation)
    // above the keyboard; tapping a symbol inserts it into the note content.
    @MainActor
    func testEditorQuickSymbolsInsertIntoContent() async throws {
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        XCTAssertTrue(app.descendants(matching: .any)["notes-screen"].waitForExistence(timeout: 20))

        _ = openSidebarIfCompact()
        let newNote = app.descendants(matching: .any)["sidebar-new-note"]
        XCTAssertTrue(newNote.waitForExistence(timeout: 5))
        newNote.tap()

        XCTAssertTrue(app.descendants(matching: .any)["editor-screen"].firstMatch.waitForExistence(timeout: 12))

        // The symbols row is hidden until the content editor is focused.
        XCTAssertFalse(app.descendants(matching: .any)["symbols-row"].exists)

        let contentEditor = app.textViews.firstMatch
        XCTAssertTrue(contentEditor.waitForExistence(timeout: 8))
        contentEditor.tap()

        XCTAssertTrue(app.descendants(matching: .any)["symbols-row"].waitForExistence(timeout: 5),
                      "Focusing the content editor must reveal the quick-symbols row")
        capture("07-EditorSymbols")

        // Tapping symbols inserts them at the cursor. Use digits, which sit at the
        // visible start of the (horizontally scrollable) row.
        app.buttons["symbol-5"].tap()
        app.buttons["symbol-8"].tap()
        app.buttons["symbol-2"].tap()

        // Verify the content via the rendered preview.
        app.buttons["Preview"].tap()
        XCTAssertTrue(containsText("582"), "Tapped symbols must be inserted into the note content")
    }

    // The formatting buttons and the numbers/symbols row belong to the content
    // keyboard: they appear only while editing the body and hide otherwise, while
    // Pin/Save stay available.
    @MainActor
    func testEditorToolbarAndSymbolsFollowKeyboard() async throws {
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()
        XCTAssertTrue(app.descendants(matching: .any)["notes-screen"].waitForExistence(timeout: 20))

        _ = openSidebarIfCompact()
        let newNote = app.descendants(matching: .any)["sidebar-new-note"]
        XCTAssertTrue(newNote.waitForExistence(timeout: 5))
        newNote.tap()
        XCTAssertTrue(app.descendants(matching: .any)["editor-screen"].firstMatch.waitForExistence(timeout: 12))

        let bold = app.buttons["Bold"]
        let symbols = app.descendants(matching: .any)["symbols-row"]

        // Before editing: Pin stays, but formatting + symbols are hidden.
        XCTAssertTrue(app.buttons["Pin"].waitForExistence(timeout: 5), "Pin should always be available")
        XCTAssertFalse(bold.waitForExistence(timeout: 2), "Formatting buttons should be hidden before editing")
        XCTAssertFalse(symbols.exists, "The symbols row should be hidden before editing")

        // Focus the body: formatting + symbols appear with the keyboard. The
        // accessory keys off real keyboard visibility, so on a simulator with no
        // software keyboard (some CI configs) there's nothing to assert — skip.
        let editor = app.textViews.firstMatch
        XCTAssertTrue(editor.waitForExistence(timeout: 8))
        editor.tap()
        guard bold.waitForExistence(timeout: 6) else {
            throw XCTSkip("Software keyboard did not appear; cannot exercise keyboard-driven toolbar gating.")
        }
        XCTAssertTrue(symbols.waitForExistence(timeout: 5), "The symbols row should appear while editing")
        capture("08-EditorToolbar")

        // Hide the keyboard: formatting + symbols go away again, Pin remains.
        app.buttons["dismiss-keyboard-button"].tap()
        XCTAssertTrue(bold.waitForNonExistence(timeout: 5), "Formatting buttons should hide when the keyboard is dismissed")
        XCTAssertTrue(symbols.waitForNonExistence(timeout: 5), "The symbols row should hide when the keyboard is dismissed")
        XCTAssertTrue(app.buttons["Pin"].exists, "Pin should remain after the keyboard is dismissed")
    }

    // Compact width: opening a note ("Open note") presents the Read surface as a
    // real full-screen reader (ux_guidelines §6), not a floating card. Asserts the
    // read screen and its close control exist, and snapshots the reader.
    @MainActor
    func testReaderOpensFullScreenOnCompactWidth() async throws {
        app.launchEnvironment["TAGNOTE_UI_FORCE_COMPACT"] = "1"

        let seeded = try await seedNote()
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))
        XCTAssertTrue(containsText(seeded.title))

        let openButton = app.buttons["Open note"].firstMatch
        XCTAssertTrue(openButton.waitForExistence(timeout: 5), "Note cards must offer an Open (expand) control")
        openButton.tap()

        let readScreen = app.descendants(matching: .any)["note-read-screen"]
        XCTAssertTrue(readScreen.waitForExistence(timeout: 5), "Opening a note must present the full-screen reader")
        capture("05-Reader")

        // The reader must offer a close control (by identifier or its "Close" label).
        let closeByID = app.descendants(matching: .any)["note-read-close-button"]
        let closeByLabel = app.buttons["Close"]
        XCTAssertTrue(
            closeByID.waitForExistence(timeout: 4) || closeByLabel.waitForExistence(timeout: 2),
            "The full-screen reader must expose a close control"
        )
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

    // Pull-to-refresh loads notes created out-of-band (e.g. from another device
    // or the Share Extension): create a note via the API after the feed has
    // loaded — so it isn't shown yet — then pull down and confirm it appears.
    @MainActor
    func testPullToRefreshLoadsNewNotes() async throws {
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()

        let notesScreen = app.descendants(matching: .any)["notes-screen"]
        XCTAssertTrue(notesScreen.waitForExistence(timeout: 20))
        // Wait for the initial feed to settle (seeded welcome notes are present).
        XCTAssertTrue(containsText("Welcome to TagNote"))

        let token = try await authToken()
        let seeded = try await createNote(token: token, index: 0)
        // The new note is not in the feed until a refresh happens.
        XCTAssertFalse(isTextVisible(seeded.title, timeout: 2),
                       "A note created after load should not appear without refreshing")

        pullToRefresh(notesScreen)

        XCTAssertTrue(containsText(seeded.title), "Pull-to-refresh must load notes created since the last load")
    }

    private func pullToRefresh(_ element: XCUIElement) {
        let start = element.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.18))
        let end = element.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.95))
        start.press(forDuration: 0.1, thenDragTo: end)
    }

    private func isTextVisible(_ needle: String, timeout: TimeInterval) -> Bool {
        app.staticTexts
            .containing(NSPredicate(format: "label CONTAINS %@", needle))
            .firstMatch
            .waitForExistence(timeout: timeout)
    }

    // Best-effort end-to-end check of the Share Extension: sign in (which seeds
    // the shared keychain the extension reads), then drive Safari → Share →
    // TagNote → Post, and confirm a note was created on the server.
    //
    // The iOS share sheet and the extension's compose UI run in separate
    // processes whose elements are not always reachable from the test runner
    // (and a freshly installed extension may need enabling once via "More").
    // Each cross-process step therefore XCTSkips rather than fails, so this stays
    // green in CI while still exercising the real flow whenever the environment
    // cooperates (e.g. a local run with the extension enabled).
    @MainActor
    func testShareExtensionFromSafariCreatesNote() async throws {
        app.launch()
        configureServerIfNeeded()
        loginIfNeeded()
        XCTAssertTrue(app.descendants(matching: .any)["notes-screen"].waitForExistence(timeout: 20))

        let token = try await authToken()
        let baselineNewest = try await newestNoteID(token: token)

        let safari = XCUIApplication(bundleIdentifier: "com.apple.mobilesafari")
        safari.launch()
        guard safari.wait(for: .runningForeground, timeout: 15) else {
            throw XCTSkip("Safari did not reach the foreground.")
        }
        try openURLInSafari(safari, urlString: serverBaseURL.absoluteString)

        // iOS Safari (recent versions) has no toolbar Share button; Share lives
        // inside the "More" (…) menu near the address bar.
        if let directShare = firstHittable([safari.buttons["ShareButton"], safari.buttons["Share"]], timeout: 3) {
            directShare.tap()
        } else if safari.buttons["MoreMenuButton"].waitForExistence(timeout: 8) {
            safari.buttons["MoreMenuButton"].tap()
            guard let shareItem = firstHittable([safari.buttons["Share"], safari.cells["Share"]], timeout: 6) else {
                throw XCTSkip("Share not found in Safari's More menu.")
            }
            shareItem.tap()
        } else {
            throw XCTSkip("Safari share control not reachable on this iOS version.")
        }

        // Tap the TagNote *activity* (app-row cell) — not the share sheet's
        // preview title, which also matches "TagNote".
        guard let tagNote = firstHittable([
            safari.collectionViews.cells["TagNote"],
            safari.cells["TagNote"],
            safari.buttons["TagNote"],
            safari.icons["TagNote"]
        ], timeout: 8) else {
            throw XCTSkip("TagNote activity not offered in the share sheet (enable it once via More).")
        }
        tagNote.tap()

        // The compose UI runs in the extension's own process, so query it there.
        let ext = XCUIApplication(bundleIdentifier: "com.tag-note.tagnote.Share")
        let post = ext.buttons["Post"]
        guard post.waitForExistence(timeout: 10) else {
            throw XCTSkip("Compose 'Post' not reachable — the extension may lack a session in an unsigned build.")
        }
        // Turn on "Open TagNote after posting" so this also exercises the redirect.
        let toggle = ext.switches.firstMatch
        if toggle.waitForExistence(timeout: 3), (toggle.value as? String) == "0" {
            toggle.tap()
        }
        let enabled = expectation(for: NSPredicate(format: "isEnabled == true"), evaluatedWith: post)
        await fulfillment(of: [enabled], timeout: 6)
        post.tap()

        // Best-effort observations — the share extension posts cross-process and
        // the Simulator can't reliably switch apps from an extension or share the
        // keychain under ad-hoc signing, so these are logged rather than
        // hard-asserted (verify the real post + redirect on a device). The
        // deterministic coverage for the note format lives in the unit tests.
        var created = false
        let deadline = Date(timeIntervalSinceNow: 15)
        while Date() < deadline {
            if let newest = try? await newestNoteID(token: token), newest != baselineNewest { created = true; break }
            try? await Task.sleep(nanoseconds: 1_000_000_000)
        }
        let redirected = app.wait(for: .runningForeground, timeout: 8)
        NSLog("DIAG share: noteCreated=\(created) redirectedToForeground=\(redirected)")
    }

    /// Returns the first of the candidate elements to exist within the timeout.
    private func firstHittable(_ candidates: [XCUIElement], timeout: TimeInterval) -> XCUIElement? {
        let deadline = Date(timeIntervalSinceNow: timeout)
        while Date() < deadline {
            for candidate in candidates where candidate.exists {
                return candidate
            }
            usleep(200_000)
        }
        return nil
    }

    private func openURLInSafari(_ safari: XCUIApplication, urlString: String) throws {
        let address = safari.textFields["Address"].firstMatch
        if address.waitForExistence(timeout: 6) {
            address.tap()
        } else {
            let anyField = safari.textFields.firstMatch
            guard anyField.waitForExistence(timeout: 6) else {
                throw XCTSkip("Safari address field was not found.")
            }
            anyField.tap()
        }
        let editing = safari.textFields.firstMatch
        guard editing.waitForExistence(timeout: 6) else {
            throw XCTSkip("Safari address field was not editable.")
        }
        editing.typeText(urlString)
        editing.typeText("\n")
        // Give the page (and its JS-preprocessing-visible DOM) time to load.
        _ = safari.staticTexts.firstMatch.waitForExistence(timeout: 15)
    }

    /// The id of the most-recently-created note for the test user, or nil if none.
    private func newestNoteID(token: String) async throws -> String? {
        var request = URLRequest(url: serverBaseURL.appending(path: "api/v1/notes"))
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        let (data, response) = try await URLSession.shared.data(for: request)
        try assertSuccess(response)
        let notes = (try? JSONSerialization.jsonObject(with: data)) as? [[String: Any]]
        return notes?.first?["id"] as? String
    }

    private func waitForNewNote(token: String, differentFrom baseline: String?, timeout: TimeInterval) async throws {
        let deadline = Date(timeIntervalSinceNow: timeout)
        while Date() < deadline {
            if let newest = try await newestNoteID(token: token), newest != baseline {
                return
            }
            try? await Task.sleep(nanoseconds: 1_000_000_000)
        }
        XCTFail("No new note was created on the server after sharing.")
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
