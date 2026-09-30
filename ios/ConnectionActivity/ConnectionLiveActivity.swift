import ActivityKit
import AppIntents
import SwiftUI
import WidgetKit

/// Lock screen + Dynamic Island for a live VPN connection.
struct ConnectionLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: ConnectionActivityAttributes.self) { context in
            LockScreenView(context: context)
                .activityBackgroundTint(nil)
                .activitySystemActionForegroundColor(Palette.connected)
        } dynamicIsland: { context in
            DynamicIsland {
                DynamicIslandExpandedRegion(.leading) {
                    Mark(size: 30, color: stateColor(context.state))
                        .padding(.leading, 4)
                }
                DynamicIslandExpandedRegion(.center) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text(context.attributes.name)
                            .font(.headline)
                            .lineLimit(1)
                        StatusLine(state: context.state)
                            .font(.subheadline)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    DisconnectButton(tunnelID: context.attributes.tunnelID)
                        .padding(.top, 4)
                }
            } compactLeading: {
                Mark(size: 18, color: stateColor(context.state))
            } compactTrailing: {
                if context.state.connected {
                    Text(context.state.since, style: .timer)
                        .monospacedDigit()
                        .frame(maxWidth: 52)
                        .foregroundStyle(Palette.connected)
                } else {
                    Image(systemName: "arrow.triangle.2.circlepath")
                        .foregroundStyle(Palette.warning)
                }
            } minimal: {
                Mark(size: 18, color: stateColor(context.state))
            }
            .keylineTint(stateColor(context.state))
        }
    }
}

private struct LockScreenView: View {
    let context: ActivityViewContext<ConnectionActivityAttributes>

    var body: some View {
        HStack(spacing: 14) {
            Mark(size: 36, color: stateColor(context.state))
            VStack(alignment: .leading, spacing: 2) {
                Text(context.attributes.name)
                    .font(.headline)
                    .lineLimit(1)
                StatusLine(state: context.state)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 8)
            DisconnectButton(tunnelID: context.attributes.tunnelID, compact: true)
        }
        .padding(16)
    }
}

/// "Connected · 12:04" (counting up) or "Reconnecting…".
private struct StatusLine: View {
    let state: ConnectionActivityAttributes.ContentState

    var body: some View {
        if state.connected {
            HStack(spacing: 4) {
                Text("Connected ·")
                Text(state.since, style: .timer).monospacedDigit()
            }
        } else {
            Text("Reconnecting…")
        }
    }
}

private struct DisconnectButton: View {
    let tunnelID: String
    var compact = false

    var body: some View {
        Button(intent: DisconnectVPNIntent(tunnelID: tunnelID)) {
            Label("Disconnect", systemImage: "power")
                .font(.subheadline.weight(.semibold))
                .labelStyle(.titleAndIcon)
                .frame(maxWidth: compact ? nil : .infinity)
        }
        .buttonStyle(.borderedProminent)
        .tint(Palette.disconnect)
        .accessibilityLabel("Disconnect VPN")
    }
}

/// The brand aperture, filled like the app's connected state.
private struct Mark: View {
    let size: CGFloat
    let color: Color

    var body: some View {
        let unit = size / 256
        ZStack {
            ApertureHexagon()
                .stroke(color, style: StrokeStyle(lineWidth: 16 * unit, lineCap: .round, lineJoin: .round))
            ApertureBlades()
                .stroke(color, style: StrokeStyle(lineWidth: 13 * unit, lineCap: .round))
            Circle().fill(color).frame(width: 30 * unit, height: 30 * unit)
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// Brand colors (the widget can't use the app's asset-based theme).
private enum Palette {
    static let connected = Color(red: 0x22 / 255, green: 0xC5 / 255, blue: 0x5E / 255)
    static let warning = Color(red: 0xF5 / 255, green: 0x9E / 255, blue: 0x0B / 255)
    static let disconnect = Color(red: 0xEA / 255, green: 0x58 / 255, blue: 0x0C / 255)
}

private func stateColor(_ state: ConnectionActivityAttributes.ContentState) -> Color {
    state.connected ? Palette.connected : Palette.warning
}
