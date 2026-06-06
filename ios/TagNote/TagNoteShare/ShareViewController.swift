import SwiftUI
import UIKit

/// Principal view controller for the Share Extension. It extracts the shared
/// items off the input items, then hosts the SwiftUI compose UI.
final class ShareViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground

        let spinner = UIActivityIndicatorView(style: .large)
        spinner.translatesAutoresizingMaskIntoConstraints = false
        spinner.startAnimating()
        view.addSubview(spinner)
        NSLayoutConstraint.activate([
            spinner.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            spinner.centerYAnchor.constraint(equalTo: view.centerYAnchor)
        ])

        let items = (extensionContext?.inputItems as? [NSExtensionItem]) ?? []
        Task { @MainActor in
            let payload = await SharePayload.extract(from: items)
            spinner.removeFromSuperview()
            self.presentComposer(for: payload)
        }
    }

    private func presentComposer(for payload: SharePayload) {
        let viewModel = ShareComposeViewModel(
            payload: payload,
            onComplete: { [weak self] in self?.complete() },
            onCancel: { [weak self] in self?.cancel() },
            onOpenApp: { [weak self] in self?.openHostApp() }
        )
        let hosting = UIHostingController(rootView: ShareComposeView(viewModel: viewModel))
        addChild(hosting)
        hosting.view.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(hosting.view)
        NSLayoutConstraint.activate([
            hosting.view.topAnchor.constraint(equalTo: view.topAnchor),
            hosting.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            hosting.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            hosting.view.trailingAnchor.constraint(equalTo: view.trailingAnchor)
        ])
        hosting.didMove(toParent: self)
    }

    private func complete() {
        extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
    }

    /// Opens the TagNote app via its `tagnote://` scheme.
    ///
    /// Pitfalls handled:
    /// 1. **Lifecycle order** — we do NOT call `completeRequest` before/around the
    ///    open. Completing hands control back to the source app and cancels the
    ///    switch (this was the bug). Instead a watchdog completes *only if the
    ///    switch didn't happen*: a successful launch suspends this extension, which
    ///    pauses the timer, so it can't cancel the launch — it just prevents a hang.
    /// 2. **openURL workaround** — `UIApplication.open` is unavailable in
    ///    extensions, so we walk the responder chain to `UIApplication` and call
    ///    `openURL:` via `perform`, and also use the public `extensionContext.open`.
    /// 3. **URL scheme** — `tagnote://` is registered in the app's Info.plist
    ///    (CFBundleURLTypes) and is the exact string used here.
    private func openHostApp() {
        guard let url = URL(string: "tagnote://shared") else {
            complete()
            return
        }
        openViaResponderChain(url)
        extensionContext?.open(url, completionHandler: nil)

        // If neither path switched apps, this extension is still active and the
        // timer fires to dismiss the sheet (no hang). If the app did open, the
        // extension is suspended and the timer is paused, so it won't fire and
        // won't cancel the launch.
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.2) { [weak self] in
            self?.complete()
        }
    }

    private func openViaResponderChain(_ url: URL) {
        let selector = NSSelectorFromString("openURL:")
        var responder: UIResponder? = self
        while let current = responder {
            if let application = current as? UIApplication, application.responds(to: selector) {
                application.perform(selector, with: url)
                return
            }
            responder = current.next
        }
    }

    private func cancel() {
        extensionContext?.cancelRequest(
            withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError)
        )
    }
}
