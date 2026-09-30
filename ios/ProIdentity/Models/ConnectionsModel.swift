import Foundation
import Observation

/// Everything Home shows: managed servers, imported tunnels, live state,
/// connect/disconnect (with two-factor), keepalive, polling and stats.
@MainActor
@Observable
final class ConnectionsModel {
    private(set) var servers: [ManagedServer] = []
    private(set) var imported: [WireGuardConfig] = []
    private(set) var tunnelStates: [String: ConnStatus] = [:]
    private(set) var busy: Set<String> = []
    private(set) var failedKey: String?
    private(set) var isRefreshing = false
    private(set) var loadedOnce = false
    private(set) var stats: TunnelStats?
    private(set) var connectedSince: Date?
    /// Short, dismissible message for the last failure.
    var message: String?
    var authPrompt: AuthPrompt?

    /// Called on an auth failure: `true` = device revoked, `false` = session expired.
    @ObservationIgnored var onAuthFailure: ((Bool) -> Void)?

    @ObservationIgnored private var sessions: [String: String] = ConnectionsModel.loadSessions() {
        didSet { UserDefaults.standard.set(sessions, forKey: Self.sessionsKey) }
    }
    @ObservationIgnored private var keepaliveTasks: [String: Task<Void, Never>] = [:]
    @ObservationIgnored private var pollTask: Task<Void, Never>?
    @ObservationIgnored private var statsTask: Task<Void, Never>?
    @ObservationIgnored private var pushTask: Task<Void, Never>?
    @ObservationIgnored private var restored = false

    private static let sessionsKey = "managed_sessions"

    var isManaged: Bool { AppSettings.shared.mode == "managed" }
    var isSignedIn: Bool { !AppSettings.shared.token.isEmpty }

    // MARK: - Derived state

    var connections: [Connection] {
        let serverRows = servers.map { s -> Connection in
            let key = Connection.key(server: s.id)
            let tunnelID = VPNManager.shared.tunnelIDForServer(s.id)
            return Connection(
                id: key, kind: .managed, refID: s.id, name: s.name,
                detail: s.location.isEmpty ? s.subnet : s.location,
                tunnelID: tunnelID, status: status(key: key, tunnelID: tunnelID)
            )
        }
        let tunnelRows = imported.map { t -> Connection in
            let key = Connection.key(tunnel: t.id)
            return Connection(
                id: key, kind: .imported, refID: t.id, name: t.name,
                detail: t.iface.addresses.first ?? "",
                tunnelID: t.id, status: status(key: key, tunnelID: t.id)
            )
        }
        return serverRows + tunnelRows
    }

    /// The connection that is up or coming up, if any.
    var active: Connection? {
        let all = connections
        return all.first { $0.status == .connected } ?? all.first { $0.status == .connecting }
    }

    /// What the big button acts on: the active one, else the last used, else the first.
    var primary: Connection? {
        let all = connections
        if let a = all.first(where: { $0.isActive }) { return a }
        let last = AppSettings.shared.lastConnection
        return all.first { $0.id == last } ?? all.first
    }

    func connection(id: String) -> Connection? { connections.first { $0.id == id } }

    func config(for connection: Connection) -> WireGuardConfig? {
        guard let tid = connection.tunnelID else { return nil }
        return VPNManager.shared.tunnelConfigs.first { $0.id == tid }
    }

    private func status(key: String, tunnelID: String?) -> ConnStatus {
        if let tid = tunnelID, let s = tunnelStates[tid], s != .disconnected { return s }
        if busy.contains(key) { return .connecting }
        if failedKey == key { return .error }
        return .disconnected
    }

    // MARK: - Lifecycle

    /// Called when Home appears / the app returns to the foreground.
    func start() async {
        reloadImported()
        if !restored {
            restored = true
            await VPNManager.shared.restoreState()
            resumeKeepalives()
        }
        await refresh()
        startLoops()
    }

    /// Called when the app goes to the background.
    func pause() {
        pollTask?.cancel(); pollTask = nil
        statsTask?.cancel(); statsTask = nil
    }

    func tunnelStateChanged(id: String, raw: String) {
        let state = ConnStatus(raw)
        let previous = tunnelStates[id]
        tunnelStates[id] = state
        if state == .connected {
            connectedSince = VPNManager.shared.connectedDate(id: id) ?? Date()
            failedKey = nil
        }
        // The tunnel went down on its own (or from iOS Settings): end the
        // server session so it doesn't linger until it times out.
        if state == .disconnected, previous == .connected,
           let serverID = sessions.first(where: { VPNManager.shared.tunnelIDForServer($0.key) == id })?.key,
           !busy.contains(Connection.key(server: serverID)) {
            Task { await endServerSession(serverID) }
        }
        if active == nil { stats = nil; connectedSince = nil }
    }

