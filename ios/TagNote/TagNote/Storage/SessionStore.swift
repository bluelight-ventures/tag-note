import Foundation
import SwiftUI

@MainActor
final class SessionStore: ObservableObject {
    @Published private(set) var serverURL: URL?
    @Published private(set) var token: String?
    @Published private(set) var user: User?
    @Published var errorMessage: String?
    @Published var isLoading = false

    /// Default hosted instance. Release builds always use this; Debug builds
    /// pre-fill it on the server-setup screen but allow overriding it.
    static let defaultServerURL = "https://tag-note.com"

    /// Whether the user may point the app at a custom server.
    /// Only Debug builds (local development and the E2E suite) expose this;
    /// the shipped App Store build is a tag-note.com-only client.
    static var allowsCustomServer: Bool {
        #if DEBUG
        return true
        #else
        return false
        #endif
    }

    let api: TagNoteAPI
    let cache: LocalCache

    var isConfigured: Bool { serverURL != nil }
    var isAuthenticated: Bool { token != nil && user != nil }

    init(api: TagNoteAPI = TagNoteAPI(), cache: LocalCache = LocalCache()) {
        self.api = api
        self.cache = cache
        Self.migrateKeychainToSharedAccessGroup()
        if ProcessInfo.processInfo.arguments.contains("-ui-testing") {
            KeychainStore.delete(Keys.serverURL)
            KeychainStore.delete(Keys.token)
        }
        if Self.allowsCustomServer {
            if let rawURL = KeychainStore.read(Keys.serverURL) {
                self.serverURL = URL(string: rawURL)
            }
        } else {
            // Release: always the hosted server. Never show the setup screen.
            self.serverURL = URL(string: Self.defaultServerURL)
        }
        self.token = KeychainStore.read(Keys.token)
        api.configure(serverURL: serverURL, token: token)
    }

    func saveServerURL(_ rawValue: String) throws {
        let normalized = rawValue.trimmingCharacters(in: .whitespacesAndNewlines)
        guard let url = URL(string: normalized), url.scheme != nil, url.host != nil else {
            throw TagNoteAPIError.invalidServerURL
        }
        serverURL = url
        KeychainStore.write(url.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/")), for: Keys.serverURL)
        api.configure(serverURL: serverURL, token: token)
    }

    func bootstrap() async {
        guard token != nil else { return }
        isLoading = true
        defer { isLoading = false }
        do {
            user = try await api.me()
        } catch {
            clearAuth()
        }
    }

    func login(email: String, password: String) async {
        await authenticate {
            try await api.login(email: email, password: password)
        }
    }

    func register(email: String, password: String, displayName: String) async {
        await authenticate {
            try await api.register(email: email, password: password, displayName: displayName)
        }
    }

    func loginWithGoogle(idToken: String) async {
        await authenticate {
            try await api.googleLogin(idToken: idToken)
        }
    }

    func loginWithApple(identityToken: String, nonce: String?, fullName: String?) async {
        await authenticate {
            try await api.appleLogin(identityToken: identityToken, nonce: nonce, fullName: fullName)
        }
    }

    func requestMagicLink(email: String) async {
        await runMessageTask(success: "Check your email for a login link.") {
            try await api.requestMagicLink(email: email)
        }
    }

    func forgotPassword(email: String) async {
        await runMessageTask(success: "Check your email for a password reset link.") {
            try await api.forgotPassword(email: email)
        }
    }

    func logout() async {
        await api.logout()
        clearAuth()
        await cache.clear()
    }

    func deleteAccount() async {
        errorMessage = nil
        isLoading = true
        defer { isLoading = false }
        do {
            try await api.deleteAccount()
            clearAuth()
            await cache.clear()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func resetServer() {
        clearAuth()
        KeychainStore.delete(Keys.serverURL)
        serverURL = nil
        api.configure(serverURL: nil, token: nil)
    }

    private func authenticate(_ action: () async throws -> AuthResponse) async {
        errorMessage = nil
        isLoading = true
        defer { isLoading = false }
        do {
            let response = try await action()
            guard let token = response.token else {
                errorMessage = response.pendingVerifyEmail.map { "Verify \($0) before logging in." } ?? "Verification required."
                return
            }
            guard let user = response.user else {
                errorMessage = "The server did not include account details."
                return
            }
            self.token = token
            self.user = user
            KeychainStore.write(token, for: Keys.token)
            api.configure(serverURL: serverURL, token: token)
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func runMessageTask(success: String, _ action: () async throws -> Void) async {
        errorMessage = nil
        isLoading = true
        defer { isLoading = false }
        do {
            try await action()
            errorMessage = success
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    /// One-time migration so the auth token and server URL move into the shared
    /// keychain access group (the first `keychain-access-groups` entry), making
    /// them readable by the Share Extension. Items written by builds that predate
    /// the entitlement live in the app's default access group; re-writing them
    /// here (delete + add) lands them in the shared group. Idempotent via a flag.
    private static func migrateKeychainToSharedAccessGroup() {
        let flag = "didMigrateKeychainToSharedGroup"
        guard !UserDefaults.standard.bool(forKey: flag) else { return }
        if let token = KeychainStore.read(Keys.token) {
            KeychainStore.write(token, for: Keys.token)
        }
        if let url = KeychainStore.read(Keys.serverURL) {
            KeychainStore.write(url, for: Keys.serverURL)
        }
        UserDefaults.standard.set(true, forKey: flag)
    }

    private func clearAuth() {
        token = nil
        user = nil
        KeychainStore.delete(Keys.token)
        api.configure(serverURL: serverURL, token: nil)
    }

    private enum Keys {
        static let serverURL = "serverURL"
        static let token = "token"
    }
}
