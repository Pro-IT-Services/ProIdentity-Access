import SwiftUI

// Shared by the app and the Live Activity widget.

/// Geometry of the ProIdentity Access aperture mark (256-unit viewBox),
/// identical to client/frontend/src/components/brand/LogoMark.tsx.
private enum ApertureGeometry {
    static let hexagon: [CGPoint] = [
        CGPoint(x: 178, y: 128), CGPoint(x: 153, y: 171.3), CGPoint(x: 103, y: 171.3),
        CGPoint(x: 78, y: 128), CGPoint(x: 103, y: 84.7), CGPoint(x: 153, y: 84.7),
    ]
    static let bladeEnds: [CGPoint] = [
        CGPoint(x: 192.2, y: 199.3), CGPoint(x: 98.3, y: 219.3), CGPoint(x: 34.1, y: 148.0),
        CGPoint(x: 63.8, y: 56.7), CGPoint(x: 157.7, y: 36.7), CGPoint(x: 221.9, y: 108.0),
    ]
    static let center = CGPoint(x: 128, y: 128)

    static func scaled(_ p: CGPoint, in rect: CGRect) -> CGPoint {
        let unit = min(rect.width, rect.height) / 256
        return CGPoint(x: rect.minX + p.x * unit, y: rect.minY + p.y * unit)
    }
}

struct ApertureHexagon: Shape {
    func path(in rect: CGRect) -> Path {
        var path = Path()
        let points = ApertureGeometry.hexagon.map { ApertureGeometry.scaled($0, in: rect) }
        path.addLines(points)
        path.closeSubpath()
        return path
    }
}

struct ApertureBlades: Shape {
    func path(in rect: CGRect) -> Path {
        var path = Path()
        for (from, to) in zip(ApertureGeometry.hexagon, ApertureGeometry.bladeEnds) {
            path.move(to: ApertureGeometry.scaled(from, in: rect))
            path.addLine(to: ApertureGeometry.scaled(to, in: rect))
        }
        return path
    }
}
