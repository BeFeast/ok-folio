import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

/// Typed client for the OK Folio Go API (`/api/v1`).
///
/// All endpoints and wire shapes are derived from the Go handlers in
/// `internal/api` (api.go route table, gallery.go, images.go, favorites.go,
/// folios.go). Pagination is the Go `limit`/`offset` pair; the `page`/`perPage`
/// arguments here are translated as `limit = perPage`,
/// `offset = (page - 1) * perPage` (page is 1-based).
///
/// Marked `@unchecked Sendable` because `URLSession` lacks a `Sendable`
/// annotation in swift-corelibs-foundation; both stored properties are
/// immutable and URLSession is documented thread-safe.
public struct FolioClient: @unchecked Sendable {
    public let baseURL: URL
    private let session: URLSession

    public init(baseURL: URL, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    // MARK: - Catalog

    /// `GET /api/v1/gallery/catalog?limit=&offset=[&q=][&favorite=true][&artist=]`
    ///
    /// - `page` is 1-based; the server speaks `limit`/`offset`.
    /// - `query` maps to the `q` parameter (NOT `query` — the `query` key in
    ///   the response is just an echo of the filter).
    /// - `favoritesOnly: true` adds `favorite=true`; false omits the parameter
    ///   entirely (sending `favorite=false` would filter to non-favorites).
    public func catalog(
        page: Int,
        perPage: Int,
        query: String?,
        favoritesOnly: Bool,
        artist: String?
    ) async throws -> CatalogPage {
        let limit = max(1, perPage)
        let offset = (max(1, page) - 1) * limit
        var items = [
            URLQueryItem(name: "limit", value: String(limit)),
            URLQueryItem(name: "offset", value: String(offset)),
        ]
        if let query, !query.isEmpty {
            items.append(URLQueryItem(name: "q", value: query))
        }
        if favoritesOnly {
            items.append(URLQueryItem(name: "favorite", value: "true"))
        }
        if let artist, !artist.isEmpty {
            items.append(URLQueryItem(name: "artist", value: artist))
        }
        let request = request(path: "api/v1/gallery/catalog", queryItems: items)
        return try await send(request, as: CatalogPage.self, path: "/api/v1/gallery/catalog")
    }

    // MARK: - Photos

    /// `GET /api/v1/photos/{id}` — snake_case detail payload (no width/height
    /// on the wire; see `Piece`).
    public func photo(id: Int) async throws -> Piece {
        let request = request(path: "api/v1/photos/\(id)")
        return try await send(request, as: Piece.self, path: "/api/v1/photos/\(id)")
    }

    /// `GET /api/v1/photos/{id}/thumbnail` (server accepts optional `?w=`).
    public func thumbnailURL(id: Int) -> URL {
        endpoint(path: "api/v1/photos/\(id)/thumbnail")
    }

    /// `GET /api/v1/photos/{id}/image` — full-size original.
    public func imageURL(id: Int) -> URL {
        endpoint(path: "api/v1/photos/\(id)/image")
    }

    /// `POST /api/v1/photos/{id}/favorite` (add) or `DELETE` (remove).
    /// Response: `{"id": n, "favorite": bool, "available": true}`.
    /// Returns the server-confirmed favorite state.
    @discardableResult
    public func setFavorite(id: Int, isFavorite: Bool) async throws -> Bool {
        var request = request(path: "api/v1/photos/\(id)/favorite")
        request.httpMethod = isFavorite ? "POST" : "DELETE"
        let envelope = try await send(
            request,
            as: FavoriteEnvelope.self,
            path: "/api/v1/photos/\(id)/favorite"
        )
        return envelope.favorite
    }

    // MARK: - Folios

    /// `GET /api/v1/folios` — `{"folios": [...]}` envelope.
    public func folios() async throws -> [Folio] {
        let request = request(path: "api/v1/folios")
        let envelope = try await send(request, as: FoliosEnvelope.self, path: "/api/v1/folios")
        return envelope.folios
    }

    /// `POST /api/v1/folios/{folioID}/pieces` with body `{"photo_id": n}`.
    /// `201 {"added": true}` on success; `200 {"added": false, "duplicate": true}`
    /// when already present.
    @discardableResult
    public func addPiece(toFolio folioID: Int, photoID: Int) async throws -> AddPieceResult {
        var request = request(path: "api/v1/folios/\(folioID)/pieces")
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["photo_id": photoID])
        return try await send(
            request,
            as: AddPieceResult.self,
            path: "/api/v1/folios/\(folioID)/pieces"
        )
    }

    /// `GET /api/v1/photos/{photoID}/folios` — folios containing the photo,
    /// same `{"folios": [...]}` envelope as the folio list.
    public func foliosContaining(photoID: Int) async throws -> [Folio] {
        let request = request(path: "api/v1/photos/\(photoID)/folios")
        let envelope = try await send(
            request,
            as: FoliosEnvelope.self,
            path: "/api/v1/photos/\(photoID)/folios"
        )
        return envelope.folios
    }

    // MARK: - URL building

