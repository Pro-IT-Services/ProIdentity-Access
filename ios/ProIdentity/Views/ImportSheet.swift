import SwiftUI
import UniformTypeIdentifiers

/// Add a WireGuard configuration from a file or pasted text.
struct ImportSheet: View {
    let model: ConnectionsModel
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var content = ""
    @State private var error: String?
    @State private var showFilePicker = false

    private static let confType = UTType(filenameExtension: "conf") ?? .data

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Button {
                        showFilePicker = true
                    } label: {
                        Label("Choose a file…", systemImage: "folder")
                    }
                    PasteButton(payloadType: String.self) { strings in
                        guard let text = strings.first else { return }
                        Task { @MainActor in load(text: text, suggestedName: nil) }
                    }
                    .labelStyle(.titleAndIcon)
                } footer: {
                    Text("Use a .conf file from your VPN provider or administrator.")
                }
                .listRowBackground(Brand.surface)

                Section("Name") {
                    TextField("e.g. Office", text: $name)
                        .textInputAutocapitalization(.words)
                }
                .listRowBackground(Brand.surface)

                Section {
                    TextEditor(text: $content)
                        .font(.monoCaption)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .frame(minHeight: 180)
                        .overlay(alignment: .topLeading) {
                            if content.isEmpty {
                                Text("[Interface]\nPrivateKey = …\nAddress = …\n\n[Peer]\n…")
                                    .font(.monoCaption)
                                    .foregroundStyle(.tertiary)
                                    .padding(.top, 8).padding(.leading, 5)
                                    .allowsHitTesting(false)
                            }
                        }
                        .accessibilityLabel("Configuration text")
                } header: {
                    Text("Configuration")
                } footer: {
                    Text("Stays on this device and is stored in the Keychain.")
                }
                .listRowBackground(Brand.surface)

                if let error {
                    Section {
                        Label(error, systemImage: "exclamationmark.triangle.fill")
                            .foregroundStyle(Brand.danger)
                            .font(.subheadline)
                    }
                    .listRowBackground(Brand.surface)
                }
            }
            .scrollContentBackground(.hidden)
            .background(Brand.background.ignoresSafeArea())
            .navigationTitle("Import configuration")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Import") { save() }
                        .disabled(content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
            .fileImporter(
                isPresented: $showFilePicker,
                allowedContentTypes: [Self.confType, .plainText, .data],
                allowsMultipleSelection: false,
                onCompletion: handleFile
            )
        }
        .tint(Brand.primary)
    }

    private func handleFile(_ result: Result<[URL], Error>) {
        switch result {
        case .failure(let err):
            error = err.localizedDescription
        case .success(let urls):
            guard let url = urls.first else { return }
            let scoped = url.startAccessingSecurityScopedResource()
            defer { if scoped { url.stopAccessingSecurityScopedResource() } }
            guard let text = try? String(contentsOf: url, encoding: .utf8) else {
                error = "This file couldn't be read."
                return
            }
            load(text: text, suggestedName: url.deletingPathExtension().lastPathComponent)
        }
    }

    private func load(text: String, suggestedName: String?) {
        error = nil
        content = text
        if let suggestedName, name.trimmingCharacters(in: .whitespaces).isEmpty {
            name = suggestedName
        }
        if WireGuardConfig.parse(name: "check", config: text) == nil {
            error = "This doesn't look like a WireGuard configuration."
        }
    }

    private func save() {
        guard WireGuardConfig.parse(name: "check", config: content) != nil else {
            error = "This doesn't look like a WireGuard configuration. It needs an [Interface] section with a PrivateKey."
            return
        }
        do {
            try model.importConfig(name: name, content: content)
            dismiss()
        } catch {
            self.error = UserMessage.from(error)
        }
    }
}
