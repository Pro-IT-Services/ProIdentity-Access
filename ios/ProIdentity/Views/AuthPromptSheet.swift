import SwiftUI

/// Two-factor step when a server asks for it before connecting:
/// a 6-digit code or a push approval, with a switch between them.
struct AuthPromptSheet: View {
    let model: ConnectionsModel
    @Environment(\.dismiss) private var dismiss
    @FocusState private var codeFocused: Bool

    var body: some View {
        NavigationStack {
            Group {
                if let prompt = model.authPrompt {
                    ScrollView {
                        VStack(spacing: 20) {
                            if prompt.method == .code {
                                codeContent(prompt)
                            } else {
                                PushApprovalView(state: prompt.push, error: prompt.error)
                            }
                        }
                        .padding(20)
                    }
                    .safeAreaInset(edge: .bottom) {
                        VStack(spacing: 8) { actions(prompt) }
                            .padding(.horizontal, 20)
                            .padding(.vertical, 12)
                    }
                    .navigationTitle(prompt.method == .code ? "Verification code" : "Approve connection")
                }
            }
            .background(Brand.background.ignoresSafeArea())
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { model.dismissAuth() }
                }
            }
        }
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
        .interactiveDismissDisabled(model.authPrompt?.submitting == true)
    }

    private func codeContent(_ prompt: AuthPrompt) -> some View {
        VStack(spacing: 16) {
            Text("Enter the 6-digit code from your authenticator app to connect to \(prompt.serverName).")
                .font(.subheadline)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
            OTPField(code: codeBinding, isError: prompt.error != nil)
                .focused($codeFocused)
                .onChange(of: prompt.code) { _, new in
                    if new.count == 6 && !prompt.submitting { Task { await model.submitCode() } }
                }
            if let error = prompt.error {
                NoticeCard(tone: .error, title: error)
            }
        }
        .onAppear { codeFocused = true }
    }

    @ViewBuilder
    private func actions(_ prompt: AuthPrompt) -> some View {
        if prompt.method == .code {
            LoadingButton(title: "Verify and connect", loading: prompt.submitting) {
                Task { await model.submitCode() }
            }
            .disabled(prompt.code.count != 6)
            Button("Send a push approval instead") { model.startPush() }
                .font(.subheadline.weight(.medium))
                .frame(minHeight: 44)
        } else {
            if prompt.push.isFinishedWithoutApproval || (prompt.error != nil && prompt.push == .idle) {
                LoadingButton(title: "Send again") { model.startPush() }
            }
            if prompt.push != .approved {
                Button("Enter a code instead") { model.useCode() }
                    .font(.subheadline.weight(.medium))
                    .frame(minHeight: 44)
            }
        }
    }

    private var codeBinding: Binding<String> {
        Binding(
            get: { model.authPrompt?.code ?? "" },
            set: { model.authPrompt?.code = $0 }
        )
    }
}
