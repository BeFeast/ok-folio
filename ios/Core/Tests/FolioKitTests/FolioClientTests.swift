import XCTest
@testable import FolioKit
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

final class FolioClientTests: XCTestCase {
    private var client: FolioClient!

    override func setUp() {
        super.setUp()
        StubURLProtocol.reset()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        client = FolioClient(
            baseURL: URL(string: "https://folio.test")!,
            session: URLSession(configuration: configuration)
        )
    }

    override func tearDown() {
        StubURLProtocol.reset()
        client = nil
        super.tearDown()
    }

    // MARK: - Helpers

    private func fixture(_ name: String) throws -> Data {
        let url = try XCTUnwrap(
            Bundle.module.url(forResource: name, withExtension: "json", subdirectory: "Fixtures"),
            "Missing fixture \(name).json"
        )
        return try Data(contentsOf: url)
    }

    private func stub(status: Int = 200, data: Data = Data()) {
        StubURLProtocol.handler = { _ in
            StubURLProtocol.StubResponse(status: status, data: data)
        }
    }

    private var lastRequest: URLRequest {
        get throws {
            try XCTUnwrap(StubURLProtocol.recorded.last, "No request was recorded")
        }
    }

    // MARK: - Catalog decoding

    func testCatalogDecodesItemsTotalAndFacets() async throws {
        stub(data: try fixture("catalog"))

        let page = try await client.catalog(
            page: 1, perPage: 20, query: nil, favoritesOnly: false, artist: nil
        )

        XCTAssertEqual(page.total, 42)
        XCTAssertEqual(page.items.count, 2)

        let first = page.items[0]
        XCTAssertEqual(first.id, 1)
        XCTAssertEqual(first.title, "Piece one")
        XCTAssertEqual(first.artist, "artist-a")
        XCTAssertTrue(first.favorite)
        XCTAssertEqual(first.width, 800)
        XCTAssertEqual(first.height, 600)
        XCTAssertEqual(first.fileName, "piece-0001.jpg")
        XCTAssertEqual(first.category, "paintings")
        XCTAssertEqual(first.status, "downloaded")
        // DownloadedAt with fractional seconds (RFC3339Nano).
        let createdAt = try XCTUnwrap(first.createdAt)
        XCTAssertEqual(createdAt.timeIntervalSince1970, 1767323045.123456, accuracy: 0.001)

        let second = page.items[1]
        XCTAssertEqual(second.id, 2)
        XCTAssertFalse(second.favorite)
        // DownloadedAt without fractional seconds must also parse.
        XCTAssertNotNil(second.createdAt)

        let facets = try XCTUnwrap(page.facets)
        XCTAssertEqual(facets.artists.map(\.id), ["artist-a", "artist-b"])
        XCTAssertEqual(facets.artists[0].displayName, "artist-a")
        XCTAssertEqual(facets.artists[0].count, 30)
        XCTAssertEqual(facets.categories.map(\.id), ["paintings", "sculpture"])
        XCTAssertEqual(facets.sources.count, 2)
        XCTAssertEqual(facets.favorites.count, 2)
        XCTAssertTrue(facets.favorites[0].favorite)
        XCTAssertEqual(facets.favorites[0].count, 5)
    }

    func testCatalogWithoutFacetsStillDecodes() async throws {
        let body = #"{"photos": [], "total": 0}"#
        stub(data: Data(body.utf8))

        let page = try await client.catalog(
            page: 1, perPage: 20, query: nil, favoritesOnly: false, artist: nil
        )

        XCTAssertEqual(page.items.count, 0)
        XCTAssertEqual(page.total, 0)
        XCTAssertNil(page.facets)
    }

    // MARK: - Catalog URL building

