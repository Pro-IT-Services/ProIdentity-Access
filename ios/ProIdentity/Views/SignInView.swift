import SwiftUI

/// Sign-in after a session expired or the user signed out. Server, device
/// registration and (when known) the username are kept.
struct SignInView: View {
    @Environment(AppModel.self) private var app
    @State private var setup = SetupModel(reauth: true)
    @State private var confirmNewServer = false

    var body: some View {
        NavigationStack {
            SignInContent(
                setup: setup,
                title: "Welcome back",
                subtitle: "Sign in to \(setup.serverHost) to load your connections.",
                notice: app.signInNotice
            ) {
                if setup.phase == .credentials {
                    Button("Continue without signing in") { app.continueSignedOut() }
                        .font(.subheadline.weight(.medium))
                        .frame(minHeight: 44)
                }
            }
            .toolbar {
                if setup.phase != .credentials {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Back") { setup.backToCredentials() }
                    }
                } else {
                    ToolbarItem(placement: .topBarTrailing) {
                        Menu {
                            Button("Use a different server", systemImage: "arrow.triangle.2.circlepath") {
                                confirmNewServer = true
                            }
                        } label: {
                            Image(systemName: "ellipsis.circle")
                        }
                        .accessibilityLabel("More options")
                    }
                }
            }
            .confirmationDialog(
                "Use a different server?",
                isPresented: $confirmNewServer,
                titleVisibility: .visible
            ) {
                Button("Set up again", role: .destructive) { app.resetAll() }
                Button("Cancel", role: .cancel) {}
            } message: {
                Text("This device will be unregistered here and all connections removed.")
            }
        }
        .tint(Brand.primary)
        .onAppear {
            setup.onComplete = { app.setupCompleted() }
            setup.onRevoked = { app.resetAll(notice: "This device is no longer registered. Set it up again.") }
        }
        .onDisappear { setup.cancel() }
    }
}

/// Username + password → code or push approval. Shared by onboarding and re-login.
struct SignInContent<Extra: View>: View {
    @Bindable var setup: SetupModel
    let title: String
    let subtitle: String
    var notice: String?
    @ViewBuilder let extra: Extra

    init(setup: SetupModel, title: String, subtitle: String, notice: String? = nil,
         @ViewBuilder extra: () -> Extra = { EmptyView() }) {
        self.setup = setup
        self.title = title
        self.subtitle = subtitle
        self.notice = notice
        self.extra = extra()
    }

    @FocusState private var focus: Field?
    private enum Field { case username, password, code }

    var body: some View {
        switch setup.phase {
        case .credentials: credentials
        case .code:        codeEntry
        case .push:        pushApproval
        }
    }

    // MARK: Credentials

    private var credentials: some View {
        StepLayout(title: title, subtitle: subtitle) {
            if let notice, setup.error == nil {
                NoticeCard(tone: .info, title: notice)
            }
            VStack(spacing: 14) {
                BrandField(label: "Username") {
                    TextField("Username", text: $setup.username)
                        .textContentType(.username)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .submitLabel(.next)
                        .focused($focus, equals: .username)
                        .onSubmit { focus = .password }
                }
                BrandField(label: "Password") {
                    SecureField("Password", text: $setup.password)
                        .textContentType(.password)
                        .submitLabel(.go)
                        .focused($focus, equals: .password)
                        .onSubmit { Task { await setup.signIn() } }
                }
            }
            if let error = setup.error {
                NoticeCard(tone: .error, title: "Couldn't sign in", message: error)
            }
        } footer: {
            LoadingButton(title: "Sign in", loading: setup.loading) {
                focus = nil
                Task { await setup.signIn() }
            }
            extra
        }
        .onAppear {
            focus = setup.username.isEmpty ? .username : .password
        }
    }

    // MARK: Code

