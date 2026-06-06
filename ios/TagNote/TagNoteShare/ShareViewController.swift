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

    /// Opens the TagNote app via its URL scheme. `UIApplication.open` is
    /// unavailable in app extensions, so walk the responder chain to whoever
    /// implements `openURL:` (the shared UIApplication).
    private func openHostApp() {
        guard let url = URL(string: "tagnote://shared") else { return }
        let selector = NSSelectorFromString("openURL:")
        var responder: UIResponder? = self
        while let current = responder {
            if current.responds(to: selector) {
                current.perform(selector, with: url)
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
