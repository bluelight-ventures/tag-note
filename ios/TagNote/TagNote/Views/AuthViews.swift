import SwiftUI

struct AuthView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var session: SessionStore
    @State private var mode: Mode = .login
    @State private var email = ""
    @State private var password = ""
    @State private var displayName = ""

    enum Mode: String, CaseIterable, Identifiable {
        case login
        case register

        var id: String { rawValue }
        var label: String { self == .login ? "Login" : "Create Account" }
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 18) {
                    BrandMark(size: 48)
                        .padding(.top, 28)

                    VStack(spacing: 4) {
                        Text("TagNote")
                            .font(.title.weight(.semibold))
                            .foregroundStyle(appState.palette.text)
                        Text("Tag your thinking. Find it instantly.")
                            .font(.subheadline)
                            .foregroundStyle(appState.palette.secondaryText)
                    }

                    Picker("Mode", selection: $mode) {
                        ForEach(Mode.allCases) { mode in
                            Text(mode.label).tag(mode)
                        }
                    }
                    .pickerStyle(.segmented)

                    VStack(spacing: 12) {
                        TextField("Email", text: $email)
                            .keyboardType(.emailAddress)
                            .textContentType(.emailAddress)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .tagNoteField()
                            .accessibilityIdentifier("login-email-field")

                        if mode == .register {
                            TextField("Display name", text: $displayName)
                                .textContentType(.name)
                                .tagNoteField()
                        }

                        SecureField("Password", text: $password)
                            .textContentType(mode == .login ? .password : .newPassword)
                            .tagNoteField()
                            .accessibilityIdentifier("login-password-field")
                    }

                    if let message = session.errorMessage {
                        Text(message)
                            .font(.footnote)
                            .foregroundStyle(message.hasPrefix("Check") ? appState.palette.accent : appState.palette.destructive)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }

                    Button {
                        Task {
                            if mode == .login {
                                await session.login(email: email, password: password)
                            } else {
                                await session.register(email: email, password: password, displayName: displayName)
                            }
                            if session.isAuthenticated {
                                await appState.refreshSettings()
                            }
                        }
                    } label: {
                        if session.isLoading {
                            ProgressView()
                                .frame(maxWidth: .infinity)
                        } else {
                            Label(mode == .login ? "Login" : "Create Account", systemImage: mode == .login ? "arrow.right.circle" : "person.badge.plus")
                                .frame(maxWidth: .infinity)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                    .disabled(session.isLoading)
                    .accessibilityIdentifier("login-submit-button")

                    HStack {
                        Button("Login without password") {
                            Task { await session.requestMagicLink(email: email) }
                        }
                        Spacer()
                        Button("Forgot password?") {
                            Task { await session.forgotPassword(email: email) }
                        }
                    }
                    .font(.footnote)

                    // Self-hosting / custom-server switching is a Debug-only
                    // affordance; the shipped build is a tag-note.com client.
                    if SessionStore.allowsCustomServer {
                        Button(role: .destructive) {
                            session.resetServer()
                        } label: {
                            Text("Change server")
                        }
                        .font(.footnote)
                        .padding(.top, 8)
                    }
                }
                .padding(20)
            }
            .background(appState.palette.background.ignoresSafeArea())
        }
    }
}

private struct TagNoteTextField: ViewModifier {
    @EnvironmentObject private var appState: AppState

    func body(content: Content) -> some View {
        content
            .padding(12)
            .background(appState.palette.card)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(appState.palette.border))
    }
}

extension View {
    func tagNoteField() -> some View {
        modifier(TagNoteTextField())
    }
}
