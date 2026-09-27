import Foundation
import NetworkExtension

// MARK: - Server data

struct ManagedServer: Identifiable, Equatable {
    let id: String
    let name: String
    let location: String
    let subnet: String

    init(_ d: [String: Any]) {
        id       = d["id"] as? String ?? ""
        name     = d["name"] as? String ?? d["hostname"] as? String ?? "Server"
        location = [d["city"], d["country"]].compactMap { $0 as? String }
                       .filter { !$0.isEmpty }.joined(separator: ", ")
        subnet   = d["subnet"] as? String ?? ""
    }
}

struct EndpointCandidate {
    let endpoint: String
    let ip: String
    let port: Int

    init?(_ d: [String: Any]) {
        endpoint = d["endpoint"] as? String ?? ""
        ip = d["ip"] as? String ?? ""
        if let p = d["port"] as? Int {
            port = p
        } else if let p = d["port"] as? NSNumber {
            port = p.intValue
        } else {
            port = 0
        }
        if endpoint.isEmpty && (ip.isEmpty || port <= 0) { return nil }
    }

    var wireGuardEndpoint: String {
        if !endpoint.isEmpty { return endpoint }
        return "\(ip):\(port)"
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

// MARK: - Unified connection model

enum ConnStatus: Equatable {
    case disconnected, connecting, connected, error

    init(_ raw: String) {
        switch raw {
        case "connected":  self = .connected
        case "connecting": self = .connecting
        case "error":      self = .error
        default:           self = .disconnected
        }
    }

    var isActive: Bool { self == .connected || self == .connecting }

    var label: String {
        switch self {
        case .connected:    return "Connected"
        case .connecting:   return "Connecting…"
        case .error:        return "Couldn't connect"
        case .disconnected: return "Not connected"
        }
    }
}

/// A managed server or an imported tunnel, shown as one row on Home.
struct Connection: Identifiable, Hashable {
    enum Kind: Hashable { case managed, imported }

    /// Stable key: "server:<id>" or "tunnel:<id>".
    let id: String
    let kind: Kind
    let refID: String
    let name: String
    /// Location (managed) or interface address (imported).
    let detail: String
    let tunnelID: String?
    var status: ConnStatus

    var isActive: Bool { status.isActive }
    var badge: String { kind == .managed ? "Managed" : "Imported" }

    static func key(server id: String) -> String { "server:\(id)" }
    static func key(tunnel id: String) -> String { "tunnel:\(id)" }
}

enum PushState: Equatable {
    case idle, pending, approved, denied, expired

    init(_ raw: String) {
        switch raw {
        case "approved": self = .approved
        case "denied":   self = .denied
        case "expired":  self = .expired
        default:         self = .pending
        }
    }

    var isFinishedWithoutApproval: Bool { self == .denied || self == .expired }
}

/// State of the two-factor prompt shown when a server requires it to connect.
struct AuthPrompt: Identifiable {
    enum Method { case code, push }

    let serverID: String
    let serverName: String
    var method: Method
    var push: PushState = .idle
    var code = ""
    var submitting = false
    var error: String?

    var id: String { serverID }
}

// MARK: - Human-readable errors

enum UserMessage {
    static func from(_ error: Error) -> String {
        if let urlError = error as? URLError {
            switch urlError.code {
            case .notConnectedToInternet, .networkConnectionLost:
                return "You're offline. Check your connection and try again."
            case .timedOut:
                return "The server took too long to respond. Try again."
            case .cannotFindHost, .cannotConnectToHost, .dnsLookupFailed:
                return "Can't reach the server. Check the address and try again."
            case .secureConnectionFailed, .serverCertificateUntrusted, .serverCertificateHasBadDate,
                 .serverCertificateNotYetValid, .serverCertificateHasUnknownRoot:
                return "The server's certificate isn't trusted."
            default:
                return "Network error. Try again."
            }
        }
        if let vpnError = error as? NEVPNError {
            switch vpnError.code {
            case .configurationReadWriteFailed:
                return "VPN permission wasn't granted. Allow the VPN configuration to connect."
            case .configurationDisabled, .configurationInvalid:
                return "The VPN configuration is not valid on this device."
            default:
                return "The VPN couldn't start. Try again."
            }
        }
        return error.localizedDescription
    }
}
