import SwiftUI

struct SettingsView: View {
    @Environment(AppModel.self) private var app
    @Environment(\.dismiss) private var dismiss

    @State private var draftURL = ""

    private var isValidURL: Bool {
        AppModel.validatedBaseURL(from: draftURL) != nil
    }

    private var isDirty: Bool {
        draftURL.trimmingCharacters(in: .whitespacesAndNewlines) != app.serverURLString
    }

    var body: some View {
        @Bindable var app = app
        NavigationStack {
            Form {
                Section {
                    TextField("https://folio.example.lan", text: $draftURL)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("Save") {
                        app.serverURLString = draftURL.trimmingCharacters(in: .whitespacesAndNewlines)
                    }
                    .disabled(!isValidURL || !isDirty)
                } header: {
                    Text("Server")
                } footer: {
                    Text("Base URL of your OK Folio instance, e.g. https://folio.your.lan")
                }

                Section("Security") {
                    Toggle("Face ID Lock", isOn: $app.faceIDLockEnabled)
                }

                Section("About") {
                    LabeledContent("Version", value: appVersion)
                }
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                // No-op when shown as onboarding root (nothing to dismiss).
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") {
                        dismiss()
                    }
                }
            }
            .onAppear {
                draftURL = app.serverURLString
            }
        }
    }

    private var appVersion: String {
        let short = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "?"
        let build = Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "?"
        return "\(short) (\(build))"
    }
}
