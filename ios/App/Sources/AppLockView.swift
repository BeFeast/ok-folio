import SwiftUI
import LocalAuthentication

/// Full-screen gate shown while the app is locked. Uses
/// `.deviceOwnerAuthentication`, which falls back to the device passcode
/// when Face ID / Touch ID is unavailable.
struct AppLockView: View {
    @Environment(AppModel.self) private var app

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
            authenticate()
        }
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
