import SwiftUI

/// One quiz (a paragraph or code block): its rendered body with numbered blanks,
/// plus a choice row per blank, with correct/incorrect feedback. Answering a
/// blank fills every occurrence of its token at once.
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

            QuizBlocksView(quiz: quiz, displays: displays)

            ForEach(quiz.blanks.indices, id: \.self) { bi in
                blankRow(bi)
            }
        }
        .padding()
        .background(.background.secondary)
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }

    private var displays: [Int: BlankDisplay] {
        var d: [Int: BlankDisplay] = [:]
        for bi in quiz.blanks.indices {
            let st = viewModel.state(quiz: quiz, blankIndex: bi)
            if let isCorrect = st.isCorrect {
                d[bi] = BlankDisplay(
                    text: quiz.blanks[bi].answer,
                    color: isCorrect ? .green.opacity(0.35) : .red.opacity(0.35)
                )
            } else {
                d[bi] = BlankDisplay(text: blankMarker(bi), color: .yellow.opacity(0.35))
            }
        }
        return d
    }

    @ViewBuilder
    private func blankRow(_ bi: Int) -> some View {
        let st = viewModel.state(quiz: quiz, blankIndex: bi)
        let answered = st.isCorrect != nil
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 6) {
                Text(blankMarker(bi)).font(.headline)
                if let isCorrect = st.isCorrect {
                    Text(isCorrect ? "正解" : "不正解（\(quiz.blanks[bi].answer)）")
                        .font(.caption).foregroundColor(isCorrect ? .green : .red)
                }
            }
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 8) {
                    ForEach(quiz.blanks[bi].choices, id: \.self) { choice in
                        Button {
                            Task { await viewModel.selectAnswer(choice, quiz: quiz, blankIndex: bi) }
                        } label: {
                            Text(choice)
                                .font(.system(.callout, design: .monospaced))
                                .padding(.horizontal, 12).padding(.vertical, 8)
                                .background(background(bi, choice, st))
                                .foregroundColor(.white)
                                .clipShape(Capsule())
                        }
                        .disabled(answered)
                    }
                }
            }
        }
    }

    private func background(_ bi: Int, _ choice: String, _ st: (selected: String?, isCorrect: Bool?)) -> Color {
        guard let selected = st.selected else { return .blue }
        if choice == quiz.blanks[bi].answer { return .green }
        if choice == selected { return .red }
        return .gray
    }
}