    func testCatalogDefaultQueryParams() async throws {
        stub(data: try fixture("catalog"))

        _ = try await client.catalog(
            page: 1, perPage: 20, query: nil, favoritesOnly: false, artist: nil
        )

        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/gallery/catalog?limit=20&offset=0"
        )
        XCTAssertEqual(try lastRequest.httpMethod, "GET")
    }

    func testCatalogPagingTranslatesPageToOffset() async throws {
        stub(data: try fixture("catalog"))

        _ = try await client.catalog(
            page: 3, perPage: 25, query: nil, favoritesOnly: false, artist: nil
        )

        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/gallery/catalog?limit=25&offset=50"
        )
    }

    func testCatalogAllFiltersBuildExactURL() async throws {
        stub(data: try fixture("catalog"))

        _ = try await client.catalog(
            page: 2, perPage: 10, query: "blue nude", favoritesOnly: true, artist: "artist-a"
        )

        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/gallery/catalog?limit=10&offset=10&q=blue%20nude&favorite=true&artist=artist-a"
        )
    }

    func testCatalogFavoritesOnlyFalseOmitsFavoriteParam() async throws {
        stub(data: try fixture("catalog"))

        _ = try await client.catalog(
            page: 1, perPage: 20, query: "x", favoritesOnly: false, artist: nil
        )

        let url = try XCTUnwrap(try lastRequest.url?.absoluteString)
        XCTAssertFalse(url.contains("favorite"), "favorite=false must never be sent: \(url)")
        XCTAssertTrue(url.contains("q=x"))
    }

    func testCatalogPercentEncodesReservedCharacters() async throws {
        stub(data: try fixture("catalog"))

        _ = try await client.catalog(
            page: 1, perPage: 20, query: "a&b=c", favoritesOnly: false, artist: "artist a"
        )

        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/gallery/catalog?limit=20&offset=0&q=a%26b%3Dc&artist=artist%20a"
        )
    }

    // MARK: - Photo detail

    func testPhotoDetailDecodesSnakeCasePayload() async throws {
        stub(data: try fixture("photo-detail"))

        let piece = try await client.photo(id: 1)

        XCTAssertEqual(try lastRequest.url?.absoluteString, "https://folio.test/api/v1/photos/1")
        XCTAssertEqual(piece.id, 1)
        XCTAssertEqual(piece.title, "Piece one")
        XCTAssertEqual(piece.artist, "artist-a")
        XCTAssertTrue(piece.favorite)
        XCTAssertEqual(piece.fileName, "piece-0001.jpg")
        XCTAssertEqual(piece.status, "downloaded")
        XCTAssertNotNil(piece.createdAt)
        // The detail endpoint has no width/height on the wire.
        XCTAssertNil(piece.width)
        XCTAssertNil(piece.height)
    }

    // MARK: - Image URLs

    func testThumbnailURL() {
        XCTAssertEqual(
            client.thumbnailURL(id: 42).absoluteString,
            "https://folio.test/api/v1/photos/42/thumbnail"
        )
    }

    func testImageURL() {
        XCTAssertEqual(
            client.imageURL(id: 42).absoluteString,
            "https://folio.test/api/v1/photos/42/image"
        )
    }

    // MARK: - Favorites

    func testSetFavoritePostsAndReturnsServerState() async throws {
        let body = #"{"id": 7, "favorite": true, "available": true}"#
        stub(data: Data(body.utf8))

        let result = try await client.setFavorite(id: 7, isFavorite: true)

        XCTAssertTrue(result)
        XCTAssertEqual(try lastRequest.httpMethod, "POST")
        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/photos/7/favorite"
        )
    }

    func testRemoveFavoriteUsesDeleteAndReturnsServerState() async throws {
        let body = #"{"id": 7, "favorite": false, "available": true}"#
        stub(data: Data(body.utf8))

        let result = try await client.setFavorite(id: 7, isFavorite: false)

        XCTAssertFalse(result)
        XCTAssertEqual(try lastRequest.httpMethod, "DELETE")
        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/photos/7/favorite"
        )
    }

    // MARK: - Folios

    func testFoliosListDecodes() async throws {
        stub(data: try fixture("folios"))

        let folios = try await client.folios()

        XCTAssertEqual(try lastRequest.url?.absoluteString, "https://folio.test/api/v1/folios")
        XCTAssertEqual(folios.count, 2)
        XCTAssertEqual(folios[0].id, 1)
        XCTAssertEqual(folios[0].name, "Folio one")
        XCTAssertEqual(folios[0].pieceCount, 3)
        XCTAssertEqual(folios[0].coverPhotoID, 7)
        XCTAssertNotNil(folios[0].createdAt)
        XCTAssertEqual(folios[1].pieceCount, 0)
        XCTAssertNil(folios[1].coverPhotoID, "cover_photo_id: null must decode as nil")
    }

    func testFoliosContainingPhotoUsesPhotoFoliosPath() async throws {
        stub(data: try fixture("folios"))

        let folios = try await client.foliosContaining(photoID: 9)

        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/photos/9/folios"
        )
        XCTAssertEqual(folios.count, 2)
    }

    // MARK: - Add piece

    func testAddPieceSuccessPostsPhotoIDBody() async throws {
        stub(status: 201, data: Data(#"{"added": true}"#.utf8))

        let result = try await client.addPiece(toFolio: 3, photoID: 11)

        XCTAssertTrue(result.added)
        XCTAssertFalse(result.duplicate)
        XCTAssertEqual(try lastRequest.httpMethod, "POST")
        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/folios/3/pieces"
        )
        let sentBody = try XCTUnwrap(StubURLProtocol.recordedBodies.last)
        let json = try XCTUnwrap(
            JSONSerialization.jsonObject(with: sentBody) as? [String: Int]
        )
        XCTAssertEqual(json, ["photo_id": 11])
    }

    func testAddPieceDuplicateVariant() async throws {
        stub(status: 200, data: Data(#"{"added": false, "duplicate": true}"#.utf8))

        let result = try await client.addPiece(toFolio: 3, photoID: 11)

        XCTAssertFalse(result.added)
        XCTAssertTrue(result.duplicate)
    }

    // MARK: - Errors

    func testHTTP500ThrowsHttpStatusWithBody() async throws {
        stub(status: 500, data: Data(#"{"error": "Failed to fetch gallery catalog"}"#.utf8))

        do {
            _ = try await client.catalog(
                page: 1, perPage: 20, query: nil, favoritesOnly: false, artist: nil
            )
            XCTFail("Expected FolioError.httpStatus")
        } catch let FolioError.httpStatus(status, body) {
            XCTAssertEqual(status, 500)
            XCTAssertTrue(body.contains("Failed to fetch gallery catalog"))
        }
    }

    func testMalformedJSONThrowsDecodingError() async throws {
        stub(data: Data("not json {".utf8))

        do {
            _ = try await client.folios()
            XCTFail("Expected FolioError.decoding")
        } catch let FolioError.decoding(_, path) {
            XCTAssertEqual(path, "/api/v1/folios")
        }
    }

    func testTransportErrorIsWrapped() async throws {
        StubURLProtocol.handler = { _ in
            throw URLError(.notConnectedToInternet)
        }

        do {
            _ = try await client.folios()
            XCTFail("Expected FolioError.transport")
        } catch FolioError.transport {
            // expected
        }
    }

    // MARK: - Upload

    /// Synthetic PascalCase DownloadedPhoto envelope as pieces.go serializes it.
    private func uploadResponseJSON(duplicate: Bool) -> Data {
        let json = """
        {
          "photo": {
            "ID": 77,
            "URL": "upload://abc123",
            "Title": "Uploaded piece",
            "Artist": "artist-u",
            "Favorite": false,
            "ImageWidth": 1024,
            "ImageHeight": 768,
            "FileName": "upload-0077.png",
            "Provider": "upload",
            "Category": "upload",
            "Status": "downloaded",
            "DownloadedAt": "2026-01-02T03:04:05Z"
          },
          "duplicate": \(duplicate)
        }
        """
        return Data(json.utf8)
    }

    /// Minimal PNG magic bytes plus trailing garbage — enough for sniffing.
    private var pngData: Data {
        Data([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02, 0x03])
    }

    func testUploadPieceSuccessDecodesPieceAndDuplicateFalse() async throws {
        stub(status: 201, data: uploadResponseJSON(duplicate: false))

        let result = try await client.uploadPiece(data: pngData, filename: "photo.png")

        XCTAssertFalse(result.duplicate)
        XCTAssertEqual(result.piece.id, 77)
        XCTAssertEqual(result.piece.title, "Uploaded piece")
        XCTAssertEqual(result.piece.artist, "artist-u")
        XCTAssertEqual(result.piece.width, 1024)
        XCTAssertEqual(result.piece.height, 768)
        XCTAssertEqual(result.piece.fileName, "upload-0077.png")
        XCTAssertEqual(result.piece.provider, "upload")
        XCTAssertEqual(result.piece.status, "downloaded")
        XCTAssertNotNil(result.piece.createdAt)
        XCTAssertEqual(try lastRequest.httpMethod, "POST")
        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/pieces"
        )
    }

    func testUploadPieceDuplicate200DecodesDuplicateTrue() async throws {
        stub(status: 200, data: uploadResponseJSON(duplicate: true))

        let result = try await client.uploadPiece(data: pngData, filename: "photo.png")

        XCTAssertTrue(result.duplicate)
        XCTAssertEqual(result.piece.id, 77, "duplicate response carries the existing photo")
    }

    func testUploadPieceBuildsMultipartBodyWithFileAndTitleParts() async throws {
        stub(status: 201, data: uploadResponseJSON(duplicate: false))

        _ = try await client.uploadPiece(
            data: pngData, filename: "photo.png", title: "My title"
        )

        XCTAssertEqual(try lastRequest.httpMethod, "POST")
        XCTAssertEqual(
            try lastRequest.url?.absoluteString,
            "https://folio.test/api/v1/pieces"
        )
        let contentType = try XCTUnwrap(
            try lastRequest.value(forHTTPHeaderField: "Content-Type")
        )
        XCTAssertTrue(
            contentType.hasPrefix("multipart/form-data; boundary="),
            "unexpected Content-Type: \(contentType)"
        )
        let boundary = String(contentType.dropFirst("multipart/form-data; boundary=".count))
        XCTAssertFalse(boundary.isEmpty)

        // URLSession exposes the body only as httpBodyStream inside
        // URLProtocol; the stub drains it into recordedBodies.
        let sentBody = try XCTUnwrap(StubURLProtocol.recordedBodies.last)
        let bodyText = String(decoding: sentBody, as: UTF8.self)
        XCTAssertTrue(bodyText.contains("--\(boundary)\r\n"))
        XCTAssertTrue(bodyText.contains("--\(boundary)--"))
        XCTAssertTrue(bodyText.contains(
            "Content-Disposition: form-data; name=\"file\"; filename=\"photo.png\"\r\n"
        ))
        XCTAssertTrue(bodyText.contains("Content-Type: image/png\r\n"))
        XCTAssertTrue(bodyText.contains("Content-Disposition: form-data; name=\"title\"\r\n\r\nMy title\r\n"))
        XCTAssertFalse(bodyText.contains("name=\"artist\""), "artist part must be omitted when nil")
        // Raw file bytes are embedded verbatim.
        XCTAssertNotNil(sentBody.range(of: pngData))
    }

    func testUploadPieceOmitsTextPartsWhenNilOrEmpty() async throws {
        stub(status: 201, data: uploadResponseJSON(duplicate: false))

        _ = try await client.uploadPiece(
            data: pngData, filename: "photo.png", title: "", artist: nil
        )

        let sentBody = try XCTUnwrap(StubURLProtocol.recordedBodies.last)
        let bodyText = String(decoding: sentBody, as: UTF8.self)
        XCTAssertFalse(bodyText.contains("name=\"title\""), "empty title must be omitted")
        XCTAssertFalse(bodyText.contains("name=\"artist\""))
        XCTAssertTrue(bodyText.contains("name=\"file\""))
    }

    func testUploadPieceSniffsContentTypeFromMagicBytes() {
        XCTAssertEqual(
            FolioClient.sniffImageContentType(Data([0xFF, 0xD8, 0xFF, 0xE0])),
            "image/jpeg"
        )
        XCTAssertEqual(FolioClient.sniffImageContentType(pngData), "image/png")
        XCTAssertEqual(
            FolioClient.sniffImageContentType(Data("GIF89a".utf8)),
            "image/gif"
        )
        XCTAssertEqual(
            FolioClient.sniffImageContentType(Data("RIFF\0\0\0\0WEBPVP8 ".utf8)),
            "image/webp"
        )
        XCTAssertEqual(
            FolioClient.sniffImageContentType(Data("plain text".utf8)),
            "application/octet-stream"
        )
    }

    func testUploadPieceHTTP500ThrowsHttpStatus() async throws {
        stub(status: 500, data: Data(#"{"error": "Failed to import piece"}"#.utf8))

        do {
            _ = try await client.uploadPiece(data: pngData, filename: "photo.png")
            XCTFail("Expected FolioError.httpStatus")
        } catch let FolioError.httpStatus(status, body) {
            XCTAssertEqual(status, 500)
            XCTAssertTrue(body.contains("Failed to import piece"))
        }
    }

    // MARK: - Error presentation

    func testErrorDescriptionSurfacesServerMessage() {
        let error = FolioError.httpStatus(400, body: #"{"error": "Unsupported image format"}"#)
        XCTAssertEqual(error.errorDescription, "Unsupported image format")
    }

    func testErrorDescriptionFallsBackToStatusAndBody() {
        XCTAssertEqual(
            FolioError.httpStatus(502, body: "bad gateway").errorDescription,
            "HTTP 502: bad gateway"
        )
        XCTAssertEqual(FolioError.httpStatus(500, body: "  ").errorDescription, "HTTP 500")
        XCTAssertEqual(
            FolioError.decoding(URLError(.cannotParseResponse), path: "/api/v1/stats").errorDescription,
            "Unexpected response from /api/v1/stats"
        )
    }

    // MARK: - Cancellation

    func testCancellationCancelsInFlightRequest() async throws {
        StubURLProtocol.hang = true

        let inFlight = Task { [client] in
            _ = try await client!.photo(id: 1)
        }
        // Let the request reach the stub before cancelling.
        for _ in 0..<100 where StubURLProtocol.recorded.isEmpty {
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        XCTAssertFalse(StubURLProtocol.recorded.isEmpty, "Request never started")
        inFlight.cancel()

        switch await inFlight.result {
        case .success:
            XCTFail("Expected the cancelled request to fail")
        case .failure:
            // URLSession reports NSURLErrorCancelled, wrapped as .transport.
            break
        }
    }
}
