import SwiftUI

/// Static brand mark for headers. Heavier strokes than the large mark so it
/// stays legible at 20–28pt.
struct ApertureMark: View {
    var size: CGFloat = 24
    var color: Color = Brand.primary

    var body: some View {
        let unit = size / 256
        ZStack {
            ApertureHexagon()
                .stroke(color, style: StrokeStyle(lineWidth: 15 * unit, lineCap: .round, lineJoin: .round))
            ApertureBlades()
                .stroke(color, style: StrokeStyle(lineWidth: 12 * unit, lineCap: .round))
            Circle().fill(color).frame(width: 28 * unit, height: 28 * unit)
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// The connection indicator: the brand aperture, alive.
///  - Disconnected: muted outline.
///  - Connecting: the aperture turns, an arc sweeps the perimeter, the hexagon breathes.
///  - Connected: filled in the connected color with a soft glow.
///  - Error: warning color.
/// Only transforms and opacity animate; with Reduce Motion it is static.
struct StatusAperture: View {
    let status: ConnStatus
    var size: CGFloat = 176

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var tint: Color { Brand.color(for: status) }
    private var animating: Bool { status == .connecting && !reduceMotion }

    var body: some View {
        TimelineView(.animation(paused: !animating)) { context in
            let t = animating ? context.date.timeIntervalSinceReferenceDate : 0
            mark(spin: (t / 4.2).truncatingRemainder(dividingBy: 1) * 360,
                 sweep: (t / 1.4).truncatingRemainder(dividingBy: 1) * 360,
                 pulse: animating ? 0.775 + 0.225 * sin(t * .pi / 0.9) : 1)
        }
        .frame(width: size, height: size)
        .animation(.easeInOut(duration: 0.3), value: status)
        .accessibilityElement()
        .accessibilityLabel("Connection status")
        .accessibilityValue(status.label)
    }

    private func mark(spin: Double, sweep: Double, pulse: Double) -> some View {
        let unit = size / 256
        let ring = 208 * unit
        return ZStack {
            if status == .connected {
                Circle()
                    .fill(RadialGradient(colors: [tint.opacity(0.30), tint.opacity(0)],
                                         center: .center, startRadius: 0, endRadius: size / 2))
                    .transition(.opacity)
            }
            Circle()
                .stroke(Brand.border, lineWidth: 3 * unit)
                .frame(width: ring, height: ring)
            if status == .connecting {
                Circle()
                    .trim(from: 0, to: 70.0 / 360)
                    .stroke(tint, style: StrokeStyle(lineWidth: 6 * unit, lineCap: .round))
                    .frame(width: ring, height: ring)
                    .rotationEffect(.degrees(sweep - 90))
            }
            ZStack {
                if status == .connected {
                    ApertureHexagon().fill(tint.opacity(0.16))
                }
                ApertureHexagon()
                    .stroke(tint, style: StrokeStyle(lineWidth: 9 * unit, lineCap: .round, lineJoin: .round))
                    .opacity(pulse)
                ApertureBlades()
                    .stroke(tint, style: StrokeStyle(lineWidth: 7 * unit, lineCap: .round))
                Circle().fill(tint).frame(width: 24 * unit, height: 24 * unit)
            }
            .rotationEffect(.degrees(spin))
        }
        .frame(width: size, height: size)
    }
}

#Preview("Status aperture") {
    HStack(spacing: 16) {
        StatusAperture(status: .disconnected, size: 80)
        StatusAperture(status: .connecting, size: 80)
        StatusAperture(status: .connected, size: 80)
        StatusAperture(status: .error, size: 80)
    }
    .padding()
    .background(Brand.background)
}
