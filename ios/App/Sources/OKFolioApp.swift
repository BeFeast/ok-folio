import SwiftUI

@main
struct OKFolioApp: App {
    @State private var model = AppModel()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
                .onChange(of: scenePhase) { _, newPhase in
                    model.handleScenePhase(newPhase)
                }
        }
    }
}

/// Switches between onboarding (no server configured), the gallery, and the
/// Face ID lock screen.
struct RootView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        ZStack {
            if model.isLocked {
                // Locked: content stays out of the view hierarchy entirely,
                // so neither snapshots nor accessibility clients can reach it.
                AppLockView()
                    .transition(.opacity)
            } else {
                content
                    .overlay {
                        if model.isCovered {
                            PrivacyCoverView()
                        }
                    }
            }
        }
    }

    @ViewBuilder
    private var content: some View {
        if model.client == nil {
            // First launch: no valid server URL yet.
            SettingsView()
        } else {
            GalleryView()
        }
    }
}
