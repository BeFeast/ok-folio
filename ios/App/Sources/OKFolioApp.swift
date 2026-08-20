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
            if model.client == nil {
                // First launch: no valid server URL yet.
                SettingsView()
            } else {
                GalleryView()
            }
            if model.isLocked {
                AppLockView()
                    .transition(.opacity)
            }
        }
    }
}
