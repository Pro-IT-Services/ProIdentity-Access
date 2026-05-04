import SwiftUI

struct SetupView: View {
    @EnvironmentObject var appState: AppState

    @State private var step: Int
    @State private var serverURL: String
    @State private var deviceName: String
    @State private var username = ""
    @State private var password = ""
    @State private var totp = ""
    @State private var requireTotp = false
    @State private var pushAuthEnabled = false
    @State private var pushRequestId: String?
    @State private var pushStatus = "idle"
    @State private var loginMode: LoginMode = .credentials
    @State private var loading = false
    @State private var errorMsg: String?

    enum LoginMode { case credentials, totp, push }

    init() {
        let s = AppSettings.shared
        var initial = 0
        if s.mode == "managed" {
            if s.serverURL.isEmpty     { initial = 1 }
            else if s.deviceID.isEmpty { initial = 2 }
            else if s.token.isEmpty    { initial = 3 }
        }
        _step = State(initialValue: initial)
        _serverURL = State(initialValue: s.serverURL)
        _deviceName = State(initialValue: Self.defaultDeviceName())
    }

    // MARK: - Body

    var body: some View {
        ZStack {
            Color.appBg.ignoresSafeArea()

            VStack(spacing: 0) {
                header.padding(.top, 72)

                Spacer()

                VStack(spacing: 16) {
                    if let err = errorMsg {
                        ErrorBanner(message: err)
                    }
                    stepContent
                        .transition(.opacity.combined(with: .move(edge: .trailing)))
                }
                .padding(.horizontal, 24)
                .animation(.easeInOut(duration: 0.2), value: step)

                Spacer()

                if step > 0 {
                    Button {
                        withAnimation { step -= 1; errorMsg = nil; requireTotp = false }
                    } label: {
                        Text("Back").foregroundColor(.appGray)
                    }
                    .padding(.bottom, 32)
                }
            }
        }
    }

    // MARK: - Header

