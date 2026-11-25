import Foundation
import ImageIO
import Observation
import SwiftUI
import UIKit
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
        /// Display name; replaced by the actual upload filename once the
        /// bytes are sniffed (extension follows the real format).
        var filename: String
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
            Item(provider: provider, filename: provider.suggestedName ?? "piece-\(index + 1)")
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
                let prepared = try await Self.prepareUpload(
                    data: data,
                    suggestedName: items[index].provider.suggestedName,
                    index: index
                )
                items[index].filename = prepared.filename
                let result = try await client.uploadPiece(
                    data: prepared.data,
                    filename: prepared.filename
                )
                items[index].state = result.duplicate ? .duplicate : .added
            } catch is CancellationError {
                items[index].state = .waiting
                return
            } catch {
                // A cancelled URLSession task surfaces as a transport error,
                // not CancellationError — do not record it as a failure.
                if Task.isCancelled {
                    items[index].state = .waiting
                    return
                }
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

    // MARK: - Upload preparation

    struct PreparedUpload {
        let data: Data
        let filename: String
    }

    enum ShareUploadError: LocalizedError {
        case unsupportedFormat
        case animatedGIFUnsupported

        var errorDescription: String? {
            switch self {
            case .unsupportedFormat:
                return "Unsupported image format"
            case .animatedGIFUnsupported:
                return "Animated GIFs are not supported"
            }
        }
    }

    /// The pieces endpoint accepts JPEG, PNG, TIFF, and WebP only. Formats it
    /// rejects — HEIC (the iPhone camera default) and single-frame GIF — are
    /// re-encoded to JPEG; animated GIFs fail visibly rather than silently
    /// losing their animation. Everything accepted is sent as the original
    /// bytes with a filename extension matching the sniffed format.
    ///
    /// `nonisolated` + async: runs off the main actor, and the re-encode goes
    /// through ImageIO's thumbnail API with a bounded max pixel size — a full
    /// `UIImage(data:)` decode of a 48 MP camera photo would exceed the share
    /// extension's memory limit and get it jetsammed.
    nonisolated static func prepareUpload(
        data: Data,
        suggestedName: String?,
        index: Int
    ) async throws -> PreparedUpload {
        let format = SniffedImageFormat.sniff(data)
        let fallbackBase = "piece-\(index + 1)"
        if format.serverAccepted {
            return PreparedUpload(
                data: data,
                filename: normalizedFilename(
                    suggestedName: suggestedName,
                    fallbackBase: fallbackBase,
                    ext: format.fileExtension
                )
            )
        }
        if format == .gif, frameCount(of: data) > 1 {
            throw ShareUploadError.animatedGIFUnsupported
        }
        guard let jpeg = transcodedJPEG(from: data) else {
            throw ShareUploadError.unsupportedFormat
        }
        let base = baseName(from: suggestedName) ?? fallbackBase
        return PreparedUpload(data: jpeg, filename: base + ".jpg")
    }

    /// Memory-bounded re-encode: decodes at most `maxPixelSize` on the long
    /// edge and bakes the EXIF orientation into the pixels.
    private nonisolated static func transcodedJPEG(from data: Data) -> Data? {
        let sourceOptions = [kCGImageSourceShouldCache: false] as CFDictionary
        guard let source = CGImageSourceCreateWithData(data as CFData, sourceOptions) else {
            return nil
        }
        let thumbnailOptions = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: 4096,
        ] as CFDictionary
        guard let cgImage = CGImageSourceCreateThumbnailAtIndex(source, 0, thumbnailOptions) else {
            return nil
        }
        return UIImage(cgImage: cgImage).jpegData(compressionQuality: 0.9)
    }

    private nonisolated static func frameCount(of data: Data) -> Int {
        let sourceOptions = [kCGImageSourceShouldCache: false] as CFDictionary
        guard let source = CGImageSourceCreateWithData(data as CFData, sourceOptions) else {
            return 0
        }
        return CGImageSourceGetCount(source)
    }

    private nonisolated static func normalizedFilename(suggestedName: String?, fallbackBase: String, ext: String) -> String {
        guard let suggestedName, !suggestedName.isEmpty else {
            return fallbackBase + "." + ext
        }
        // The server derives the served Content-Type from the stored
        // extension, so a missing one must come from the sniffed format.
        return (suggestedName as NSString).pathExtension.isEmpty
            ? suggestedName + "." + ext
            : suggestedName
    }

    private nonisolated static func baseName(from suggestedName: String?) -> String? {
        guard let suggestedName, !suggestedName.isEmpty else { return nil }
        let base = (suggestedName as NSString).deletingPathExtension
        return base.isEmpty ? nil : base
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

/// Image container sniffed from magic bytes; drives the accept-or-transcode
/// decision and the filename extension.
enum SniffedImageFormat {
    case jpeg
    case png
    case gif
    case webp
    case tiff
    case heic
    case unknown

    /// Formats `POST /api/v1/pieces` accepts as-is.
    var serverAccepted: Bool {
        switch self {
        case .jpeg, .png, .webp, .tiff:
            return true
        case .gif, .heic, .unknown:
            return false
        }
    }

    var fileExtension: String {
        switch self {
        case .jpeg: return "jpg"
        case .png: return "png"
        case .gif: return "gif"
        case .webp: return "webp"
        case .tiff: return "tiff"
        case .heic: return "heic"
        case .unknown: return "bin"
        }
    }

    static func sniff(_ data: Data) -> SniffedImageFormat {
        guard data.count >= 12 else { return .unknown }
        let bytes = [UInt8](data.prefix(12))
        if bytes[0] == 0xFF, bytes[1] == 0xD8, bytes[2] == 0xFF {
            return .jpeg
        }
        if bytes[0] == 0x89, bytes[1] == 0x50, bytes[2] == 0x4E, bytes[3] == 0x47 {
            return .png
        }
        if bytes[0] == 0x47, bytes[1] == 0x49, bytes[2] == 0x46, bytes[3] == 0x38 {
            return .gif
        }
        if bytes[0] == 0x52, bytes[1] == 0x49, bytes[2] == 0x46, bytes[3] == 0x46,
           bytes[8] == 0x57, bytes[9] == 0x45, bytes[10] == 0x42, bytes[11] == 0x50 {
            return .webp
        }
        if (bytes[0] == 0x49 && bytes[1] == 0x49 && bytes[2] == 0x2A && bytes[3] == 0x00)
            || (bytes[0] == 0x4D && bytes[1] == 0x4D && bytes[2] == 0x00 && bytes[3] == 0x2A) {
            return .tiff
        }
        // ISO BMFF: "ftyp" at offset 4, HEIF-family brand at offset 8.
        if bytes[4] == 0x66, bytes[5] == 0x74, bytes[6] == 0x79, bytes[7] == 0x70 {
            let brand = String(decoding: bytes[8..<12], as: UTF8.self)
            if ["heic", "heix", "hevc", "hevx", "heif", "mif1", "msf1"].contains(brand) {
                return .heic
            }
        }
        return .unknown
    }
}
