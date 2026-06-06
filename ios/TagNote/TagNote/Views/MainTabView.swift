import SwiftUI

struct MainTabView: View {
    @EnvironmentObject private var session: SessionStore

    var body: some View {
        WebStyleAppShell(api: session.api, cache: session.cache)
    }
}

private struct WebStyleAppShell: View {
    @EnvironmentObject private var appState: AppState
    @Environment(\.horizontalSizeClass) private var horizontalSizeClass
    @Environment(\.scenePhase) private var scenePhase
    let api: TagNoteAPI
    let cache: LocalCache
    @StateObject private var notesViewModel: NotesViewModel
    @StateObject private var tagsViewModel: TagsViewModel
    @StateObject private var trashViewModel: TrashViewModel
    @State private var selection: AppSection = .notes
    @State private var isSidebarOpen = false
    @State private var activeEditorSheet: AppEditorSheet?
    @State private var readNote: SubNote?
    @State private var didAutoOpenEditor = false

    init(api: TagNoteAPI, cache: LocalCache) {
        self.api = api
        self.cache = cache
        _notesViewModel = StateObject(wrappedValue: NotesViewModel(api: api, cache: cache))
        _tagsViewModel = StateObject(wrappedValue: TagsViewModel(api: api, cache: cache))
        _trashViewModel = StateObject(wrappedValue: TrashViewModel(api: api))
    }

    // Persistent sidebar when the window is regular-width (iPad full screen or a
    // wide split / Stage Manager window), and the slide-over drawer when compact
    // (iPhone, slide-over, narrow split). horizontalSizeClass is the reliable,
    // built-in "is the window wide enough" signal — the iOS equivalent of a web
    // media query — and it updates as the window is resized.
    private var usesPersistentSidebar: Bool {
        if ProcessInfo.processInfo.environment["TAGNOTE_UI_FORCE_COMPACT"] == "1" { return false }
        return horizontalSizeClass == .regular
    }

    var body: some View {
        Group {
            if usesPersistentSidebar {
                regularLayout
            } else {
                compactLayout
            }
        }
        .overlay {
            if let readNote {
                ResizableNoteReadPanel(
                    note: readNote,
                    availableTags: notesViewModel.availableTags,
                    onClose: {
                        self.readNote = nil
                    },
                    onEdit: {
                        self.readNote = nil
                        activeEditorSheet = .edit(readNote)
                    }
                )
                .environmentObject(appState)
            }
        }
        .task {
            await notesViewModel.loadCached()
            notesViewModel.mergeOptimistic(SharedNoteInbox.drain())
            await notesViewModel.refresh()
            if shouldAutoOpenEditor {
                didAutoOpenEditor = true
                activeEditorSheet = .create
            }
            if ProcessInfo.processInfo.environment["TAGNOTE_UI_OPEN_SIDEBAR"] == "1" {
                isSidebarOpen = true
            }
            if let preset = ProcessInfo.processInfo.environment["TAGNOTE_UI_PRESELECT_TAGS"], !preset.isEmpty {
                await notesViewModel.setTagFilters(preset.split(separator: ",").map { String($0).trimmingCharacters(in: .whitespaces) })
            }
        }
        // Refresh when the app returns to the foreground so notes created
        // elsewhere — e.g. via the Share Extension from another app — show up
        // without a manual pull-to-refresh. onChange only fires on real
        // transitions, so this does not double-run the initial .task load.
        .onChange(of: scenePhase) { _, phase in
            guard phase == .active, appState.session.isAuthenticated else { return }
            // Show anything the Share Extension just created instantly, then
            // reconcile with the server.
            notesViewModel.mergeOptimistic(SharedNoteInbox.drain())
            Task {
                await notesViewModel.refresh()
                await tagsViewModel.refresh()
            }
        }
        .sheet(item: $activeEditorSheet, onDismiss: { Task { await notesViewModel.refresh() } }) { sheet in
            switch sheet {
            case .create:
                EditorView(viewModel: EditorViewModel(note: nil, api: api))
                    .environmentObject(appState)
            case .edit(let note):
                EditorView(viewModel: EditorViewModel(note: note, api: api))
                    .environmentObject(appState)
            }
        }
    }

