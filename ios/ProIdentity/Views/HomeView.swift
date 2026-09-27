import SwiftUI

/// The connection hub: status hero, all connections, one primary action.
/// iPhone: stack navigation. iPad / regular width: list + detail split.
struct HomeView: View {
    @Environment(AppModel.self) private var app
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.scenePhase) private var scenePhase

    @State private var path: [String] = []
    @State private var selection: String?
    @State private var showImport = false
    @State private var showSettings = false

    private var model: ConnectionsModel { app.connections }

    var body: some View {
        Group {
            if sizeClass == .regular {
                NavigationSplitView {
                    content(selected: selection) { selection = $0.id }
                        .navigationSplitViewColumnWidth(min: 340, ideal: 400)
                } detail: {
                    NavigationStack {
                        if let id = selection {
                            ConnectionDetailView(connectionID: id) { selection = nil }
                                .id(id)
                        } else {
                            ContentUnavailableView(
                                "Select a connection",
                                systemImage: "network",
                                description: Text("Details and traffic for a connection appear here.")
                            )
                            .background(Brand.background)
                        }
                    }
                }
            } else {
                NavigationStack(path: $path) {
                    content(selected: nil) { path.append($0.id) }
                        .navigationDestination(for: String.self) { id in
                            ConnectionDetailView(connectionID: id) { path.removeAll() }
                        }
                }
            }
        }
        .tint(Brand.primary)
        .sheet(item: authPromptBinding) { _ in
            AuthPromptSheet(model: model)
        }
        .sheet(isPresented: $showImport) {
            ImportSheet(model: model)
        }
        .sheet(isPresented: $showSettings) {
            SettingsView()
        }
        .task { await model.start() }
        .onChange(of: scenePhase) { _, phase in
            switch phase {
            case .active:     Task { await model.start() }
            case .background: model.pause()
            default:          break
            }
        }
    }

    private var authPromptBinding: Binding<AuthPrompt?> {
        Binding(
            get: { model.authPrompt },
            set: { if $0 == nil { model.dismissAuth() } }
        )
    }

    private func content(selected: String?, onOpen: @escaping (Connection) -> Void) -> some View {
        HomeContent(
            model: model,
            isManaged: app.isManaged,
            selected: selected,
            onOpen: onOpen,
            onImport: { showImport = true },
            onSignIn: { app.showSignIn() }
        )
        .toolbar {
            ToolbarItem(placement: .principal) { BrandTitle() }
            ToolbarItemGroup(placement: .topBarTrailing) {
                Button { showImport = true } label: {
                    Image(systemName: "plus")
                }
                .accessibilityLabel("Import configuration")
                Button { showSettings = true } label: {
                    Image(systemName: "gearshape")
                }
                .accessibilityLabel("Settings")
            }
        }
        .navigationBarTitleDisplayMode(.inline)
    }
}

private struct HomeContent: View {
    let model: ConnectionsModel
    let isManaged: Bool
    let selected: String?
    let onOpen: (Connection) -> Void
    let onImport: () -> Void
    let onSignIn: () -> Void

