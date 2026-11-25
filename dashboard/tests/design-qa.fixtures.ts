import type { Page, Route } from "@playwright/test";

type Photo = {
  ID: number;
  URL: string;
  SourcePage: string;
  Title: string;
  Artist: string;
  UploadDate: string;
  FilePath: string;
  FileName: string;
  ImageWidth: number;
  ImageHeight: number;
  CapturedAt: string | null;
  CameraMake: string;
  CameraModel: string;
  LensModel: string;
  Orientation: string;
  GPSLatitude: number | null;
  GPSLongitude: number | null;
  DownloadedAt: string;
  FileSize: number;
  Notes: string;
  keywords: string[];
  manual_fields: string[];
  Favorite: boolean;
  Status: string;
};

const palette = [
  ["#7C2420", "#E7E1D5"],
  ["#315F72", "#F3EFE7"],
  ["#7B5E35", "#FBF9F3"],
  ["#4F6B43", "#E7E1D5"],
  ["#8A4B5E", "#FBF9F3"],
  ["#2F4F66", "#F3EFE7"],
];

function photo(id: number, title: string, artist: string, favorite = false): Photo {
  return {
    ID: id,
    URL: `https://fixture.ok-folio.invalid/pieces/${id}.jpg`,
    SourcePage: `https://sight.photo/artist/${artist.toLowerCase().replaceAll(" ", "-")}/${id}`,
    Title: title,
    Artist: artist,
    UploadDate: `2026-06-${String((id % 20) + 1).padStart(2, "0")}T00:00:00Z`,
    FilePath: `/fixture/pieces/${id}.jpg`,
    FileName: `${title.toLowerCase().replaceAll(" ", "-")}.jpg`,
    ImageWidth: id % 2 === 0 ? 1800 : 1400,
    ImageHeight: id % 2 === 0 ? 1200 : 1700,
    CapturedAt: null,
    CameraMake: "Fixture",
    CameraModel: "QA",
    LensModel: "",
    Orientation: "landscape",
    GPSLatitude: null,
    GPSLongitude: null,
    DownloadedAt: `2026-06-${String((id % 20) + 1).padStart(2, "0")}T12:00:00Z`,
    FileSize: 1_200_000 + id * 1024,
    Notes: id % 3 === 0 ? "Collected for visual QA density and metadata rendering." : "",
    keywords: id % 2 === 0 ? ["painting", "folio"] : ["photography", "study"],
    manual_fields: [],
    Favorite: favorite,
    Status: "downloaded",
  };
}

export const photos = [
  photo(101, "Red Room Study", "Mara Vale", true),
  photo(102, "Quiet Window", "Ilya Stone"),
  photo(103, "Blue Archive", "Nina Park"),
  photo(104, "Field Notes", "Oren Kai"),
  photo(105, "Paper Moon", "Mara Vale"),
  photo(106, "North Wall", "Theo Lin", true),
  photo(107, "Catalog Fragment", "Sana Rhee"),
  photo(108, "Evening Proof", "Ilya Stone"),
  photo(109, "Studio Shelf", "Nina Park"),
  photo(110, "Green Passage", "Oren Kai"),
  photo(111, "Low Sun", "Mara Vale"),
  photo(112, "Contact Sheet", "Theo Lin"),
];

const folios = [
  { id: 1, name: "Reference Walls", cover_photo_id: 101, piece_count: 6, created_at: "2026-06-01T10:00:00Z", updated_at: "2026-06-30T10:00:00Z" },
  { id: 2, name: "Color Studies", cover_photo_id: 106, piece_count: 4, created_at: "2026-06-02T10:00:00Z", updated_at: "2026-06-29T10:00:00Z" },
  { id: 5, name: "Night Studies", cover_photo_id: 107, piece_count: 5, created_at: "2026-06-05T10:00:00Z", updated_at: "2026-06-26T10:00:00Z" },
  { id: 6, name: "Paper Trails", cover_photo_id: 109, piece_count: 8, created_at: "2026-06-06T10:00:00Z", updated_at: "2026-06-25T10:00:00Z" },
  { id: 7, name: "Warm Light", cover_photo_id: 110, piece_count: 3, created_at: "2026-06-07T10:00:00Z", updated_at: "2026-06-24T10:00:00Z" },
  { id: 8, name: "Cold Frames", cover_photo_id: 112, piece_count: 6, created_at: "2026-06-08T10:00:00Z", updated_at: "2026-06-23T10:00:00Z" },
  { id: 9, name: "Long Exposures", cover_photo_id: 108, piece_count: 4, created_at: "2026-06-09T10:00:00Z", updated_at: "2026-06-22T10:00:00Z" },
  { id: 10, name: "Grain & Ink", cover_photo_id: 103, piece_count: 9, created_at: "2026-06-10T10:00:00Z", updated_at: "2026-06-21T10:00:00Z" },
  // Kept last so the dense grid also exercises the "first piece" and empty-matte covers.
  { id: 3, name: "Source Queue", cover_photo_id: null, piece_count: 1, created_at: "2026-06-03T10:00:00Z", updated_at: "2026-06-28T10:00:00Z" },
  { id: 4, name: "Empty Holding", cover_photo_id: null, piece_count: 0, created_at: "2026-06-04T10:00:00Z", updated_at: "2026-06-27T10:00:00Z" },
];

