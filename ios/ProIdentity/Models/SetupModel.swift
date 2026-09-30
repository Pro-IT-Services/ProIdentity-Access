import Foundation
import Observation
import UIKit

/// Onboarding (mode → server → register → sign in) and the sign-in-only
/// flow used when a session expires.
@MainActor
@Observable
final class SetupModel {
    enum Step: Hashable { case server, register, signIn }
    enum Phase { case credentials, code, push }

    /// Navigation path; the mode picker is the root.
    var path: [Step] = []

    var serverURL: String
    var deviceName: String
    var username: String
    var password = ""
    var code = ""
    private(set) var phase: Phase = .credentials
    private(set) var push: PushState = .idle
    private(set) var pushAvailable = false
    private(set) var loading = false
    var error: String?

    /// True when only the password/2FA is needed (server and device are kept).
    let isReauth: Bool

    @ObservationIgnored var onComplete: (() -> Void)?
    /// The server no longer knows this device — the app must be set up again.
    @ObservationIgnored var onRevoked: (() -> Void)?
    @ObservationIgnored private var pushTask: Task<Void, Never>?

    init(reauth: Bool = false) {
        let s = AppSettings.shared
        isReauth = reauth
        serverURL = s.serverURL
        username = s.username
        deviceName = Self.defaultDeviceName()
        if !reauth && s.mode == "managed" {
            // Resume where a previous onboarding stopped.
            if s.serverURL.isEmpty { path = [] }
            else if s.deviceID.isEmpty { path = [.server, .register] }
            else { path = [.server, .register, .signIn] }
        }
    }

    var serverHost: String { URL(string: AppSettings.shared.serverURL)?.host() ?? AppSettings.shared.serverURL }

    // MARK: - Mode

    func chooseManaged() {
        AppSettings.shared.mode = "managed"
        error = nil
        path.append(.server)
    }

    func chooseStandalone() {
        AppSettings.shared.mode = "standalone"
        AppSettings.shared.setupDone = true
        onComplete?()
    }

    // MARK: - Server

    func submitServer() {
        error = nil
        guard let url = Self.normalizeServerURL(serverURL) else {
            error = String(localized: "Enter a valid address, for example vpn.company.com.")
            return
        }
        if url != AppSettings.shared.serverURL {
            // A different server means a new registration.
            AppSettings.shared.resetToRegister()
        }
        AppSettings.shared.serverURL = url
        serverURL = url
        path.append(AppSettings.shared.deviceID.isEmpty ? .register : .signIn)
    }

    static func normalizeServerURL(_ raw: String) -> String? {
        var value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !value.isEmpty else { return nil }
        if !value.lowercased().hasPrefix("http://") && !value.lowercased().hasPrefix("https://") {
            value = "https://" + value
        }
        while value.hasSuffix("/") { value.removeLast() }
        guard let comps = URLComponents(string: value),
              let host = comps.host, !host.isEmpty, host.contains(".") || host == "localhost",
              comps.query == nil, comps.fragment == nil else { return nil }
        let localhost = host == "localhost" || host == "127.0.0.1"
        guard comps.scheme?.lowercased() == "https" || localhost else { return nil }
        return value
    }

    // MARK: - Register

    func register() async {
        loading = true; error = nil
        defer { loading = false }
        do {
            let trimmed = deviceName.trimmingCharacters(in: .whitespacesAndNewlines)
            let (priv, pub) = DeviceCrypto.shared.generateKeyPair()
            let resp = try await ManagedClient.shared.register(
                serverURL: AppSettings.shared.serverURL,
                deviceName: trimmed.isEmpty ? Self.defaultDeviceName() : trimmed,
                publicKey: pub
            )
            guard let deviceID = resp["device_id"] as? String,
                  let serverPubKey = resp["server_public_key"] as? String else {
                throw APIError.serverError(String(localized: "This doesn't look like a ProIdentity Access server."))
            }
            let s = AppSettings.shared
            s.clientPrivateKey = priv
            s.serverPublicKey = serverPubKey
            s.deviceID = deviceID
            path.append(.signIn)
        } catch {
            self.error = UserMessage.from(error)
        }
    }

