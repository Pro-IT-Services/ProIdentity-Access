import Foundation
import NetworkExtension
import Security

/// WireGuard tunnel manager using NETunnelProviderManager.
/// NOTE: Requires the "Network Extension" capability (com.apple.developer.networking.networkextension)
/// and a paid Apple Developer account. Without it, connect/disconnect will fail with a permission error.
/// Config storage and listing work without the entitlement.
class VPNManager {
    static let shared = VPNManager()

    private let storageKey = "wg_tunnels"
    private var providerManagers: [String: NETunnelProviderManager] = [:]
    private var observers: [String: NSObjectProtocol] = [:]
    var onStateChanged: ((String, String) -> Void)? // (tunnelID, state)

    // MARK: - Config storage (Keychain)
    private var configs: [WireGuardConfig] {
        get {
            if let data = keychainData(for: storageKey),
               let decoded = try? JSONDecoder().decode([WireGuardConfig].self, from: data) {
                return decoded
            }
            if let legacy = UserDefaults.standard.data(forKey: storageKey),
               let decoded = try? JSONDecoder().decode([WireGuardConfig].self, from: legacy) {
                setKeychainData(legacy, for: storageKey)
                UserDefaults.standard.removeObject(forKey: storageKey)
                return decoded
            }
            return []
        }
        set {
            if let data = try? JSONEncoder().encode(newValue) {
                setKeychainData(data, for: storageKey)
                UserDefaults.standard.removeObject(forKey: storageKey)
            }
        }
    }

    // MARK: - Public config access (for native UI)
    var tunnelConfigs: [WireGuardConfig] { configs }

    // MARK: - List tunnels
    func listTunnels() -> [[String: Any]] {
        let cfgs = configs
        let connectedIDs = Set(providerManagers.compactMap { (id, mgr) -> String? in
            mgr.connection.status == .connected ? id : nil
        })
        return cfgs.map { c in
            let status = connectedIDs.contains(c.id) ? "connected" : "disconnected"
            return tunnelToDict(c, status: status)
        }
    }

    // MARK: - Import tunnel
    func importTunnel(name: String, configContent: String) throws -> [String: Any] {
        guard let config = WireGuardConfig.parse(name: name, config: configContent) else {
            throw VPNError.invalidConfig
        }
        var current = configs
        current.append(config)
        configs = current
        return tunnelToDict(config, status: "disconnected")
    }

    // MARK: - Delete tunnel
    func deleteTunnel(id: String) async throws {
        stopObserving(id)
        if let mgr = providerManagers[id] {
            try? await mgr.removeFromPreferences()
            providerManagers.removeValue(forKey: id)
        } else if let mgr = try? await NETunnelProviderManager.loadAllFromPreferences()
                    .first(where: { Self.tunnelID(of: $0) == id }) {
            try? await mgr.removeFromPreferences()
        }
        configs = configs.filter { $0.id != id }
    }

    // MARK: - Connect
    func connectTunnel(id: String) async throws {
        guard let config = configs.first(where: { $0.id == id }) else {
            throw VPNError.tunnelNotFound
        }
        let mgr = try await loadOrCreateManager(for: config)
        guard let session = mgr.connection as? NETunnelProviderSession else {
            throw VPNError.vpnUnavailable
        }
        onStateChanged?(id, "connecting")
        try session.startTunnel(options: nil)
        observeManager(mgr, tunnelID: id)
    }

    // MARK: - Disconnect
    func disconnectTunnel(id: String) {
        if let mgr = providerManagers[id],
           let session = mgr.connection as? NETunnelProviderSession {
            session.stopTunnel()
        }
        onStateChanged?(id, "disconnected")
    }

    /// Stops a tunnel even when this process hasn't loaded its profile yet
    /// (the app launched in the background by the Live Activity button).
    func stopTunnel(id: String) async {
        if providerManagers[id] == nil,
           let managers = try? await NETunnelProviderManager.loadAllFromPreferences(),
           let mgr = managers.first(where: { Self.tunnelID(of: $0) == id }) {
            providerManagers[id] = mgr
            observeManager(mgr, tunnelID: id)
        }
        disconnectTunnel(id: id)
    }

    // MARK: - Live state

    func status(id: String) -> String {
        guard let mgr = providerManagers[id] else { return "disconnected" }
        return Self.statusString(mgr.connection.status)
    }

    /// When the system reports the tunnel came up (drives the session timer).
    func connectedDate(id: String) -> Date? {
        providerManagers[id]?.connection.connectedDate
    }