    private var codeEntry: some View {
        StepLayout(title: "Verification code", subtitle: "Enter the 6-digit code from your authenticator app.") {
            OTPField(code: $setup.code, isError: setup.error != nil)
                .focused($focus, equals: .code)
                .onChange(of: setup.code) { _, new in
                    if new.count == 6 && !setup.loading { Task { await setup.submitCode() } }
                }
            if let error = setup.error {
                NoticeCard(tone: .error, title: error)
            }
        } footer: {
            LoadingButton(title: "Verify", loading: setup.loading) {
                Task { await setup.submitCode() }
            }
            .disabled(setup.code.count != 6)
            if setup.pushAvailable {
                Button("Send a push approval instead") { Task { await setup.usePush() } }
                    .font(.subheadline.weight(.medium))
                    .frame(minHeight: 44)
            }
        }
        .onAppear { focus = .code }
    }

    // MARK: Push

    private var pushApproval: some View {
        StepLayout(title: "Approve sign-in", subtitle: "We sent a request to your ProIdentity authenticator.") {
            PushApprovalView(state: setup.push, error: setup.error)
        } footer: {
            if setup.push.isFinishedWithoutApproval || setup.error != nil {
                LoadingButton(title: "Send again", loading: setup.loading) {
                    Task { await setup.retryPush() }
                }
            }
            if setup.push != .approved {
                Button("Enter a code instead") { setup.useCode() }
                    .font(.subheadline.weight(.medium))
                    .frame(minHeight: 44)
            }
        }
    }
}

// MARK: - Shared 2FA pieces

/// Six-digit code field with one-time-code autofill.
struct OTPField: View {
    @Binding var code: String
    var isError = false

    var body: some View {
        TextField("000000", text: $code)
            .keyboardType(.numberPad)
            .textContentType(.oneTimeCode)
            .font(.system(size: 34, weight: .semibold, design: .monospaced))
            .kerning(8)
            .multilineTextAlignment(.center)
            .frame(minHeight: 64)
            .background(Brand.surface, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .strokeBorder(isError ? Brand.danger : Brand.border, lineWidth: isError ? 1.5 : 1)
            )
            .onChange(of: code) { _, new in
                let digits = String(new.filter(\.isNumber).prefix(6))
                if digits != new { code = digits }
            }
            .accessibilityLabel("Verification code")
    }
}

/// Status of a push approval request.
struct PushApprovalView: View {
    let state: PushState
    var error: String?

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var icon: String {
        switch state {
        case .approved: return "checkmark.shield.fill"
        case .denied:   return "xmark.shield.fill"
        case .expired:  return "clock.badge.exclamationmark"
        default:        return "iphone.radiowaves.left.and.right"
        }
    }

    private var color: Color {
        switch state {
        case .approved: return Brand.connected
        case .denied:   return Brand.danger
        case .expired:  return Brand.warning
        default:        return Brand.primary
        }
    }

    private var headline: String {
        switch state {
        case .approved: return "Approved"
        case .denied:   return "Request denied"
        case .expired:  return "Request expired"
        default:        return "Waiting for approval…"
        }
    }

    private var detail: String {
        switch state {
        case .approved: return "Finishing up…"
        case .denied:   return "The request was declined on your phone. Send a new one to try again."
        case .expired:  return "No answer in time. Send a new request to try again."
        default:        return "Open the notification on your phone and approve the request."
        }
    }

    var body: some View {
        VStack(spacing: 14) {
            Image(systemName: icon)
                .font(.system(size: 48))
                .foregroundStyle(color)
                .symbolEffect(.pulse, options: .repeating, isActive: state == .pending && !reduceMotion)
                .frame(width: 96, height: 96)
                .background(color.opacity(0.12), in: Circle())
            Text(headline).font(.title3.weight(.semibold))
            Text(detail).font(.subheadline).foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
            if let error {
                NoticeCard(tone: .error, title: error)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 20)
        .accessibilityElement(children: .combine)
        .sensoryFeedback(.success, trigger: state) { _, new in new == .approved }
    }
}
