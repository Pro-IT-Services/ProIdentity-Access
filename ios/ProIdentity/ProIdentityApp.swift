import SwiftUI

@main
struct ProIdentityApp: App {
    @State private var app = AppModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(app)
        }
    }
}

/// Onboarding → (sign in) → Home. Follows the system light/dark setting.
struct RootView: View {
    @Environment(AppModel.self) private var app

    var body: some View {
        ZStack {
            Brand.background.ignoresSafeArea()
            switch app.route {
            case .onboarding:
                OnboardingView().transition(.opacity)
            case .signIn:
                SignInView().transition(.opacity)
            case .home:
                HomeView().transition(.opacity)
            }
        }
        .animation(.easeInOut(duration: 0.25), value: app.route)
    }
}
