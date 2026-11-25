import SwiftUI
import UIKit
import UniformTypeIdentifiers

/// Principal class of the OKFolioShare extension (see Share/Info.plist).
/// Thin UIKit shell: collects the shared image attachments, then hands the
/// whole flow to the SwiftUI `ShareView` via `ShareUploadModel`.
final class ShareViewController: UIViewController {
    private var model: ShareUploadModel?

    override func viewDidLoad() {
        super.viewDidLoad()
        guard model == nil else { return }

        let model = ShareUploadModel(providers: imageProviders()) { [weak self] outcome in
            self?.finish(outcome)
        }
        self.model = model

        let host = UIHostingController(rootView: ShareView(model: model))
        addChild(host)
        host.view.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(host.view)
        NSLayoutConstraint.activate([
            host.view.topAnchor.constraint(equalTo: view.topAnchor),
            host.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            host.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            host.view.trailingAnchor.constraint(equalTo: view.trailingAnchor),
        ])
        host.didMove(toParent: self)

        model.start()
    }

    /// Image attachments across all input items, in share-sheet order.
    private func imageProviders() -> [NSItemProvider] {
        let items = extensionContext?.inputItems.compactMap { $0 as? NSExtensionItem } ?? []
        return items
            .flatMap { $0.attachments ?? [] }
            .filter { $0.hasItemConformingToTypeIdentifier(UTType.image.identifier) }
    }

    // MARK: - Request completion (exactly once)

    private var finished = false

    private func finish(_ outcome: ShareOutcome) {
        guard !finished else { return }
        finished = true
        switch outcome {
        case .completed:
            extensionContext?.completeRequest(returningItems: [])
        case .cancelled:
            extensionContext?.cancelRequest(withError: CocoaError(.userCancelled))
        }
    }
}
