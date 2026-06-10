import SwiftUI

/// Renders code blocks (code_block / mask) as monospaced text with horizontal
/// scrolling, on a darker code surface. Each mask is a coloured token.
struct CodeBlocksView: View {
    let blocks: [Block]
    let displays: [Int: BlankDisplay]

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Text(attributed)
                .font(Theme.mono(13))
                .padding(14)
        }
        .background(Theme.bg)
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(Theme.border, lineWidth: 1))
        .clipShape(RoundedRectangle(cornerRadius: 10))
    }

    var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .codeBlock(let s):
                var a = AttributedString(s)
                a.font = Theme.mono(13)
                a.foregroundColor = Theme.textPrimary
                out.append(a)
            case .mask(let bi):
                let d = displays[bi] ?? BlankDisplay(text: blankMarker(bi), color: Theme.accent)
                var a = AttributedString(d.text)
                a.font = Theme.mono(13, .bold)
                a.foregroundColor = Theme.onAccent
                a.backgroundColor = d.color
                out.append(a)
            case .text, .inlineCode, .unknown:
                continue
            }
        }
        return out
    }
}