    // Desktop-like split: fixed sidebar rail + the active surface fills the rest.
    private var regularLayout: some View {
        GeometryReader { geometry in
            HStack(spacing: 0) {
                SidebarView(
                    selection: $selection,
                    isOpen: .constant(true),
                    isPersistent: true,
                    safeAreaTop: geometry.safeAreaInsets.top,
                    safeAreaBottom: geometry.safeAreaInsets.bottom,
                    notesViewModel: notesViewModel,
                    tagsViewModel: tagsViewModel,
                    createNote: { activeEditorSheet = .create }
                )
                .frame(width: 280)

                Rectangle()
                    .fill(appState.palette.border)
                    .frame(width: 1)
                    .ignoresSafeArea()

                content
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .background(appState.palette.background.ignoresSafeArea())
    }

    // Phone: header + slide-over drawer.
    private var compactLayout: some View {
        GeometryReader { geometry in
            let sidebarWidth = min(geometry.size.width * 0.72, 280)
            ZStack(alignment: .leading) {
                appState.palette.background
                    .ignoresSafeArea()

                VStack(spacing: 0) {
                    MobileHeader {
                        withAnimation(.easeInOut(duration: 0.22)) {
                            isSidebarOpen = true
                        }
                    }

                    content
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                }

                if isSidebarOpen {
                    Color.black.opacity(0.42)
                        .ignoresSafeArea()
                        .onTapGesture {
                            withAnimation(.easeInOut(duration: 0.22)) {
                                isSidebarOpen = false
                            }
                        }
                }

                SidebarView(
                    selection: $selection,
                    isOpen: $isSidebarOpen,
                    isPersistent: false,
                    safeAreaTop: geometry.safeAreaInsets.top,
                    safeAreaBottom: geometry.safeAreaInsets.bottom,
                    notesViewModel: notesViewModel,
                    tagsViewModel: tagsViewModel,
                    createNote: {
                        activeEditorSheet = .create
                    }
                )
                .frame(width: sidebarWidth)
                .offset(x: isSidebarOpen ? 0 : -sidebarWidth)
                .animation(.easeInOut(duration: 0.22), value: isSidebarOpen)
                .shadow(color: .black.opacity(isSidebarOpen ? 0.28 : 0), radius: 18, x: 4, y: 0)
            }
            // The hamburger can sit under the Stage Manager window grabber in a
            // small iPad window, so a left-edge swipe must also open the drawer —
            // otherwise the sidebar (the only way to reach search / tags /
            // settings at compact width) becomes unreachable.
            .overlay(alignment: .leading) {
                if !isSidebarOpen {
                    Color.clear
                        .frame(width: 20)
                        .frame(maxHeight: .infinity)
                        .contentShape(Rectangle())
                        .gesture(
                            DragGesture(minimumDistance: 12)
                                .onEnded { value in
                                    if value.translation.width > 40 {
                                        withAnimation(.easeInOut(duration: 0.22)) {
                                            isSidebarOpen = true
                                        }
                                    }
                                }
                        )
                        .ignoresSafeArea()
                }
            }
        }
    }

    @ViewBuilder
    private var content: some View {
        switch selection {
        case .notes:
            NotesView(viewModel: notesViewModel) { note in
                readNote = note
            }
        case .tags:
            TagsView(viewModel: tagsViewModel)
        case .trash:
            TrashView(viewModel: trashViewModel)
        case .settings:
            SettingsView()
        }
    }

    private var shouldAutoOpenEditor: Bool {
        guard !didAutoOpenEditor else { return false }
        return ProcessInfo.processInfo.environment["TAGNOTE_UI_CREATE_NOTE"] == "1" ||
            ProcessInfo.processInfo.arguments.contains("-ui-create-note")
    }
}

private enum AppEditorSheet: Identifiable {
    case create
    case edit(SubNote)

    var id: String {
        switch self {
        case .create:
            return "create"
        case .edit(let note):
            return "edit-\(note.routeID)"
        }
    }
}

private enum AppSection: String, CaseIterable, Identifiable {
    case notes
    case tags
    case trash
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .notes: return "Notes"
        case .tags: return "Tags"
        case .trash: return "Trash"
        case .settings: return "Settings"
        }
    }

    var icon: String {
        switch self {
        case .notes: return "doc.text"
        case .tags: return "tag"
        case .trash: return "trash"
        case .settings: return "gearshape"
        }
    }
}

private struct MobileHeader: View {
    @EnvironmentObject private var appState: AppState
    let openSidebar: () -> Void