    private var header: some View {
        VStack(spacing: 10) {
            Image(systemName: "shield.lefthalf.filled")
                .font(.system(size: 52))
                .foregroundStyle(Color.appAccent)
            Text("ProIdentity Access")
                .font(.largeTitle).fontWeight(.bold).foregroundColor(.white)
            Text(stepSubtitle)
                .font(.subheadline).foregroundColor(.appGray)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 32)
        }
    }

    private var stepSubtitle: String {
        switch step {
        case 0: return "Choose how to use ProIdentity Access"
        case 1: return "Enter your management server URL"
        case 2: return "Register this device with the server"
        case 3: return loginMode == .push ? "Approve push notification" : requireTotp ? "Enter your authentication code" : "Sign in to your account"
        default: return ""
        }
    }

    // MARK: - Step Content

    @ViewBuilder
    private var stepContent: some View {
        switch step {
        case 0: modeStep
        case 1: serverStep
        case 2: registerStep
        case 3: loginStep
        default: EmptyView()
        }
    }

    private var modeStep: some View {
        VStack(spacing: 12) {
            ModeCard(
                icon: "server.rack",
                title: "Managed",
                description: "Connect to a ProIdentity Access server. Tunnels are assigned automatically."
            ) {
                AppSettings.shared.mode = "managed"
                serverURL = AppSettings.shared.serverURL
                withAnimation { step = 1; errorMsg = nil }
            }
            ModeCard(
                icon: "doc.plaintext",
                title: "Standalone",
                description: "Import .conf files manually. No server required."
            ) {
                AppSettings.shared.mode = "standalone"
                AppSettings.shared.setupDone = true
                appState.completeSetup()
            }
        }
    }

    private var serverStep: some View {
        VStack(spacing: 16) {
            TextField("https://vpn.example.com", text: $serverURL)
                .textFieldStyle(AppFieldStyle())
                .keyboardType(.URL)
                .textContentType(.URL)
                .autocapitalization(.none)
                .autocorrectionDisabled()

            PrimaryButton("Continue") {
                let url = serverURL.trimmingCharacters(in: .whitespaces)
                guard !url.isEmpty else { errorMsg = "Enter a server URL"; return }
                AppSettings.shared.serverURL = url
                withAnimation { step = 2; errorMsg = nil }
            }
        }
    }

    private var registerStep: some View {
        VStack(spacing: 16) {
            TextField("Device name", text: $deviceName)
                .textFieldStyle(AppFieldStyle())
                .autocapitalization(.words)

            PrimaryButton("Register Device", loading: loading) {
                Task { await registerDevice() }
            }
        }
    }

    private var loginStep: some View {
        VStack(spacing: 12) {
            if loginMode == .push {
                VStack(spacing: 16) {
                    Image(systemName: pushStatus == "approved" ? "checkmark.shield.fill" : "iphone.radiowaves.left.and.right")
                        .font(.system(size: 46))
                        .foregroundColor(pushStatus == "approved" ? .appGreen : .appAccent)
                        .opacity(pushStatus == "pending" ? 0.7 : 1.0)
                        .animation(.easeInOut(duration: 0.8).repeatForever(autoreverses: true), value: pushStatus == "pending")
                    Text(pushStatus == "pending" ? "Waiting for approval…" :
                         pushStatus == "approved" ? "Approved — signing in…" :
                         pushStatus == "denied" ? "Denied" : "Expired")
                        .font(.headline).foregroundColor(.white)
                    Text(pushStatus == "pending" ? "Check your phone for a push notification." : "")
                        .font(.subheadline).foregroundColor(.appGray)
                    if pushStatus == "denied" || pushStatus == "expired" {
                        PrimaryButton("Try again", loading: loading) { Task { await login() } }
                    }
                    if pushStatus != "approved" {
                        Button { loginMode = .totp } label: {
                            Label("Enter code manually", systemImage: "number.circle")
                                .font(.caption).foregroundColor(.appGray)
                        }
                    }
                }
            } else if loginMode == .totp {
                VStack(spacing: 8) {
                    Image(systemName: "lock.shield.fill")
                        .font(.system(size: 36)).foregroundColor(.appAccent)
                    Text("Two-factor authentication")
                        .font(.headline).foregroundColor(.white)
                }
                .padding(.bottom, 4)

                TextField("6-digit code", text: $totp)
                    .textFieldStyle(AppFieldStyle())
                    .keyboardType(.numberPad)
                    .textContentType(.oneTimeCode)
                    .multilineTextAlignment(.center)
                    .font(.title3.monospacedDigit())

                PrimaryButton("Verify", loading: loading) { Task { await loginWithTotp() } }

                if pushAuthEnabled {
                    Button { loginMode = .push; Task { await login() } } label: {
                        Label("Use push notification", systemImage: "iphone.radiowaves.left.and.right")
                            .font(.caption).foregroundColor(.appGray)
                    }
                }
            } else {
                TextField("Username", text: $username)
                    .textFieldStyle(AppFieldStyle())
                    .textContentType(.username)
                    .autocapitalization(.none)
                    .autocorrectionDisabled()

                SecureField("Password", text: $password)
                    .textFieldStyle(AppFieldStyle())
                    .textContentType(.password)

                PrimaryButton("Sign In", loading: loading) { Task { await login() } }
            }
        }
    }

    // MARK: - Actions

    private func registerDevice() async {
        loading = true; errorMsg = nil
        do {
            let registrationName = deviceName.trimmingCharacters(in: .whitespacesAndNewlines)
            let finalDeviceName = registrationName.isEmpty ? Self.defaultDeviceName() : registrationName
            let (priv, pub) = DeviceCrypto.shared.generateKeyPair()
            let resp = try await ManagedClient.shared.register(
                serverURL: AppSettings.shared.serverURL,
                deviceName: finalDeviceName,
                publicKey: pub
            )
            guard let deviceID = resp["device_id"] as? String,
                  let serverPubKey = resp["server_public_key"] as? String else {
                throw APIError.serverError("Missing device_id or server_public_key")
            }
            let s = AppSettings.shared
            s.clientPrivateKey = priv
            s.serverPublicKey = serverPubKey
            s.deviceID = deviceID
            withAnimation { step = 3; errorMsg = nil }
        } catch {
            errorMsg = error.localizedDescription
        }
        loading = false
    }

    private static func defaultDeviceName() -> String {
        let name = UIDevice.current.name.trimmingCharacters(in: .whitespacesAndNewlines)
        return name.isEmpty ? "iOS Device" : name
    }

    private func login() async {
        loading = true; errorMsg = nil
        do {
            let key = try ManagedClient.shared.aesKey()
            let resp = try await ManagedClient.shared.login(
                username: username, password: password, totpCode: "", aesKey: key
            )
            if let rt = resp["require_totp"] as? Bool, rt {
                requireTotp = true
                let isPush = resp["push_auth_enabled"] as? Bool ?? false
                pushAuthEnabled = isPush
                if isPush, let reqID = resp["push_request_id"] as? String {
                    pushRequestId = reqID
                    loginMode = .push
                    loading = false
                    await pollPushLogin(requestID: reqID)
                } else {
                    loginMode = .totp
                    loading = false
                }
                return
            }
            completeLoginWith(resp)
        } catch {
            errorMsg = error.localizedDescription
        }
        loading = false
    }

    private func loginWithTotp() async {
        loading = true; errorMsg = nil
        do {
            let key = try ManagedClient.shared.aesKey()
            let resp = try await ManagedClient.shared.login(
                username: username, password: password, totpCode: totp, aesKey: key
            )
            if let rt = resp["require_totp"] as? Bool, rt {
                errorMsg = "Invalid code"
                loading = false
                return
            }
            completeLoginWith(resp)
        } catch {
            errorMsg = error.localizedDescription
        }
        loading = false
    }

    private func pollPushLogin(requestID: String) async {
        pushStatus = "pending"
        do {
            while pushStatus == "pending" {
                try await Task.sleep(nanoseconds: 2_000_000_000)
                let status = try await ManagedClient.shared.pollPushStatus(requestID: requestID)
                await MainActor.run { pushStatus = status }
                if status == "approved" {
                    loading = true
                    let key = try ManagedClient.shared.aesKey()
                    let resp = try await ManagedClient.shared.login(
                        username: username, password: password, totpCode: "",
                        pushAuthID: requestID, aesKey: key
                    )
                    completeLoginWith(resp)
                    return
                }
            }
        } catch {
            await MainActor.run { errorMsg = error.localizedDescription; loading = false }
        }
    }

    private func completeLoginWith(_ resp: [String: Any]) {
        guard let token = resp["token"] as? String, !token.isEmpty else {
            errorMsg = "Missing token in response"
            return
        }
        let s = AppSettings.shared
        s.token = token
        s.username = username
        s.isAdmin = resp["is_admin"] as? Bool ?? false
        s.vpnName = resp["vpn_name"] as? String ?? ""
        s.totpEnabled = resp["totp_enabled"] as? Bool ?? false
        s.setupDone = true
        appState.completeSetup()
    }
}

// MARK: - Mode Card

struct ModeCard: View {
    let icon: String
    let title: String
    let description: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 16) {
                Image(systemName: icon)
                    .font(.title2)
                    .foregroundColor(.appAccent)
                    .frame(width: 40)

                VStack(alignment: .leading, spacing: 4) {
                    Text(title).font(.headline).foregroundColor(.white)
                    Text(description).font(.caption).foregroundColor(.appGray)
                        .multilineTextAlignment(.leading).lineLimit(2)
                }

                Spacer()

                Image(systemName: "chevron.right")
                    .font(.caption).foregroundColor(.appGray.opacity(0.6))
            }
            .padding(16)
            .background(Color.appSurface)
            .cornerRadius(14)
            .overlay(
                RoundedRectangle(cornerRadius: 14)
                    .stroke(Color.white.opacity(0.07), lineWidth: 1)
            )
        }
    }
}