const folioPieces = new Map<number, Photo[]>([
  [1, photos.slice(0, 6)],
  [2, photos.slice(5, 9)],
  [3, photos.slice(10, 11)],
  [4, []],
]);

function json(data: unknown) {
  return {
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(data),
  };
}

function imageSvg(id: number): string {
  const [fg, bg] = palette[id % palette.length];
  const title = photos.find((item) => item.ID === id)?.Title ?? `Piece ${id}`;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="900" viewBox="0 0 1200 900">
    <rect width="1200" height="900" fill="${bg}"/>
    <rect x="72" y="72" width="1056" height="756" fill="${fg}" opacity=".92"/>
    <circle cx="900" cy="230" r="118" fill="${bg}" opacity=".28"/>
    <path d="M120 710 C300 540 430 620 560 485 C710 330 840 520 1080 300 L1080 828 L120 828 Z" fill="${bg}" opacity=".34"/>
    <text x="96" y="150" fill="${bg}" font-family="Georgia, serif" font-size="52">${title}</text>
  </svg>`;
}

function catalog(url: URL) {
  const offset = Number(url.searchParams.get("offset") ?? "0");
  const limit = Number(url.searchParams.get("limit") ?? "100");
  const page = photos.slice(offset, offset + limit);
  return {
    photos: page,
    total: photos.length,
    limit,
    offset,
    provider: "",
    source: "",
    category: url.searchParams.get("category") ?? "",
    artist: url.searchParams.get("artist") ?? "",
    favorite: url.searchParams.has("favorite") ? url.searchParams.get("favorite") === "true" : null,
    query: url.searchParams.get("q") ?? "",
    providers: [{ id: "sight.photo", display_name: "sight.photo", count: photos.length, sources: [{ id: "telegram:studio", display_name: "Studio stream", count: photos.length }] }],
    facets: {
      sources: [{ id: "telegram:studio", display_name: "Studio stream", count: photos.length }],
      categories: [
        { id: "painting", display_name: "Painting", count: 7 },
        { id: "photography", display_name: "Photography", count: 5 },
      ],
      artists: [
        { id: "mara-vale", display_name: "Mara Vale", count: 3 },
        { id: "ilya-stone", display_name: "Ilya Stone", count: 2 },
        { id: "nina-park", display_name: "Nina Park", count: 2 },
      ],
      favorites: [{ id: "favorites", display_name: "Favorites", favorite: true, count: 2 }],
    },
  };
}

async function routeApi(route: Route) {
  const request = route.request();
  const url = new URL(request.url());
  const path = url.pathname;

  if (request.method() !== "GET") {
    await route.fulfill(json({ ok: true }));
    return;
  }

  if (/^\/api\/v1\/photos\/\d+\/(thumbnail|image)$/.test(path)) {
    const id = Number(path.split("/")[4]);
    await route.fulfill({ status: 200, contentType: "image/svg+xml", body: imageSvg(id) });
    return;
  }

  if (path === "/health") {
    await route.fulfill(json({ status: "healthy", database: "connected", time: "2026-07-01T00:00:00Z" }));
    return;
  }
  if (path === "/api/v1/stats") {
    await route.fulfill(json({ total_photos: photos.length, unique_artists: 6, total_size_bytes: 18432000, last_download: "2026-06-30T10:00:00Z" }));
    return;
  }
  if (path === "/api/v1/gallery/catalog") {
    await route.fulfill(json(catalog(url)));
    return;
  }
  if (/^\/api\/v1\/gallery\/\d+\/similar$/.test(path)) {
    const id = Number(path.split("/")[4]);
    const limit = Number(url.searchParams.get("limit") ?? "12");
    await route.fulfill(json({
      pieces: photos
        .filter((item) => item.ID !== id)
        .slice(0, limit)
        .map((item, index) => ({ ...item, distance: (index + 1) / 100 })),
    }));
    return;
  }
  if (path === "/api/v1/artists") {
    await route.fulfill(json({ artists: catalog(url).facets.artists.map((item) => ({ artist: item.display_name, photo_count: item.count, total_size: item.count * 1024 })), total: 3, limit: 500, offset: 0, sort: "count" }));
    return;
  }
  if (path === "/api/v1/folios") {
    await route.fulfill(json({ folios }));
    return;
  }
  if (/^\/api\/v1\/folios\/\d+\/pieces$/.test(path)) {
    const id = Number(path.split("/")[4]);
    const pieces = folioPieces.get(id) ?? [];
    const offset = Number(url.searchParams.get("offset") ?? "0");
    const limit = Number(url.searchParams.get("limit") ?? "100");
    await route.fulfill(json({ photos: pieces.slice(offset, offset + limit), total: pieces.length, limit, offset }));
    return;
  }
  if (path === "/api/v1/inbox/counts") {
    await route.fulfill(json({ counts: { duplicate: 2, ambiguous: 1 }, total: 3 }));
    return;
  }
  if (path === "/api/v1/inbox") {
    await route.fulfill(json({
      items: [
        { id: 501, provider_id: "sight.photo", dedupe_key: "dup-101", source_id: "telegram:studio", media_id: "m101", source_url: photos[0].SourcePage, title: "Red Room Study", artist: "Mara Vale", status: "duplicate", reason: "Likely duplicate", cover_photo_id: 101, created_at: "2026-06-30T10:00:00Z", updated_at: "2026-06-30T10:00:00Z" },
        { id: 502, provider_id: "sight.photo", dedupe_key: "amb-108", source_id: "webgallery:daily", media_id: "m108", source_url: photos[7].SourcePage, title: "Evening Proof", artist: "Ilya Stone", status: "ambiguous", reason: "Needs review", cover_photo_id: 108, created_at: "2026-06-30T11:00:00Z", updated_at: "2026-06-30T11:00:00Z" },
      ],
      total: 2,
      limit: 50,
      offset: 0,
    }));
    return;
  }
  if (path === "/api/v1/streams/connectors/status") {
    await route.fulfill(json({ connectors: [
      { id: "telegram", display_name: "Telegram", health: "healthy", state: "idle", last_sync: "2026-06-30T12:00:00Z", counts: { downloaded: 8, failed: 0, pending: 1, total: 9 }, sources: [{ id: "telegram:studio", display_name: "Studio stream", provider_id: "telegram", last_sync: "2026-06-30T12:00:00Z", counts: { downloaded: 8, failed: 0, pending: 1, total: 9 } }], recent_runs: [], recent_errors: [] },
      { id: "webgallery", display_name: "Web Gallery", health: "syncing", state: "syncing", last_sync: "2026-06-30T12:30:00Z", counts: { downloaded: 4, failed: 0, pending: 2, total: 6 }, sources: [], recent_runs: [], recent_errors: [] },
    ] }));
    return;
  }
  if (path === "/api/v1/settings/connector-sources") {
    await route.fulfill(json({ sources: [
      { id: 31, type: "telegram", chat_id: "fixture-studio", label: "Studio stream", enabled: true, target_folio_id: 1, show_in_library: true, created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-30T00:00:00Z" },
      { id: 32, type: "webgallery", chat_id: "fixture-daily", label: "Daily image board", config: { list_url: "https://fixture.ok-folio.invalid", pagination: { strategy: "none" }, selectors: { item_link: "a.card", image: { selector: "img", attr: "src" } }, item_link_filter: [] }, enabled: true, target_folio_id: 2, show_in_library: false, created_at: "2026-06-01T00:00:00Z", updated_at: "2026-06-30T00:00:00Z" },
    ] }));
    return;
  }

  await route.fulfill(json({}));
}

export async function installDesignQaFixtures(page: Page) {
  await page.route("**/api/v1/**", routeApi);
  await page.route("**/health", routeApi);
}
