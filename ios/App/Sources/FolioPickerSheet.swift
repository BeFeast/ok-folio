import SwiftUI
import FolioKit

/// Sheet listing folios; tapping one adds the photo and shows inline feedback
/// before auto-dismissing.
struct FolioPickerSheet: View {
    @Environment(AppModel.self) private var app
    @Environment(\.dismiss) private var dismiss

    let photoID: Int

    private enum RowState {
        case working
        case added
        case duplicate
        case failed(String)
    }

    @State private var folios: [Folio] = []
    @State private var isLoading = true
    @State private var errorMessage: String?
    @State private var rowStates: [Int: RowState] = [:]

    var body: some View {
        NavigationStack {
            Group {
                if isLoading {
                    ProgressView()
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                } else if let errorMessage {
                    VStack(spacing: 12) {
                        Text(errorMessage)
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                        Button("Retry") {
                            Task { await load() }
                        }
                        .buttonStyle(.bordered)
                    }
                    .padding()
                } else if folios.isEmpty {
                    ContentUnavailableView("No Folios", systemImage: "rectangle.stack")
                } else {
                    List(folios) { folio in
                        row(for: folio)
                    }
                }
            }
            .navigationTitle("Add to Folio")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }
            }
            .task {
                await load()
            }
        }
        .presentationDetents([.medium, .large])
    }

    private func row(for folio: Folio) -> some View {
        Button {
            add(to: folio)
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text(folio.name)
                        .foregroundStyle(.primary)
                    Text("\(folio.pieceCount) pieces")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                Spacer()
                feedback(for: folio)
            }
        }
        .disabled(isRowBusy(folio))
    }

    @ViewBuilder
    private func feedback(for folio: Folio) -> some View {
        switch rowStates[folio.id] {
        case .working:
            ProgressView()
        case .added:
            Label("Added", systemImage: "checkmark.circle.fill")
                .font(.caption)
                .foregroundStyle(.green)
        case .duplicate:
            Text("Already in folio")
                .font(.caption)
                .foregroundStyle(.secondary)
        case .failed(let message):
            Text(message)
                .font(.caption2)
                .foregroundStyle(.red)
                .lineLimit(2)
        case nil:
            EmptyView()
        }
    }

    private func isRowBusy(_ folio: Folio) -> Bool {
        if case .working = rowStates[folio.id] {
            return true
        }
        return false
    }

    private func load() async {
        guard let client = app.client else {
            errorMessage = "No server configured."
            isLoading = false
            return
        }
        isLoading = true
        errorMessage = nil
        do {
            folios = try await client.folios()
            isLoading = false
        } catch {
            errorMessage = String(describing: error)
            isLoading = false
        }
    }

    private func add(to folio: Folio) {
        guard let client = app.client else { return }
        rowStates[folio.id] = .working
        Task {
            do {
                let result = try await client.addPiece(toFolio: folio.id, photoID: photoID)
                rowStates[folio.id] = result.duplicate ? .duplicate : .added
                try? await Task.sleep(for: .milliseconds(600))
                dismiss()
            } catch {
                rowStates[folio.id] = .failed(String(describing: error))
            }
        }
    }
}
