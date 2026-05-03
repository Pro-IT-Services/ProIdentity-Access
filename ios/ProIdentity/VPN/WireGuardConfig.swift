import Foundation

struct WireGuardPeer: Codable {
    var publicKey: String
    var endpoint: String
    var allowedIPs: [String]
    var persistentKeepalive: Int?
    var presharedKey: String?
}

struct WireGuardInterface: Codable {
    var privateKey: String
    var addresses: [String]
    var dns: [String]
    var mtu: Int?
    var listenPort: Int?
}

struct WireGuardConfig: Codable, Identifiable {
    var id: String
    var name: String
    var iface: WireGuardInterface
    var peers: [WireGuardPeer]
    var isManaged: Bool = false
    var managedServerID: String?

    // Serialize back to .conf format
    func toConfigString() -> String {
        var lines = ["[Interface]"]
        lines.append("PrivateKey = \(iface.privateKey)")
        if !iface.addresses.isEmpty { lines.append("Address = \(iface.addresses.joined(separator: ", "))") }
        if !iface.dns.isEmpty { lines.append("DNS = \(iface.dns.joined(separator: ", "))") }
        if let mtu = iface.mtu { lines.append("MTU = \(mtu)") }
        if let port = iface.listenPort { lines.append("ListenPort = \(port)") }
        for peer in peers {
            lines.append("\n[Peer]")
            lines.append("PublicKey = \(peer.publicKey)")
            if let psk = peer.presharedKey, !psk.isEmpty { lines.append("PresharedKey = \(psk)") }
            if !peer.allowedIPs.isEmpty { lines.append("AllowedIPs = \(peer.allowedIPs.joined(separator: ", "))") }
            if !peer.endpoint.isEmpty { lines.append("Endpoint = \(peer.endpoint)") }
            if let ka = peer.persistentKeepalive { lines.append("PersistentKeepalive = \(ka)") }
        }
        return lines.joined(separator: "\n")
    }

    // Parse from .conf string
    static func parse(name: String, config: String) -> WireGuardConfig? {
        var iface = WireGuardInterface(privateKey: "", addresses: [], dns: [])
        var peers: [WireGuardPeer] = []
        var currentPeer: WireGuardPeer?
        var inPeer = false

        for raw in config.components(separatedBy: .newlines) {
            let line = raw.trimmingCharacters(in: .whitespaces)
            if line.isEmpty || line.hasPrefix("#") { continue }
            if line.lowercased() == "[interface]" { inPeer = false; continue }
            if line.lowercased() == "[peer]" {
                if let p = currentPeer { peers.append(p) }
                currentPeer = WireGuardPeer(publicKey: "", endpoint: "", allowedIPs: [])
                inPeer = true
                continue
            }
            let parts = line.components(separatedBy: "=")
            guard parts.count >= 2 else { continue }
            let key = parts[0].trimmingCharacters(in: .whitespaces).lowercased()
            let val = parts.dropFirst().joined(separator: "=").trimmingCharacters(in: .whitespaces)

            if inPeer {
                switch key {
                case "publickey": currentPeer?.publicKey = val
                case "endpoint": currentPeer?.endpoint = val
                case "allowedips": currentPeer?.allowedIPs = val.components(separatedBy: ",").map { $0.trimmingCharacters(in: .whitespaces) }
                case "persistentkeepalive": currentPeer?.persistentKeepalive = Int(val)
                case "presharedkey": currentPeer?.presharedKey = val
                default: break
                }
            } else {
                switch key {
                case "privatekey": iface.privateKey = val
                case "address": iface.addresses = val.components(separatedBy: ",").map { $0.trimmingCharacters(in: .whitespaces) }
                case "dns": iface.dns = val.components(separatedBy: ",").map { $0.trimmingCharacters(in: .whitespaces) }
                case "mtu": iface.mtu = Int(val)
                case "listenport": iface.listenPort = Int(val)
                default: break
                }
            }
        }
        if let p = currentPeer { peers.append(p) }
        guard !iface.privateKey.isEmpty else { return nil }
        return WireGuardConfig(id: UUID().uuidString, name: name, iface: iface, peers: peers)
    }
}
