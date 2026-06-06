import SwiftUI

/// Renders a quiz body from its pre-parsed v2 blocks, dispatching on kind.
/// `revealedAnswer` is nil while unanswered (mask shows a placeholder) and the
/// answer text once answered; `isCorrect` tints the mask.
struct QuizBlocksView: View {
    let quiz: Quiz
    let revealedAnswer: String?
    let isCorrect: Bool?

    var body: some View {
        switch quiz.kind {
        case .prose:
            ProseBlocksView(blocks: quiz.blocks, mask: maskText, tint: tint)
        case .code:
            CodeBlocksView(blocks: quiz.blocks, mask: maskText, tint: tint)
        }
    }

    private var maskText: String { revealedAnswer ?? "______" }

    private var tint: Color {
        guard let isCorrect else { return .yellow.opacity(0.35) }
        return isCorrect ? .green.opacity(0.35) : .red.opacity(0.35)
    }
}
