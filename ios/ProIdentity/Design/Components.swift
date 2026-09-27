import SwiftUI

// MARK: - Formatting

enum Format {
    private static let byteFormatter: ByteCountFormatter = {
        let f = ByteCountFormatter()
        f.countStyle = .binary
        f.allowsNonnumericFormatting = false // "0 bytes", not "Zero KB"
        return f
    }()

    static func bytes(_ value: Int64) -> String {
        byteFormatter.string(fromByteCount: value)
    }

    static func duration(_ seconds: TimeInterval) -> String {
        let s = max(0, Int(seconds))
        let h = s / 3600, m = (s % 3600) / 60, sec = s % 60
        return h > 0 ? String(format: "%d:%02d:%02d", h, m, sec) : String(format: "%02d:%02d", m, sec)
    }

    static func ago(_ date: Date) -> String {
        let seconds = Int(Date().timeIntervalSince(date))
        if seconds < 5 { return "Just now" }
        if seconds < 60 { return "\(seconds)s ago" }
        if seconds < 3600 { return "\(seconds / 60)m ago" }
        return "\(seconds / 3600)h ago"
    }
}

// MARK: - Brand title

struct BrandTitle: View {
    var body: some View {
        HStack(spacing: 8) {
            ApertureMark(size: 22)
            Text("ProIdentity Access").font(.headline)
        }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.isHeader)
    }
}

// MARK: - Status

struct StatusDot: View {
    let status: ConnStatus
    var body: some View {
        Circle()
            .fill(Brand.color(for: status))
            .frame(width: 8, height: 8)
            .accessibilityHidden(true)
    }
}

struct KindBadge: View {
    let text: String
    var body: some View {
        Text(text)
            .font(.caption2.weight(.semibold))
            .foregroundStyle(Brand.muted)
            .padding(.horizontal, 7).padding(.vertical, 3)
            .background(Brand.surfaceHigh, in: Capsule())
    }
}

// MARK: - Connection row

struct ConnectionRow: View {
    let connection: Connection
    let onOpen: () -> Void
    let onToggle: () -> Void

    var body: some View {
        HStack(spacing: 12) {
            Button(action: onOpen) {
                HStack(spacing: 12) {
                    Image(systemName: connection.kind == .managed ? "server.rack" : "doc.text")
                        .font(.body)
                        .foregroundStyle(connection.isActive ? Brand.color(for: connection.status) : Brand.muted)
                        .frame(width: 36, height: 36)
                        .background(Brand.surfaceHigh, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
                    VStack(alignment: .leading, spacing: 3) {
                        HStack(spacing: 6) {
                            Text(connection.name)
                                .font(.body.weight(.semibold))
                                .foregroundStyle(.primary)
                                .lineLimit(1)
                            KindBadge(text: connection.badge)
                        }
                        HStack(spacing: 6) {
                            StatusDot(status: connection.status)
                            Text(connection.status == .disconnected && !connection.detail.isEmpty
                                 ? connection.detail : connection.status.label)
                                .font(connection.status == .disconnected ? .monoCaption : .caption)
                                .foregroundStyle(connection.status == .disconnected ? Brand.muted : Brand.color(for: connection.status))
                                .lineLimit(1)
                        }
                    }
                    Spacer(minLength: 0)
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityElement(children: .combine)
            .accessibilityLabel("\(connection.name), \(connection.badge), \(connection.status.label)")
            .accessibilityHint("Shows details")

            toggle
        }
        .padding(.leading, 12).padding(.trailing, 6).padding(.vertical, 8)
        .card(radius: 16)
    }

    @ViewBuilder
    private var toggle: some View {
        Button(action: onToggle) {
            ZStack {
                if connection.status == .connecting {
                    ProgressView().controlSize(.small)
                } else {
                    Image(systemName: connection.isActive ? "stop.fill" : "play.fill")
                        .font(.subheadline)
                        .foregroundStyle(connection.isActive ? Brand.disconnect : Brand.primary)
                }
            }
            .frame(width: 44, height: 44)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(connection.isActive ? "Disconnect \(connection.name)" : "Connect \(connection.name)")
    }
}

// MARK: - Primary action

struct PrimaryActionBar: View {
    let primary: Connection?
    let onConnect: (Connection) -> Void
    let onDisconnect: (Connection) -> Void

    var body: some View {
        if let c = primary {
            let (title, color): (String, Color) = switch c.status {
            case .connected:  ("Disconnect", Brand.disconnect)
            case .connecting: ("Cancel", Brand.muted)
            default:          (c.status == .error ? "Try again" : "Connect", Brand.primary)
            }
            VStack(spacing: 0) {
                Button {
                    c.isActive ? onDisconnect(c) : onConnect(c)
                } label: {
                    VStack(spacing: 2) {
                        Text(title).font(.headline)
                        if !c.isActive {
                            Text(c.name).font(.caption).opacity(0.85).lineLimit(1)
                        }
                    }
                    .frame(maxWidth: .infinity, minHeight: 50)
                }
                .buttonStyle(FilledButtonStyle(color: color))
                .accessibilityLabel(c.isActive ? "\(title) \(c.name)" : "Connect to \(c.name)")
            }
            .padding(.horizontal, 16)
            .padding(.top, 10)
            .padding(.bottom, 8)
            .background(.bar)
        }
    }
}

// MARK: - Buttons & fields

struct FilledButtonStyle: ButtonStyle {
    var color: Color = Brand.primary
    @Environment(\.isEnabled) private var isEnabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.headline)
            .foregroundStyle(.white)
            .frame(maxWidth: .infinity, minHeight: 50)
            .background(color.opacity(isEnabled ? 1 : 0.45), in: RoundedRectangle(cornerRadius: 14, style: .continuous))
            .opacity(configuration.isPressed ? 0.85 : 1)
            .scaleEffect(configuration.isPressed ? 0.98 : 1)
            .animation(.easeOut(duration: 0.15), value: configuration.isPressed)
    }
}

/// Primary button with an inline spinner.
struct LoadingButton: View {
    let title: String
    var loading = false
    var color: Color = Brand.primary
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: 8) {
                if loading { ProgressView().tint(.white) }
                Text(title)
            }
        }
        .buttonStyle(FilledButtonStyle(color: color))
        .disabled(loading)
    }
}

