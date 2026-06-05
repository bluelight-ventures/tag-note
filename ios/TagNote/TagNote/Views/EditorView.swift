import PhotosUI
import SwiftUI

struct EditorView: View {
    @Environment(\.dismiss) private var dismiss
    @EnvironmentObject private var appState: AppState
    @StateObject var viewModel: EditorViewModel
    @State private var tagDraft = ""
    @State private var selectedPhoto: PhotosPickerItem?
    @State private var showDiscardPrompt = false
    @State private var showDeletePrompt = false

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                tagEditor
                    .padding(.horizontal)
                    .padding(.vertical, 10)
                    .background(appState.palette.background)

                Picker("Editor Mode", selection: $viewModel.previewMode) {
                    ForEach(PreviewMode.allCases) { mode in
                        Text(mode.label).tag(mode)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)
                .padding(.bottom, 8)

                if viewModel.previewMode == .write {
                    TextEditor(text: $viewModel.content)
                        .font(.body.monospaced())
                        .foregroundStyle(appState.palette.text)
                        .scrollContentBackground(.hidden)
                        .background(appState.palette.card)
                        .onChange(of: viewModel.content) { _, _ in viewModel.scheduleAutosave() }
                        .accessibilityIdentifier("note-content-editor")
                } else {
                    ScrollView {
                        markdownPreview
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .padding()
                    }
                    .background(appState.palette.card)
                }
            }
            .background(appState.palette.background.ignoresSafeArea())
            .navigationTitle(viewModel.isNewNote ? "New note" : "Edit note")
            .accessibilityIdentifier("editor-screen")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem {
                    Button("Close") {
                        close()
                    }
                }
                ToolbarItem(placement: .principal) {
                    VStack(spacing: 3) {
                        Text(viewModel.isNewNote ? "New note" : "Edit note")
                            .font(.headline)
                        SaveStatusPill(status: viewModel.saveStatus)
                            .environmentObject(appState)
                    }
                }
                ToolbarItem {
                    Menu {
                        Button {
                            Task { await viewModel.saveNow() }
                        } label: {
                            Label("Save now", systemImage: "checkmark")
                        }
                        if !viewModel.isNewNote {
                            Button(role: .destructive) {
                                showDeletePrompt = true
                            } label: {
                                Label("Delete", systemImage: "trash")
                            }
                        }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                }
            }
            .safeAreaInset(edge: .bottom) {
                formattingBar
            }
            .onChange(of: selectedPhoto) { _, item in
                Task { await loadPhoto(item) }
            }
            .alert("Discard unsaved changes?", isPresented: $showDiscardPrompt) {
                Button("Keep editing", role: .cancel) {}
                Button("Discard", role: .destructive) { dismiss() }
            } message: {
                Text("Your latest edits have not been saved.")
            }
            .alert("Delete note?", isPresented: $showDeletePrompt) {
                Button("Cancel", role: .cancel) {}
                Button("Delete", role: .destructive) {
                    Task {
                        try? await viewModel.delete()
                        dismiss()
                    }
                }
            }
            .alert("Error", isPresented: Binding(get: { viewModel.errorMessage != nil }, set: { if !$0 { viewModel.errorMessage = nil } })) {
                Button("OK", role: .cancel) {}
            } message: {
                Text(viewModel.errorMessage ?? "")
            }
        }
    }

    private var tagEditor: some View {
        VStack(alignment: .leading, spacing: 8) {
            ScrollView(.horizontal, showsIndicators: false) {
                HStack {
                    ForEach(viewModel.tags, id: \.self) { tag in
                        HStack(spacing: 4) {
                            Text(tag)
                            Button {
                                viewModel.removeTag(tag)
                            } label: {
                                Image(systemName: "xmark.circle.fill")
                            }
                        }
                        .font(.caption.weight(.medium))
                        .padding(.horizontal, 10)
                        .padding(.vertical, 6)
                        .background(appState.palette.tagBackground)
                        .clipShape(Capsule())
                    }
                }
            }

            HStack {
                TextField("Add tag", text: $tagDraft)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .onSubmit(commitTagDraft)
                    .onChange(of: tagDraft) { _, value in
                        // Commit on space, comma, or enter — matches the web chip input (ux_guidelines §12).
                        if let last = value.last, last == " " || last == "," || last == "\n" {
                            commitTagDraft()
                            return
                        }
                        Task { await viewModel.autocomplete(value) }
                    }
                    .accessibilityIdentifier("tag-input-field")
                Button {
                    commitTagDraft()
                } label: {
                    Image(systemName: "plus.circle.fill")
                }
                .disabled(tagDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                .accessibilityIdentifier("add-tag-button")
            }
            .padding(10)
            .background(appState.palette.card)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border))

            if !tagDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 6) {
                        if showGhostChip {
                            GhostTagChip(label: normalizedDraft) {
                                commitTagDraft()
                            }
                        }
                        ForEach(viewModel.suggestions, id: \.self) { suggestion in
                            TagChip(suggestion) {
                                viewModel.addTag(suggestion)
                                tagDraft = ""
                            }
                        }
                    }
                }
            }
        }
    }

    private var normalizedDraft: String {
        tagDraft.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    // A ghost chip lets the author create a tag that doesn't exist yet (ux_guidelines §12).
    private var showGhostChip: Bool {
        !normalizedDraft.isEmpty
            && !viewModel.tags.contains(normalizedDraft)
            && !viewModel.suggestions.contains { $0.lowercased() == normalizedDraft }
    }

    private var formattingBar: some View {
        HStack {
            formatButton("B", systemImage: "bold") { wrapSelection(prefix: "**", suffix: "**") }
            formatButton("I", systemImage: "italic") { wrapSelection(prefix: "*", suffix: "*") }
            formatButton("H", systemImage: "textformat.size") { insert(prefix: "## ") }
            formatButton("List", systemImage: "list.bullet") { insert(prefix: "- ") }
            formatButton("Quote", systemImage: "quote.opening") { insert(prefix: "> ") }
            PhotosPicker(selection: $selectedPhoto, matching: .images) {
                Image(systemName: "photo")
                    .frame(width: 28, height: 28)
            }
            .accessibilityLabel("Insert image")
            Spacer()
            Button {
                Task { await viewModel.togglePin() }
            } label: {
                Image(systemName: viewModel.isPinned ? "pin.fill" : "pin")
                    .frame(width: 28, height: 28)
            }
            .accessibilityLabel(viewModel.isPinned ? "Unpin" : "Pin")
            Button {
                Task { await viewModel.saveNow() }
            } label: {
                Image(systemName: "checkmark.circle")
            }
        }
        .padding(.horizontal)
        .padding(.vertical, 8)
        .background(.bar)
    }

    private var markdownPreview: some View {
        MarkdownPreviewView(document: .parse(viewModel.content))
            .environmentObject(appState)
    }

    private func formatButton(_ title: String, systemImage: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: systemImage)
                .frame(width: 28, height: 28)
        }
        .accessibilityLabel(title)
    }

    private func commitTagDraft() {
        viewModel.addTag(tagDraft)
        tagDraft = ""
    }

    private func insert(prefix: String) {
        viewModel.content += viewModel.content.hasSuffix("\n") || viewModel.content.isEmpty ? prefix : "\n\(prefix)"
        viewModel.scheduleAutosave()
    }

    private func wrapSelection(prefix: String, suffix: String) {
        viewModel.content += "\(prefix)\(suffix)"
        viewModel.scheduleAutosave()
    }

    private func close() {
        if viewModel.canCloseCleanly {
            dismiss()
        } else {
            showDiscardPrompt = true
        }
    }

    private func loadPhoto(_ item: PhotosPickerItem?) async {
        guard let item,
              let data = try? await item.loadTransferable(type: Data.self) else {
            return
        }
        await viewModel.uploadImage(data: data, mimeType: "image/jpeg")
        selectedPhoto = nil
    }
}

