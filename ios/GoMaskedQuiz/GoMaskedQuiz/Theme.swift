import SwiftUI
import UIKit

/// "Terminal Editorial" design tokens. Colours are dynamic (light/dark) so the
/// app follows the system appearance; the cyan accent and monospaced code carry
/// the identity in both modes.
enum Theme {
    static let bg = Color(light: 0xEEF1F4, dark: 0x0B0E14)
    static let surface = Color(light: 0xFFFFFF, dark: 0x141A23)
    static let surfaceElevated = Color(light: 0xFFFFFF, dark: 0x1B2330)
    static let border = Color(light: 0xD7DDE3, dark: 0x2A3340)
    static let accent = Color(light: 0x0883A0, dark: 0x29D3E0)  // Go-cyan, tuned per mode
    static var accentSoft: Color { accent.opacity(0.14) }
    static let onAccent = Color(light: 0xFFFFFF, dark: 0x06222A) // text on a coloured token
    static let textPrimary = Color(light: 0x1F2328, dark: 0xE6EDF3)
    static let textSecondary = Color(light: 0x656D76, dark: 0x8B949E)
    static let textFaint = Color(light: 0x8C959F, dark: 0x5A6573)
    static let success = Color(light: 0x1A7F37, dark: 0x3FB950)
    static let danger = Color(light: 0xCF222E, dark: 0xF85149)

    static func display(_ size: CGFloat, _ weight: Font.Weight = .bold) -> Font {
        .system(size: size, weight: weight, design: .rounded)
    }
    static func body(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        .system(size: size, weight: weight)
    }
    static func mono(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        .system(size: size, weight: weight, design: .monospaced)
    }
}

extension Color {
    init(hex: UInt, alpha: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: alpha
        )
    }

    /// A dynamic colour that resolves to `light` or `dark` per the trait style.
    init(light: UInt, dark: UInt) {
        self.init(uiColor: UIColor { traits in
            UIColor(rgb: traits.userInterfaceStyle == .dark ? dark : light)
        })
    }
}

private extension UIColor {
    convenience init(rgb hex: UInt) {
        self.init(
            red: CGFloat((hex >> 16) & 0xFF) / 255,
            green: CGFloat((hex >> 8) & 0xFF) / 255,
            blue: CGFloat(hex & 0xFF) / 255,
            alpha: 1
        )
    }
}

/// A thin progress rail used in cards and headers.
struct ProgressRail: View {
    let rate: Double
    var done: Bool = false

    var body: some View {
        GeometryReader { geo in
            ZStack(alignment: .leading) {
                Capsule().fill(Theme.border)
                Capsule()
                    .fill(done ? Theme.success : Theme.accent)
                    .frame(width: max(0, min(1, rate)) * geo.size.width)
            }
        }
        .frame(height: 4)
    }
}
