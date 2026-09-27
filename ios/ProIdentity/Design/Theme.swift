import SwiftUI
import UIKit

/// Aperture brand palette — ported from the desktop client, with light
/// counterparts chosen for WCAG AA contrast. Fixed brand colors, not tinted
/// by the system, so the security brand reads the same everywhere.
enum Brand {
    static let background  = dynamic(light: 0xF6F7F9, dark: 0x0F1115)
    static let surface     = dynamic(light: 0xFFFFFF, dark: 0x15181E)
    static let surfaceHigh = dynamic(light: 0xEEF0F4, dark: 0x1C2027)
    static let border      = dynamic(light: 0xE1E4EA, dark: 0x262A33)
    static let primary     = dynamic(light: 0x2563EB, dark: 0x3B82F6)
    static let connected   = dynamic(light: 0x15803D, dark: 0x22C55E)
    static let warning     = dynamic(light: 0xB45309, dark: 0xF59E0B)
    static let disconnect  = dynamic(light: 0xC2410C, dark: 0xF97316)
    static let danger      = dynamic(light: 0xB91C1C, dark: 0xEF4444)
    static let muted       = dynamic(light: 0x5B6472, dark: 0x9AA3B2)

    static func color(for status: ConnStatus) -> Color {
        switch status {
        case .connected:    return connected
        case .connecting:   return primary
        case .error:        return warning
        case .disconnected: return muted
        }
    }

    private static func dynamic(light: UInt32, dark: UInt32) -> Color {
        Color(UIColor { traits in
            UIColor(rgb: traits.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

private extension UIColor {
    convenience init(rgb: UInt32) {
        self.init(
            red: CGFloat((rgb >> 16) & 0xFF) / 255,
            green: CGFloat((rgb >> 8) & 0xFF) / 255,
            blue: CGFloat(rgb & 0xFF) / 255,
            alpha: 1
        )
    }
}

extension Font {
    /// IPs, endpoints, counters.
    static let mono = Font.system(.subheadline, design: .monospaced)
    static let monoCaption = Font.system(.caption, design: .monospaced)
}

/// Rounded card on the brand surface with a hairline border.
struct CardBackground: ViewModifier {
    var radius: CGFloat = 20

    func body(content: Content) -> some View {
        content
            .background(Brand.surface, in: RoundedRectangle(cornerRadius: radius, style: .continuous))
            .overlay(
                RoundedRectangle(cornerRadius: radius, style: .continuous)
                    .strokeBorder(Brand.border, lineWidth: 1)
            )
    }
}

extension View {
    func card(radius: CGFloat = 20) -> some View { modifier(CardBackground(radius: radius)) }
}
