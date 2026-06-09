import SwiftUI

/// Renders prose blocks (text / inline_code / mask) as one AttributedString.
/// Each mask shows its BlankDisplay (number marker or answer) with a tint.
struct ProseBlocksView: View {
    let blocks: [Block]
    let displays: [Int: BlankDisplay]

    var body: some View {
        Text(attributed)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityElement(children: .combine)
    }

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
            case .mask(let bi):
                out.append(maskAttributed(bi))
            case .codeBlock, .unknown:
                continue
            }
        }
        return out
    }

    private func maskAttributed(_ blankIndex: Int) -> AttributedString {
        let d = displays[blankIndex] ?? BlankDisplay(text: blankMarker(blankIndex), color: .yellow.opacity(0.35))
        var a = AttributedString(" \(d.text) ")
        a.font = .system(.body, design: .monospaced).bold()
        a.backgroundColor = d.color
        return a
    }
}