/// Save-status indicator with the five canonical states, each carrying a color
/// *and* an icon/label so meaning never relies on color alone (ux_guidelines §16, §27).
private struct SaveStatusPill: View {
    @EnvironmentObject private var appState: AppState
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    let status: SaveStatus
    @State private var pulse = false

    var body: some View {
        if status == .idle {
            EmptyView()
        } else {
            HStack(spacing: 6) {
                Image(systemName: icon)
                    .font(.system(size: 10, weight: .bold))
                    .opacity(isSaving && pulse ? 0.4 : 1)
                    .scaleEffect(isSaving && pulse ? 0.82 : 1)
                Text(label)
                    .font(.system(size: 12, weight: .semibold))
                    .lineLimit(1)
            }
            .foregroundStyle(color)
            .padding(.horizontal, 9)
            .padding(.vertical, 3)
            .background(color.opacity(0.12))
            .overlay(Capsule().stroke(color.opacity(0.32), lineWidth: 1))
            .clipShape(Capsule())
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("Save status: \(label)")
            .onAppear { updatePulse() }
            .onChange(of: status) { _, _ in updatePulse() }
        }
    }

    private var isSaving: Bool { status == .saving }

    private func updatePulse() {
        guard isSaving, !reduceMotion else {
            pulse = false
            return
        }
        withAnimation(.easeInOut(duration: 1).repeatForever(autoreverses: true)) {
            pulse = true
        }
    }

