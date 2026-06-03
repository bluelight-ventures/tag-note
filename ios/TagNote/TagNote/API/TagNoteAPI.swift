import Foundation

enum TagNoteAPIError: LocalizedError {
    case invalidServerURL
    case invalidResponse
    case http(Int, String)
    case missingToken

    var errorDescription: String? {
        switch self {
        case .invalidServerURL:
            return "Enter a valid server URL."
        case .invalidResponse:
            return "The server returned an invalid response."
        case .http(_, let message):
            return message
        case .missingToken:
            return "Login required."
        }
    }
}

final class TagNoteAPI {
    private let session: URLSession
    private let encoder: JSONEncoder
    private let decoder: JSONDecoder

    var baseURL: URL?
    var token: String?

    init(session: URLSession = .shared) {
        self.session = session
        self.encoder = JSONEncoder()
        self.decoder = JSONDecoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let value = try container.decode(String.self)
            if let date = ISO8601DateFormatter.tagnoteFractional.date(from: value) ?? ISO8601DateFormatter.tagnote.date(from: value) {
                return date
            }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "Invalid ISO8601 date: \(value)")
        }
        encoder.dateEncodingStrategy = .iso8601
    }

    func configure(serverURL: URL?, token: String?) {
        self.baseURL = serverURL
        self.token = token
    }

    func login(email: String, password: String) async throws -> AuthResponse {
        try await request("POST", "/auth/login", body: ["email": email, "password": password], authorized: false)
    }

    func register(email: String, password: String, displayName: String) async throws -> AuthResponse {
        try await request("POST", "/auth/register", body: ["email": email, "password": password, "display_name": displayName], authorized: false)
    }

    func appleLogin(identityToken: String, nonce: String?, fullName: String?) async throws -> AuthResponse {
        var body: [String: String] = ["identity_token": identityToken]
        if let nonce, !nonce.isEmpty { body["nonce"] = nonce }
        if let fullName, !fullName.isEmpty { body["full_name"] = fullName }
        return try await request("POST", "/auth/apple", body: body, authorized: false)
    }

    func requestMagicLink(email: String) async throws {
        let _: EmptyResponse = try await request("POST", "/auth/magic-link", body: ["email": email], authorized: false)
    }

    func forgotPassword(email: String) async throws {
        let _: EmptyResponse = try await request("POST", "/auth/forgot-password", body: ["email": email], authorized: false)
    }

    func me() async throws -> User {
        try await request("GET", "/auth/me")
    }

    func logout() async {
        try? await requestNoContent("POST", "/auth/logout")
    }

    func deleteAccount() async throws {
        try await requestNoContent("DELETE", "/auth/account")
    }

    func listNotes(tags: [String], query: String, sort: String, limit: Int, offset: Int) async throws -> [SubNote] {
        var items: [URLQueryItem] = tags.map { URLQueryItem(name: "tag", value: $0) }
        if !query.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            items.append(URLQueryItem(name: "q", value: query))
        }
        if !sort.isEmpty {
            items.append(URLQueryItem(name: "sort", value: sort))
        }
        items.append(URLQueryItem(name: "limit", value: String(limit)))
        items.append(URLQueryItem(name: "offset", value: String(offset)))
        return try await request("GET", "/notes", queryItems: items)
    }

    func getNote(id: String) async throws -> SubNote {
        try await request("GET", "/notes/\(id.urlPathEscaped)")
    }

    func createNote(content: String, tags: [String]) async throws -> CreateNoteResponse {
        try await request("POST", "/notes", body: CreateNoteRequest(content: content, tags: tags))
    }

    func updateNote(id: String, content: String?, tags: [String]?) async throws -> SubNote {
        try await request("PUT", "/notes/\(id.urlPathEscaped)", body: UpdateNoteRequest(content: content, tags: tags))
    }

    func togglePin(id: String) async throws {
        try await requestNoContent("PUT", "/notes/\(id.urlPathEscaped)/pin")
    }

    func deleteNote(id: String) async throws {
        try await requestNoContent("DELETE", "/notes/\(id.urlPathEscaped)")
    }

    func restoreNote(id: String) async throws {
        try await requestNoContent("PUT", "/notes/\(id.urlPathEscaped)/restore")
    }

    func purgeNote(id: String) async throws {
        try await requestNoContent("DELETE", "/notes/\(id.urlPathEscaped)/permanent")
    }

    func listTrash() async throws -> [SubNote] {
        try await request("GET", "/notes/trash")
    }

    func listTagsDetailed() async throws -> [TagInfo] {
        try await request("GET", "/tags/detailed")
    }

    func autocompleteTags(query: String, limit: Int = 8) async throws -> [String] {
        try await request("GET", "/tags/autocomplete", queryItems: [
            URLQueryItem(name: "q", value: query),
            URLQueryItem(name: "limit", value: String(limit))
        ])
    }

    func approveTag(_ name: String) async throws {
        try await requestNoContent("PUT", "/tags/\(name.urlPathEscaped)/approve")
    }

    func approveAllTags() async throws {
        try await requestNoContent("PUT", "/tags/approve-all")
    }

    func renameTag(_ name: String, to newName: String) async throws {
        try await requestNoContent("PUT", "/tags/\(name.urlPathEscaped)/rename", body: TagRenameRequest(newName: newName))
    }

    func updateTagPriority(_ name: String, importance: Int, urgency: Int) async throws {
        try await requestNoContent("PUT", "/tags/\(name.urlPathEscaped)/priority", body: TagPriorityRequest(importance: importance, urgency: urgency))
    }

    func deleteTag(_ name: String) async throws {
        try await requestNoContent("DELETE", "/tags/\(name.urlPathEscaped)")
    }

    func getSettings() async throws -> TagNoteSettings {
        try await request("GET", "/settings")
    }

    func saveSettings(_ settings: TagNoteSettings) async throws -> TagNoteSettings {
        try await request("PUT", "/settings", body: settings)
    }

    func uploadImage(data: Data, fileName: String, mimeType: String) async throws -> String {
        guard let url = endpoint("/images") else { throw TagNoteAPIError.invalidServerURL }
        guard let token else { throw TagNoteAPIError.missingToken }

        let boundary = "Boundary-\(UUID().uuidString)"
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
        request.httpBody = MultipartFormData(boundary: boundary)
            .appendFile(fieldName: "file", fileName: fileName, mimeType: mimeType, data: data)
            .finalized()

        let response: ImageUploadResponse = try await perform(request)
        return response.data.filePath
    }

    private func request<T: Decodable, Body: Encodable>(_ method: String, _ path: String, queryItems: [URLQueryItem] = [], body: Body? = Optional<Data>.none, authorized: Bool = true) async throws -> T {
        var request = try makeRequest(method, path, queryItems: queryItems, authorized: authorized)
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try encoder.encode(body)
        }
        return try await perform(request)
    }

    private func request<T: Decodable>(_ method: String, _ path: String, queryItems: [URLQueryItem] = [], authorized: Bool = true) async throws -> T {
        let request = try makeRequest(method, path, queryItems: queryItems, authorized: authorized)
        return try await perform(request)
    }

    private func requestNoContent<Body: Encodable>(_ method: String, _ path: String, body: Body? = Optional<Data>.none) async throws {
        var request = try makeRequest(method, path, authorized: true)
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try encoder.encode(body)
        }
        try await performNoContent(request)
    }

    private func requestNoContent(_ method: String, _ path: String) async throws {
        try await performNoContent(try makeRequest(method, path, authorized: true))
    }

    private func makeRequest(_ method: String, _ path: String, queryItems: [URLQueryItem] = [], authorized: Bool) throws -> URLRequest {
        guard var components = endpointComponents(path) else { throw TagNoteAPIError.invalidServerURL }
        components.queryItems = queryItems.isEmpty ? nil : queryItems
        guard let url = components.url else { throw TagNoteAPIError.invalidServerURL }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if authorized {
            guard let token else { throw TagNoteAPIError.missingToken }
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        return request
    }

    private func endpoint(_ path: String) -> URL? {
        endpointComponents(path)?.url
    }

    private func endpointComponents(_ path: String) -> URLComponents? {
        guard let baseURL else { return nil }
        guard var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false) else { return nil }
        let basePath = components.path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        let cleanPath = path.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        components.path = "/" + ([basePath, "api", "v1", cleanPath].filter { !$0.isEmpty }.joined(separator: "/"))
        return components
    }

    private func perform<T: Decodable>(_ request: URLRequest) async throws -> T {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw TagNoteAPIError.invalidResponse }
        guard (200..<300).contains(http.statusCode) else {
            throw TagNoteAPIError.http(http.statusCode, decodeErrorMessage(data) ?? "Request failed with status \(http.statusCode).")
        }
        if data.isEmpty, T.self == EmptyResponse.self {
            return EmptyResponse() as! T
        }
        return try decoder.decode(T.self, from: data)
    }

    private func performNoContent(_ request: URLRequest) async throws {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw TagNoteAPIError.invalidResponse }
        guard (200..<300).contains(http.statusCode) else {
            throw TagNoteAPIError.http(http.statusCode, decodeErrorMessage(data) ?? "Request failed with status \(http.statusCode).")
        }
    }

    private func decodeErrorMessage(_ data: Data) -> String? {
        guard !data.isEmpty else { return nil }
        if let payload = try? decoder.decode(APIErrorResponse.self, from: data) {
            return payload.error ?? payload.message
        }
        return String(data: data, encoding: .utf8)
    }
}

