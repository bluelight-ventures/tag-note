import SwiftUI

@MainActor
final class ShareComposeViewModel: ObservableObject {
    @Published var content: String
    @Published var tags: [String] = []
    @Published var tagDraft: String = ""
    @Published var mode: ShareContentMode = .link
    @Published var isPosting = false
    @Published var errorMessage: String?
    @Published private(set) var isLoggedOut = false
    /// When on, posting opens TagNote afterwards instead of returning to the
    /// source app. Remembered across shares.
    @Published var openAppAfterPost: Bool {
        didSet { UserDefaults.standard.set(openAppAfterPost, forKey: Self.openAppKey) }
    }

    private static let openAppKey = "shareOpenAppAfterPost"

    let payload: SharePayload
    private let onComplete: () -> Void
    private let onCancel: () -> Void
    private let onOpenApp: () -> Void

    /// The Link / Full-page control only applies to web pages whose readable
    /// text was captured (Safari shares).
    var showsModeToggle: Bool { payload.kind == .webPage && payload.hasArticleText }
    var isImageShare: Bool { payload.kind == .image }
    var canPost: Bool {
        !isPosting && (isImageShare || !content.trimmed.isEmpty || !tags.isEmpty)
    }

    init(
        payload: SharePayload,
        onComplete: @escaping () -> Void,
        onCancel: @escaping () -> Void,
        onOpenApp: @escaping () -> Void
    ) {
        self.payload = payload
        self.onComplete = onComplete
        self.onCancel = onCancel
        self.onOpenApp = onOpenApp
        self.content = payload.markdown(mode: .link)
        self.isLoggedOut = ShareCredentials.load() == nil
        self.openAppAfterPost = UserDefaults.standard.bool(forKey: Self.openAppKey)
    }

    /// Toggling the capture mode regenerates the body from the shared page.
    func setMode(_ newMode: ShareContentMode) {
        guard newMode != mode else { return }
        mode = newMode
        content = payload.markdown(mode: newMode)
    }

    func commitTagDraft() {
        let raw = tagDraft.replacingOccurrences(of: ",", with: " ")
        for token in raw.split(separator: " ") {
            let tag = String(token).trimmed.lowercased()
            guard !tag.isEmpty, tag != "$default", !tags.contains(tag) else { continue }
            tags.append(tag)
        }
        tagDraft = ""
    }

    func removeTag(_ tag: String) {
        tags.removeAll { $0 == tag }
    }

    func cancel() { onCancel() }

    func post() async {
        commitTagDraft()
        guard let credentials = ShareCredentials.load() else {
            isLoggedOut = true
            return
        }
        errorMessage = nil
        isPosting = true
        defer { isPosting = false }

        let api = TagNoteAPI()
        api.configure(serverURL: credentials.serverURL, token: credentials.token)
        do {
            var body = content.trimmed
            if payload.kind == .image, let data = payload.imageData {
                let path = try await api.uploadImage(
                    data: data,
                    fileName: payload.imageFileName ?? "shared-image.jpg",
                    mimeType: payload.imageMimeType ?? "image/jpeg"
                )
                body = ShareMarkdownBuilder.appendingImage(path: path, to: body)
            }
            let created = try await api.createNote(content: body, tags: tags)
            // Hand the new note to the app so it appears immediately on next
            // foreground, ahead of the network refresh.
            SharedNoteInbox.append(SubNote(
                id: created.id,
                shortID: created.shortID,
                content: body,
                snippet: nil,
                createdAt: created.createdAt,
                updatedAt: nil,
                tags: tags,
                pinned: false
            ))
            if openAppAfterPost {
                onOpenApp()
            }
            onComplete()
        } catch {
            errorMessage = (error as? LocalizedError)?.errorDescription ?? error.localizedDescription
        }
    }
}

struct ShareComposeView: View {
    @ObservedObject var viewModel: ShareComposeViewModel
    @FocusState private var tagFieldFocused: Bool

    var body: some View {
        NavigationStack {
            Group {
                if viewModel.isLoggedOut {
                    loggedOut
                } else {
                    composer
                }
            }
            .navigationTitle("New note")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { viewModel.cancel() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if viewModel.isPosting {
                        ProgressView()
                    } else if !viewModel.isLoggedOut {
                        Button("Post") { Task { await viewModel.post() } }
                            .disabled(!viewModel.canPost)
                    }
                }
            }
        }
    }

    private var loggedOut: some View {
        VStack(spacing: 12) {
            Image(systemName: "person.crop.circle.badge.exclamationmark")
                .font(.system(size: 40, weight: .semibold))
                .foregroundStyle(.secondary)
            Text("Open TagNote and sign in to share notes.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var composer: some View {
        VStack(alignment: .leading, spacing: 12) {
            tagEditor

            if viewModel.showsModeToggle {
                Picker("Capture", selection: Binding(
                    get: { viewModel.mode },
                    set: { viewModel.setMode($0) }
                )) {
                    Text("Link").tag(ShareContentMode.link)
                    Text("Full page").tag(ShareContentMode.fullPage)
                }
                .pickerStyle(.segmented)
            }

            TextEditor(text: $viewModel.content)
                .font(.body)
                .frame(minHeight: 140)
                .overlay(RoundedRectangle(cornerRadius: 8).stroke(Color(.separator)))

            if viewModel.isImageShare {
                Label("An image will be attached to this note.", systemImage: "photo")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }

            Toggle("Open TagNote after posting", isOn: $viewModel.openAppAfterPost)
                .font(.callout)

            if let error = viewModel.errorMessage {
                Text(error)
                    .font(.footnote)
                    .foregroundStyle(.red)
            }

            Spacer(minLength: 0)
        }
        .padding()
    }

    private var tagEditor: some View {
        VStack(alignment: .leading, spacing: 8) {
            if !viewModel.tags.isEmpty {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 6) {
                        ForEach(viewModel.tags, id: \.self) { tag in
                            HStack(spacing: 4) {
                                Text("#\(tag)")
                                Button {
                                    viewModel.removeTag(tag)
                                } label: {
                                    Image(systemName: "xmark.circle.fill")
                                }
                                .buttonStyle(.plain)
                            }
                            .font(.callout)
                            .padding(.horizontal, 10)
                            .padding(.vertical, 5)
                            .background(Color(.secondarySystemBackground))
                            .clipShape(Capsule())
                        }
                    }
                }
            }

            TextField("Add tag", text: $viewModel.tagDraft)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .focused($tagFieldFocused)
                .submitLabel(.done)
                .onSubmit { viewModel.commitTagDraft() }
                .onChange(of: viewModel.tagDraft) { _, value in
                    if value.hasSuffix(" ") || value.hasSuffix(",") {
                        viewModel.commitTagDraft()
                    }
                }
                .padding(10)
                .background(Color(.secondarySystemBackground))
                .clipShape(RoundedRectangle(cornerRadius: 8))
        }
    }
}

private extension String {
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}
