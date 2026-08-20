import SwiftUI
import Observation
import FolioKit

/// Paged catalog state shared by the grid and the full-screen viewer.
@MainActor
@Observable
final class GalleryModel {
    var items: [Piece] = []
    var total = 0
    var isLoading = false
    var errorMessage: String?
    var favoritesOnly = false
    var query = ""

    private var client: FolioClient?
    private var nextPage = 1
    private let perPage = 50
    /// Bumped on reload so in-flight responses for a stale filter are dropped.
    private var generation = 0

    var canLoadMore: Bool {
        nextPage == 1 || items.count < total
    }

    func configure(client: FolioClient?) {
        self.client = client
    }

    func reload() async {
        generation &+= 1
        nextPage = 1
        items = []
        total = 0
        errorMessage = nil
        isLoading = false
        await loadNextPage()
    }

    /// Load the next page when the user is ~10 items from the end.
    func loadMoreIfNeeded(index: Int) {
        guard index >= items.count - 10, canLoadMore, !isLoading else { return }
        Task { await self.loadNextPage() }
    }

    func retry() async {
        errorMessage = nil
        if items.isEmpty {
            await reload()
        } else {
            await loadNextPage()
        }
    }

    func loadNextPage() async {
        guard let client, !isLoading, canLoadMore else { return }
        let requestGeneration = generation
        isLoading = true
        do {
            let page = try await client.catalog(
                page: nextPage,
                perPage: perPage,
                query: query.isEmpty ? nil : query,
                favoritesOnly: favoritesOnly,
                artist: nil
            )
            guard requestGeneration == generation else { return }
            items.append(contentsOf: page.items)
            total = page.total
            nextPage += 1
            isLoading = false
        } catch {
            guard requestGeneration == generation else { return }
            errorMessage = String(describing: error)
            isLoading = false
        }
    }

    /// Optimistic favorite toggle; reverts on error, adopts the
    /// server-confirmed value on success.
    func toggleFavorite(at index: Int) {
        guard items.indices.contains(index), let client else { return }
        let piece = items[index]
        let newValue = !piece.favorite
        items[index].favorite = newValue
        Task {
            do {
                let confirmed = try await client.setFavorite(id: piece.id, isFavorite: newValue)
                if self.items.indices.contains(index), self.items[index].id == piece.id {
                    self.items[index].favorite = confirmed
                }
            } catch {
                if self.items.indices.contains(index), self.items[index].id == piece.id {
                    self.items[index].favorite = piece.favorite
                }
            }
        }
    }
}

struct GalleryView: View {
    @Environment(AppModel.self) private var app
    @State private var model = GalleryModel()
    @State private var searchText = ""
    @State private var showSettings = false
    @State private var viewerSelection: ViewerSelection?

    private struct ViewerSelection: Identifiable {
        let id: Int // index into model.items
    }

    private let columns = [GridItem(.adaptive(minimum: 110), spacing: 2)]

    var body: some View {
        NavigationStack {
            ScrollView {
                if let message = model.errorMessage {
                    errorBanner(message)
                }
                grid
                if model.isLoading {
                    ProgressView()
                        .padding()
                }
                if !model.isLoading, model.errorMessage == nil, model.items.isEmpty {
                    ContentUnavailableView(
                        model.query.isEmpty ? "No Pieces" : "No Results",
                        systemImage: "photo.on.rectangle.angled"
                    )
                    .padding(.top, 80)
                }
            }
            .navigationTitle("OK Folio")
            .navigationBarTitleDisplayMode(.inline)
            .searchable(text: $searchText, prompt: "Search")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        model.favoritesOnly.toggle()
                        Task { await model.reload() }
                    } label: {
                        Image(systemName: model.favoritesOnly ? "heart.fill" : "heart")
                    }
                    .accessibilityLabel("Favorites only")
                }
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        showSettings = true
                    } label: {
                        Image(systemName: "gearshape")
                    }
                    .accessibilityLabel("Settings")
                }
            }
            .refreshable {
                await model.reload()
            }
            .task {
                model.configure(client: app.client)
                if model.items.isEmpty {
                    await model.reload()
                }
            }
            .task(id: searchText) {
                // Debounce typing before hitting the API.
                try? await Task.sleep(for: .milliseconds(400))
                guard !Task.isCancelled, model.query != searchText else { return }
                model.query = searchText
                await model.reload()
            }
            .onChange(of: app.serverURLString) {
                model.configure(client: app.client)
                Task { await model.reload() }
            }
            .sheet(isPresented: $showSettings) {
                SettingsView()
            }
            .fullScreenCover(item: $viewerSelection) { selection in
                PieceViewerView(model: model, startIndex: selection.id)
            }
        }
    }

    private var grid: some View {
        LazyVGrid(columns: columns, spacing: 2) {
            ForEach(Array(model.items.enumerated()), id: \.element.id) { index, piece in
                cell(piece: piece, index: index)
                    .onAppear {
                        model.loadMoreIfNeeded(index: index)
                    }
            }
        }
    }

    @ViewBuilder
    private func cell(piece: Piece, index: Int) -> some View {
        Color.clear
            .aspectRatio(1, contentMode: .fit)
            .overlay {
                if let client = app.client {
                    CachedImage(
                        url: client.thumbnailURL(id: piece.id),
                        session: app.imageSession,
                        contentMode: .fill
                    )
                }
            }
            .clipped()
            .overlay(alignment: .bottomTrailing) {
                if piece.favorite {
                    Image(systemName: "heart.fill")
                        .font(.caption2)
                        .foregroundStyle(.white)
                        .shadow(color: .black.opacity(0.6), radius: 2)
                        .padding(4)
                }
            }
            .contentShape(Rectangle())
            .onTapGesture {
                viewerSelection = ViewerSelection(id: index)
            }
    }

    private func errorBanner(_ message: String) -> some View {
        HStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.red)
            Text(message)
                .font(.footnote)
                .lineLimit(2)
            Spacer()
            Button("Retry") {
                Task { await model.retry() }
            }
            .font(.footnote.bold())
        }
        .padding(10)
        .background(.red.opacity(0.12), in: RoundedRectangle(cornerRadius: 8))
        .padding(.horizontal, 8)
        .padding(.top, 4)
    }
}
