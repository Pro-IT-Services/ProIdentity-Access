import SwiftUI

extension Notification.Name {
    static let authExpired = Notification.Name("authExpired")
}

// MARK: - Models

struct ManagedServer: Identifiable {
    let id: String
    let name: String
    let location: String
    var isConnected: Bool
    var tunnelID: String

    init(_ d: [String: Any]) {
        id       = d["id"] as? String ?? ""
        name     = d["name"] as? String ?? d["hostname"] as? String ?? "Server"
        location = [d["city"], d["country"]].compactMap { $0 as? String }
                       .filter { !$0.isEmpty }.joined(separator: ", ")
        isConnected = d["connected"] as? Bool ?? false
        tunnelID    = d["tunnel_id"] as? String ?? ""
    }
}

enum ConnectError: LocalizedError {
    case requireTotp
    case requirePushAuth
    var errorDescription: String? {
        switch self {
        case .requireTotp: return "Two-factor authentication required"
        case .requirePushAuth: return "Push authentication required"
        }
    }
}

// MARK: - ServerManager

@MainActor
class ServerManager: ObservableObject {
    @Published var servers: [ManagedServer] = []
    @Published var isRefreshing = false
    @Published var error: String?

    private var activeSessions: [String: String] = [:]
    private var keepaliveTimers: [String: Timer] = [:]

    func refresh() async {
        isRefreshing = true; error = nil
        do {
            let key = try ManagedClient.shared.aesKey()
            let raw = try await ManagedClient.shared.listServers(aesKey: key)
            let activeIDs = Set(VPNManager.shared.activeServerIDs())
            servers = raw.map { d -> ManagedServer in
                var s = ManagedServer(d)
                s.isConnected = activeIDs.contains(s.id)
                s.tunnelID    = VPNManager.shared.tunnelIDForServer(s.id) ?? ""
                return s
            }
        } catch {
            if handleAuthFailure(error) {
                isRefreshing = false
                return
            }
            self.error = error.localizedDescription
        }
        isRefreshing = false
    }

    private var pollTimer: Timer?
    private var authCheckTimer: Timer?

    func startPolling() {
        pollTimer?.invalidate()
        pollTimer = Timer.scheduledTimer(withTimeInterval: 10, repeats: true) { [weak self] _ in
            Task { await self?.refresh() }
        }
        authCheckTimer?.invalidate()
        authCheckTimer = Timer.scheduledTimer(withTimeInterval: 10, repeats: true) { [weak self] _ in
            Task { await self?.checkAuth() }
        }
    }

    func stopPolling() {
        pollTimer?.invalidate(); pollTimer = nil
        authCheckTimer?.invalidate(); authCheckTimer = nil
    }

    private func checkAuth() async {
        guard let key = try? ManagedClient.shared.aesKey() else { return }
        do {
            try await ManagedClient.shared.checkAuth(aesKey: key)
        } catch {
            _ = handleAuthFailure(error)
        }
    }

    func connect(serverID: String, serverName: String, totp: String, pushAuthID: String = "") async throws {
        do {
            let key = try ManagedClient.shared.aesKey()
            let (wgPriv, wgPub) = DeviceCrypto.shared.generateKeyPair()

            let resp = try await ManagedClient.shared.createSession(
                serverID: serverID, clientPublicKey: wgPub, totpCode: totp, pushAuthID: pushAuthID, aesKey: key
            )
            if let rt = resp["require_totp"] as? Bool, rt {
                if resp["push_auth_enabled"] as? Bool == true {
                    throw ConnectError.requirePushAuth
                }
                throw ConnectError.requireTotp
            }
            guard let wgConfig = resp["wg_config"] as? String else {
                throw APIError.serverError("Missing wg_config in response")
            }
            guard let sessionID = resp["session_id"] as? String else {
                throw APIError.serverError("Missing session_id in response")
            }

            let configWithKey = injectPrivateKey(config: wgConfig, privateKey: wgPriv)
            let cfg = try VPNManager.shared.importManagedTunnel(
                name: serverName, configContent: configWithKey, serverID: serverID
            )
            try await VPNManager.shared.connectTunnel(id: cfg.id)

            activeSessions[serverID] = sessionID
            startKeepalive(serverID: serverID, sessionID: sessionID)
            await refresh()
        } catch {
            if handleAuthFailure(error) { return }
            throw error
        }
    }