    // On iPad the top bar only appears in a windowed mode (Split View / Stage
    // Manager / Slide Over), where the system puts its window control (•••) in
    // the top-leading corner — right over a leading menu button. Put the menu
    // button on the trailing edge there so it stays visible and tappable; keep
    // it leading on iPhone, which never shows that system control.
    private var menuOnTrailing: Bool { UIDevice.current.userInterfaceIdiom == .pad }

    var body: some View {
        HStack(spacing: 16) {
            if !menuOnTrailing { menuButton }

            BrandMark(size: 32)

            Text("TagNote")
                .font(.system(size: 24, weight: .heavy))
                .foregroundStyle(appState.palette.text)

            Spacer()

            if menuOnTrailing { menuButton }
        }
        // Indent the leading edge on iPad so the brand clears the system •••.
        .padding(.leading, menuOnTrailing ? 64 : 16)
        .padding(.trailing, 16)
        .frame(height: 48)
        .background(appState.palette.card)
        .overlay(alignment: .bottom) {
            Rectangle()
                .fill(appState.palette.border)
                .frame(height: 1)
        }
    }

    private var menuButton: some View {
        Button(action: openSidebar) {
            Image(systemName: "line.3.horizontal")
                .font(.system(size: 20, weight: .semibold))
                .frame(width: 36, height: 36)
        }
        .buttonStyle(.plain)
        .foregroundStyle(appState.palette.text)
        .accessibilityLabel("Open menu")
        .accessibilityIdentifier("sidebar-open-button")
    }
}