    private var label: String {
        switch status {
        case .unsaved: return "Unsaved"
        case .saving: return "Saving…"
        case .saved: return "Saved"
        case .invalid: return "Invalid"
        case .failed: return "Failed"
        case .idle: return ""
        }
    }

    private var icon: String {
        switch status {
        case .unsaved: return "pencil"
        case .saving: return "arrow.triangle.2.circlepath"
        case .saved: return "checkmark"
        case .invalid: return "exclamationmark.triangle"
        case .failed: return "wifi.slash"
        case .idle: return ""
        }
    }

    private var color: Color {
        switch status {
        case .unsaved, .saving: return appState.palette.warning
        case .saved: return appState.palette.success
        case .invalid, .failed: return appState.palette.destructive
        case .idle: return appState.palette.secondaryText
        }
    }
}

/// Dashed "create" chip for a tag that does not exist yet (ux_guidelines §12).
private struct GhostTagChip: View {
    @EnvironmentObject private var appState: AppState
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 4) {
                Image(systemName: "plus")
                Text("#\(label)")
            }
            .font(.system(size: 15, weight: .semibold))
            .lineLimit(1)
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
            .foregroundStyle(appState.palette.secondaryText)
            .overlay(
                Capsule().stroke(style: StrokeStyle(lineWidth: 1, dash: [4, 3]))
                    .foregroundStyle(appState.palette.border)
            )
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Create tag \(label)")
    }
}