    func disconnect(serverID: String) async {
        if let tunnelID = VPNManager.shared.tunnelIDForServer(serverID) {
            VPNManager.shared.disconnectTunnel(id: tunnelID)
        }
        keepaliveTimers[serverID]?.invalidate()
        keepaliveTimers.removeValue(forKey: serverID)
        if let sid = activeSessions[serverID], let key = try? ManagedClient.shared.aesKey() {
            await ManagedClient.shared.deleteSession(sessionID: sid, aesKey: key)
        }
        activeSessions.removeValue(forKey: serverID)
        await refresh()
    }

    private func startKeepalive(serverID: String, sessionID: String) {
        keepaliveTimers[serverID]?.invalidate()
        keepaliveTimers[serverID] = Timer.scheduledTimer(withTimeInterval: 25, repeats: true) { [weak self] _ in
            guard let self,
                  let sid = self.activeSessions[serverID],
                  let key = try? ManagedClient.shared.aesKey() else { return }
            Task {
                do {
                    try await ManagedClient.shared.keepalive(sessionID: sid, aesKey: key)
                } catch {
                    await MainActor.run { _ = self.handleAuthFailure(error) }
                }
            }
        }
    }

    @discardableResult
    func handleAuthFailure(_ error: Error) -> Bool {
        switch error {
        case APIError.deviceRevoked, APIError.authInvalid:
            stopPolling()
            self.error = "Login expired or revoked. Please set up again."
            NotificationCenter.default.post(name: .authExpired, object: nil)
            return true
        default:
            return false
        }
    }

    private func injectPrivateKey(config: String, privateKey: String) -> String {
        var lines = config.components(separatedBy: "\n")
        var inInterface = false
        var privKeyIndex = -1
        var interfaceIndex = -1
        for (i, line) in lines.enumerated() {
            let t = line.trimmingCharacters(in: .whitespaces)
            if t.lowercased() == "[interface]" { inInterface = true; interfaceIndex = i }
            else if t.hasPrefix("[") && i != interfaceIndex { inInterface = false }
            if inInterface && t.lowercased().hasPrefix("privatekey") { privKeyIndex = i; break }
        }
        if privKeyIndex >= 0 { lines[privKeyIndex] = "PrivateKey = \(privateKey)" }
        else if interfaceIndex >= 0 { lines.insert("PrivateKey = \(privateKey)", at: interfaceIndex + 1) }
        return lines.joined(separator: "\n")
    }
}

// MARK: - ServerListView

struct ServerListView: View {
    @EnvironmentObject var appState: AppState
    @StateObject private var mgr = ServerManager()

    @State private var actionLoading: [String: Bool] = [:]
    @State private var pendingConnect: (id: String, name: String)?
    @State private var showAuthSheet = false
    @State private var totpCode = ""
    @State private var authMode: AuthSheetMode = .totp
    @State private var pushRequestId: String?
    @State private var pushStatus = "idle"
    @State private var showSettings = false

    enum AuthSheetMode { case totp, push }

