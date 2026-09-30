import Foundation
import Observation

/// Top-level app state: which screen is showing, and the session rules.
///  - Device revoked by an admin → wipe everything, back to onboarding.
///  - Session expired → keep server, device and username; sign in again.
@MainActor
@Observable
final class AppModel {
    enum Route: Equatable { case onboarding, signIn, home }

    private(set) var route: Route
    /// Why the sign-in screen is showing (expired session etc.).
    var signInNotice: String?
    /// Shown once on onboarding after a revocation.
    var onboardingNotice: String?

    let connections = ConnectionsModel()

    init() {
        let s = AppSettings.shared
        if !UserDefaults.standard.bool(forKey: "reset_v1_done") {
            s.resetToRegister()
            UserDefaults.standard.set(true, forKey: "reset_v1_done")
        }
        route = Self.startRoute()

        VPNManager.shared.onStateChanged = { [weak self] id, state in
            Task { @MainActor [weak self] in
                self?.connections.tunnelStateChanged(id: id, raw: state)
            }
        }
        connections.onAuthFailure = { [weak self] revoked in
            self?.handleAuthFailure(revoked: revoked)
        }
    }

    private static func startRoute() -> Route {
        let s = AppSettings.shared
        guard s.setupDone else { return .onboarding }
        if s.mode == "managed" && s.token.isEmpty { return .signIn }
        return .home
    }

    var isManaged: Bool { AppSettings.shared.mode == "managed" }

    // MARK: - Transitions

    func setupCompleted() {
        signInNotice = nil
        onboardingNotice = nil
        route = .home
    }

    /// Managed mode without signing in: imported tunnels still work.
    func continueSignedOut() {
        signInNotice = nil
        route = .home
    }

    func showSignIn() {
        route = .signIn
    }

    func signOut() async {
        await connections.endManagedSessions(notifyServer: true)
        if let key = try? ManagedClient.shared.aesKey() {
            try? await ManagedClient.shared.deleteAuthSession(aesKey: key)
        }
        AppSettings.shared.clearSession()
        AppSettings.shared.username = ""
        signInNotice = nil
        route = .signIn
    }

    func resetAll(notice: String? = nil) {
        connections.teardown()
        AppSettings.shared.wipeAll()
        AppSettings.shared.lastConnection = ""
        VPNManager.shared.wipeStoredConfigs()
        UserDefaults.standard.removeObject(forKey: "managed_sessions")
        onboardingNotice = notice
        signInNotice = nil
        route = .onboarding
    }

    @ObservationIgnored private var endingSession = false

    func handleAuthFailure(revoked: Bool) {
        guard !endingSession else { return }
        endingSession = true
        if revoked {
            resetAll(notice: String(localized: "This device was removed by your administrator. Set it up again to continue."))
            endingSession = false
            return
        }
        AppSettings.shared.clearSession()
        signInNotice = String(localized: "Your session expired. Sign in again to continue.")
        route = .signIn
        Task {
            await connections.endManagedSessions(notifyServer: false)
            endingSession = false
        }
    }
}
