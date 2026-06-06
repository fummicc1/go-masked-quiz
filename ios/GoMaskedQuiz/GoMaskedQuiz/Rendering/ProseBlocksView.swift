import SwiftUI

/// Renders prose blocks (text / inline_code / mask) as a single AttributedString.
struct ProseBlocksView: View {
    let blocks: [Block]
    let mask: String
    let tint: Color

    var body: some View {
        Text(attributed)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityElement(children: .combine)
    }

    /// Exposed for testing: the composed AttributedString.
    var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .text(let s):
                out.append(AttributedString(s))
            case .inlineCode(let s):
                var a = AttributedString(s)
                a.font = .system(.body, design: .monospaced)
                a.backgroundColor = .secondary.opacity(0.15)
                out.append(a)
            case .mask:
                var a = AttributedString(mask)
                a.font = .system(.body, design: .monospaced).bold()
                a.backgroundColor = tint
                out.append(a)
            case .codeBlock, .unknown:
                continue // not expected in prose quizzes
            }
        }
        return out
    }
}
