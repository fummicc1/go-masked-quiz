import SwiftUI

/// Renders prose blocks (text / inline_code / mask) as one AttributedString.
/// Each mask shows its BlankDisplay (number marker or answer) as a coloured
/// token; inline code is cyan monospaced.
struct ProseBlocksView: View {
    let blocks: [Block]
    let displays: [Int: BlankDisplay]

    var body: some View {
        Text(attributed)
            .lineSpacing(6)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityElement(children: .combine)
    }

    var attributed: AttributedString {
        var out = AttributedString()
        for block in blocks {
            switch block {
            case .text(let s):
                var a = AttributedString(s)
                a.font = Theme.body(16)
                a.foregroundColor = Theme.textPrimary
                out.append(a)
            case .inlineCode(let s):
                var a = AttributedString(s)
                a.font = Theme.mono(15)
                a.foregroundColor = Theme.accent
                a.backgroundColor = Theme.surfaceElevated
                out.append(a)
            case .mask(let bi):
                out.append(maskToken(bi))
            case .codeBlock, .unknown:
                continue
            }
        }
        return out
    }

    private func maskToken(_ blankIndex: Int) -> AttributedString {
        let d = displays[blankIndex] ?? BlankDisplay(text: blankMarker(blankIndex), color: Theme.accent)
        var a = AttributedString(" \(d.text) ")
        a.font = Theme.mono(15, .bold)
        a.foregroundColor = Theme.onAccent
        a.backgroundColor = d.color
        return a
    }
}