/// A labelled text-field container on the brand surface.
struct BrandField<Field: View>: View {
    let label: String
    @ViewBuilder let field: Field

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(label).font(.subheadline.weight(.medium)).foregroundStyle(.secondary)
            field
                .padding(.horizontal, 14)
                .frame(minHeight: 50)
                .card(radius: 12)
        }
    }
}

// MARK: - Notices

struct NoticeCard: View {
    enum Tone { case info, warning, error }

    let tone: Tone
    let title: String
    var message: String? = nil
    var actionTitle: String? = nil
    var action: (() -> Void)? = nil
    var onDismiss: (() -> Void)? = nil

    private var color: Color {
        switch tone {
        case .info: return Brand.primary
        case .warning: return Brand.warning
        case .error: return Brand.danger
        }
    }

    private var icon: String {
        switch tone {
        case .info: return "info.circle.fill"
        case .warning: return "exclamationmark.triangle.fill"
        case .error: return "xmark.octagon.fill"
        }
    }

    var body: some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: icon).foregroundStyle(color).font(.body)
            VStack(alignment: .leading, spacing: 4) {
                Text(title).font(.subheadline.weight(.semibold))
                if let message {
                    Text(message).font(.subheadline).foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if let actionTitle, let action {
                    Button(actionTitle, action: action)
                        .font(.subheadline.weight(.semibold))
                        .tint(color)
                        .padding(.top, 4)
                }
            }
            Spacer(minLength: 0)
            if let onDismiss {
                Button(action: onDismiss) {
                    Image(systemName: "xmark").font(.caption.weight(.bold)).foregroundStyle(.secondary)
                        .frame(width: 32, height: 32)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Dismiss")
            }
        }
        .padding(14)
        .background(color.opacity(0.10), in: RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(color.opacity(0.25)))
        .accessibilityElement(children: .contain)
    }
}

/// Label/value row for detail lists; mono for addresses.
struct InfoRow: View {
    let label: String
    let value: String
    var mono = false

    var body: some View {
        LabeledContent(label) {
            Text(value)
                .font(mono ? .mono : .body)
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.trailing)
                .textSelection(.enabled)
        }
    }
}