    /// Re-attach to tunnels the system still has — e.g. still connected after
    /// the app was relaunched — so the UI shows the real state.
    func restoreState() async {
        guard let managers = try? await NETunnelProviderManager.loadAllFromPreferences() else { return }
        let known = Set(configs.map(\.id))
        var connected = Set<String>()
        for mgr in managers {
            guard let id = Self.tunnelID(of: mgr), known.contains(id) else { continue }
            if mgr.connection.status == .connected {
                connected.insert(id)
                syncActivity(id: id, mgr: mgr)
            }
            guard providerManagers[id] == nil else { continue }
            providerManagers[id] = mgr
            observeManager(mgr, tunnelID: id)
            onStateChanged?(id, Self.statusString(mgr.connection.status))
        }
        await MainActor.run { ConnectionActivityController.reconcile(connectedTunnelIDs: connected) }
    }

    /// Mirrors a tunnel's state into its Live Activity (lock screen / Dynamic Island).
    private func syncActivity(id: String, mgr: NETunnelProviderManager) {
        let name = configs.first(where: { $0.id == id })?.name ?? mgr.localizedDescription ?? "VPN"
        let status = mgr.connection.status
        let since = mgr.connection.connectedDate
        Task { @MainActor in
            ConnectionActivityController.statusChanged(tunnelID: id, name: name, status: status, connectedDate: since)
        }
    }

    /// Live counters. The tunnel extension answers "stats" with the WireGuard
    /// runtime configuration (rx_bytes / tx_bytes / last_handshake_time_sec).
    func fetchStats(id: String) async -> TunnelStats? {
        guard let session = providerManagers[id]?.connection as? NETunnelProviderSession,
              session.status == .connected else { return nil }
        return await withCheckedContinuation { (cont: CheckedContinuation<TunnelStats?, Never>) in
            let once = ResumeOnce(cont)
            DispatchQueue.main.asyncAfter(deadline: .now() + 2) { once.resume(nil) }
            do {
                try session.sendProviderMessage(Data("stats".utf8)) { data in
                    once.resume(data.flatMap { String(data: $0, encoding: .utf8) }.map(Self.parseRuntime))
                }
            } catch {
                once.resume(nil)
            }
        }
    }

    private static func parseRuntime(_ uapi: String) -> TunnelStats {
        var rx: Int64 = 0, tx: Int64 = 0, handshake: Int64 = 0
        for line in uapi.split(separator: "\n") {
            let parts = line.split(separator: "=", maxSplits: 1)
            guard parts.count == 2 else { continue }
            let value = Int64(parts[1]) ?? 0
            switch parts[0] {
            case "rx_bytes": rx += value
            case "tx_bytes": tx += value
            case "last_handshake_time_sec": handshake = max(handshake, value)
            default: break
            }
        }
        return TunnelStats(
            rxBytes: rx, txBytes: tx,
            lastHandshake: handshake > 0 ? Date(timeIntervalSince1970: TimeInterval(handshake)) : nil
        )
    }

    private static func statusString(_ status: NEVPNStatus) -> String {
        switch status {
        case .connected:     return "connected"
        case .connecting:    return "connecting"
        case .reasserting:   return "connecting"
        case .invalid:       return "error"
        case .disconnecting, .disconnected: return "disconnected"
        @unknown default:    return "disconnected"
        }
    }

    // MARK: - Managed tunnel (from server config)
    func importManagedTunnel(name: String, configContent: String, serverID: String) throws -> WireGuardConfig {
        guard var config = WireGuardConfig.parse(name: name, config: configContent) else {
            throw VPNError.invalidConfig
        }
        config.isManaged = true
        config.managedServerID = serverID
        var current = configs
        if let existing = current.first(where: { $0.managedServerID == serverID }) {
            config.id = existing.id // keep the same VPN profile across sessions
        }
        current.removeAll { $0.managedServerID == serverID }
        current.append(config)
        configs = current
        return config
    }

    /// Imported (non-managed) tunnels, in import order.
    var importedConfigs: [WireGuardConfig] { configs.filter { !$0.isManaged } }

    /// Stops and removes every server-assigned tunnel (used on sign-out);
    /// imported tunnels are left alone.
    func removeManagedTunnels() async {
        for cfg in configs where cfg.isManaged {
            disconnectTunnel(id: cfg.id)
            try? await deleteTunnel(id: cfg.id)
        }
    }

    func tunnelIDForServer(_ serverID: String) -> String? {
        configs.first(where: { $0.managedServerID == serverID })?.id
    }

    func activeServerIDs() -> [String] {
        configs.filter { cfg in
            guard cfg.managedServerID != nil else { return false }
            return providerManagers[cfg.id]?.connection.status == .connected
        }.compactMap { $0.managedServerID }
    }

    func wipeStoredConfigs() {
        for (_, manager) in providerManagers {
            if let session = manager.connection as? NETunnelProviderSession {
                session.stopTunnel()
            }
            manager.removeFromPreferences { _ in }
        }
        providerManagers.removeAll()
        observers.keys.forEach(stopObserving)
        deleteKeychainData(for: storageKey)
        UserDefaults.standard.removeObject(forKey: storageKey)
    }

