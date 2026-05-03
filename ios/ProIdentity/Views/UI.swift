import SwiftUI

// MARK: - Color Palette

extension Color {
    static let appBg       = Color(red: 0.059, green: 0.067, blue: 0.071)
    static let appSurface  = Color(red: 0.098, green: 0.110, blue: 0.137)
    static let appSurface2 = Color(red: 0.122, green: 0.137, blue: 0.165)
    static let appAccent   = Color(red: 0.506, green: 0.549, blue: 0.973)
    static let appGreen    = Color(red: 0.133, green: 0.773, blue: 0.369)
    static let appAmber    = Color(red: 0.961, green: 0.620, blue: 0.043)
    static let appRed      = Color(red: 0.937, green: 0.267, blue: 0.267)
    static let appGray     = Color(white: 0.55)
}

// MARK: - TextField Style

struct AppFieldStyle: TextFieldStyle {
    func _body(configuration: TextField<_Label>) -> some View {
        configuration
            .padding(.horizontal, 14)
            .padding(.vertical, 13)
            .background(Color.appSurface2)
            .foregroundColor(.white)
            .cornerRadius(10)
            .overlay(
                RoundedRectangle(cornerRadius: 10)
                    .stroke(Color.white.opacity(0.08), lineWidth: 1)
            )
    }
}

// MARK: - Primary Button

struct PrimaryButton: View {
    let title: String
    var loading: Bool = false
    var destructive: Bool = false
    let action: () -> Void

    init(_ title: String, loading: Bool = false, destructive: Bool = false, action: @escaping () -> Void) {
        self.title = title
        self.loading = loading
        self.destructive = destructive
        self.action = action
    }

    private var bg: Color { destructive ? .appRed : .appAccent }

    var body: some View {
        Button(action: action) {
            HStack(spacing: 8) {
                if loading {
                    ProgressView().tint(.white).scaleEffect(0.85)
                }
                Text(title).fontWeight(.semibold)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 14)
            .background(loading ? bg.opacity(0.55) : bg)
            .foregroundColor(.white)
            .cornerRadius(12)
        }
        .disabled(loading)
    }
}

// MARK: - Status Badge

struct StatusBadge: View {
    let status: String

    private var color: Color {
        switch status {
        case "connected":  return .appGreen
        case "connecting": return .appAmber
        case "error":      return .appRed
        default:           return .appGray
        }
    }

    private var label: String {
        switch status {
        case "connected":     return "Connected"
        case "connecting":    return "Connecting"
        case "disconnecting": return "Disconnecting"
        case "error":         return "Error"
        default:              return "Disconnected"
        }
    }

    var body: some View {
        HStack(spacing: 5) {
            Circle().fill(color).frame(width: 6, height: 6)
            Text(label).font(.caption.weight(.medium)).foregroundColor(color)
        }
        .padding(.horizontal, 8).padding(.vertical, 4)
        .background(color.opacity(0.12))
        .cornerRadius(20)
    }
}

// MARK: - Error Banner

struct ErrorBanner: View {
    let message: String

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundColor(.appRed)
            Text(message)
                .font(.footnote)
                .foregroundColor(.white.opacity(0.9))
        }
        .padding(.horizontal, 14).padding(.vertical, 10)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.appRed.opacity(0.12))
        .cornerRadius(10)
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .stroke(Color.appRed.opacity(0.25), lineWidth: 1)
        )
    }
}
