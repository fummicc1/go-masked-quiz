import SwiftUI

/// How a blank's mask should render: the text shown in place of the blank (a
/// number marker while unanswered, the answer once answered) and its tint.
struct BlankDisplay {
    let text: String
    let color: Color
}

/// A circled-number marker for a blank index (①②③…), falling back to (n).
func blankMarker(_ index: Int) -> String {
    let circled = ["①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨"]
    return index < circled.count ? circled[index] : "(\(index + 1))"
}

/// Renders a quiz body from its v3 blocks, dispatching on kind. `displays` maps
/// blank index → how that mask should render.
struct QuizBlocksView: View {
    let quiz: Quiz
    let displays: [Int: BlankDisplay]

    var body: some View {
        switch quiz.kind {
        case .prose, .llm:
            // LLM-generated quizzes use the same text/mask block shape as
            // mechanical prose quizzes (never code_block), so they share a
            // renderer.
            ProseBlocksView(blocks: quiz.blocks, displays: displays)
        case .code:
            CodeBlocksView(blocks: quiz.blocks, displays: displays)
        }
    }
}
