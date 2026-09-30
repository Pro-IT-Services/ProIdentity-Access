import AppIntents
import Foundation

/// The Live Activity's Disconnect button. iOS runs a LiveActivityIntent in
/// the app's process (launching it in the background if needed), so the
/// widget only references it; the work happens in the app.
struct DisconnectVPNIntent: LiveActivityIntent {
    static var title: LocalizedStringResource = "Disconnect VPN"
    static var isDiscoverable = false

    @Parameter(title: "Connection")
    var tunnelID: String

    init() {}

    init(tunnelID: String) {
        self.tunnelID = tunnelID
    }

    func perform() async throws -> some IntentResult {
        #if !WIDGET_EXTENSION
        await ConnectionActivityController.disconnect(tunnelID: tunnelID)
        #endif
        return .result()
    }
}
