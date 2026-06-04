import AuthenticationServices
import CryptoKit
import GoogleSignIn
import SwiftUI
import UIKit

struct AuthView: View {
    @EnvironmentObject private var appState: AppState
    @EnvironmentObject private var session: SessionStore
    @State private var mode: Mode = .login
    @State private var email = ""
    @State private var password = ""
    @State private var displayName = ""
    @State private var appleRawNonce: String?

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

                    HStack(spacing: 10) {
                        Rectangle().fill(appState.palette.border).frame(height: 1)
                        Text("or")
                            .font(.footnote)
                            .foregroundStyle(appState.palette.secondaryText)
                        Rectangle().fill(appState.palette.border).frame(height: 1)
                    }
                    .padding(.vertical, 2)

                    SignInWithAppleButton(.signIn) { request in
                        let raw = randomNonceString()
                        appleRawNonce = raw
                        request.requestedScopes = [.fullName, .email]
                        request.nonce = sha256Hex(raw)
                    } onCompletion: { result in
                        handleAppleResult(result)
                    }
                    .signInWithAppleButtonStyle(appState.palette.isDark ? .white : .black)
                    .frame(height: 48)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .accessibilityIdentifier("apple-signin-button")

                    Button(action: handleGoogleSignIn) {
                        Text("Continue with Google")
                            .font(.body.weight(.medium))
                            .frame(maxWidth: .infinity)
                            .frame(height: 48)
                    }
                    .buttonStyle(.bordered)
                    .tint(appState.palette.text)
                    .accessibilityIdentifier("google-signin-button")

                    // Custom-server switching is a Debug-only affordance; the
                    // shipped build is a tag-note.com client.
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

    private func handleAppleResult(_ result: Result<ASAuthorization, Error>) {
        switch result {
        case .success(let authorization):
            guard
                let credential = authorization.credential as? ASAuthorizationAppleIDCredential,
                let tokenData = credential.identityToken,
                let identityToken = String(data: tokenData, encoding: .utf8)
            else {
                session.errorMessage = "Apple sign-in failed."
                return
            }
            // fullName is only provided on the first authorization.
            let fullName = [credential.fullName?.givenName, credential.fullName?.familyName]
                .compactMap { $0 }
                .joined(separator: " ")
            let nonce = appleRawNonce
            Task {
                await session.loginWithApple(
                    identityToken: identityToken,
                    nonce: nonce,
                    fullName: fullName.isEmpty ? nil : fullName
                )
                if session.isAuthenticated {
                    await appState.refreshSettings()
                }
            }
        case .failure(let error):
            // Don't show an error when the user simply cancels the sheet.
            if let authError = error as? ASAuthorizationError, authError.code == .canceled {
                return
            }
            session.errorMessage = "Apple sign-in failed."
        }
    }

    private func handleGoogleSignIn() {
        guard let presenting = topViewController() else {
            session.errorMessage = "Google sign-in failed."
            return
        }
        GIDSignIn.sharedInstance.signIn(withPresenting: presenting) { result, error in
            if let error {
                // Don't show an error when the user cancels the sheet.
                if let gid = error as? GIDSignInError, gid.code == .canceled {
                    return
                }
                session.errorMessage = "Google sign-in failed."
                return
            }
            guard let idToken = result?.user.idToken?.tokenString else {
                session.errorMessage = "Google sign-in failed."
                return
            }
            Task {
                await session.loginWithGoogle(idToken: idToken)
                if session.isAuthenticated {
                    await appState.refreshSettings()
                }
            }
        }
    }
}

// Finds the frontmost view controller to present the Google sign-in sheet from.
private func topViewController() -> UIViewController? {
    let scene = UIApplication.shared.connectedScenes
        .compactMap { $0 as? UIWindowScene }
        .first { $0.activationState == .foregroundActive } ?? UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
    var top = scene?.keyWindow?.rootViewController
        ?? scene?.windows.first?.rootViewController
    while let presented = top?.presentedViewController {
        top = presented
    }
    return top
}

// Generates a cryptographically random nonce string (Apple's recommended
// pattern). The raw value goes to the backend; its SHA-256 goes to Apple.
private func randomNonceString(length: Int = 32) -> String {
    precondition(length > 0)
    var randomBytes = [UInt8](repeating: 0, count: length)
    let status = SecRandomCopyBytes(kSecRandomDefault, randomBytes.count, &randomBytes)
    if status != errSecSuccess {
        fatalError("Unable to generate nonce: SecRandomCopyBytes failed (\(status))")
    }
    let charset = Array("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-._")
    return String(randomBytes.map { charset[Int($0) % charset.count] })
}

private func sha256Hex(_ input: String) -> String {
    SHA256.hash(data: Data(input.utf8)).map { String(format: "%02x", $0) }.joined()
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
