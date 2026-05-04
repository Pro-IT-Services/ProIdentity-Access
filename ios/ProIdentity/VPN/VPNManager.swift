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
        guard var config = WireGuardConfig.parse(name: name, config: configContent) else {
            throw VPNError.invalidConfig
        }
        var current = configs
        current.append(config)
        configs = current
        return tunnelToDict(config, status: "disconnected")
    }

    // MARK: - Delete tunnel
    func deleteTunnel(id: String) async throws {
        if let mgr = providerManagers[id] {
            try? await mgr.removeFromPreferences()
            providerManagers.removeValue(forKey: id)
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

    // MARK: - Stats (bytes, last handshake)
    func getStats(id: String) -> [String: Any] {
        // Real stats require querying the tunnel provider via IPC.
        // Return zeros when not available.
        return ["tunnel_id": id, "rx_bytes": 0, "tx_bytes": 0, "last_handshake": ""]
    }

    // MARK: - Managed tunnel (from server config)
    func importManagedTunnel(name: String, configContent: String, serverID: String) throws -> WireGuardConfig {
        guard var config = WireGuardConfig.parse(name: name, config: configContent) else {
            throw VPNError.invalidConfig
        }
        config.isManaged = true
        config.managedServerID = serverID
        var current = configs.filter { $0.managedServerID != serverID } // replace existing
        current.append(config)
        configs = current
        return config
    }

    func tunnelIDForServer(_ serverID: String) -> String? {
        configs.first(where: { $0.managedServerID == serverID })?.id
    }

    func activeServerIDs() -> [String] {
        configs.filter { cfg in
            guard let sid = cfg.managedServerID else { return false }
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
        deleteKeychainData(for: storageKey)
        UserDefaults.standard.removeObject(forKey: storageKey)
    }

    // MARK: - Private helpers
    private func loadOrCreateManager(for config: WireGuardConfig) async throws -> NETunnelProviderManager {
        if let existing = providerManagers[config.id] { return existing }

        let managers = try await NETunnelProviderManager.loadAllFromPreferences()
        if let existing = managers.first(where: { ($0.localizedDescription ?? "") == config.id }) {
            providerManagers[config.id] = existing
            return existing
        }

        let mgr = NETunnelProviderManager()
        mgr.localizedDescription = config.id

        let proto = NETunnelProviderProtocol()
        proto.providerBundleIdentifier = "com.proidentity.ios.tunnel" // Network Extension bundle ID
        proto.serverAddress = config.peers.first?.endpoint ?? "WireGuard"
        proto.providerConfiguration = [
            "wg-config": config.toConfigString(),
            "tunnel-id": config.id
        ]
        mgr.protocolConfiguration = proto
        mgr.isEnabled = true

        try await mgr.saveToPreferences()
        try await mgr.loadFromPreferences()
        providerManagers[config.id] = mgr
        return mgr
    }

    private func observeManager(_ mgr: NETunnelProviderManager, tunnelID: String) {
        NotificationCenter.default.addObserver(forName: .NEVPNStatusDidChange, object: mgr.connection, queue: .main) { [weak self] _ in
            let status: String
            switch mgr.connection.status {
            case .connected:     status = "connected"
            case .connecting:    status = "connecting"
            case .disconnecting: status = "disconnected"
            case .disconnected:  status = "disconnected"
            case .invalid:       status = "error"
            case .reasserting:   status = "connecting"
            @unknown default:    status = "disconnected"
            }
            self?.onStateChanged?(tunnelID, status)
        }
    }

    private func keychainData(for account: String) -> Data? {
        let query: [CFString: Any] = [
            kSecClass: kSecClassGenericPassword,
            kSecAttrService: "com.proidentity.ios.vpn",
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
            kSecAttrService: "com.proidentity.ios.vpn",
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
            kSecAttrService: "com.proidentity.ios.vpn",
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
