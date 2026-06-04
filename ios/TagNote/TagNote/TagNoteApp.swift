import GoogleSignIn
import SwiftUI

@main
struct TagNoteApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(appState)
                .environmentObject(appState.session)
                .task {
                    await appState.loadCachedSettings()
                    await appState.session.bootstrap()
                    if appState.session.isAuthenticated {
                        await appState.refreshSettings()
                    }
                }
                .onOpenURL { url in
                    GIDSignIn.sharedInstance.handle(url)
                }
        }
    }
}
