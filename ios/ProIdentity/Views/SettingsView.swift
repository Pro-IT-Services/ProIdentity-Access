import SwiftUI

struct SettingsView: View {
    @Environment(AppModel.self) private var app
    @Environment(\.dismiss) private var dismiss

    @State private var confirmSignOut = false
    @State private var confirmReset = false

    private let settings = AppSettings.shared

    private var version: String {
        let info = Bundle.main.infoDictionary
        let short = info?["CFBundleShortVersionString"] as? String ?? "—"
        let build = info?["CFBundleVersion"] as? String ?? ""
        return build.isEmpty ? short : "\(short) (\(build))"
    }

    private var serverHost: String {
        URL(string: settings.serverURL)?.host() ?? settings.serverURL
    }

    var body: some View {
        NavigationStack {
            Form {
                if app.isManaged {
                    Section("Account") {
                        InfoRow(label: "Signed in as", value: app.connections.isSignedIn ? settings.username : "Signed out")
                        InfoRow(label: "Server", value: serverHost)
                        if !settings.vpnName.isEmpty {
                            InfoRow(label: "Organization", value: settings.vpnName)
                        }
                    }
                    .listRowBackground(Brand.surface)

                    Section {
                        if app.connections.isSignedIn {
                            Button("Sign out", role: .destructive) { confirmSignOut = true }
                        } else {
                            Button("Sign in") {
                                dismiss()
                                app.showSignIn()
                            }
                        }
                    } footer: {
                        Text("Signing out disconnects server connections. Imported configurations stay on this device.")
                    }
                    .listRowBackground(Brand.surface)
                }

                Section("This device") {
                    InfoRow(label: "Mode", value: app.isManaged ? "Organization" : "Configuration files")
                    InfoRow(label: "Configurations", value: "\(app.connections.imported.count) imported")
                }
                .listRowBackground(Brand.surface)

                Section("About") {
                    InfoRow(label: "Version", value: version)
                    Link(destination: URL(string: "https://access.proidentity.cloud/")!) {
                        LabeledContent("Website") {
                            Image(systemName: "arrow.up.right.square").foregroundStyle(.secondary)
                        }
                    }
                    Link(destination: URL(string: "https://access.proidentity.cloud/privacy")!) {
                        LabeledContent("Privacy policy") {
                            Image(systemName: "arrow.up.right.square").foregroundStyle(.secondary)
                        }
                    }
                    NavigationLink("Open-source licenses") { LicensesView() }
                }
                .listRowBackground(Brand.surface)

                Section {
                    Button("Reset app", role: .destructive) { confirmReset = true }
                } footer: {
                    Text("Removes all connections, the device registration and settings, and returns to setup.")
                }
                .listRowBackground(Brand.surface)
            }
            .scrollContentBackground(.hidden)
            .background(Brand.background.ignoresSafeArea())
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { dismiss() }
                }
            }
            .confirmationDialog(
                "Sign out of \(settings.username)?",
                isPresented: $confirmSignOut,
                titleVisibility: .visible
            ) {
                Button("Sign out", role: .destructive) {
                    dismiss()
                    Task { await app.signOut() }
                }
                Button("Cancel", role: .cancel) {}
            } message: {
                Text("Server connections will be disconnected.")
            }
            .confirmationDialog(
                "Reset the app?",
                isPresented: $confirmReset,
                titleVisibility: .visible
            ) {
                Button("Reset", role: .destructive) {
                    dismiss()
                    app.resetAll()
                }
                Button("Cancel", role: .cancel) {}
            } message: {
                Text("All connections and settings are removed from this device. This can't be undone.")
            }
        }
        .tint(Brand.primary)
    }
}
