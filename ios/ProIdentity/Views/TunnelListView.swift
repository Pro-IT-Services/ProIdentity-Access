import SwiftUI
import UniformTypeIdentifiers

struct TunnelListView: View {
    @EnvironmentObject var appState: AppState
    @State private var tunnels: [WireGuardConfig] = []
    @State private var actionLoading: [String: Bool] = [:]
    @State private var errorMsg: String?
    @State private var showImport = false
    @State private var showSettings = false

    var body: some View {
        NavigationView {
            ZStack {
                Color.appBg.ignoresSafeArea()
                if tunnels.isEmpty { emptyState } else { tunnelList }
            }
            .navigationTitle("Tunnels")
            .navigationBarTitleDisplayMode(.large)
            .toolbar {
                ToolbarItem(placement: .navigationBarLeading) {
                    Button { showSettings = true } label: {
                        Image(systemName: "gearshape")
                    }
                    .tint(.appAccent)
                }
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button { showImport = true } label: {
                        Image(systemName: "plus")
                    }
                    .tint(.appAccent)
                }
            }
            .fileImporter(
                isPresented: $showImport,
                allowedContentTypes: [.text, .data, UTType(filenameExtension: "conf") ?? .data],
                allowsMultipleSelection: false,
                onCompletion: handleImport
            )
            .sheet(isPresented: $showSettings) {
                SettingsView()
            }
            .onAppear { loadTunnels() }
        }
        .navigationViewStyle(.stack)
    }

    private var tunnelList: some View {
        ScrollView {
            LazyVStack(spacing: 12) {
                if let err = errorMsg {
                    ErrorBanner(message: err).padding(.horizontal)
                }
                ForEach(tunnels) { tunnel in
                    TunnelCard(
                        tunnel: tunnel,
                        status: appState.tunnelStatus(tunnel.id),
                        isLoading: actionLoading[tunnel.id] ?? false,
                        onConnect:    { Task { await connect(tunnel) } },
                        onDisconnect: { disconnect(tunnel) },
                        onDelete:     { Task { await delete(tunnel) } }
                    )
                    .padding(.horizontal)
                }
            }
            .padding(.vertical, 12)
        }
    }

    private var emptyState: some View {
        VStack(spacing: 16) {
            Image(systemName: "network.slash")
                .font(.system(size: 48)).foregroundColor(.appGray)
            Text("No Tunnels")
                .font(.title3.weight(.semibold)).foregroundColor(.white)
            Text("Import a .conf file to get started.")
                .font(.subheadline).foregroundColor(.appGray).multilineTextAlignment(.center)
            Button("Import Config") { showImport = true }
                .buttonStyle(.borderedProminent).tint(.appAccent)
        }
        .padding()
    }

    private func loadTunnels() {
        tunnels = VPNManager.shared.tunnelConfigs
    }

    private func connect(_ tunnel: WireGuardConfig) async {
        actionLoading[tunnel.id] = true; errorMsg = nil
        do {
            try await VPNManager.shared.connectTunnel(id: tunnel.id)
        } catch {
            errorMsg = error.localizedDescription
        }
        actionLoading[tunnel.id] = false
    }

    private func disconnect(_ tunnel: WireGuardConfig) {
        VPNManager.shared.disconnectTunnel(id: tunnel.id)
    }

    private func delete(_ tunnel: WireGuardConfig) async {
        do {
            try await VPNManager.shared.deleteTunnel(id: tunnel.id)
            loadTunnels()
        } catch {
            errorMsg = error.localizedDescription
        }
    }

    private func handleImport(_ result: Result<[URL], Error>) {
        switch result {
        case .failure(let err):
            errorMsg = err.localizedDescription
        case .success(let urls):
            guard let url = urls.first else { return }
            guard url.startAccessingSecurityScopedResource() else {
                errorMsg = "Access denied to file"
                return
            }
            defer { url.stopAccessingSecurityScopedResource() }
            guard let content = try? String(contentsOf: url, encoding: .utf8) else {
                errorMsg = "Could not read file"
                return
            }
            let name = url.deletingPathExtension().lastPathComponent
            do {
                _ = try VPNManager.shared.importTunnel(name: name, configContent: content)
                loadTunnels()
            } catch {
                errorMsg = error.localizedDescription
            }
        }
    }
}

// MARK: - Tunnel Card

struct TunnelCard: View {
    let tunnel: WireGuardConfig
    let status: String
    let isLoading: Bool
    let onConnect: () -> Void
    let onDisconnect: () -> Void
    let onDelete: () -> Void

    private var isConnected: Bool { status == "connected" }
    private var isActive: Bool { status == "connected" || status == "connecting" }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(tunnel.name).font(.headline).foregroundColor(.white)
                    if !tunnel.iface.addresses.isEmpty {
                        Text(tunnel.iface.addresses.joined(separator: ", "))
                            .font(.caption).foregroundColor(.appGray)
                    }
                }
                Spacer()
                StatusBadge(status: status)
            }

            if let ep = tunnel.peers.first?.endpoint, !ep.isEmpty {
                Text(ep)
                    .font(.caption2).foregroundColor(.appGray.opacity(0.7))
                    .lineLimit(1)
            }

            HStack(spacing: 8) {
                if isActive {
                    Button(action: onDisconnect) {
                        Label("Disconnect", systemImage: "stop.circle.fill")
                            .font(.subheadline.weight(.medium))
                            .frame(maxWidth: .infinity).padding(.vertical, 9)
                            .background(Color.appRed.opacity(0.14))
                            .foregroundColor(.appRed)
                            .cornerRadius(9)
                    }
                } else {
                    Button(action: onConnect) {
                        HStack(spacing: 6) {
                            if isLoading {
                                ProgressView().tint(.appAccent).scaleEffect(0.8)
                            } else {
                                Image(systemName: "play.circle.fill")
                            }
                            Text("Connect")
                        }
                        .font(.subheadline.weight(.medium))
                        .frame(maxWidth: .infinity).padding(.vertical, 9)
                        .background(Color.appAccent.opacity(0.14))
                        .foregroundColor(.appAccent)
                        .cornerRadius(9)
                    }
                    .disabled(isLoading)

                    Button(action: onDelete) {
                        Image(systemName: "trash.fill")
                            .frame(width: 40, height: 38)
                            .background(Color.appRed.opacity(0.1))
                            .foregroundColor(.appRed)
                            .cornerRadius(9)
                    }
                }
            }
        }
        .padding(16)
        .background(Color.appSurface)
        .cornerRadius(14)
        .overlay(
            RoundedRectangle(cornerRadius: 14)
                .stroke(isConnected ? Color.appGreen.opacity(0.3) : Color.white.opacity(0.07), lineWidth: 1)
        )
    }
}