struct MarkdownPreviewView: View {
    @EnvironmentObject private var appState: AppState
    let document: MarkdownDocument

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(Array(document.blocks.enumerated()), id: \.offset) { _, block in
                blockView(block)
            }
        }
        .foregroundStyle(appState.palette.text)
        .textSelection(.enabled)
    }

    private func blockView(_ block: MarkdownBlock) -> AnyView {
        switch block {
        case let .heading(level, text):
            return AnyView(inlineText(text)
                .font(headingFont(level))
                .fontWeight(.semibold)
                .frame(maxWidth: .infinity, alignment: .leading))
        case let .paragraph(text):
            return AnyView(inlineText(text)
                .font(.body)
                .lineSpacing(4)
                .frame(maxWidth: .infinity, alignment: .leading))
        case let .image(source, alt):
            return AnyView(MarkdownImageView(source: source, alt: alt, baseURL: appState.session.serverURL))
        case let .unorderedList(items):
            return AnyView(VStack(alignment: .leading, spacing: 6) {
                ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                    HStack(alignment: .top, spacing: 8) {
                        Text("•")
                            .font(.body.weight(.semibold))
                        inlineText(item.inlineText)
                            .font(.body)
                            .lineSpacing(4)
                    }
                }
            })
        case let .orderedList(start, items):
            return AnyView(VStack(alignment: .leading, spacing: 6) {
                ForEach(Array(items.enumerated()), id: \.offset) { index, item in
                    HStack(alignment: .top, spacing: 8) {
                        Text("\(start + index).")
                            .font(.body.weight(.semibold))
                            .frame(minWidth: 24, alignment: .trailing)
                        inlineText(item.inlineText)
                            .font(.body)
                            .lineSpacing(4)
                    }
                }
            })
        case let .quote(blocks):
            return AnyView(HStack(alignment: .top, spacing: 10) {
                Rectangle()
                    .fill(appState.palette.border)
                    .frame(width: 3)
                VStack(alignment: .leading, spacing: 8) {
                    ForEach(Array(blocks.enumerated()), id: \.offset) { _, nested in
                        blockView(nested)
                    }
                }
                .font(.body.italic())
                .foregroundStyle(appState.palette.secondaryText)
            })
        case let .code(text):
            return AnyView(ScrollView(.horizontal, showsIndicators: false) {
                Text(text)
                    .font(.system(.body, design: .monospaced))
                    .padding(10)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .background(appState.palette.background)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border)))
        case .horizontalRule:
            return AnyView(Rectangle()
                .fill(appState.palette.border)
                .frame(height: 1)
                .padding(.vertical, 4))
        }
    }

    private func inlineText(_ markdown: String) -> Text {
        let options = AttributedString.MarkdownParsingOptions(interpretedSyntax: .inlineOnlyPreservingWhitespace)
        if let attributed = try? AttributedString(markdown: markdown, options: options) {
            return Text(attributed)
        }
        return Text(markdown)
    }

    private func headingFont(_ level: Int) -> Font {
        switch level {
        case 1:
            return .title2
        case 2:
            return .title3
        default:
            return .headline
        }
    }
}

// Renders a markdown image. Relative sources (e.g. `/uploads/x.png`) are
// resolved against the server base URL; uploads are link-private so no auth
// header is needed.
private struct MarkdownImageView: View {
    @EnvironmentObject private var appState: AppState
    let source: String
    let alt: String
    let baseURL: URL?

    private var resolvedURL: URL? {
        if let direct = URL(string: source), direct.scheme != nil {
            return direct
        }
        guard let baseURL else { return nil }
        return URL(string: source, relativeTo: baseURL)?.absoluteURL
    }

    var body: some View {
        Group {
            if let resolvedURL {
                AsyncImage(url: resolvedURL) { phase in
                    switch phase {
                    case .empty:
                        placeholder(systemImage: "photo", text: "Loading image…")
                            .overlay(ProgressView())
                    case let .success(image):
                        image
                            .resizable()
                            .scaledToFit()
                            .frame(maxWidth: .infinity)
                            .clipShape(RoundedRectangle(cornerRadius: 10))
                    case .failure:
                        placeholder(systemImage: "exclamationmark.triangle", text: alt.isEmpty ? "Image unavailable" : alt)
                    @unknown default:
                        placeholder(systemImage: "photo", text: alt.isEmpty ? "Image" : alt)
                    }
                }
            } else {
                placeholder(systemImage: "photo", text: alt.isEmpty ? "Image" : alt)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityLabel(alt.isEmpty ? "Image" : alt)
    }

    private func placeholder(systemImage: String, text: String) -> some View {
        HStack(spacing: 8) {
            Image(systemName: systemImage)
            Text(text)
        }
        .font(.footnote)
        .foregroundStyle(appState.palette.secondaryText)
        .frame(maxWidth: .infinity, minHeight: 120)
        .background(appState.palette.background)
        .clipShape(RoundedRectangle(cornerRadius: 10))
    }
}
