import Foundation

/// App Group shared by the OKFolio app and the OKFolioShare extension.
/// The share extension cannot import app sources, so the same constant is
/// duplicated in `ios/Share/Sources/AppGroup.swift` — keep both in sync.
enum AppGroup {
    static let identifier = "group.com.befeast.okfolio"
}
