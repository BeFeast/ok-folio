import SwiftUI
import FolioKit

/// Full-screen, horizontally paged viewer over the gallery's loaded items.
struct PieceViewerView: View {
    @Environment(AppModel.self) private var app
    @Environment(\.dismiss) private var dismiss

    let model: GalleryModel
    @State private var index: Int
    @State private var chromeVisible = true
    @State private var showFolioPicker = false

    init(model: GalleryModel, startIndex: Int) {
        self.model = model
        _index = State(initialValue: startIndex)
    }

    private var currentPiece: Piece? {
        model.items.indices.contains(index) ? model.items[index] : nil
    }

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()
            pager
            if chromeVisible {
                chrome
            }
        }
        .statusBarHidden(!chromeVisible)
        .privacyCovered()
        .sheet(isPresented: $showFolioPicker) {
            if let piece = currentPiece {
                FolioPickerSheet(photoID: piece.id)
            }
        }
    }

    private var pager: some View {
        TabView(selection: $index) {
            ForEach(Array(model.items.enumerated()), id: \.element.id) { i, piece in
                Group {
                    if let client = app.client {
                        ZoomablePieceView(
                            url: client.imageURL(id: piece.id),
                            session: app.imageSession
                        ) {
                            withAnimation(.easeInOut(duration: 0.2)) {
                                chromeVisible.toggle()
                            }
                        }
                    }
                }
                .tag(i)
            }
        }
        .tabViewStyle(.page(indexDisplayMode: .never))
        .ignoresSafeArea()
        .onChange(of: index) { _, newIndex in
            // Keep paging seamless near the end of the loaded window.
            model.loadMoreIfNeeded(index: newIndex)
        }
    }

    private var chrome: some View {
        VStack {
            HStack {
                Button {
                    dismiss()
                } label: {
                    Image(systemName: "xmark")
                        .font(.body.bold())
                        .foregroundStyle(.white)
                        .padding(10)
                        .background(.black.opacity(0.5), in: Circle())
                }
                .accessibilityLabel("Close")
                Spacer()
            }
            .padding()
            Spacer()
            if let piece = currentPiece {
                bottomBar(for: piece)
            }
        }
    }

    private func bottomBar(for piece: Piece) -> some View {
        HStack(spacing: 16) {
            VStack(alignment: .leading, spacing: 2) {
                Text(piece.title.isEmpty ? "Untitled" : piece.title)
                    .font(.headline)
                    .lineLimit(1)
                if !piece.artist.isEmpty {
                    Text(piece.artist)
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer()
            Button {
                model.toggleFavorite(at: index)
            } label: {
                Image(systemName: piece.favorite ? "heart.fill" : "heart")
                    .font(.title3)
                    .foregroundStyle(piece.favorite ? .red : .primary)
            }
            .accessibilityLabel(piece.favorite ? "Remove from favorites" : "Add to favorites")
            Button {
                showFolioPicker = true
            } label: {
                Image(systemName: "rectangle.stack.badge.plus")
                    .font(.title3)
            }
            .accessibilityLabel("Add to folio")
        }
        .padding()
        .background(.ultraThinMaterial)
    }
}

/// One zoomable page: pinch to zoom, double-tap to toggle 1x/2.5x, drag to
/// pan while zoomed, single tap to toggle chrome.
private struct ZoomablePieceView: View {
    let url: URL
    let session: URLSession
    let onSingleTap: () -> Void

    @State private var zoom: CGFloat = 1
    @State private var gestureZoom: CGFloat = 1
    @State private var offset: CGSize = .zero
    @State private var gestureOffset: CGSize = .zero

    private var isZoomed: Bool { zoom > 1 }

    var body: some View {
        GeometryReader { proxy in
            CachedImage(url: url, session: session, contentMode: .fit)
                .frame(width: proxy.size.width, height: proxy.size.height)
                .scaleEffect(zoom * gestureZoom)
                .offset(
                    x: offset.width + gestureOffset.width,
                    y: offset.height + gestureOffset.height
                )
                .onTapGesture(count: 2) {
                    toggleZoom()
                }
                .onTapGesture {
                    onSingleTap()
                }
                .gesture(magnifyGesture)
                // Pan only while zoomed so TabView paging keeps working at 1x.
                .gesture(panGesture, including: isZoomed ? .all : .subviews)
        }
    }

    private var magnifyGesture: some Gesture {
        MagnifyGesture()
            .onChanged { value in
                gestureZoom = value.magnification
            }
            .onEnded { value in
                zoom = min(max(zoom * value.magnification, 1), 4)
                gestureZoom = 1
                if zoom <= 1.01 {
                    withAnimation(.easeOut(duration: 0.2)) {
                        zoom = 1
                        offset = .zero
                    }
                }
            }
    }

    private var panGesture: some Gesture {
        DragGesture()
            .onChanged { value in
                gestureOffset = value.translation
            }
            .onEnded { value in
                offset.width += value.translation.width
                offset.height += value.translation.height
                gestureOffset = .zero
            }
    }

    private func toggleZoom() {
        withAnimation(.spring(duration: 0.3)) {
            if isZoomed {
                zoom = 1
                offset = .zero
            } else {
                zoom = 2.5
            }
        }
    }
}