private struct SidebarView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var session: SessionStore
    @Binding var selection: AppSection
    @Binding var isOpen: Bool
    var isPersistent = false
    var safeAreaTop: CGFloat = 0
    var safeAreaBottom: CGFloat = 0
    @ObservedObject var notesViewModel: NotesViewModel
    @ObservedObject var tagsViewModel: TagsViewModel
    let createNote: () -> Void
    @State private var tagSearch = ""
    @State private var searchTask: Task<Void, Never>?

    private var unreviewedTagCount: Int {
        notesViewModel.availableTags.filter(\.isUnreviewed).count
    }

    private var visibleTags: [TagInfo] {
        let q = tagSearch.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !q.isEmpty else { return notesViewModel.availableTags }
        return notesViewModel.availableTags.filter { $0.name.localizedCaseInsensitiveContains(q) }
    }

    var body: some View {
        // The drawer fills the full height (card bleeds behind the status bar /
        // home indicator), but the ScrollView's FRAME is pinned to the safe area
        // by fixed spacers, so scrolled content is clipped at the status bar
        // instead of sliding under the clock. The insets are measured by the
        // parent layout's root GeometryReader and passed in.
        ZStack {
            appState.palette.card
            VStack(spacing: 0) {
                // Reserve space for the Stage Manager window grabber (the
                // top-leading ••• control) so the sidebar's scrolled content
                // never slides under it. In full screen / on iPhone this is just
                // the regular status-bar inset.
                Color.clear.frame(height: max(safeAreaTop, 44))
                ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                VStack(alignment: .leading, spacing: 18) {
                    HStack(spacing: 8) {
                        BrandMark(size: 32)
                        Text("TagNote")
                            .font(.system(size: 24, weight: .heavy))
                            .foregroundStyle(appState.palette.text)
                    }

                    HStack(spacing: 14) {
                        Text(shortUserLabel)
                            .font(.system(size: 13, weight: .medium))
                            .lineLimit(1)
                            .foregroundStyle(appState.palette.secondaryText)
                        // Export/import live in the web app (desktop), where the
                        // file system is easy to reach; iPhone/iPad omit them.
                        SidebarIconButton(systemName: "sun.max", label: "Theme") {
                            cycleTheme()
                        }
                        Button("Logout") {
                            Task { await session.logout() }
                        }
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundStyle(appState.palette.secondaryText)
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 16)

                Divider().overlay(appState.palette.border)

                VStack(alignment: .leading, spacing: 8) {
                    Button {
                        selection = .notes
                        createNote()
                        close()
                    } label: {
                        HStack(spacing: 18) {
                            Image(systemName: "plus")
                            Text("New note")
                        }
                    }
                    .sidebarNavStyle(active: false)
                    .accessibilityIdentifier("sidebar-new-note")

                    ForEach(AppSection.allCases.filter { $0 != .settings }) { section in
                        Button {
                            selection = section
                            close()
                        } label: {
                            HStack(spacing: 18) {
                                Image(systemName: section.icon)
                                Text(section.title)
                                if section == .tags, unreviewedTagCount > 0 {
                                    Spacer()
                                    Text("\(unreviewedTagCount)")
                                        .font(.system(size: 15, weight: .bold))
                                        .monospacedDigit()
                                        .padding(.horizontal, 12)
                                        .padding(.vertical, 7)
                                        .foregroundStyle(appState.palette.card)
                                        .background(appState.palette.warning)
                                        .clipShape(Capsule())
                                        .accessibilityLabel("\(unreviewedTagCount) unreviewed tags")
                                }
                            }
                        }
                        .sidebarNavStyle(active: selection == section)
                        .accessibilityIdentifier("sidebar-\(section.rawValue)-button")
                    }

                    Button {
                        selection = .settings
                        close()
                    } label: {
                        HStack(spacing: 18) {
                            Image(systemName: AppSection.settings.icon)
                            Text(AppSection.settings.title)
                        }
                    }
                    .sidebarNavStyle(active: selection == .settings)
                    .accessibilityIdentifier("sidebar-settings-button")
                }
                .padding(18)

                SidebarDivider()

                VStack(alignment: .leading, spacing: 14) {
                    SidebarSectionTitle("Search & Filter")
                    SidebarSearchField(
                        placeholder: "Search content...",
                        text: $notesViewModel.query,
                        accessibilityID: "note-search-field"
                    )
                    .onChange(of: notesViewModel.query) { _, _ in
                        searchTask?.cancel()
                        searchTask = Task {
                            do {
                                try await Task.sleep(nanoseconds: 350_000_000)
                            } catch {
                                return
                            }
                            guard !Task.isCancelled else { return }
                            await notesViewModel.refresh()
                        }
                    }

                    TagFilterChipField(notesViewModel: notesViewModel)

                    Menu {
                        Picker("Sort", selection: $notesViewModel.sort) {
                            Text("Newest first").tag("newest")
                            Text("Recently updated").tag("updated")
                        }
                    } label: {
                        HStack {
                            Text(notesViewModel.sort == "updated" ? "Recently updated" : "Newest first")
                            Spacer()
                            Image(systemName: "chevron.down")
                        }
                        .font(.system(size: 18, weight: .medium))
                        .foregroundStyle(appState.palette.secondaryText)
                        .padding(.horizontal, 16)
                        .frame(height: 56)
                        .background(appState.palette.background)
                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border, lineWidth: 1))
                    }
                    .onChange(of: notesViewModel.sort) { _, _ in
                        Task { await notesViewModel.refresh() }
                    }
                }
                .padding(.horizontal, 20)
                .padding(.vertical, 16)

                SidebarDivider()

                VStack(alignment: .leading, spacing: 14) {
                    SidebarSectionTitle("Tags")
                    SidebarSearchField(
                        placeholder: "Search tags...",
                        text: $tagSearch,
                        accessibilityID: "sidebar-tag-search-field"
                    )
                    FlexibleChipGrid(tags: visibleTags, selectedTags: notesViewModel.selectedTags) { tag in
                        Task { await notesViewModel.toggleTagFilter(tag.name) }
                    }
                }
                .padding(.horizontal, 20)
                .padding(.vertical, 16)
            }
                }
                // Clip to the ScrollView's own frame so scrolled content is cut
                // off at the status bar / home indicator instead of drawing into
                // the safe-area spacers (the drawer ignores the safe area).
                .clipped()
                .defaultScrollAnchor(ProcessInfo.processInfo.environment["TAGNOTE_UI_SCROLL_BOTTOM"] == "1" ? .bottom : nil)
                Color.clear.frame(height: safeAreaBottom)
            }
        }
        .ignoresSafeArea()
        .task {
            await tagsViewModel.loadCached()
            await tagsViewModel.refresh()
            await notesViewModel.refreshTagFilters()
        }
    }

    private var shortUserLabel: String {
        guard let email = session.user?.email, !email.isEmpty else { return "You" }
        return "\(String(email.prefix(3)))..."
    }

    private func close() {
        guard !isPersistent else { return }
        withAnimation(.easeInOut(duration: 0.22)) {
            isOpen = false
        }
    }

    private func cycleTheme() {
        let all = Theme.allCases
        guard let index = all.firstIndex(where: { $0.rawValue == appState.settings.theme }) else {
            var settings = appState.settings
            settings.theme = Theme.everforestDark.rawValue
            Task { await appState.saveSettings(settings) }
            return
        }
        var settings = appState.settings
        settings.theme = all[(index + 1) % all.count].rawValue
        Task { await appState.saveSettings(settings) }
    }

}