    var body: some View {
        NavigationView {
            ZStack {
                Color.appBg.ignoresSafeArea()
                content
            }
            .navigationTitle("Servers")
            .navigationBarTitleDisplayMode(.large)
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button { showSettings = true } label: {
                        Image(systemName: "person.circle")
                    }
                    .tint(.appAccent)
                }
            }
            .sheet(isPresented: $showSettings) {
                SettingsView()
            }
            .sheet(isPresented: $showAuthSheet, onDismiss: {
                if let id = pendingConnect?.id { actionLoading[id] = false }
                totpCode = ""
                pendingConnect = nil
                pushRequestId = nil
                pushStatus = "idle"
            }) {
                AuthSheet(
                    mode: $authMode,
                    code: $totpCode,
                    pushStatus: $pushStatus,
                    isLoading: actionLoading[pendingConnect?.id ?? ""] ?? false,
                    onSubmitTotp: {
                        guard let sv = pendingConnect else { return }
                        Task { await performConnect(serverID: sv.id, serverName: sv.name, totp: totpCode) }
                    },
                    onSwitchToTotp: { authMode = .totp },
                    onSwitchToPush: {
                        authMode = .push
                        guard let sv = pendingConnect else { return }
                        Task { await startPushAuth(serverID: sv.id, serverName: sv.name) }
                    },
                    onRetryPush: {
                        guard let sv = pendingConnect else { return }
                        Task { await startPushAuth(serverID: sv.id, serverName: sv.name) }
                    }
                )
            }
            .task {
                await mgr.refresh()
                mgr.startPolling()
            }
            .onDisappear { mgr.stopPolling() }
        }
        .navigationViewStyle(.stack)
    }

    @ViewBuilder
    private var content: some View {
        if mgr.isRefreshing && mgr.servers.isEmpty {
            ProgressView().tint(.appAccent)
        } else if mgr.servers.isEmpty && mgr.error == nil {
            emptyState
        } else {
            serverList
        }
    }

    private var serverList: some View {
        ScrollView {
            LazyVStack(spacing: 12) {
                if let err = mgr.error {
                    ErrorBanner(message: err).padding(.horizontal)
                }
                ForEach(mgr.servers) { server in
                    ServerCard(
                        server: server,
                        vpnStatus: vpnStatus(for: server),
                        isLoading: actionLoading[server.id] ?? false,
                        onConnect:    { Task { await performConnect(serverID: server.id, serverName: server.name, totp: "") } },
                        onDisconnect: { Task { await performDisconnect(server.id) } }
                    )
                    .padding(.horizontal)
                }
            }
            .padding(.vertical, 12)
        }
        .refreshable { await mgr.refresh() }
    }

    private var emptyState: some View {
        VStack(spacing: 14) {
            Image(systemName: "server.rack").font(.system(size: 46)).foregroundColor(.appGray)
            Text("No Servers").font(.title3.weight(.semibold)).foregroundColor(.white)
            Text("No servers are available on your account.")
                .font(.subheadline).foregroundColor(.appGray)
        }
    }

    private func vpnStatus(for server: ManagedServer) -> String {
        guard !server.tunnelID.isEmpty else {
            return server.isConnected ? "connected" : "disconnected"
        }
        return appState.tunnelStatus(server.tunnelID)
    }

    private func performConnect(serverID: String, serverName: String, totp: String, pushAuthID: String = "") async {
        actionLoading[serverID] = true
        do {
            try await mgr.connect(serverID: serverID, serverName: serverName, totp: totp, pushAuthID: pushAuthID)
            showAuthSheet = false
            totpCode = ""
            pendingConnect = nil
        } catch ConnectError.requirePushAuth {
            pendingConnect = (serverID, serverName)
            authMode = .push
            showAuthSheet = true
            await startPushAuth(serverID: serverID, serverName: serverName)
        } catch ConnectError.requireTotp {
            pendingConnect = (serverID, serverName)
            authMode = .totp
            showAuthSheet = true
        } catch {
            if mgr.handleAuthFailure(error) {
                actionLoading[serverID] = false
                return
            }
            mgr.error = error.localizedDescription
        }
        actionLoading[serverID] = false
    }

    private func startPushAuth(serverID: String, serverName: String) async {
        pushStatus = "pending"
        do {
            let key = try ManagedClient.shared.aesKey()
            let resp = try await ManagedClient.shared.createPushAuth(context: "Connect to \(serverName)", aesKey: key)
            guard let reqID = resp["request_id"] as? String else { return }
            pushRequestId = reqID

            // Poll every 2s
            while pushStatus == "pending" {
                try await Task.sleep(nanoseconds: 2_000_000_000)
                let status = try await ManagedClient.shared.pollPushStatus(requestID: reqID)
                await MainActor.run { pushStatus = status }
                if status == "approved" {
                    await performConnect(serverID: serverID, serverName: serverName, totp: "", pushAuthID: reqID)
                    return
                }
            }
        } catch {
            if mgr.handleAuthFailure(error) {
                return
            }
            mgr.error = error.localizedDescription
        }
    }

    private func performDisconnect(_ serverID: String) async {
        actionLoading[serverID] = true
        await mgr.disconnect(serverID: serverID)
        actionLoading[serverID] = false
    }
}

// MARK: - Server Card