private struct EmptyResponse: Codable {}

private struct ImageUploadResponse: Decodable {
    struct Payload: Decodable {
        let filePath: String
    }
    let data: Payload
}

private struct MultipartFormData {
    let boundary: String
    private var data = Data()

    init(boundary: String) {
        self.boundary = boundary
    }

    func appendFile(fieldName: String, fileName: String, mimeType: String, data fileData: Data) -> MultipartFormData {
        var copy = self
        copy.data.append("--\(boundary)\r\n")
        copy.data.append("Content-Disposition: form-data; name=\"\(fieldName)\"; filename=\"\(fileName)\"\r\n")
        copy.data.append("Content-Type: \(mimeType)\r\n\r\n")
        copy.data.append(fileData)
        copy.data.append("\r\n")
        return copy
    }

    func finalized() -> Data {
        var copy = data
        copy.append("--\(boundary)--\r\n")
        return copy
    }
}

private extension Data {
    mutating func append(_ string: String) {
        append(Data(string.utf8))
    }
}

private extension String {
    var urlPathEscaped: String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/")
        return addingPercentEncoding(withAllowedCharacters: allowed) ?? self
    }
}

private extension ISO8601DateFormatter {
    static let tagnoteFractional: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()

    static let tagnote: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter
    }()
}
