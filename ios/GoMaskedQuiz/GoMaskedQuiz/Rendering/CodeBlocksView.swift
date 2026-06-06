import SwiftUI

/// Renders code blocks (code_block / mask) as monospaced text with horizontal
/// scrolling so long lines don't wrap awkwardly.
struct CodeBlocksView: View {
    let blocks: [Block]
    let mask: String
    let tint: Color

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Text(attributed)
                .font(.system(.body, design: .monospaced))
                .padding(12)
        }
        .background(.secondary.opacity(0.1))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    /// Exposed for testing: the composed AttributedString.
    var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .codeBlock(let s):
                out.append(AttributedString(s))
            case .mask:
                var a = AttributedString(mask)
                a.font = .system(.body, design: .monospaced).bold()
                a.backgroundColor = tint
                out.append(a)
            case .text, .inlineCode, .unknown:
                continue // not expected in code quizzes
            }
        }
        return out
    }
}
