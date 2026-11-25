import Foundation

// MARK: - Piece

/// One catalog piece (the Go API calls these "photos").
///
/// The Go wire format has two shapes for the same entity:
/// - `GET /api/v1/gallery/catalog` items serialize `database.DownloadedPhoto`
///   whose fields mostly carry NO json tags, so keys are Go field names
///   verbatim: `ID`, `Title`, `Artist`, `Favorite`, `ImageWidth`,
///   `ImageHeight`, `FileName`, `Provider`, `Category`, `Status`,
///   `DownloadedAt` (RFC3339, nullable).
/// - `GET /api/v1/photos/{id}` builds a snake_case map by hand: `id`, `title`,
///   `artist`, `favorite`, `file_name`, `provider`, `category`, `status`,
///   `downloaded_at`. That map has NO width/height fields, which is why
///   `width`/`height` are optional here.
///
/// `Piece.init(from:)` accepts both shapes transparently.
public struct Piece: Sendable, Identifiable, Decodable {
    public let id: Int
    public let title: String
    public let artist: String
    /// Mutable so UI code can flip it optimistically after `setFavorite`.
    public var favorite: Bool
    /// Pixel width. Present in catalog items (`ImageWidth`); absent in the
    /// photo-detail payload.
    public let width: Int?
    /// Pixel height. Present in catalog items (`ImageHeight`); absent in the
    /// photo-detail payload.
    public let height: Int?
    /// Wire `DownloadedAt` / `downloaded_at` (nullable RFC3339).
    public let createdAt: Date?
    public let fileName: String?
    public let provider: String?
    public let category: String?
    public let status: String?

    public init(
        id: Int,
        title: String,
        artist: String,
        favorite: Bool,
        width: Int? = nil,
        height: Int? = nil,
        createdAt: Date? = nil,
        fileName: String? = nil,
        provider: String? = nil,
        category: String? = nil,
        status: String? = nil
    ) {
        self.id = id
        self.title = title
        self.artist = artist
        self.favorite = favorite
        self.width = width
        self.height = height
        self.createdAt = createdAt
        self.fileName = fileName
        self.provider = provider
        self.category = category
        self.status = status
    }

    /// Catalog shape: untagged Go struct fields serialize as Go identifiers.
    private enum CatalogKeys: String, CodingKey {
        case id = "ID"
        case title = "Title"
        case artist = "Artist"
        case favorite = "Favorite"
        case width = "ImageWidth"
        case height = "ImageHeight"
        case createdAt = "DownloadedAt"
        case fileName = "FileName"
        case provider = "Provider"
        case category = "Category"
        case status = "Status"
    }

    /// Photo-detail shape: hand-built snake_case map in images.go.
    private enum DetailKeys: String, CodingKey {
        case id
        case title
        case artist
        case favorite
        case createdAt = "downloaded_at"
        case fileName = "file_name"
        case provider
        case category
        case status
    }

    public init(from decoder: Decoder) throws {
        let catalog = try decoder.container(keyedBy: CatalogKeys.self)
        if catalog.contains(.id) {
            id = try catalog.decode(Int.self, forKey: .id)
            title = try catalog.decodeIfPresent(String.self, forKey: .title) ?? ""
            artist = try catalog.decodeIfPresent(String.self, forKey: .artist) ?? ""
            favorite = try catalog.decodeIfPresent(Bool.self, forKey: .favorite) ?? false
            width = try catalog.decodeIfPresent(Int.self, forKey: .width)
            height = try catalog.decodeIfPresent(Int.self, forKey: .height)
            createdAt = try catalog.decodeIfPresent(Date.self, forKey: .createdAt)
            fileName = try catalog.decodeIfPresent(String.self, forKey: .fileName)
            provider = try catalog.decodeIfPresent(String.self, forKey: .provider)
            category = try catalog.decodeIfPresent(String.self, forKey: .category)
            status = try catalog.decodeIfPresent(String.self, forKey: .status)
        } else {
            let detail = try decoder.container(keyedBy: DetailKeys.self)
            id = try detail.decode(Int.self, forKey: .id)
            title = try detail.decodeIfPresent(String.self, forKey: .title) ?? ""
            artist = try detail.decodeIfPresent(String.self, forKey: .artist) ?? ""
            favorite = try detail.decodeIfPresent(Bool.self, forKey: .favorite) ?? false
            width = nil
            height = nil
            createdAt = try detail.decodeIfPresent(Date.self, forKey: .createdAt)
            fileName = try detail.decodeIfPresent(String.self, forKey: .fileName)
            provider = try detail.decodeIfPresent(String.self, forKey: .provider)
            category = try detail.decodeIfPresent(String.self, forKey: .category)
            status = try detail.decodeIfPresent(String.self, forKey: .status)
        }
    }
}

// MARK: - Catalog page

/// Response of `GET /api/v1/gallery/catalog`.
///
/// Wire envelope (gallery.go `galleryCatalogResponse`): `photos`, `total`,
/// `limit`, `offset`, echoes of the filters, `providers`, and `facets`.
public struct CatalogPage: Sendable, Decodable {
    /// Wire `photos`.
    public let items: [Piece]
    /// Wire `total` (int64 count across all pages).
    public let total: Int
    /// Wire `facets`. Always present on the wire, optional here so partial
    /// payloads still decode.
    public let facets: Facets?

