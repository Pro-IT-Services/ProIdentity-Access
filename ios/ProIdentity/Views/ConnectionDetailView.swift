import SwiftUI

/// Status, traffic and addressing for one connection. Keys are never shown.
struct ConnectionDetailView: View {
    @Environment(AppModel.self) private var app
    let connectionID: String
    /// Called after the connection was deleted, so the container can navigate away.
    let onDeleted: () -> Void

    @State private var confirmDelete = false

    private var model: ConnectionsModel { app.connections }

    var body: some View {
        if let c = model.connection(id: connectionID) {
            details(c)
        } else {
            ContentUnavailableView(
                "Connection unavailable",
                systemImage: "network.slash",
                description: Text("It was removed or is no longer assigned to you.")
            )
            .background(Brand.background)
        }
    }

    private func details(_ c: Connection) -> some View {
        let config = model.config(for: c)
        let server = model.servers.first { c.kind == .managed && $0.id == c.refID }

        return List {
            Section {
                VStack(spacing: 10) {
                    StatusAperture(status: c.status, size: 104)
                    Text(c.status.label)
                        .font(.headline)
                        .foregroundStyle(c.status == .disconnected ? Color.primary : Brand.color(for: c.status))
                    KindBadge(text: c.badge)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 8)
                .listRowBackground(Color.clear)
            }

            if c.status == .connected {
                Section("Session") {
                    if let since = model.connectedSince {
                        TimelineView(.periodic(from: .now, by: 1)) { context in
                            InfoRow(label: "Duration", value: Format.duration(context.date.timeIntervalSince(since)), mono: true)
                        }
                    }
                    InfoRow(label: "Received", value: Format.bytes(model.stats?.rxBytes ?? 0), mono: true)
                    InfoRow(label: "Sent", value: Format.bytes(model.stats?.txBytes ?? 0), mono: true)
                    if let handshake = model.stats?.lastHandshake {
                        TimelineView(.periodic(from: .now, by: 5)) { _ in
                            InfoRow(label: "Last handshake", value: Format.ago(handshake))
                        }
                    }
                }
                .listRowBackground(Brand.surface)
            }

            Section {
                InfoRow(label: "Type", value: c.kind == .managed
                        ? String(localized: "Managed by your organization")
                        : String(localized: "Imported configuration"))
                if let server, !server.location.isEmpty {
                    InfoRow(label: "Location", value: server.location)
                }
                if let config {
                    if !config.iface.addresses.isEmpty {
                        InfoRow(label: "Address", value: config.iface.addresses.joined(separator: "\n"), mono: true)
                    }
                    if !config.iface.dns.isEmpty {
                        InfoRow(label: "DNS", value: config.iface.dns.joined(separator: "\n"), mono: true)
                    }
                    if let endpoint = config.peers.first?.endpoint, !endpoint.isEmpty {
                        InfoRow(label: "Endpoint", value: endpoint, mono: true)
                    }
                    let routes = config.peers.flatMap(\.allowedIPs)
                    if !routes.isEmpty {
                        InfoRow(label: "Routes", value: routes.joined(separator: "\n"), mono: true)
                    }
                } else if let server, !server.subnet.isEmpty {
                    InfoRow(label: "Network", value: server.subnet, mono: true)
                }
            } header: {
                Text("Details")
            } footer: {
                if c.kind == .managed && c.status != .connected {
                    Text("A fresh key and address are issued each time you connect.")
                }
            }
            .listRowBackground(Brand.surface)

            if c.kind == .imported {
                Section {
                    Button("Delete configuration", role: .destructive) { confirmDelete = true }
                        .disabled(c.isActive)
                } footer: {
                    if c.isActive { Text("Disconnect before deleting.") }
                }
                .listRowBackground(Brand.surface)
            }
        }
        .scrollContentBackground(.hidden)
        .background(Brand.background.ignoresSafeArea())
        .navigationTitle(c.name)
        .navigationBarTitleDisplayMode(.inline)
        .safeAreaInset(edge: .bottom) {
            actionButton(c)
                .padding(.horizontal, 16)
                .padding(.vertical, 10)
                .frame(maxWidth: 640)
                .frame(maxWidth: .infinity)
                .background(.bar)
        }
        .confirmationDialog("Delete \(c.name)?", isPresented: $confirmDelete, titleVisibility: .visible) {
            Button("Delete", role: .destructive) {
                Task {
                    await model.deleteImported(c)
                    onDeleted()
                }
            }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("The configuration is removed from this device. This can't be undone.")
        }
    }

    @ViewBuilder
    private func actionButton(_ c: Connection) -> some View {
        switch c.status {
        case .connected:
            Button("Disconnect") { Task { await model.disconnect(c) } }
                .buttonStyle(FilledButtonStyle(color: Brand.disconnect))
        case .connecting:
            Button("Cancel") { Task { await model.disconnect(c) } }
                .buttonStyle(FilledButtonStyle(color: Brand.muted))
        default:
            let title: LocalizedStringKey = c.status == .error ? "Try again" : "Connect"
            Button(title) { Task { await model.connect(c) } }
                .buttonStyle(FilledButtonStyle())
        }
    }
}
