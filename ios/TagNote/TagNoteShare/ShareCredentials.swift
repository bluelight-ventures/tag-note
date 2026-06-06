import Foundation

/// The session the Share Extension reuses to talk to the backend, read from the
/// shared keychain access group the main app writes to on login.
struct ShareCredentials {
    let serverURL: URL
    let token: String

    /// Default hosted server, used when the app stored no custom server URL
    /// (Release builds only ever use this; Debug builds may store a custom one).
    static let defaultServerURL = "https://tag-note.com"

    static func load() -> ShareCredentials? {
        guard let token = KeychainStore.read("token"), !token.isEmpty else { return nil }
        let raw = KeychainStore.read("serverURL") ?? defaultServerURL
        guard let url = URL(string: raw) else { return nil }
        return ShareCredentials(serverURL: url, token: token)
    }
}
