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
                            session: app.imageSession,
                            onSingleTap: {
                                withAnimation(.easeInOut(duration: 0.2)) {
                                    chromeVisible.toggle()
                                }
                            },
                            onDismiss: { dismiss() }
                        )
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
                    .foregroundStyle(.white)
                    .lineLimit(1)
                if !piece.artist.isEmpty {
                    Button {
                        filterByArtist(piece.artist)
                    } label: {
                        HStack(spacing: 4) {
                            Image(systemName: "person")
                                .font(.caption)
                            Text(piece.artist)
                                .font(.subheadline)
                                .underline()
                                .lineLimit(1)
                        }
                        .foregroundStyle(.white.opacity(0.8))
                    }
                    .accessibilityLabel("Show pieces by \(piece.artist)")
                }
            }
            Spacer()
            Button {
                model.toggleFavorite(at: index)
            } label: {
                Image(systemName: piece.favorite ? "heart.fill" : "heart")
                    .font(.title3)
                    .foregroundStyle(piece.favorite ? .red : .white)
            }
            .accessibilityLabel(piece.favorite ? "Remove from favorites" : "Add to favorites")
            Button {
                showFolioPicker = true
            } label: {
                Image(systemName: "rectangle.stack.badge.plus")
                    .font(.title3)
                    .foregroundStyle(.white)
            }
            .accessibilityLabel("Add to folio")
        }
        .padding()
        // Semi-transparent scrim (not material) so the image stays readable
        // underneath, matching the close-button chrome.
        .background(.black.opacity(0.45))
    }

    /// Closes the viewer and filters the gallery to this artist.
    private func filterByArtist(_ artist: String) {
        dismiss()
        Task { await model.setArtistFilter(artist) }
    }
}

/// One zoomable page: pinch to zoom, double-tap to toggle 1x/2.5x, drag to
/// pan while zoomed, single tap to toggle chrome, swipe down at 1x to
/// dismiss the viewer.
private struct ZoomablePieceView: View {
    let url: URL
    let session: URLSession
    let onSingleTap: () -> Void
    let onDismiss: () -> Void

    @State private var zoom: CGFloat = 1
    @State private var gestureZoom: CGFloat = 1
    @State private var offset: CGSize = .zero
    @State private var gestureOffset: CGSize = .zero
    @State private var dismissDrag: CGFloat = 0

    private var isZoomed: Bool { zoom > 1 }

    var body: some View {
        GeometryReader { proxy in
            CachedImage(url: url, session: session, contentMode: .fit)
                .frame(width: proxy.size.width, height: proxy.size.height)
                .scaleEffect(zoom * gestureZoom)
                .offset(
                    x: offset.width + gestureOffset.width,
                    y: offset.height + gestureOffset.height + dismissDrag
                )
                .onTapGesture(count: 2) {
                    toggleZoom()
                }
                .onTapGesture {
                    onSingleTap()
                }
                .gesture(magnifyGesture(in: proxy.size))
                // Pan only while zoomed so TabView paging keeps working at 1x.
                .gesture(panGesture(in: proxy.size), including: isZoomed ? .all : .subviews)
                // Simultaneous so horizontal paging drags pass through
                // untouched; only a downward-dominant drag at 1x reacts.
                .simultaneousGesture(dismissGesture)
        }
    }

    private func magnifyGesture(in size: CGSize) -> some Gesture {
        MagnifyGesture()
            .onChanged { value in
                gestureZoom = value.magnification
            }
            .onEnded { value in
                let settled = min(max(zoom * value.magnification, 1), 4)
                // One animated transaction, including the gesture-state
                // reset: settling zoom or offset outside it makes the view
                // snap by the clamped amount on release.
                withAnimation(.easeOut(duration: 0.25)) {
                    gestureZoom = 1
                    if settled <= 1.01 {
                        zoom = 1
                        offset = .zero
                    } else {
                        zoom = settled
                        offset = clamped(offset, zoom: settled, in: size)
                    }
                }
            }
    }

    private func panGesture(in size: CGSize) -> some Gesture {
        DragGesture()
            .onChanged { value in
                gestureOffset = value.translation
            }
            .onEnded { value in
                let proposed = CGSize(
                    width: offset.width + value.translation.width,
                    height: offset.height + value.translation.height
                )
                withAnimation(.easeOut(duration: 0.2)) {
                    gestureOffset = .zero
                    offset = clamped(proposed, zoom: zoom, in: size)
                }
            }
    }

    private var dismissGesture: some Gesture {
        DragGesture(minimumDistance: 25)
            .onChanged { value in
                guard !isZoomed,
                      value.translation.height > 0,
                      value.translation.height > abs(value.translation.width)
                else { return }
                dismissDrag = value.translation.height
            }
            .onEnded { _ in
                guard !isZoomed else { return }
                if dismissDrag > 120 {
                    onDismiss()
                } else {
                    withAnimation(.easeOut(duration: 0.2)) {
                        dismissDrag = 0
                    }
                }
            }
    }

    /// Keeps the pan within the scaled content's bounds (view size is used
    /// as the content bound, which is conservative for letterboxed images).
    private func clamped(_ offset: CGSize, zoom: CGFloat, in size: CGSize) -> CGSize {
        let maxX = max(0, (zoom - 1) * size.width / 2)
        let maxY = max(0, (zoom - 1) * size.height / 2)
        return CGSize(
            width: min(max(offset.width, -maxX), maxX),
            height: min(max(offset.height, -maxY), maxY)
        )
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
