import SwiftUI
import UIKit

/// Principal view controller for the Share Extension. It extracts the shared
/// items, then hosts the SwiftUI compose UI. (Scaffold: a placeholder for now;
/// item extraction and the compose UI are added in later commits.)
final class ShareViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()

        let root = SharePlaceholderView(
            onCancel: { [weak self] in self?.cancel() }
        )
        let hosting = UIHostingController(rootView: root)
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

    private func cancel() {
        extensionContext?.cancelRequest(
            withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError)
        )
    }
}

private struct SharePlaceholderView: View {
    let onCancel: () -> Void

    var body: some View {
        NavigationStack {
            VStack(spacing: 12) {
                Image(systemName: "square.and.arrow.down")
                    .font(.system(size: 40, weight: .semibold))
                Text("Share to TagNote")
                    .font(.headline)
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: onCancel)
                }
            }
        }
    }
}