    private enum CodingKeys: String, CodingKey {
        case items = "photos"
        case total
        case facets
    }

    public init(items: [Piece], total: Int, facets: Facets?) {
        self.items = items
        self.total = total
        self.facets = facets
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        items = try container.decodeIfPresent([Piece].self, forKey: .items) ?? []
        total = try container.decodeIfPresent(Int.self, forKey: .total) ?? 0
        facets = try container.decodeIfPresent(Facets.self, forKey: .facets)
    }
}

/// Wire `facets` object: `{sources, categories, artists, favorites}`.
public struct Facets: Sendable, Decodable {
    public let artists: [Facet]
    public let categories: [Facet]
    public let sources: [Facet]
    public let favorites: [FavoriteFacet]

    private enum CodingKeys: String, CodingKey {
        case artists, categories, sources, favorites
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        artists = try container.decodeIfPresent([Facet].self, forKey: .artists) ?? []
        categories = try container.decodeIfPresent([Facet].self, forKey: .categories) ?? []
        sources = try container.decodeIfPresent([Facet].self, forKey: .sources) ?? []
        favorites = try container.decodeIfPresent([FavoriteFacet].self, forKey: .favorites) ?? []
    }
}

/// One facet entry: `{"id", "display_name", "count"}`.
public struct Facet: Sendable, Decodable {
    public let id: String
    public let displayName: String
    public let count: Int

    private enum CodingKeys: String, CodingKey {
        case id
        case displayName = "display_name"
        case count
    }
}

/// Favorite facet entry: `{"id", "display_name", "favorite", "count"}`.
public struct FavoriteFacet: Sendable, Decodable {
    public let id: String
    public let displayName: String
    public let favorite: Bool
    public let count: Int

    private enum CodingKeys: String, CodingKey {
        case id
        case displayName = "display_name"
        case favorite
        case count
    }
}

// MARK: - Folio

/// One folio (`database.Folio`, fully json-tagged in Go): `id`, `name`,
/// `piece_count`, `cover_photo_id` (nullable), `created_at`, `updated_at`.
public struct Folio: Sendable, Identifiable, Decodable {
    public let id: Int
    public let name: String
    /// Wire `piece_count`.
    public let pieceCount: Int
    /// Wire `cover_photo_id`; nil when the cover auto-resolves server-side.
    public let coverPhotoID: Int?
    public let createdAt: Date?
    public let updatedAt: Date?

    private enum CodingKeys: String, CodingKey {
        case id
        case name
        case pieceCount = "piece_count"
        case coverPhotoID = "cover_photo_id"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(Int.self, forKey: .id)
        name = try container.decode(String.self, forKey: .name)
        pieceCount = try container.decodeIfPresent(Int.self, forKey: .pieceCount) ?? 0
        coverPhotoID = try container.decodeIfPresent(Int.self, forKey: .coverPhotoID)
        createdAt = try container.decodeIfPresent(Date.self, forKey: .createdAt)
        updatedAt = try container.decodeIfPresent(Date.self, forKey: .updatedAt)
    }
}

// MARK: - Add piece result

/// Result of `POST /api/v1/folios/{id}/pieces`.
///
/// Wire: `201 {"added": true}` on success, `200 {"added": false,
/// "duplicate": true}` when the piece is already in the folio. `duplicate`
/// is absent from the success payload, so it defaults to false.
public struct AddPieceResult: Sendable, Decodable {
    public let added: Bool
    public let duplicate: Bool

    private enum CodingKeys: String, CodingKey {
        case added, duplicate
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        added = try container.decodeIfPresent(Bool.self, forKey: .added) ?? false
        duplicate = try container.decodeIfPresent(Bool.self, forKey: .duplicate) ?? false
    }
}

// MARK: - Upload result

/// Result of `POST /api/v1/pieces` (manual image upload).
///
/// Wire: `201 {"photo": ..., "duplicate": false}` for a fresh import,
/// `200 {"photo": ..., "duplicate": true}` when identical content (matched by
/// content hash server-side) was already imported — `piece` is then the
/// pre-existing photo.
public struct UploadResult: Sendable {
    public let piece: Piece
    public let duplicate: Bool
}

// MARK: - Internal envelopes

/// `{"folios": [...]}` — shared by `GET /api/v1/folios` and
/// `GET /api/v1/photos/{id}/folios` (folios.go `foliosResponse`).
struct FoliosEnvelope: Decodable {
    let folios: [Folio]
}

/// `{"id", "favorite", "available"}` — favorites.go set/get responses.
struct FavoriteEnvelope: Decodable {
    let id: Int
    let favorite: Bool
}

/// `{"photo": <PascalCase DownloadedPhoto>, "duplicate": bool}` — pieces.go
/// `createPieceResponse`. The photo serializes exactly like a catalog item,
/// so `Piece`'s catalog branch decodes it.
struct UploadEnvelope: Decodable {
    let photo: Piece
    let duplicate: Bool
}
