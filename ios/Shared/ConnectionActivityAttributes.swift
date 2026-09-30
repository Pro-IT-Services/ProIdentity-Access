import ActivityKit
import Foundation

/// The Live Activity shown while a VPN connection is up (lock screen and
/// Dynamic Island) — iOS has no persistent notification like Android's.
/// Shared by the app (starts/updates/ends it) and the widget (draws it).
struct ConnectionActivityAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        /// false while iOS re-establishes the tunnel (network change etc.).
        var connected: Bool
        /// When the tunnel came up; drives the running timer.
        var since: Date
    }

    /// VPNManager tunnel id, used by the Disconnect button.
    var tunnelID: String
    /// Connection name shown on the lock screen (no addresses or keys).
    var name: String
}
