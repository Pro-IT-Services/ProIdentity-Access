import SwiftUI

@main
struct ProIdentityApp: App {
    @StateObject private var appState = AppState()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(appState)
                .preferredColorScheme(.dark)
        }
    }
}

struct RootView: View {
    @EnvironmentObject var appState: AppState

    var body: some View {
        Group {
            if appState.setupDone {
                if appState.mode == "managed" {
                    ServerListView()
                } else {
                    TunnelListView()
                }
            } else {
                SetupView()
            }
        }
        .animation(.easeInOut(duration: 0.25), value: appState.setupDone)
        .onReceive(NotificationCenter.default.publisher(for: .authExpired)) { _ in
            appState.resetAll()
        }
    }
}

// MARK: - AppState

@MainActor
class AppState: ObservableObject {
    @Published var setupDone: Bool
    @Published var mode: String
    @Published var username: String
    @Published var tunnelStates: [String: String] = [:]

    init() {
        let s = AppSettings.shared
        if !UserDefaults.standard.bool(forKey: "reset_v1_done") {
            s.resetToRegister()
            UserDefaults.standard.set(true, forKey: "reset_v1_done")
        }
        setupDone = s.setupDone
        mode = s.mode
        username = s.username

        VPNManager.shared.onStateChanged = { [weak self] id, state in
            Task { @MainActor [weak self] in
                self?.tunnelStates[id] = state
            }
        }
    }

    func completeSetup() {
        let s = AppSettings.shared
        setupDone = true
        mode = s.mode
        username = s.username
    }

    func signOut() {
        AppSettings.shared.token = ""
        AppSettings.shared.username = ""
        username = ""
        setupDone = false
    }

    func resetAll() {
        AppSettings.shared.wipeAll()
        VPNManager.shared.wipeStoredConfigs()
        UserDefaults.standard.removeObject(forKey: "reset_v1_done")
        setupDone = false
        mode = ""
        username = ""
    }

    func tunnelStatus(_ id: String) -> String {
        tunnelStates[id] ?? "disconnected"
    }
}
