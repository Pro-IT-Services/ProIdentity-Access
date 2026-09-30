import ActivityKit
import Foundation
import NetworkExtension

/// Keeps one Live Activity per connected tunnel in step with the VPN state:
/// started when a tunnel connects, "Reconnecting…" while iOS re-establishes
/// it, ended when it goes down.
@MainActor
enum ConnectionActivityController {
    private typealias LiveActivity = Activity<ConnectionActivityAttributes>

    static func statusChanged(tunnelID: String, name: String, status: NEVPNStatus, connectedDate: Date?) {
        switch status {
        case .connected:
            let state = ConnectionActivityAttributes.ContentState(connected: true, since: connectedDate ?? Date())
            if let activity = activity(for: tunnelID) {
                Task { await activity.update(ActivityContent(state: state, staleDate: nil)) }
            } else {
                start(tunnelID: tunnelID, name: name, state: state)
            }
        case .connecting, .reasserting:
            // Only while an activity is already showing (a reconnect); a new
            // connection gets its activity once it's up.
            if let activity = activity(for: tunnelID) {
                let state = ConnectionActivityAttributes.ContentState(connected: false, since: activity.content.state.since)
                Task { await activity.update(ActivityContent(state: state, staleDate: nil)) }
            }
        case .disconnecting, .disconnected, .invalid:
            end(tunnelID: tunnelID)
        @unknown default:
            break
        }
    }

    /// Ends activities whose tunnel is no longer up, e.g. disconnected from
    /// iOS Settings while the app wasn't running.
    static func reconcile(connectedTunnelIDs: Set<String>) {
        for activity in LiveActivity.activities where !connectedTunnelIDs.contains(activity.attributes.tunnelID) {
            Task { await activity.end(nil, dismissalPolicy: .immediate) }
        }
    }

    /// The Live Activity's Disconnect button. The app may have just been
    /// launched in the background for it, so the tunnel is looked up afresh;
    /// the usual state handling then ends the server session.
    static func disconnect(tunnelID: String) async {
        await VPNManager.shared.stopTunnel(id: tunnelID)
        end(tunnelID: tunnelID)
    }

    private static func activity(for tunnelID: String) -> LiveActivity? {
        LiveActivity.activities.first { $0.attributes.tunnelID == tunnelID }
    }

    private static func start(tunnelID: String, name: String, state: ConnectionActivityAttributes.ContentState) {
        guard ActivityAuthorizationInfo().areActivitiesEnabled else { return }
        do {
            _ = try LiveActivity.request(
                attributes: ConnectionActivityAttributes(tunnelID: tunnelID, name: name),
                content: ActivityContent(state: state, staleDate: nil),
                pushType: nil
            )
        } catch {
            // Not allowed from the background, or Live Activities are off for
            // the app; the connection itself is unaffected.
        }
    }

    private static func end(tunnelID: String) {
        for activity in LiveActivity.activities where activity.attributes.tunnelID == tunnelID {
            Task { await activity.end(nil, dismissalPolicy: .immediate) }
        }
    }
}