    var body: some View {
        let connections = model.connections
        let active = model.active
        let primary = model.primary

        ScrollView {
            LazyVStack(spacing: 12) {
                if isManaged && !model.isSignedIn {
                    NoticeCard(
                        tone: .info,
                        title: "You're signed out",
                        message: "Sign in to load your organization's connections.",
                        actionTitle: "Sign in",
                        action: onSignIn
                    )
                }
                if let message = model.message {
                    NoticeCard(tone: .error, title: message, onDismiss: { model.message = nil })
                        .transition(.opacity)
                }

                ConnectionHero(model: model, active: active, primary: primary)

                if connections.isEmpty {
                    emptyState.padding(.top, 8)
                } else {
                    Text("Connections")
                        .font(.footnote.weight(.semibold))
                        .foregroundStyle(.secondary)
                        .textCase(.uppercase)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.top, 8)
                        .padding(.leading, 4)
                        .accessibilityAddTraits(.isHeader)
                    ForEach(connections) { c in
                        ConnectionRow(
                            connection: c,
                            onOpen: { onOpen(c) },
                            onToggle: {
                                Task {
                                    if c.isActive { await model.disconnect(c) } else { await model.connect(c) }
                                }
                            }
                        )
                        .overlay(
                            RoundedRectangle(cornerRadius: 16, style: .continuous)
                                .strokeBorder(Brand.primary, lineWidth: selected == c.id ? 2 : 0)
                        )
                    }
                }
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 8)
            .frame(maxWidth: 640)
            .frame(maxWidth: .infinity)
            .animation(.easeInOut(duration: 0.2), value: connections.map(\.status))
        }
        .refreshable { await model.refresh() }
        .background(Brand.background.ignoresSafeArea())
        .safeAreaInset(edge: .bottom) {
            PrimaryActionBar(
                primary: primary,
                onConnect: { c in Task { await model.connect(c) } },
                onDisconnect: { c in Task { await model.disconnect(c) } }
            )
        }
        .sensoryFeedback(trigger: active?.status) { old, new in
            switch (old, new) {
            case (_, .connected?): return .success
            case (.connected?, nil), (.connected?, .disconnected?): return .impact(weight: .light)
            default: return nil
            }
        }
        .sensoryFeedback(.error, trigger: model.failedKey) { _, new in new != nil }
    }

    @ViewBuilder
    private var emptyState: some View {
        if !model.loadedOnce {
            ProgressView().frame(maxWidth: .infinity).padding(.vertical, 40)
        } else {
            let (title, message, icon): (String, String, String) =
                if isManaged && model.isSignedIn {
                    ("No connections assigned",
                     "Your administrator hasn't given you access to a server yet. Pull down to refresh.",
                     "server.rack")
                } else if isManaged {
                    ("Nothing here yet",
                     "Sign in to see your organization's servers, or import a configuration file.",
                     "server.rack")
                } else {
                    ("No configurations yet",
                     "Import a WireGuard configuration file to get started.",
                     "doc.text")
                }
            ContentUnavailableView {
                Label(title, systemImage: icon)
            } description: {
                Text(message)
            } actions: {
                Button("Import configuration", action: onImport)
                    .buttonStyle(.bordered)
            }
        }
    }
}

// MARK: - Hero

struct ConnectionHero: View {
    let model: ConnectionsModel
    let active: Connection?
    let primary: Connection?

    private var status: ConnStatus {
        active?.status ?? (primary?.status == .error ? .error : .disconnected)
    }

    private var subtitle: String {
        if let active { return active.name }
        if let primary { return "Ready to connect to \(primary.name)" }
        return "Add a connection to get started"
    }

    var body: some View {
        VStack(spacing: 0) {
            StatusAperture(status: status, size: 168)
                .padding(.bottom, 18)
            Text(status.label)
                .font(.title2.weight(.semibold))
                .foregroundStyle(status == .disconnected ? Color.primary : Brand.color(for: status))
                .contentTransition(.opacity)
            Text(subtitle)
                .font(.body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .lineLimit(2)
                .padding(.top, 4)
            if status == .connected {
                LiveStats(model: model, detail: active?.detail ?? "")
                    .transition(.opacity.combined(with: .move(edge: .top)))
            }
        }
        .padding(.vertical, 28)
        .padding(.horizontal, 20)
        .frame(maxWidth: .infinity)
        .card(radius: 24)
        .animation(.easeInOut(duration: 0.25), value: status)
        .accessibilityElement(children: .contain)
    }
}

private struct LiveStats: View {
    let model: ConnectionsModel
    let detail: String

    var body: some View {
        VStack(spacing: 10) {
            if let since = model.connectedSince {
                TimelineView(.periodic(from: .now, by: 1)) { context in
                    Text(Format.duration(context.date.timeIntervalSince(since)))
                        .font(.system(.title3, design: .monospaced).weight(.medium))
                        .monospacedDigit()
                        .accessibilityLabel("Connected for \(Format.duration(context.date.timeIntervalSince(since)))")
                }
            }
            HStack(spacing: 24) {
                traffic("arrow.down", "Received", model.stats?.rxBytes ?? 0)
                traffic("arrow.up", "Sent", model.stats?.txBytes ?? 0)
            }
            if !detail.isEmpty {
                Text(detail).font(.monoCaption).foregroundStyle(.secondary)
            }
        }
        .padding(.top, 16)
    }

    private func traffic(_ icon: String, _ label: String, _ bytes: Int64) -> some View {
        Label {
            Text(Format.bytes(bytes)).font(.mono).monospacedDigit()
        } icon: {
            Image(systemName: icon).foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(label) \(Format.bytes(bytes))")
    }
}
