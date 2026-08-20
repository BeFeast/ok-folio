import Foundation
import Observation
import SwiftUI
import FolioKit

/// App-wide state: persisted settings, the API client, and the shared
/// image-loading session with its disk cache.
@Observable
final class AppModel {
    private enum Keys {
        static let serverURL = "serverURLString"
        static let faceIDLock = "faceIDLockEnabled"
    }

    var serverURLString: String {
        didSet {
            UserDefaults.standard.set(serverURLString, forKey: Keys.serverURL)
            client = Self.makeClient(from: serverURLString)
        }
    }

    var faceIDLockEnabled: Bool {
        didSet {
            UserDefaults.standard.set(faceIDLockEnabled, forKey: Keys.faceIDLock)
        }
    }

    /// Gates the UI behind `AppLockView` when true.
    var isLocked: Bool

    /// Rebuilt whenever `serverURLString` changes; nil until a valid URL is set.
    private(set) var client: FolioClient?

    /// Session used for thumbnails and full images. Backed by a large URLCache
    /// so images persist on disk across launches (AsyncImage would bypass it).
    let imageSession: URLSession

    init() {
        let defaults = UserDefaults.standard
        let storedURL = defaults.string(forKey: Keys.serverURL) ?? ""
        let lockEnabled = defaults.bool(forKey: Keys.faceIDLock)
        serverURLString = storedURL
        faceIDLockEnabled = lockEnabled
        isLocked = lockEnabled

        let cacheDirectory = FileManager.default
            .urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("FolioImageCache", isDirectory: true)
        let cache = URLCache(
            memoryCapacity: 64 * 1024 * 1024,
            diskCapacity: 1024 * 1024 * 1024,
            directory: cacheDirectory
        )
        let configuration = URLSessionConfiguration.default
        configuration.urlCache = cache
        configuration.requestCachePolicy = .returnCacheDataElseLoad
        imageSession = URLSession(configuration: configuration)

        client = Self.makeClient(from: storedURL)
    }

    func handleScenePhase(_ phase: ScenePhase) {
        guard faceIDLockEnabled else { return }
        if phase == .background {
            isLocked = true
        }
    }

    // MARK: - URL validation

    static func makeClient(from string: String) -> FolioClient? {
        guard let url = validatedBaseURL(from: string) else { return nil }
        return FolioClient(baseURL: url)
    }

    /// Accepts only absolute http(s) URLs with a host.
    static func validatedBaseURL(from string: String) -> URL? {
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
