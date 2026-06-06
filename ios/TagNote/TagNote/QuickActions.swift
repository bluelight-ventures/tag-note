import GoogleSignIn
import SwiftUI
import UIKit

/// Bridges Home Screen quick actions (long-press the app icon) into SwiftUI. The
/// app shell observes `newNoteRequested` and opens the composer.
final class QuickActionCenter: ObservableObject {
    static let shared = QuickActionCenter()
    static let newNoteType = "com.tag-note.tagnote.new-note"

    @Published var newNoteRequested = false

    @discardableResult
    func handle(_ shortcutItem: UIApplicationShortcutItem) -> Bool {
        guard shortcutItem.type == Self.newNoteType else { return false }
        newNoteRequested = true
        return true
    }
}

final class AppDelegate: NSObject, UIApplicationDelegate {
    func application(
        _ application: UIApplication,
        configurationForConnecting connectingSceneSession: UISceneSession,
        options: UIScene.ConnectionOptions
    ) -> UISceneConfiguration {
        // Cold launch from a quick action arrives here.
        if let shortcutItem = options.shortcutItem {
            QuickActionCenter.shared.handle(shortcutItem)
        }
        let configuration = UISceneConfiguration(name: nil, sessionRole: connectingSceneSession.role)
        configuration.delegateClass = QuickActionSceneDelegate.self
        return configuration
    }
}

/// Forwards quick actions invoked while the app is already running. Providing a
/// scene delegate can intercept URL delivery, so it also forwards open-URL
/// contexts to Google Sign-In (the OAuth callback) — `onOpenURL` does the same,
/// and GIDSignIn safely ignores a URL it has already handled, so either path
/// works. The `tagnote://` scheme just foregrounds the app, which happens
/// regardless.
final class QuickActionSceneDelegate: NSObject, UIWindowSceneDelegate {
    func windowScene(
        _ windowScene: UIWindowScene,
        performActionFor shortcutItem: UIApplicationShortcutItem,
        completionHandler: @escaping (Bool) -> Void
    ) {
        completionHandler(QuickActionCenter.shared.handle(shortcutItem))
    }

    func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
        for context in URLContexts {
            GIDSignIn.sharedInstance.handle(context.url)
        }
    }
}