    private func endpoint(path: String, queryItems: [URLQueryItem] = []) -> URL {
        guard var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false) else {
            preconditionFailure("FolioClient baseURL is not a valid URL: \(baseURL)")
        }
        var basePath = components.path
        if basePath.hasSuffix("/") {
            basePath.removeLast()
        }
        components.path = basePath + "/" + path
        if !queryItems.isEmpty {
            components.queryItems = queryItems
        }
        guard let url = components.url else {
            preconditionFailure("FolioClient failed to build URL for path: \(path)")
        }
        return url
    }

    private func request(path: String, queryItems: [URLQueryItem] = []) -> URLRequest {
        URLRequest(url: endpoint(path: path, queryItems: queryItems))
    }

    // MARK: - Transport

    private func send<T: Decodable>(
        _ request: URLRequest,
        as type: T.Type,
        path: String
    ) async throws -> T {
        let (data, response) = try await perform(request)
        guard (200..<300).contains(response.statusCode) else {
            throw FolioError.httpStatus(
                response.statusCode,
                body: String(decoding: data, as: UTF8.self)
            )
        }
        do {
            return try Self.makeDecoder().decode(T.self, from: data)
        } catch {
            throw FolioError.decoding(error, path: path)
        }
    }

    /// Completion-handler bridge instead of `URLSession.data(for:)` so the
    /// same code path works on swift-corelibs-foundation (Linux CI).
    private func perform(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        try await withCheckedThrowingContinuation { continuation in
            let task = session.dataTask(with: request) { data, response, error in
                if let error {
                    continuation.resume(throwing: FolioError.transport(error))
                    return
                }
                guard let http = response as? HTTPURLResponse else {
                    continuation.resume(throwing: FolioError.transport(URLError(.badServerResponse)))
                    return
                }
                continuation.resume(returning: (data ?? Data(), http))
            }
            task.resume()
        }
    }

    // MARK: - Decoding

    /// Go's `time.Time` marshals as RFC3339Nano — fractional seconds are
    /// present only when non-zero, so both variants must parse.
    /// A fresh decoder per call: JSONDecoder is not documented thread-safe,
    /// and responses decode concurrently on URLSession callback queues.
    static func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let raw = try container.decode(String.self)
            if let date = parseRFC3339(raw) {
                return date
            }
            throw DecodingError.dataCorruptedError(
                in: container,
                debugDescription: "Unrecognized RFC3339 date: \(raw)"
            )
        }
        return decoder
    }

    private static let isoFractional: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()

    private static let isoPlain: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter
    }()

    /// ISO8601DateFormatter is not documented thread-safe; concurrent
    /// responses decode in parallel, so serialize formatter access.
    private static let dateParseLock = NSLock()

    private static func parseRFC3339(_ value: String) -> Date? {
        dateParseLock.lock()
        defer { dateParseLock.unlock() }
        return isoFractional.date(from: value) ?? isoPlain.date(from: value)
    }
}

// MARK: - Upload

extension FolioClient {
    /// Uploads an image to `POST /api/v1/pieces` as multipart/form-data.
    ///
    /// The body carries a required `file` part (content type sniffed from the
    /// image's magic bytes) plus optional `title`/`artist` text parts, sent
    /// only when non-nil and non-empty. The server answers
    /// `201 {"photo": ..., "duplicate": false}` for a fresh import and
    /// `200 {"photo": ..., "duplicate": true}` when identical content was
    /// already imported; both decode to `UploadResult`.
    public func uploadPiece(
        data: Data,
        filename: String,
        title: String? = nil,
        artist: String? = nil
    ) async throws -> UploadResult {
        let boundary = "FolioKit-" + UUID().uuidString
        var request = request(path: "api/v1/pieces")
        request.httpMethod = "POST"
        request.setValue(
            "multipart/form-data; boundary=\(boundary)",
            forHTTPHeaderField: "Content-Type"
        )
        request.httpBody = Self.multipartBody(
            boundary: boundary,
            fileData: data,
            filename: filename,
            fields: [("title", title), ("artist", artist)]
        )
        let envelope = try await send(request, as: UploadEnvelope.self, path: "/api/v1/pieces")
        return UploadResult(piece: envelope.photo, duplicate: envelope.duplicate)
    }

    /// Hand-built multipart/form-data body: text fields first, the file part
    /// last, CRLF line endings throughout, nothing percent-encoded (the file
    /// part is raw bytes).
    private static func multipartBody(
        boundary: String,
        fileData: Data,
        filename: String,
        fields: [(name: String, value: String?)]
    ) -> Data {
        var body = Data()
        func append(_ string: String) {
            body.append(Data(string.utf8))
        }
        for (name, value) in fields {
            guard let value, !value.isEmpty else { continue }
            append("--\(boundary)\r\n")
            append("Content-Disposition: form-data; name=\"\(name)\"\r\n\r\n")
            append(value)
            append("\r\n")
        }
        append("--\(boundary)\r\n")
        append("Content-Disposition: form-data; name=\"file\"; filename=\"\(filename)\"\r\n")
        append("Content-Type: \(sniffImageContentType(fileData))\r\n\r\n")
        body.append(fileData)
        append("\r\n--\(boundary)--\r\n")
        return body
    }

    /// Best-effort image content type from magic bytes. The server re-detects
    /// the real format from the bytes anyway, so unknown data just falls back
    /// to application/octet-stream.
    static func sniffImageContentType(_ data: Data) -> String {
        if data.starts(with: [0xFF, 0xD8, 0xFF]) {
            return "image/jpeg"
        }
        if data.starts(with: [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]) {
            return "image/png"
        }
        if data.starts(with: Array("GIF8".utf8)) {
            return "image/gif"
        }
        if data.count >= 12,
           data.starts(with: Array("RIFF".utf8)),
           data.dropFirst(8).prefix(4).elementsEqual(Array("WEBP".utf8)) {
            return "image/webp"
        }
        return "application/octet-stream"
    }
}
