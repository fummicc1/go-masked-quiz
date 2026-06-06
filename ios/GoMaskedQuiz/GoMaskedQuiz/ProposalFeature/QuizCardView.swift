import SwiftUI

/// One quiz: its rendered body plus tappable choices, with correct/incorrect
/// feedback. (Choice colouring ported from se-masked-quiz's QuizSelectionsView.)
struct QuizCardView: View {
    let quiz: Quiz
    @ObservedObject var viewModel: QuizViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                Text("Q\(quiz.index + 1)").font(.caption).foregroundStyle(.secondary)
                Text(quiz.kind == .code ? "コード" : "本文")
                    .font(.caption2)
                    .padding(.horizontal, 6).padding(.vertical, 2)
                    .background(.secondary.opacity(0.15))
                    .clipShape(Capsule())
            }

            QuizBlocksView(quiz: quiz, revealedAnswer: revealed, isCorrect: isCorrect)

            ForEach(quiz.choices, id: \.self) { choice in
                Button {
                    Task { await viewModel.selectAnswer(choice, for: quiz) }
                } label: {
                    Text(choice)
                        .font(.system(.body, design: .monospaced))
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 10)
                        .background(background(for: choice))
                        .foregroundColor(.white)
                        .clipShape(RoundedRectangle(cornerRadius: 10))
                }
                .disabled(answered)
            }

            if let isCorrect {
                Text(isCorrect ? "正解！" : "不正解… 正解は \(quiz.answer)")
                    .font(.subheadline.bold())
                    .foregroundColor(isCorrect ? .green : .red)
            }
        }
        .padding()
        .background(.background.secondary)
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }

    private var selected: String? { viewModel.selectedAnswer[quiz.index] }
    private var isCorrect: Bool? { viewModel.isCorrect[quiz.index] }
    private var answered: Bool { isCorrect != nil }
    private var revealed: String? { answered ? quiz.answer : nil }

    private func background(for choice: String) -> Color {
        guard let selected else { return .blue } // unanswered
        if choice == quiz.answer { return .green }
        if choice == selected { return .red } // chosen but wrong
        return .gray
    }
}