private struct SidebarIconButton: View {
    @EnvironmentObject private var appState: AppState
    let systemName: String
    let label: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Image(systemName: systemName)
                .font(.system(size: 19, weight: .semibold))
                .frame(width: 28, height: 28)
        }
        .buttonStyle(.plain)
        .foregroundStyle(appState.palette.secondaryText)
        .accessibilityLabel(label)
    }
}

private struct SidebarSectionTitle: View {
    @EnvironmentObject private var appState: AppState
    let title: String

    init(_ title: String) {
        self.title = title
    }

    var body: some View {
        Text(title)
            .font(.system(size: 15, weight: .heavy))
            .textCase(.uppercase)
            .foregroundStyle(appState.palette.secondaryText)
    }
}

private struct SidebarSearchField: View {
    @EnvironmentObject private var appState: AppState
    let placeholder: String
    @Binding var text: String
    let accessibilityID: String

    var body: some View {
        TextField(placeholder, text: $text)
            .font(.system(size: 15, weight: .medium))
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .padding(.horizontal, 10)
            .frame(height: 42)
            .foregroundStyle(appState.palette.text)
            .background(appState.palette.background)
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border, lineWidth: 1))
            .accessibilityIdentifier(accessibilityID)
    }
}

private struct SidebarDivider: View {
    @EnvironmentObject private var appState: AppState

    var body: some View {
        Rectangle()
            .fill(appState.palette.border)
            .frame(height: 1)
    }
}

private struct FlexibleChipGrid: View {
    let tags: [TagInfo]
    let selectedTags: [String]
    let onTap: (TagInfo) -> Void

    var body: some View {
        FlowLayout(spacing: 6, rowSpacing: 6) {
            ForEach(tags) { tag in
                TagChip(tag.name, isActive: selectedTags.contains(tag.name), tagInfo: tag) {
                    onTap(tag)
                }
            }
        }
    }
}

// Web-parity "Filter by tags" control: active filters render as removable,
// priority-colored chips inside a bordered field with an inline text input that
// commits on space / comma / return (matches the web sidebar chip input).
private struct TagFilterChipField: View {
    @EnvironmentObject private var appState: AppState
    @ObservedObject var notesViewModel: NotesViewModel
    @State private var draft = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            FlowLayout(spacing: 6, rowSpacing: 6) {
                ForEach(notesViewModel.selectedTags, id: \.self) { tag in
                    RemovableTagChip(
                        label: tag,
                        tagInfo: notesViewModel.availableTags.first { $0.name == tag }
                    ) {
                        Task { await notesViewModel.toggleTagFilter(tag) }
                    }
                }

                TextField(notesViewModel.selectedTags.isEmpty ? "Filter by tags..." : "", text: $draft)
                    .font(.system(size: 15, weight: .medium))
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .foregroundStyle(appState.palette.text)
                    .frame(minWidth: 90, minHeight: 34, alignment: .center)
                    .onChange(of: draft) { _, value in
                        if let last = value.last, last == " " || last == "," || last == "\n" {
                            commitDraft()
                        }
                    }
                    .onSubmit(commitDraft)
                    .accessibilityIdentifier("tag-filter-input")
            }
            .padding(10)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(appState.palette.background)
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(fieldBorderColor, lineWidth: 1))
            .accessibilityIdentifier("tag-filter-summary")

            if !suggestions.isEmpty {
                TagFilterSuggestionsList(suggestions: suggestions) { tag in
                    selectSuggestion(tag)
                }
                .padding(.top, 4)
                .transition(.opacity.combined(with: .move(edge: .top)))
                .accessibilityIdentifier("tag-filter-suggestions")
            }
        }
    }

    private var normalizedDraft: String {
        draft
            .trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "#"))
            .lowercased()
    }

    private var fieldBorderColor: Color {
        suggestions.isEmpty ? appState.palette.border : appState.palette.accent
    }

    private var suggestions: [TagInfo] {
        let query = normalizedDraft
        guard !query.isEmpty else { return [] }
        let selected = Set(notesViewModel.selectedTags.map { $0.lowercased() })
        return notesViewModel.availableTags
            .filter { tag in
                !selected.contains(tag.name.lowercased())
                    && tag.name.localizedCaseInsensitiveContains(query)
            }
            .sorted { lhs, rhs in
                let lhsPrefix = lhs.name.range(of: query, options: [.caseInsensitive, .anchored]) != nil
                let rhsPrefix = rhs.name.range(of: query, options: [.caseInsensitive, .anchored]) != nil
                if lhsPrefix != rhsPrefix { return lhsPrefix && !rhsPrefix }
                if lhs.noteCount != rhs.noteCount { return lhs.noteCount > rhs.noteCount }
                return lhs.name.localizedCaseInsensitiveCompare(rhs.name) == .orderedAscending
            }
            .prefix(6)
            .map { $0 }
    }

    private func commitDraft() {
        let tag = normalizedDraft
        draft = ""
        guard !tag.isEmpty, !notesViewModel.selectedTags.contains(tag) else { return }
        Task { await notesViewModel.setTagFilters(notesViewModel.selectedTags + [tag]) }
    }

    private func selectSuggestion(_ tag: TagInfo) {
        draft = ""
        Task { await notesViewModel.setTagFilters(notesViewModel.selectedTags + [tag.name]) }
    }
}

