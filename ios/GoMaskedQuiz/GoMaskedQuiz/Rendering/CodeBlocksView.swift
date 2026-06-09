import SwiftUI

/// Renders code blocks (code_block / mask) as monospaced text with horizontal
/// scrolling. Each mask shows its BlankDisplay (number marker or answer).
struct CodeBlocksView: View {
    let blocks: [Block]
    let displays: [Int: BlankDisplay]

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Text(attributed)
                .font(.system(.body, design: .monospaced))
                .padding(12)
        }
        .background(.secondary.opacity(0.1))
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .codeBlock(let s):
                out.append(AttributedString(s))
            case .mask(let bi):
                let d = displays[bi] ?? BlankDisplay(text: blankMarker(bi), color: .yellow.opacity(0.35))
                var a = AttributedString(d.text)
                a.font = .system(.body, design: .monospaced).bold()
                a.backgroundColor = d.color
                out.append(a)
            case .text, .inlineCode, .unknown:
                continue
            }
        }
        return out
    }
}