struct ServerCard: View {
    let server: ManagedServer
    let vpnStatus: String
    let isLoading: Bool
    let onConnect: () -> Void
    let onDisconnect: () -> Void

    private var isConnected: Bool { vpnStatus == "connected" }
    private var isActive: Bool { vpnStatus == "connected" || vpnStatus == "connecting" }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(server.name).font(.headline).foregroundColor(.white)
                    if !server.location.isEmpty {
                        Text(server.location).font(.caption).foregroundColor(.appGray)
                    }
                }
                Spacer()
                StatusBadge(status: vpnStatus)
            }

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

// MARK: - Auth Sheet (Push + TOTP dual mode)

struct AuthSheet: View {
    @Environment(\.dismiss) private var dismiss
    @Binding var mode: ServerListView.AuthSheetMode
    @Binding var code: String
    @Binding var pushStatus: String
    let isLoading: Bool
    let onSubmitTotp: () -> Void
    let onSwitchToTotp: () -> Void
    let onSwitchToPush: () -> Void
    let onRetryPush: () -> Void

    var body: some View {
        NavigationView {
            ZStack {
                Color.appBg.ignoresSafeArea()
                VStack(spacing: 28) {
                    if mode == .push {
                        pushContent
                    } else {
                        totpContent
                    }
                    Spacer()
                }
                .padding(.top, 40)
            }
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .navigationBarLeading) {
                    Button("Cancel") { dismiss() }.foregroundColor(.appGray)
                }
            }
        }
        .presentationDetents([.medium])
        .navigationViewStyle(.stack)
    }

    private var pushContent: some View {
        VStack(spacing: 16) {
            VStack(spacing: 10) {
                Image(systemName: pushStatus == "approved" ? "checkmark.shield.fill" : "iphone.radiowaves.left.and.right")
                    .font(.system(size: 46))
                    .foregroundColor(pushStatus == "approved" ? .appGreen : .appAccent)
                    .opacity(pushStatus == "pending" ? 0.7 : 1.0)
                    .animation(.easeInOut(duration: 0.8).repeatForever(autoreverses: true), value: pushStatus == "pending")
                Text(pushStatus == "pending" ? "Waiting for approval…" :
                     pushStatus == "approved" ? "Approved — connecting…" :
                     pushStatus == "denied" ? "Denied" : "Expired")
                    .font(.title2.weight(.bold)).foregroundColor(.white)
                Text(pushStatus == "pending" ? "Check your phone for a push notification." :
                     pushStatus == "denied" ? "The request was denied." :
                     pushStatus == "expired" ? "The request expired." : "")
                    .font(.subheadline).foregroundColor(.appGray)
                    .multilineTextAlignment(.center)
            }
            if pushStatus == "denied" || pushStatus == "expired" {
                PrimaryButton("Try again", loading: false, action: onRetryPush)
                    .padding(.horizontal)
            }
            if pushStatus != "approved" {
                Button { onSwitchToTotp() } label: {
                    Label("Enter code manually", systemImage: "number.circle")
                        .font(.caption).foregroundColor(.appGray)
                }
            }
        }
    }

    private var totpContent: some View {
        VStack(spacing: 28) {
            VStack(spacing: 10) {
                Image(systemName: "lock.shield.fill")
                    .font(.system(size: 46)).foregroundColor(.appAccent)
                Text("Two-Factor Auth")
                    .font(.title2.weight(.bold)).foregroundColor(.white)
                Text("Enter the 6-digit code from your authenticator app.")
                    .font(.subheadline).foregroundColor(.appGray)
                    .multilineTextAlignment(.center)
            }

            TextField("000000", text: $code)
                .textFieldStyle(AppFieldStyle())
                .keyboardType(.numberPad)
                .textContentType(.oneTimeCode)
                .font(.title2.monospacedDigit())
                .multilineTextAlignment(.center)
                .padding(.horizontal)

            PrimaryButton("Verify", loading: isLoading, action: onSubmitTotp)
                .padding(.horizontal)

            Button { onSwitchToPush() } label: {
                Label("Use push notification", systemImage: "iphone.radiowaves.left.and.right")
                    .font(.caption).foregroundColor(.appGray)
            }
        }
    }
}