private struct TagFilterSuggestionsList: View {
    @EnvironmentObject private var appState: AppState
    let suggestions: [TagInfo]
    let onSelect: (TagInfo) -> Void

    var body: some View {
        VStack(spacing: 0) {
            ForEach(suggestions) { tag in
                Button {
                    onSelect(tag)
                } label: {
                    HStack(spacing: 10) {
                        Circle()
                            .fill(priorityColor(for: tag))
                            .frame(width: 9, height: 9)
                        Text(tag.name)
                            .font(.system(size: 16, weight: .medium))
                            .lineLimit(1)
                            .foregroundStyle(appState.palette.text)
                        Spacer()
                        Text("I:\(tag.importance) U:\(tag.urgency)")
                            .font(.system(size: 13, weight: .semibold))
                            .monospacedDigit()
                            .foregroundStyle(appState.palette.secondaryText)
                    }
                    .padding(.horizontal, 12)
                    .frame(height: 42)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Filter by \(tag.name)")

                if tag.id != suggestions.last?.id {
                    Divider().overlay(appState.palette.border)
                }
            }
        }
        .background(appState.palette.card)
        .clipShape(RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border, lineWidth: 1))
        .shadow(color: Color.black.opacity(appState.palette.isDark ? 0.28 : 0.12), radius: 16, x: 0, y: 8)
    }

    private func priorityColor(for tag: TagInfo) -> Color {
        TagPriority.cardStyle(importance: tag.importance, urgency: tag.urgency, isDark: appState.palette.isDark)?.border
            ?? appState.palette.secondaryText
    }
}

private struct RemovableTagChip: View {
    @EnvironmentObject private var appState: AppState
    let label: String
    var tagInfo: TagInfo?
    let onRemove: () -> Void

    var body: some View {
        HStack(spacing: 5) {
            Text("#\(label)")
                .font(.system(size: 15, weight: .bold))
                .lineLimit(1)
            Button(action: onRemove) {
                Image(systemName: "xmark")
                    .font(.system(size: 10, weight: .bold))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Remove \(label) filter")
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .foregroundStyle(foregroundColor)
        .background(backgroundColor)
        .overlay(Capsule().stroke(borderColor, lineWidth: 1))
        .clipShape(Capsule())
    }

    private var pill: (background: Color, text: Color)? {
        guard let info = tagInfo else { return nil }
        return TagPriority.pillStyle(importance: info.importance, urgency: info.urgency, isActive: true, isDark: appState.palette.isDark)
    }
    private var backgroundColor: Color { pill?.background ?? appState.palette.accent }
    private var foregroundColor: Color { pill?.text ?? appState.palette.card }
    private var borderColor: Color { (pill?.text ?? appState.palette.accent).opacity(0.5) }
}

private extension View {
    func sidebarNavStyle(active: Bool) -> some View {
        modifier(SidebarNavModifier(active: active))
    }
}

private struct SidebarNavModifier: ViewModifier {
    @EnvironmentObject private var appState: AppState
    let active: Bool

    func body(content: Content) -> some View {
        content
            .font(.system(size: 19, weight: active ? .bold : .semibold))
            .foregroundStyle(active ? appState.palette.text : appState.palette.secondaryText)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 12)
            .frame(height: 46)
            .background(active ? appState.palette.tagBackground : Color.clear)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .buttonStyle(.plain)
    }
}
