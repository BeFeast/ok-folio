import SwiftUI

/// Async image view that loads through an injected URLSession so responses go
/// through the shared disk-backed URLCache (AsyncImage uses its own session
/// and would not benefit from it).
struct CachedImage: View {
    let url: URL
    let session: URLSession
    var contentMode: ContentMode = .fill

    @State private var image: UIImage?
    @State private var failed = false

    var body: some View {
        ZStack {
            if let image {
                Image(uiImage: image)
                    .resizable()
                    .aspectRatio(contentMode: contentMode)
            } else if failed {
                Image(systemName: "photo")
                    .font(.title2)
                    .foregroundStyle(.secondary)
            } else {
                Color(uiColor: .systemGray5)
            }
        }
        .task(id: url) {
            await load()
        }
    }

    private func load() async {
        failed = false
        image = nil
        do {
            let (data, response) = try await session.data(from: url)
            guard !Task.isCancelled else { return }
            guard let http = response as? HTTPURLResponse,
                  (200..<300).contains(http.statusCode),
                  let decoded = UIImage(data: data)
            else {
                failed = true
                return
            }
            image = decoded
        } catch {
            if !Task.isCancelled {
                failed = true
            }
        }
    }
}