    private func startLoops() {
        pollTask?.cancel()
        pollTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(10))
                guard let self, !Task.isCancelled else { return }
                await self.checkAuth()
                await self.refresh(silently: true)
            }
        }
        statsTask?.cancel()
        statsTask = Task { [weak self] in
            while !Task.isCancelled {
                guard let self else { return }
                await self.updateStats()
                try? await Task.sleep(for: .seconds(2))
            }
        }
    }

    private func updateStats() async {
        guard let a = active, a.status == .connected, let tid = a.tunnelID else {
            stats = nil
            return
        }
        connectedSince = VPNManager.shared.connectedDate(id: tid) ?? connectedSince
        if let s = await VPNManager.shared.fetchStats(id: tid) { stats = s }
    }

    // MARK: - Refresh

    func reloadImported() {
        imported = VPNManager.shared.importedConfigs
    }

    func refresh(silently: Bool = false) async {
        reloadImported()
        guard isManaged, isSignedIn else {
            servers = []
            loadedOnce = true
            return
        }
        if !silently { isRefreshing = true }
        defer { isRefreshing = false; loadedOnce = true }
        do {
            let key = try ManagedClient.shared.aesKey()
            let raw = try await ManagedClient.shared.listServers(aesKey: key)
            servers = raw.map(ManagedServer.init).filter { !$0.id.isEmpty }
        } catch {
            if handleAuthFailure(error) { return }
            if !silently { message = UserMessage.from(error) }
        }
    }

    private func checkAuth() async {
        guard isManaged, isSignedIn, let key = try? ManagedClient.shared.aesKey() else { return }
        do {
            try await ManagedClient.shared.checkAuth(aesKey: key)
        } catch {
            // Transient network errors are ignored; only auth failures end the session.
            _ = handleAuthFailure(error)
        }
    }

    // MARK: - Connect / disconnect

    func connect(_ c: Connection) async {
        message = nil
        failedKey = nil
        // iOS runs one VPN at a time: hand over cleanly.
        if let current = active, current.id != c.id { await disconnect(current) }
        switch c.kind {
        case .managed:
            await connectServer(id: c.refID, name: c.name)
        case .imported:
            busy.insert(c.id)
            defer { busy.remove(c.id) }
            do {
                try await VPNManager.shared.connectTunnel(id: c.refID)
                AppSettings.shared.lastConnection = c.id
            } catch {
                fail(c.id, error)
            }
        }
    }

    func disconnect(_ c: Connection) async {
        switch c.kind {
        case .managed:
            busy.insert(c.id)
            defer { busy.remove(c.id) }
            await endServerSession(c.refID)
        case .imported:
            VPNManager.shared.disconnectTunnel(id: c.refID)
        }
    }

    private func connectServer(id: String, name: String, totp: String = "", pushAuthID: String = "") async {
        let key = Connection.key(server: id)
        busy.insert(key)
        defer { busy.remove(key) }
        do {
            try await startServerSession(serverID: id, serverName: name, totp: totp, pushAuthID: pushAuthID)
            AppSettings.shared.lastConnection = key
            pushTask?.cancel()
            authPrompt = nil
        } catch ConnectError.requirePushAuth {
            if authPrompt == nil {
                authPrompt = AuthPrompt(serverID: id, serverName: name, method: .push)
                startPush()
            } else {
                authPrompt?.submitting = false
            }
        } catch ConnectError.requireTotp {
            if authPrompt == nil {
                authPrompt = AuthPrompt(serverID: id, serverName: name, method: .code)
            } else {
                // A code was sent and rejected.
                authPrompt?.submitting = false
                authPrompt?.code = ""
                authPrompt?.error = String(localized: "That code didn't work. Check your authenticator and try again.")
            }
        } catch {
            if handleAuthFailure(error) { return }
            if authPrompt != nil {
                authPrompt?.submitting = false
                authPrompt?.error = UserMessage.from(error)
            } else {
                fail(key, error)
            }
        }
    }

    private func startServerSession(serverID: String, serverName: String, totp: String, pushAuthID: String) async throws {
        let key = try ManagedClient.shared.aesKey()
        let (wgPriv, wgPub) = DeviceCrypto.shared.generateKeyPair()
        let resp = try await ManagedClient.shared.createSession(
            serverID: serverID, clientPublicKey: wgPub, totpCode: totp, pushAuthID: pushAuthID, aesKey: key
        )
        if resp["require_totp"] as? Bool == true {
            throw resp["push_auth_enabled"] as? Bool == true ? ConnectError.requirePushAuth : ConnectError.requireTotp
        }
        guard let wgConfig = resp["wg_config"] as? String else {
            throw APIError.serverError(String(localized: "The server didn't send a configuration."))
        }
        guard let sessionID = resp["session_id"] as? String else {
            throw APIError.serverError(String(localized: "The server didn't start a session."))
        }
        let config = Self.injectPrivateKey(config: wgConfig, privateKey: wgPriv)
        let endpoints = (resp["endpoints"] as? [[String: Any]] ?? []).compactMap(EndpointCandidate.init)
        do {
            try await connectEndpointCandidates(name: serverName, configContent: config, serverID: serverID, endpoints: endpoints)
        } catch {
            await ManagedClient.shared.deleteSession(sessionID: sessionID, aesKey: key)
            throw error
        }
        sessions[serverID] = sessionID
        startKeepalive(serverID: serverID)
    }

    private func endServerSession(_ serverID: String) async {
        if let tunnelID = VPNManager.shared.tunnelIDForServer(serverID) {
            VPNManager.shared.disconnectTunnel(id: tunnelID)
        }
        keepaliveTasks.removeValue(forKey: serverID)?.cancel()
        // One-time config: its keys are gone with the session.
        VPNManager.shared.forgetManagedTunnel(serverID: serverID)
        if let sid = sessions.removeValue(forKey: serverID), let key = try? ManagedClient.shared.aesKey() {
            await ManagedClient.shared.deleteSession(sessionID: sid, aesKey: key)
        }
    }

    private func fail(_ key: String, _ error: Error) {
        failedKey = key
        message = UserMessage.from(error)
    }

    // MARK: - Two-factor prompt

    func submitCode() async {
        guard var prompt = authPrompt else { return }
        let code = prompt.code.filter(\.isNumber)
        guard code.count == 6 else {
            authPrompt?.error = String(localized: "Enter the 6-digit code.")
            return
        }
        prompt.submitting = true
        prompt.error = nil
        authPrompt = prompt
        await connectServer(id: prompt.serverID, name: prompt.serverName, totp: code)
    }

    func startPush() {
        guard let prompt = authPrompt else { return }
        pushTask?.cancel()
        authPrompt?.method = .push
        authPrompt?.push = .pending
        authPrompt?.error = nil
        pushTask = Task { [weak self] in
            do {
                let key = try ManagedClient.shared.aesKey()
                let resp = try await ManagedClient.shared.createPushAuth(context: "Connect to \(prompt.serverName)", aesKey: key)
                guard let requestID = resp["request_id"] as? String else {
                    throw APIError.serverError(String(localized: "Push approval isn't available right now."))
                }
                let deadline = Date().addingTimeInterval(180)
                while !Task.isCancelled {
                    try await Task.sleep(for: .seconds(2))
                    guard !Task.isCancelled, let self else { return }
                    var state = PushState.expired
                    if Date() < deadline {
                        state = PushState(try await ManagedClient.shared.pollPushStatus(requestID: requestID))
                    }
                    guard !Task.isCancelled, self.authPrompt?.serverID == prompt.serverID else { return }
                    self.authPrompt?.push = state
                    if state == .approved {
                        self.authPrompt?.submitting = true
                        await self.connectServer(id: prompt.serverID, name: prompt.serverName, pushAuthID: requestID)
                        return
                    }
                    if state != .pending { return }
                }
            } catch is CancellationError {
                return
            } catch {
                guard let self, !Task.isCancelled else { return }
                if self.handleAuthFailure(error) { return }
                self.authPrompt?.push = .idle
                self.authPrompt?.error = UserMessage.from(error)
            }
        }
    }

    func useCode() {
        pushTask?.cancel()
        authPrompt?.method = .code
        authPrompt?.error = nil
    }

    func dismissAuth() {
        pushTask?.cancel()
        authPrompt = nil
    }

    // MARK: - Imported tunnels

    @discardableResult
    func importConfig(name: String, content: String) throws -> String {
        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let base = trimmed.isEmpty ? String(localized: "Imported tunnel") : trimmed
        let taken = Set(imported.map(\.name))
        var finalName = base
        var n = 2
        while taken.contains(finalName) { finalName = "\(base) \(n)"; n += 1 }
        let dict = try VPNManager.shared.importTunnel(name: finalName, configContent: content)
        reloadImported()
        let id = dict["id"] as? String ?? ""
        return Connection.key(tunnel: id)
    }

    func deleteImported(_ c: Connection) async {
        guard c.kind == .imported else { return }
        if c.isActive { VPNManager.shared.disconnectTunnel(id: c.refID) }
        do {
            try await VPNManager.shared.deleteTunnel(id: c.refID)
        } catch {
            message = UserMessage.from(error)
        }
        tunnelStates.removeValue(forKey: c.refID)
        if AppSettings.shared.lastConnection == c.id { AppSettings.shared.lastConnection = "" }
        reloadImported()
    }

    // MARK: - Sessions & keepalive

    private func startKeepalive(serverID: String) {
        keepaliveTasks[serverID]?.cancel()
        keepaliveTasks[serverID] = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(25))
                guard let self, !Task.isCancelled,
                      let sid = self.sessions[serverID],
                      let key = try? ManagedClient.shared.aesKey() else { return }
                do {
                    try await ManagedClient.shared.keepalive(sessionID: sid, aesKey: key)
                } catch {
                    // A dropped network is fine; retry on the next tick.
                    if self.handleAuthFailure(error) { return }
                }
            }
        }
    }

    /// After a relaunch, keep sessions whose tunnel is still up alive and
    /// close the ones whose tunnel is gone.
    private func resumeKeepalives() {
        for (serverID, sessionID) in sessions {
            let tid = VPNManager.shared.tunnelIDForServer(serverID)
            if let tid, VPNManager.shared.status(id: tid) == "connected" {
                startKeepalive(serverID: serverID)
            } else {
                sessions.removeValue(forKey: serverID)
                VPNManager.shared.forgetManagedTunnel(serverID: serverID)
                if let key = try? ManagedClient.shared.aesKey() {
                    Task { await ManagedClient.shared.deleteSession(sessionID: sessionID, aesKey: key) }
                }
            }
        }
    }

    /// Sign-out / expiry: close sessions, remove server tunnels, keep imported ones.
    func endManagedSessions(notifyServer: Bool) async {
        pushTask?.cancel()
        authPrompt = nil
        keepaliveTasks.values.forEach { $0.cancel() }
        keepaliveTasks.removeAll()
        if notifyServer, let key = try? ManagedClient.shared.aesKey() {
            for sid in sessions.values {
                await ManagedClient.shared.deleteSession(sessionID: sid, aesKey: key)
            }
        }
        sessions.removeAll()
        await VPNManager.shared.removeManagedTunnels()
        servers = []
        failedKey = nil
        if AppSettings.shared.lastConnection.hasPrefix("server:") { AppSettings.shared.lastConnection = "" }
    }

    /// Full reset: forget everything in memory.
    func teardown() {
        pause()
        pushTask?.cancel()
        keepaliveTasks.values.forEach { $0.cancel() }
        keepaliveTasks.removeAll()
        sessions.removeAll()
        servers = []
        imported = []
        tunnelStates = [:]
        busy = []
        failedKey = nil
        message = nil
        authPrompt = nil
        stats = nil
        connectedSince = nil
        loadedOnce = false
        restored = false
    }

    @discardableResult
    private func handleAuthFailure(_ error: Error) -> Bool {
        switch error {
        case APIError.deviceRevoked:
            onAuthFailure?(true)
            return true
        case APIError.authInvalid:
            onAuthFailure?(false)
            return true
        default:
            return false
        }
    }

    // MARK: - Config helpers

    private func connectEndpointCandidates(name: String, configContent: String, serverID: String, endpoints: [EndpointCandidate]) async throws {
        var lastError: Error?
        for candidate in Self.endpointConfigs(base: configContent, endpoints: endpoints) {
            do {
                let cfg = try VPNManager.shared.importManagedTunnel(name: name, configContent: candidate, serverID: serverID)
                try await VPNManager.shared.connectTunnel(id: cfg.id)
                return
            } catch {
                lastError = error
            }
        }
        VPNManager.shared.forgetManagedTunnel(serverID: serverID)
        throw lastError ?? APIError.serverError(String(localized: "None of the server's endpoints could be reached."))
    }

    private static func injectPrivateKey(config: String, privateKey: String) -> String {
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

    private static func endpointConfigs(base: String, endpoints: [EndpointCandidate]) -> [String] {
        var seen = Set<String>()
        let configs = endpoints.compactMap { ep -> String? in
            let endpoint = ep.wireGuardEndpoint
            guard !endpoint.isEmpty, seen.insert(endpoint).inserted else { return nil }
            return replaceEndpoint(config: base, endpoint: endpoint)
        }
        return configs.isEmpty ? [base] : configs
    }

    private static func replaceEndpoint(config: String, endpoint: String) -> String {
        var lines = config.components(separatedBy: "\n")
        for (i, line) in lines.enumerated()
        where line.trimmingCharacters(in: .whitespaces).lowercased().hasPrefix("endpoint") {
            lines[i] = "Endpoint = \(endpoint)"
            return lines.joined(separator: "\n")
        }
        return config
    }

    private static func loadSessions() -> [String: String] {
        UserDefaults.standard.dictionary(forKey: sessionsKey) as? [String: String] ?? [:]
    }
}