    // MARK: - Private helpers
    private func loadOrCreateManager(for config: WireGuardConfig) async throws -> NETunnelProviderManager {
        var mgr = providerManagers[config.id]
        if mgr == nil {
            let managers = try await NETunnelProviderManager.loadAllFromPreferences()
            mgr = managers.first(where: { Self.tunnelID(of: $0) == config.id })
        }
        let manager = mgr ?? NETunnelProviderManager()
        // The profile name shown in iOS Settings > VPN.
        manager.localizedDescription = config.name

        // Always re-apply the configuration: managed tunnels get a fresh key
        // and address for every session but keep the same profile.
        let proto = NETunnelProviderProtocol()
        proto.providerBundleIdentifier = "com.proidentity.access.tunnel" // Network Extension bundle ID
        proto.serverAddress = config.peers.first?.endpoint ?? "WireGuard"
        proto.providerConfiguration = [
            "wg-config": config.toConfigString(),
            "tunnel-id": config.id
        ]
        manager.protocolConfiguration = proto
        manager.isEnabled = true

        try await manager.saveToPreferences()
        try await manager.loadFromPreferences()
        providerManagers[config.id] = manager
        return manager
    }

    /// Profiles store their tunnel id in the provider configuration; older
    /// builds used the id as the profile name.
    private static func tunnelID(of mgr: NETunnelProviderManager) -> String? {
        let proto = mgr.protocolConfiguration as? NETunnelProviderProtocol
        return proto?.providerConfiguration?["tunnel-id"] as? String ?? mgr.localizedDescription
    }

    private func observeManager(_ mgr: NETunnelProviderManager, tunnelID: String) {
        // One observer per tunnel; reconnecting used to stack duplicate observers.
        if let old = observers[tunnelID] { NotificationCenter.default.removeObserver(old) }
        observers[tunnelID] = NotificationCenter.default.addObserver(
            forName: .NEVPNStatusDidChange, object: mgr.connection, queue: .main
        ) { [weak self] _ in
            self?.onStateChanged?(tunnelID, Self.statusString(mgr.connection.status))
            self?.syncActivity(id: tunnelID, mgr: mgr)
        }
    }

    private func stopObserving(_ tunnelID: String) {
        if let old = observers.removeValue(forKey: tunnelID) {
            NotificationCenter.default.removeObserver(old)
        }
    }

    private func keychainData(for account: String) -> Data? {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: "com.proidentity.access.vpn",
            kSecAttrAccount: account,
            kSecReturnData: true,
            kSecMatchLimit: kSecMatchLimitOne
        ]
        var result: AnyObject?
        guard SecItemCopyMatching(query as CFDictionary, &result) == errSecSuccess else {
            return nil
        }
        return result as? Data
    }

    private func setKeychainData(_ data: Data, for account: String) {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: "com.proidentity.access.vpn",
            kSecAttrAccount: account
        ]
        SecItemDelete(query as CFDictionary)
        var add = query
        add[kSecValueData] = data
        add[kSecAttrAccessible] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        SecItemAdd(add as CFDictionary, nil)
    }

    private func deleteKeychainData(for account: String) {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: "com.proidentity.access.vpn",
            kSecAttrAccount: account
        ]
        SecItemDelete(query as CFDictionary)
    }

    // MARK: - Serialise to JS-expected dict
    private func tunnelToDict(_ c: WireGuardConfig, status: String) -> [String: Any] {
        var d: [String: Any] = [
            "id": c.id,
            "name": c.name,
            "status": status,
            "addresses": c.iface.addresses,
            "dns": c.iface.dns,
            "private_key": "",
            "is_managed": c.isManaged,
            "peers": c.peers.map { p -> [String: Any] in
                var pd: [String: Any] = [
                    "public_key": "",
                    "endpoint": p.endpoint,
                    "allowed_ips": p.allowedIPs
                ]
                if let ka = p.persistentKeepalive { pd["persistent_keepalive"] = ka }
                return pd
            }
        ]
        if let mtu = c.iface.mtu { d["mtu"] = mtu }
        if let port = c.iface.listenPort { d["listen_port"] = port }
        return d
    }
}

struct TunnelStats {
    let rxBytes: Int64
    let txBytes: Int64
    let lastHandshake: Date?
}

/// Resumes a continuation exactly once (reply vs. timeout race).
private final class ResumeOnce<T>: @unchecked Sendable {
    private let lock = NSLock()
    private var continuation: CheckedContinuation<T, Never>?
    init(_ continuation: CheckedContinuation<T, Never>) { self.continuation = continuation }
    func resume(_ value: T) {
        lock.lock(); defer { lock.unlock() }
        continuation?.resume(returning: value)
        continuation = nil
    }
}

enum VPNError: LocalizedError {
    case invalidConfig, tunnelNotFound, vpnUnavailable
    var errorDescription: String? {
        switch self {
        case .invalidConfig:   return "Invalid WireGuard config"
        case .tunnelNotFound:  return "Tunnel not found"
        case .vpnUnavailable:  return "VPN unavailable — Network Extension entitlement required"
        }
    }
}
