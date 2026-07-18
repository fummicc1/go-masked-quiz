import SwiftUI

/// One quiz (paragraph or code block): its rendered body with numbered blanks,
/// plus a choice row per blank, on an elevated card.
struct QuizCardView: View {
    let quiz: Quiz
    @ObservedObject var viewModel: QuizViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 8) {
                Text("Q\(quiz.index + 1)")
                    .font(Theme.mono(12, .semibold))
                    .foregroundStyle(Theme.textSecondary)
                Text(kindLabel)
                    .font(Theme.mono(10, .medium))
                    .foregroundStyle(Theme.accent)
                    .padding(.horizontal, 7).padding(.vertical, 2)
                    .background(Theme.accentSoft)
                    .clipShape(Capsule())
                Spacer()
            }

            QuizBlocksView(quiz: quiz, displays: displays)

            ForEach(quiz.blanks.indices, id: \.self) { bi in
                Divider().overlay(Theme.border)
                blankRow(bi)
            }
        }
        .padding(16)
        .background(Theme.surfaceElevated)
        .overlay(RoundedRectangle(cornerRadius: 16).stroke(Theme.border, lineWidth: 1))
        .clipShape(RoundedRectangle(cornerRadius: 16))
    }

    private var kindLabel: String {
        switch quiz.kind {
        case .code: "code"
        case .prose: "prose"
        case .llm: "llm"
        }
    }

    private var displays: [Int: BlankDisplay] {
        var d: [Int: BlankDisplay] = [:]
        for bi in quiz.blanks.indices {
            let st = viewModel.state(quiz: quiz, blankIndex: bi)
            if let isCorrect = st.isCorrect {
                d[bi] = BlankDisplay(text: quiz.blanks[bi].answer, color: isCorrect ? Theme.success : Theme.danger)
            } else {
                d[bi] = BlankDisplay(text: blankMarker(bi), color: Theme.accent)
            }
        }
        return d
    }

    @ViewBuilder
    private func blankRow(_ bi: Int) -> some View {
        let st = viewModel.state(quiz: quiz, blankIndex: bi)
        let answered = st.isCorrect != nil
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Text(blankMarker(bi))
                    .font(Theme.mono(15, .bold))
                    .foregroundStyle(Theme.accent)
                if let isCorrect = st.isCorrect {
                    Text(isCorrect ? "correct" : "answer: \(quiz.blanks[bi].answer)")
                        .font(Theme.mono(11))
                        .foregroundStyle(isCorrect ? Theme.success : Theme.danger)
                }
            }
            LazyVGrid(columns: [GridItem(.flexible(), spacing: 8), GridItem(.flexible(), spacing: 8)], spacing: 8) {
                ForEach(quiz.blanks[bi].choices, id: \.self) { choice in
                    Button {
                        Task { await viewModel.selectAnswer(choice, quiz: quiz, blankIndex: bi) }
                    } label: {
                        let style = chipStyle(bi, choice, st)
                        Text(choice)
                            .font(Theme.mono(14, .medium))
                            .foregroundStyle(style.fg)
                            .lineLimit(1)
                            .minimumScaleFactor(0.7)
                            .frame(maxWidth: .infinity, minHeight: 22)
                            .padding(.horizontal, 12).padding(.vertical, 10)
                            .background(style.bg)
                            .overlay(RoundedRectangle(cornerRadius: 10).stroke(style.border, lineWidth: 1))
                            .clipShape(RoundedRectangle(cornerRadius: 10))
                    }
                    .disabled(answered)
                }
            }
        }
    }

    private func chipStyle(_ bi: Int, _ choice: String, _ st: (selected: String?, isCorrect: Bool?)) -> (bg: Color, fg: Color, border: Color) {
        guard let selected = st.selected else {
            return (Theme.accentSoft, Theme.textPrimary, Theme.accent.opacity(0.4)) // unanswered
        }
        if choice == quiz.blanks[bi].answer {
            return (Theme.success, Theme.onAccent, .clear)
        }
        if choice == selected {
            return (Theme.danger, Theme.onAccent, .clear)
        }
        return (Theme.surface.opacity(0.5), Theme.textFaint, .clear)
    }
}
