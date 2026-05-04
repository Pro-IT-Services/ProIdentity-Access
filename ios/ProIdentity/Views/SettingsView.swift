import SwiftUI

struct SettingsView: View {
    @EnvironmentObject var appState: AppState
    @Environment(\.dismiss) private var dismiss

    private var isManaged: Bool { AppSettings.shared.mode == "managed" }
    @State private var showLogoutConfirm = false
    @State private var showResetConfirm = false

    var body: some View {
        NavigationView {
            ZStack {
                Color.appBg.ignoresSafeArea()
                List {
                    if isManaged {
                        Section("Account") {
                            row("Username", value: AppSettings.shared.username)
                            row("Server", value: AppSettings.shared.serverURL)
                        }
                        .listRowBackground(Color.appSurface)

                        Section {
                            Button("Sign Out", role: .destructive) {
                                showLogoutConfirm = true
                            }
                        }
                        .listRowBackground(Color.appSurface)
                    }

                    Section {
                        Button("Reset App", role: .destructive) {
                            showResetConfirm = true
                        }
                    } footer: {
                        Text("Removes all settings and returns to the setup screen.")
                            .foregroundColor(.appGray)
                    }
                    .listRowBackground(Color.appSurface)

                    Section("About") {
                        row("Mode", value: isManaged ? "Managed" : "Standalone")
                        row("Version", value: Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0")
                    }
                    .listRowBackground(Color.appSurface)
                }
                .listStyle(.insetGrouped)
                .scrollContentBackground(.hidden)
            }
            .navigationTitle("Settings")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button("Done") { dismiss() }.tint(.appAccent)
                }
            }
            .confirmationDialog(
                "Sign out of \(AppSettings.shared.username)?",
                isPresented: $showLogoutConfirm,
                titleVisibility: .visible
            ) {
                Button("Sign Out", role: .destructive) {
                    dismiss()
                    Task {
                        if let key = try? ManagedClient.shared.aesKey() {
                            try? await ManagedClient.shared.deleteAuthSession(aesKey: key)
                        }
                        appState.signOut()
                    }
                }
                Button("Cancel", role: .cancel) {}
            }
            .confirmationDialog(
                "Reset all data?",
                isPresented: $showResetConfirm,
                titleVisibility: .visible
            ) {
                Button("Reset", role: .destructive) {
                    dismiss()
                    appState.resetAll()
                }
                Button("Cancel", role: .cancel) {}
            }
        }
        .navigationViewStyle(.stack)
    }

    private func row(_ label: String, value: String) -> some View {
        HStack {
            Text(label).foregroundColor(.white)
            Spacer()
            Text(value)
                .foregroundColor(.appGray)
                .lineLimit(1)
                .truncationMode(.middle)
                .frame(maxWidth: 200, alignment: .trailing)
        }
    }
}