    // MARK: - Sign in

    func signIn() async {
        error = nil
        let user = username.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !user.isEmpty, !password.isEmpty else {
            error = String(localized: "Enter your username and password.")
            return
        }
        username = user
        loading = true
        defer { loading = false }
        do {
            let key = try ManagedClient.shared.aesKey()
            let resp = try await ManagedClient.shared.login(username: user, password: password, totpCode: "", aesKey: key)
            if resp["require_totp"] as? Bool == true {
                pushAvailable = resp["push_auth_enabled"] as? Bool ?? false
                code = ""
                if pushAvailable, let requestID = resp["push_request_id"] as? String {
                    phase = .push
                    waitForPush(requestID)
                } else {
                    phase = .code
                }
                return
            }
            finish(resp)
        } catch {
            report(error)
        }
    }

    func submitCode() async {
        let digits = code.filter(\.isNumber)
        guard digits.count == 6 else {
            error = String(localized: "Enter the 6-digit code.")
            return
        }
        loading = true; error = nil
        defer { loading = false }
        do {
            let key = try ManagedClient.shared.aesKey()
            let resp = try await ManagedClient.shared.login(username: username, password: password, totpCode: digits, aesKey: key)
            if resp["require_totp"] as? Bool == true {
                code = ""
                error = String(localized: "That code didn't work. Check your authenticator and try again.")
                return
            }
            finish(resp)
        } catch {
            code = ""
            report(error)
        }
    }

    /// Sends a fresh push request (login again with the stored password).
    func retryPush() async {
        pushTask?.cancel()
        await signIn()
    }

    func useCode() {
        pushTask?.cancel()
        error = nil
        phase = .code
    }

    func usePush() async {
        error = nil
        await signIn()
    }

    func backToCredentials() {
        pushTask?.cancel()
        error = nil
        code = ""
        push = .idle
        phase = .credentials
    }

    private func waitForPush(_ requestID: String) {
        pushTask?.cancel()
        push = .pending
        pushTask = Task { [weak self] in
            let deadline = Date().addingTimeInterval(180)
            do {
                while !Task.isCancelled {
                    try await Task.sleep(for: .seconds(2))
                    guard let self, !Task.isCancelled else { return }
                    var state = PushState.expired
                    if Date() < deadline {
                        state = PushState(try await ManagedClient.shared.pollPushStatus(requestID: requestID))
                    }
                    guard !Task.isCancelled else { return }
                    self.push = state
                    if state == .approved {
                        self.loading = true
                        defer { self.loading = false }
                        let key = try ManagedClient.shared.aesKey()
                        let resp = try await ManagedClient.shared.login(
                            username: self.username, password: self.password, totpCode: "",
                            pushAuthID: requestID, aesKey: key
                        )
                        self.finish(resp)
                        return
                    }
                    if state != .pending { return }
                }
            } catch is CancellationError {
                return
            } catch {
                guard let self, !Task.isCancelled else { return }
                self.push = .idle
                self.report(error)
            }
        }
    }

    private func report(_ error: Error) {
        if case APIError.deviceRevoked = error, let onRevoked {
            pushTask?.cancel()
            onRevoked()
            return
        }
        self.error = UserMessage.from(error)
    }

    private func finish(_ resp: [String: Any]) {
        guard let token = resp["token"] as? String, !token.isEmpty else {
            error = String(localized: "The server didn't return a session. Try again.")
            return
        }
        let s = AppSettings.shared
        s.token = token
        s.username = username
        s.isAdmin = resp["is_admin"] as? Bool ?? false
        s.vpnName = resp["vpn_name"] as? String ?? ""
        s.totpEnabled = resp["totp_enabled"] as? Bool ?? false
        s.setupDone = true
        password = ""
        code = ""
        pushTask?.cancel()
        onComplete?()
    }

    func cancel() {
        pushTask?.cancel()
    }

    static func defaultDeviceName() -> String {
        let name = UIDevice.current.name.trimmingCharacters(in: .whitespacesAndNewlines)
        return name.isEmpty ? "iPhone" : name
    }
}
