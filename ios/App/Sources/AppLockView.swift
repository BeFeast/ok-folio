import SwiftUI
import LocalAuthentication

/// Full-screen gate shown while the app is locked. Uses
/// `.deviceOwnerAuthentication`, which falls back to the device passcode
/// when Face ID / Touch ID is unavailable.
struct AppLockView: View {
    @Environment(AppModel.self) private var app
    @Environment(\.scenePhase) private var scenePhase

    @State private var isAuthenticating = false
    @State private var failed = false

    var body: some View {
        ZStack {
            Rectangle()
                .fill(.regularMaterial)
                .ignoresSafeArea()
            VStack(spacing: 16) {
                Image(systemName: "lock.fill")
                    .font(.system(size: 40))
                    .foregroundStyle(.secondary)
                Text("OK Folio is locked")
                    .font(.headline)
                if failed {
                    Button("Unlock") {
                        authenticate()
                    }
                    .buttonStyle(.borderedProminent)
                }
            }
        }
        .task {
            authenticateIfNeeded()
        }
        .onChange(of: scenePhase) { _, newPhase in
            if newPhase == .active {
                authenticateIfNeeded()
            }
        }
    }

    /// Auto-prompt only while the scene is active — starting the biometric
    /// prompt during backgrounding fails immediately. After a failed or
    /// cancelled attempt, wait for the explicit Unlock button instead of
    /// re-prompting on the .inactive/.active bounce the prompt itself causes.
    private func authenticateIfNeeded() {
        guard scenePhase == .active, !failed else { return }
        authenticate()
    }

    private func authenticate() {
        guard !isAuthenticating else { return }
        isAuthenticating = true
        failed = false

        let context = LAContext()
        var policyError: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &policyError) else {
            // No biometrics and no passcode set: do not lock the user out.
            app.isLocked = false
            isAuthenticating = false
            return
        }
        context.evaluatePolicy(
            .deviceOwnerAuthentication,
            localizedReason: "Unlock OK Folio"
        ) { success, _ in
            Task { @MainActor in
                isAuthenticating = false
                if success {
                    app.isLocked = false
                } else {
                    failed = true
                }
            }
        }
    }
}

/// Opaque cover shown while the scene is inactive (app switcher, incoming
/// call) so system snapshots never capture gallery content. No controls:
/// it disappears on its own when the scene becomes active again.
struct PrivacyCoverView: View {
    var body: some View {
        ZStack {
            Rectangle()
                .fill(.regularMaterial)
                .ignoresSafeArea()
            Image(systemName: "photo.stack")
                .font(.system(size: 40))
                .foregroundStyle(.secondary)
        }
    }
}
