import NetworkExtension
import WireGuardKit

class PacketTunnelProvider: NEPacketTunnelProvider {
    private lazy var adapter: WireGuardAdapter = {
        WireGuardAdapter(with: self) { _, message in
            NSLog("[WG] %@", message)
        }
    }()

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        guard let proto = protocolConfiguration as? NETunnelProviderProtocol,
              let providerConfig = proto.providerConfiguration,
              let wgConfig = providerConfig["wg-config"] as? String else {
            completionHandler(PacketTunnelError.missingConfig)
            return
        }

        guard let tunnelConfig = parseWgQuickConfig(wgConfig) else {
            completionHandler(PacketTunnelError.invalidConfig)
            return
        }

        adapter.start(tunnelConfiguration: tunnelConfig) { adapterError in
            if let error = adapterError {
                NSLog("[WG] Failed to start: %@", "\(error)")
                completionHandler(error)
            } else {
                completionHandler(nil)
            }
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        adapter.stop { _ in completionHandler() }
    }

    override func handleAppMessage(_ messageData: Data, completionHandler: ((Data?) -> Void)?) {
        if String(data: messageData, encoding: .utf8) == "stats" {
            adapter.getRuntimeConfiguration { config in
                completionHandler?(config?.data(using: .utf8))
            }
        } else {
            completionHandler?(nil)
        }
    }

    private func parseWgQuickConfig(_ config: String) -> TunnelConfiguration? {
        var privateKey: PrivateKey?
        var addresses: [IPAddressRange] = []
        var dns: [DNSServer] = []
        var mtu: UInt16?
        var listenPort: UInt16?
        var peers: [PeerConfiguration] = []

        var currentPeerPublicKey: PublicKey?
        var currentPeerEndpoint: Endpoint?
        var currentPeerAllowedIPs: [IPAddressRange] = []
        var currentPeerKeepAlive: UInt16?
        var currentPeerPresharedKey: PreSharedKey?
        var inPeer = false

        func flushPeer() {
            guard let pubKey = currentPeerPublicKey else { return }
            var peer = PeerConfiguration(publicKey: pubKey)
            peer.endpoint = currentPeerEndpoint
            peer.allowedIPs = currentPeerAllowedIPs
            peer.persistentKeepAlive = currentPeerKeepAlive
            peer.preSharedKey = currentPeerPresharedKey
            peers.append(peer)
            currentPeerPublicKey = nil
            currentPeerEndpoint = nil
            currentPeerAllowedIPs = []
            currentPeerKeepAlive = nil
            currentPeerPresharedKey = nil
        }

        for line in config.components(separatedBy: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty || trimmed.hasPrefix("#") { continue }

            if trimmed.lowercased() == "[interface]" { flushPeer(); inPeer = false; continue }
            if trimmed.lowercased() == "[peer]" { flushPeer(); inPeer = true; continue }

            guard let eq = trimmed.firstIndex(of: "=") else { continue }
            let key = trimmed[..<eq].trimmingCharacters(in: .whitespaces).lowercased()
            let val = trimmed[trimmed.index(after: eq)...].trimmingCharacters(in: .whitespaces)

            if !inPeer {
                switch key {
                case "privatekey":
                    privateKey = PrivateKey(base64Key: val)
                case "address":
                    addresses += val.components(separatedBy: ",").compactMap {
                        IPAddressRange(from: $0.trimmingCharacters(in: .whitespaces))
                    }
                case "dns":
                    dns += val.components(separatedBy: ",").compactMap {
                        DNSServer(from: $0.trimmingCharacters(in: .whitespaces))
                    }
                case "mtu":
                    mtu = UInt16(val)
                case "listenport":
                    listenPort = UInt16(val)
                default: break
                }
            } else {
                switch key {
                case "publickey":
                    currentPeerPublicKey = PublicKey(base64Key: val)
                case "endpoint":
                    currentPeerEndpoint = Endpoint(from: val)
                case "allowedips":
                    currentPeerAllowedIPs += val.components(separatedBy: ",").compactMap {
                        IPAddressRange(from: $0.trimmingCharacters(in: .whitespaces))
                    }
                case "persistentkeepalive":
                    currentPeerKeepAlive = UInt16(val)
                case "presharedkey":
                    currentPeerPresharedKey = PreSharedKey(base64Key: val)
                default: break
                }
            }
        }
        flushPeer()

        guard let privKey = privateKey else { return nil }
        var iface = InterfaceConfiguration(privateKey: privKey)
        iface.addresses = addresses
        iface.dns = dns
        iface.mtu = mtu
        iface.listenPort = listenPort
        return TunnelConfiguration(name: "tunnel", interface: iface, peers: peers)
    }
}

enum PacketTunnelError: LocalizedError {
    case missingConfig, invalidConfig
    var errorDescription: String? {
        switch self {
        case .missingConfig: return "Missing WireGuard configuration"
        case .invalidConfig: return "Invalid WireGuard configuration"
        }
    }
}
