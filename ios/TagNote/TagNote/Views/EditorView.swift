import PhotosUI
import SwiftUI
import UIKit

struct EditorView: View {
    @Environment(\.dismiss) private var dismiss
    @EnvironmentObject private var appState: AppState
    @StateObject var viewModel: EditorViewModel
    @State private var tagDraft = ""
    @State private var selectedPhoto: PhotosPickerItem?
    @State private var showDiscardPrompt = false
    @State private var showDeletePrompt = false
    @State private var isContentEditing = false
    @State private var keyboardVisible = false
    @State private var editorController = MarkdownEditorController()
    @FocusState private var tagFieldFocused: Bool

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
                    MarkdownTextView(
                        text: $viewModel.content,
                        isFocused: $isContentEditing,
                        controller: editorController,
                        textColor: UIColor(appState.palette.text),
                        tintColor: UIColor(appState.palette.accent)
                    )
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
            // The toolbar is always available (pin/save are useful outside
            // editing too), but the numbers/symbols row belongs to the content
            // keyboard, so only show it while that keyboard is actually up. Keying
            // off real keyboard visibility avoids it lingering after dismissal.
            .safeAreaInset(edge: .bottom) {
                VStack(spacing: 0) {
                    if isEditingContent {
                        symbolsRow
                    }
                    formattingBar
                }
            }
            .onReceive(NotificationCenter.default.publisher(for: UIResponder.keyboardWillShowNotification)) { _ in
                keyboardVisible = true
            }
            .onReceive(NotificationCenter.default.publisher(for: UIResponder.keyboardWillHideNotification)) { _ in
                keyboardVisible = false
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
                    if viewModel.tags.isEmpty {
                        DefaultTagChip()
                    }
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
                    .focused($tagFieldFocused)
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

    // Quick-insert numbers and common punctuation (English + Chinese) so authors
    // don't have to switch the system keyboard to its "123" plane while writing.
    private static let quickSymbols: [String] = [
        "1", "2", "3", "4", "5", "6", "7", "8", "9", "0",
        // Markdown syntax characters, up front where they're easy to reach.
        "#", "*", "_", "`", "-", "[", "]", "(", ")", ">", "~", "|",
        ".", ",", "?", "!", ":", ";", "'", "\"", "/", "\\", "@", "&", "+", "=",
        "，", "。", "？", "！", "、", "：", "；", "（", "）", "“", "”", "《", "》", "—", "…"
    ]

    private var symbolsRow: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 3) {
                ForEach(Self.quickSymbols, id: \.self) { symbol in
                    Button {
                        editorController.insert(symbol)
                    } label: {
                        Text(symbol)
                            .font(.system(size: 16))
                            .frame(minWidth: 24, minHeight: 33)
                            .background(appState.palette.card)
                            .clipShape(RoundedRectangle(cornerRadius: 5))
                    }
                    .buttonStyle(.plain)
                    .foregroundStyle(appState.palette.text)
                    .accessibilityIdentifier("symbol-\(symbol)")
                }
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 5)
        }
        .background(.bar)
        .accessibilityIdentifier("symbols-row")
    }

    /// The content keyboard is up, so markdown editing controls are relevant.
    private var isEditingContent: Bool { isContentEditing && keyboardVisible }

    private var formattingBar: some View {
        HStack(spacing: 6) {
            // Formatting actions only make sense while editing the note body, so
            // hide them when the content keyboard is down. Pin/Save stay so they
            // remain reachable outside editing.
            if isEditingContent {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 2) {
                        formatButton("Bold", systemImage: "bold") { editorController.toggleWrap(prefix: "**", suffix: "**") }
                        formatButton("Italic", systemImage: "italic") { editorController.toggleWrap(prefix: "*", suffix: "*") }
                        formatButton("Strikethrough", systemImage: "strikethrough") { editorController.toggleWrap(prefix: "~~", suffix: "~~") }
                        formatButton("Code", systemImage: "chevron.left.forwardslash.chevron.right") { editorController.toggleWrap(prefix: "`", suffix: "`") }
                        formatButton("Heading", systemImage: "textformat.size") { editorController.toggleLinePrefix("## ") }
                        formatButton("Bulleted list", systemImage: "list.bullet") { editorController.toggleLinePrefix("- ") }
                        formatButton("Numbered list", systemImage: "list.number") { editorController.toggleOrderedList() }
                        formatButton("Quote", systemImage: "text.quote") { editorController.toggleLinePrefix("> ") }
                        formatButton("Link", systemImage: "link") { editorController.insertLink() }
                        PhotosPicker(selection: $selectedPhoto, matching: .images) {
                            Image(systemName: "photo")
                                .frame(width: 28, height: 28)
                        }
                        .accessibilityLabel("Insert image")
                    }
                }
            } else {
                Spacer()
            }
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
                    .frame(width: 28, height: 28)
            }
            .accessibilityLabel("Save now")
            // Only offer hide-keyboard when a keyboard is actually up.
            if keyboardVisible && (isContentEditing || tagFieldFocused) {
                Button {
                    if isContentEditing { editorController.resignFocus() }
                    tagFieldFocused = false
                } label: {
                    Image(systemName: "keyboard.chevron.compact.down")
                        .frame(width: 28, height: 28)
                }
                .accessibilityIdentifier("dismiss-keyboard-button")
                .accessibilityLabel("Hide keyboard")
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
        case .failed: return "Failed"
        case .idle: return ""
        }
    }

    private var icon: String {
        switch status {
        case .unsaved: return "pencil"
        case .saving: return "arrow.triangle.2.circlepath"
        case .saved: return "checkmark"
        case .failed: return "wifi.slash"
        case .idle: return ""
        }
    }

    private var color: Color {
        switch status {
        case .unsaved, .saving: return appState.palette.warning
        case .saved: return appState.palette.success
        case .failed: return appState.palette.destructive
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
struct MarkdownImageView: View {
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
