import Foundation
import Observation
import SwiftUI
import UniformTypeIdentifiers
import FolioKit

/// How the extension request should be closed. The hosting
/// `ShareViewController` maps this onto `completeRequest` / `cancelRequest`
/// and guarantees exactly-once semantics.
enum ShareOutcome {
    case completed
    case cancelled
}

/// State machine for the share flow: loads each attachment's original bytes
/// and uploads them sequentially through `FolioClient.uploadPiece`.
@MainActor
@Observable
final class ShareUploadModel {
    enum ItemState: Equatable {
        case waiting
        case uploading
        case added
        case duplicate
        case error(String)
    }

    struct Item: Identifiable {
        let id = UUID()
        let provider: NSItemProvider
        let filename: String
        var state: ItemState = .waiting
    }

    private(set) var items: [Item]
    private(set) var isRunning = false
    /// True once the run ended with at least one failed item; the UI then
    /// shows Done instead of auto-dismissing.
    private(set) var finishedWithErrors = false

    /// Nil when the group defaults hold no valid server URL.
    let client: FolioClient?

    private let onFinish: (ShareOutcome) -> Void
    private var uploadTask: Task<Void, Never>?

    init(providers: [NSItemProvider], onFinish: @escaping (ShareOutcome) -> Void) {
        self.onFinish = onFinish
        items = providers.enumerated().map { index, provider in
            Item(provider: provider, filename: provider.suggestedName ?? "piece-\(index + 1).jpg")
        }
        client = Self.makeClient()
    }

    var finishedCount: Int {
        items.filter { $0.state == .added || $0.state == .duplicate }.count
    }

    // MARK: - Flow

    func start() {
        guard let client, !items.isEmpty, uploadTask == nil else { return }
        isRunning = true
        uploadTask = Task { await self.run(client) }
    }

    /// Cancels remaining uploads and asks the host to cancel the request.
    func cancel() {
        uploadTask?.cancel()
        isRunning = false
        onFinish(.cancelled)
    }

    /// Shown after failures so the user can read the errors first.
    func done() {
        onFinish(.completed)
    }

    private func run(_ client: FolioClient) async {
        for index in items.indices {
            if Task.isCancelled { return }
            items[index].state = .uploading
            do {
                let data = try await Self.loadData(from: items[index].provider)
                let result = try await client.uploadPiece(
                    data: data,
                    filename: items[index].filename
                )
                items[index].state = result.duplicate ? .duplicate : .added
            } catch is CancellationError {
                items[index].state = .waiting
                return
            } catch {
                items[index].state = .error(error.localizedDescription)
            }
        }
        if Task.isCancelled { return }
        isRunning = false

        let anyFailed = items.contains { item in
            if case .error = item.state { return true }
            return false
        }
        if anyFailed {
            finishedWithErrors = true
        } else {
            // Let the user see the final checkmarks before dismissing.
            try? await Task.sleep(nanoseconds: 800_000_000)
            if Task.isCancelled { return }
            onFinish(.completed)
        }
    }

    /// Original bytes of the attachment (no transcoding, keeps metadata).
    private static func loadData(from provider: NSItemProvider) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            _ = provider.loadDataRepresentation(forTypeIdentifier: UTType.image.identifier) { data, error in
                if let data {
                    continuation.resume(returning: data)
                } else {
                    continuation.resume(throwing: error ?? CocoaError(.fileReadUnknown))
                }
            }
        }
    }

    // MARK: - Server URL from the shared App Group

    private static func makeClient() -> FolioClient? {
        let stored = UserDefaults(suiteName: AppGroup.identifier)?
            .string(forKey: "serverURLString") ?? ""
        guard let url = validatedBaseURL(from: stored) else { return nil }
        return FolioClient(baseURL: url)
    }

    /// Accepts only absolute http(s) URLs with a host.
    /// Mirrors `AppModel.validatedBaseURL` in the app target.
    private static func validatedBaseURL(from string: String) -> URL? {
        let trimmed = string.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty,
              let url = URL(string: trimmed),
              let scheme = url.scheme?.lowercased(),
              scheme == "http" || scheme == "https",
              let host = url.host(), !host.isEmpty
        else {
            return nil
        }
        return url
    }
}

/// Minimal share UI: a row per image with its upload state, an overall
/// progress bar, and Cancel / Done in the navigation bar.
struct ShareView: View {
    let model: ShareUploadModel

    var body: some View {
        NavigationStack {
            content
                .navigationTitle("Add to Folio")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    if model.finishedWithErrors {
                        ToolbarItem(placement: .confirmationAction) {
                            Button("Done") {
                                model.done()
                            }
                        }
                    } else {
                        ToolbarItem(placement: .cancellationAction) {
                            Button("Cancel") {
                                model.cancel()
                            }
                        }
                    }
                }
        }
    }

    @ViewBuilder
    private var content: some View {
        if model.client == nil {
            ContentUnavailableView(
                "No Server Configured",
                systemImage: "externaldrive.badge.questionmark",
                description: Text("Set the server address in OK Folio first")
            )
        } else if model.items.isEmpty {
            ContentUnavailableView(
                "No Images",
                systemImage: "photo",
                description: Text("Nothing shareable was found in the selection.")
            )
        } else {
            List {
                if model.items.count > 1 {
                    Section {
                        ProgressView(
                            value: Double(model.finishedCount),
                            total: Double(model.items.count)
                        )
                    }
                }
                Section {
                    ForEach(model.items) { item in
                        row(for: item)
                    }
                }
            }
        }
    }

    private func row(for item: ShareUploadModel.Item) -> some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text(item.filename)
                    .lineLimit(1)
                    .truncationMode(.middle)
                if case .error(let message) = item.state {
                    Text(message)
                        .font(.footnote)
                        .foregroundStyle(.red)
                }
            }
            Spacer()
            stateBadge(for: item.state)
        }
    }

    @ViewBuilder
    private func stateBadge(for state: ShareUploadModel.ItemState) -> some View {
        switch state {
        case .waiting:
            Image(systemName: "circle.dotted")
                .foregroundStyle(.secondary)
        case .uploading:
            ProgressView()
        case .added:
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(.green)
        case .duplicate:
            Text("Already in Folio")
                .font(.footnote)
                .foregroundStyle(.secondary)
        case .error:
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.red)
        }
    }
}
