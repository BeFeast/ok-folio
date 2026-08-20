// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "FolioKit",
    platforms: [
        .iOS(.v17),
        .macOS(.v13),
    ],
    products: [
        .library(name: "FolioKit", targets: ["FolioKit"]),
    ],
    targets: [
        .target(
            name: "FolioKit",
            path: "Sources/FolioKit"
        ),
        .testTarget(
            name: "FolioKitTests",
            dependencies: ["FolioKit"],
            path: "Tests/FolioKitTests",
            resources: [
                .copy("Fixtures"),
            ]
        ),
    ]
)
