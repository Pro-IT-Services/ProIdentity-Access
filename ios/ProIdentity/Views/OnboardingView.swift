import SwiftUI

/// First run: choose a mode, then (managed) server → register → sign in.
struct OnboardingView: View {
    @Environment(AppModel.self) private var app
    @State private var setup = SetupModel()

    var body: some View {
        @Bindable var setup = setup
        NavigationStack(path: $setup.path) {
            ModeStep(setup: setup, notice: app.onboardingNotice)
                .navigationDestination(for: SetupModel.Step.self) { step in
                    switch step {
                    case .server:   ServerStep(setup: setup)
                    case .register: RegisterStep(setup: setup)
                    case .signIn:   SignInStep(setup: setup)
                    }
                }
        }
        .tint(Brand.primary)
        .onAppear {
            setup.onComplete = { app.setupCompleted() }
            setup.onRevoked = { app.resetAll(notice: String(localized: "This device is no longer registered. Set it up again.")) }
        }
        .onChange(of: setup.path) { setup.error = nil }
    }
}

// MARK: - Layout

/// Title + content, with the primary action pinned in the thumb zone.
struct StepLayout<Content: View, Footer: View>: View {
    let title: LocalizedStringKey
    let subtitle: LocalizedStringKey
    @ViewBuilder let content: Content
    @ViewBuilder let footer: Footer

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                VStack(alignment: .leading, spacing: 8) {
                    Text(title).font(.largeTitle.bold()).accessibilityAddTraits(.isHeader)
                    Text(subtitle).font(.body).foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                content
            }
            .padding(.horizontal, 20)
            .padding(.top, 8)
            .padding(.bottom, 24)
            .frame(maxWidth: 520, alignment: .leading)
            .frame(maxWidth: .infinity)
        }
        .scrollDismissesKeyboard(.interactively)
        .background(Brand.background.ignoresSafeArea())
        .safeAreaInset(edge: .bottom) {
            VStack(spacing: 10) { footer }
                .padding(.horizontal, 20)
                .padding(.vertical, 12)
                .frame(maxWidth: 520)
                .frame(maxWidth: .infinity)
                .background(Brand.background)
        }
        .navigationBarTitleDisplayMode(.inline)
    }
}

// MARK: - Mode

private struct ModeStep: View {
    let setup: SetupModel
    let notice: String?

    var body: some View {
        ScrollView {
            VStack(spacing: 28) {
                VStack(spacing: 16) {
                    ApertureMark(size: 96)
                        .padding(20)
                        .background(Brand.primary.opacity(0.10), in: Circle())
                    Text(verbatim: "ProIdentity Access")
                        .font(.largeTitle.bold())
                        .multilineTextAlignment(.center)
                        .accessibilityAddTraits(.isHeader)
                    Text("Secure access to your organization's network.")
                        .font(.body).foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                }
                .padding(.top, 32)

                if let notice {
                    NoticeCard(tone: .warning, title: String(localized: "Set up again"), message: notice)
                }

                PrivacyDisclosure()

                VStack(alignment: .leading, spacing: 12) {
                    Text("How will you connect?")
                        .font(.headline)
                    OptionCard(
                        icon: "building.2",
                        title: "With my organization",
                        message: "Sign in to your company's ProIdentity server. Connections are assigned to you."
                    ) { setup.chooseManaged() }
                    OptionCard(
                        icon: "doc.badge.plus",
                        title: "With a configuration file",
                        message: "Import WireGuard configuration files yourself. No account needed."
                    ) { setup.chooseStandalone() }
                }
            }
            .padding(.horizontal, 20)
            .padding(.bottom, 32)
            .frame(maxWidth: 520)
            .frame(maxWidth: .infinity)
        }
        .background(Brand.background.ignoresSafeArea())
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// What the app uses, shown before the service is used (App Review 5.4).
private struct PrivacyDisclosure: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Label("Your privacy", systemImage: "hand.raised")
                .font(.subheadline.weight(.semibold))
            Text("With an organization account, its server processes your username, this device's name and key, your VPN address, connection times and the amount of data transferred, to run and secure your access. With configuration files, nothing is sent anywhere except your VPN traffic. Nothing is sold or shared with third parties, and there are no ads or tracking.")
                .font(.footnote)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            Link("Privacy policy", destination: URL(string: "https://access.proidentity.cloud/privacy")!)
                .font(.footnote.weight(.semibold))
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Brand.surface, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).stroke(Brand.border))
    }
}

private struct OptionCard: View {
    let icon: String
    let title: LocalizedStringKey
    let message: LocalizedStringKey
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 14) {
                Image(systemName: icon)
                    .font(.title3)
                    .foregroundStyle(Brand.primary)
                    .frame(width: 44, height: 44)
                    .background(Brand.primary.opacity(0.12), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
                VStack(alignment: .leading, spacing: 4) {
                    Text(title).font(.body.weight(.semibold)).foregroundStyle(.primary)
                    Text(message).font(.subheadline).foregroundStyle(.secondary)
                        .multilineTextAlignment(.leading)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer(minLength: 0)
                Image(systemName: "chevron.right").font(.footnote.weight(.semibold)).foregroundStyle(.tertiary)
            }
            .padding(16)
            .card(radius: 16)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

// MARK: - Server

private struct ServerStep: View {
    @Bindable var setup: SetupModel
    @FocusState private var focused: Bool

    var body: some View {
        StepLayout(
            title: "Your server",
            subtitle: "Enter the address your administrator gave you."
        ) {
            BrandField(label: "Server address") {
                TextField("vpn.company.com", text: $setup.serverURL)
                    .keyboardType(.URL)
                    .textContentType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .submitLabel(.continue)
                    .focused($focused)
                    .onSubmit { setup.submitServer() }
            }
            if let error = setup.error {
                NoticeCard(tone: .error, title: error)
            }
        } footer: {
            LoadingButton(title: "Continue") { setup.submitServer() }
                .disabled(setup.serverURL.trimmingCharacters(in: .whitespaces).isEmpty)
        }
        .onAppear { focused = setup.serverURL.isEmpty }
    }
}

// MARK: - Register

private struct RegisterStep: View {
    @Bindable var setup: SetupModel

    var body: some View {
        StepLayout(
            title: "Name this device",
            subtitle: setup.serverHost.isEmpty
                ? "Your administrator sees this name in the device list. This registers it."
                : "Your administrator sees this name in the device list. This registers it with \(setup.serverHost)."
        ) {
            BrandField(label: "Device name") {
                TextField("Device name", text: $setup.deviceName)
                    .textInputAutocapitalization(.words)
                    .submitLabel(.done)
                    .onSubmit { Task { await setup.register() } }
            }
            Label("A unique encryption key is created on this device. It never leaves it.", systemImage: "lock.shield")
                .font(.footnote).foregroundStyle(.secondary)
            if let error = setup.error {
                NoticeCard(tone: .error, title: String(localized: "Couldn't register"), message: error)
            }
        } footer: {
            LoadingButton(title: "Register device", loading: setup.loading) {
                Task { await setup.register() }
            }
        }
    }
}

// MARK: - Sign in (onboarding)

private struct SignInStep: View {
    @Bindable var setup: SetupModel

    var body: some View {
        SignInContent(setup: setup, title: "Sign in", subtitle: "Use your \(setup.serverHost) account.")
            .navigationBarBackButtonHidden(setup.phase != .credentials)
            .toolbar {
                if setup.phase != .credentials {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Back") { setup.backToCredentials() }
                    }
                }
            }
            .onDisappear { setup.cancel() }
    }
}
