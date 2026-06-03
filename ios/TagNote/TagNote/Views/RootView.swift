import SwiftUI

struct RootView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var session: SessionStore

    var body: some View {
        Group {
            if !session.isConfigured {
                ServerSetupView()
            } else if session.isAuthenticated && shouldOpenEditorForUITest {
                EditorView(viewModel: EditorViewModel(note: nil, api: session.api))
                    .environmentObject(appState)
            } else if session.isAuthenticated {
                MainTabView()
            } else if session.isLoading {
                ProgressView("Loading TagNote")
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .background(appState.palette.background)
            } else {
                AuthView()
            }
        }
        .tint(appState.palette.accent)
    }

    private var shouldOpenEditorForUITest: Bool {
        ProcessInfo.processInfo.environment["TAGNOTE_UI_CREATE_NOTE"] == "1" ||
            ProcessInfo.processInfo.arguments.contains("-ui-create-note")
    }
}

struct ServerSetupView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var session: SessionStore
    // Pre-fill the default hosted server for real users, but start empty under UI
    // testing so the E2E suite types its own server URL into a clean field
    // (otherwise a residual pre-fill can point the app at production).
    @State private var serverURL = ProcessInfo.processInfo.arguments.contains("-ui-testing") ? "" : SessionStore.defaultServerURL
    @State private var errorMessage: String?

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 18) {
                Spacer()

                BrandMark(size: 54)

                VStack(alignment: .leading, spacing: 6) {
                    Text("TagNote")
                        .font(.largeTitle.weight(.semibold))
                        .foregroundStyle(appState.palette.text)
                    Text("Connect to your TagNote server.")
                        .foregroundStyle(appState.palette.secondaryText)
                }

                VStack(alignment: .leading, spacing: 8) {
                    Text("Server URL")
                        .font(.subheadline.weight(.medium))
                        .foregroundStyle(appState.palette.text)
                    TextField("https://notes.example.com", text: $serverURL)
                        .keyboardType(.URL)
                        .textContentType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .padding(12)
                        .background(appState.palette.card)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border))
                        .accessibilityIdentifier("server-url-field")
                }

                if let errorMessage {
                    Text(errorMessage)
                        .font(.footnote)
                        .foregroundStyle(appState.palette.destructive)
                }

                Button {
                    do {
                        try session.saveServerURL(serverURL)
                    } catch {
                        errorMessage = error.localizedDescription
                    }
                } label: {
                    Label("Continue", systemImage: "arrow.right")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
                .controlSize(.large)
                .accessibilityIdentifier("server-continue-button")

                Spacer()
            }
            .padding(24)
            .background(appState.palette.background.ignoresSafeArea())
        }
    }
}

struct BrandMark: View {
    @EnvironmentObject private var appState: AppState
    let size: CGFloat

    var body: some View {
        ZStack {
            RoundedRectangle(cornerRadius: size * 0.18)
                .fill(appState.palette.accent)
            Image(systemName: "tag")
                .font(.system(size: size * 0.52, weight: .semibold))
                .foregroundStyle(appState.palette.card)
        }
        .frame(width: size, height: size)
    }
}
