import SwiftUI

/// Identifies one blank within a proposal: which quiz, which blank.
struct BlankKey: Hashable {
    let quizIndex: Int
    let blankIndex: Int
}

/// Drives one proposal's quizzes: tracks selected answers per blank, scores
/// them, and persists/restores progress. Each quiz is one unit (paragraph or
/// code block) with one or more blanks answered inline.
@MainActor
final class QuizViewModel: ObservableObject {
    @Published var allQuiz: [Quiz] = []
    @Published var selected: [BlankKey: String] = [:]
    @Published var correct: [BlankKey: Bool] = [:]
    @Published var currentScore: ProposalScore?
    @Published var isShowingResetAlert = false
    @Published var isConfigured = false

    let proposal: Proposal
    private let store: ScoreStore

    init(proposal: Proposal, store: ScoreStore) {
        self.proposal = proposal
        self.store = store
    }

    func configure() async {
        allQuiz = proposal.quizzes
        if let existing = await store.getScore(for: proposal.id) {
            currentScore = existing
            for r in existing.questionResults {
                let k = BlankKey(quizIndex: r.quizIndex, blankIndex: r.blankIndex)
                selected[k] = r.userAnswer
                correct[k] = r.isCorrect
            }
        }
        isConfigured = true
    }

    /// Records the answer for one blank, scores it, and persists.
    func selectAnswer(_ choice: String, quiz: Quiz, blankIndex: Int) async {
        let k = BlankKey(quizIndex: quiz.index, blankIndex: blankIndex)
        guard correct[k] == nil, blankIndex < quiz.blanks.count else { return }
        selected[k] = choice
        correct[k] = (choice == quiz.blanks[blankIndex].answer)
        await updateScore()
    }

    func state(quiz: Quiz, blankIndex: Int) -> (selected: String?, isCorrect: Bool?) {
        let k = BlankKey(quizIndex: quiz.index, blankIndex: blankIndex)
        return (selected[k], correct[k])
    }

    func resetQuiz() async {
        await store.resetScore(for: proposal.id)
        selected = [:]
        correct = [:]
        currentScore = nil
    }

    var totalBlanks: Int { allQuiz.reduce(0) { $0 + $1.blanks.count } }

    var progress: ProposalProgress {
        ProposalProgress(
            proposalId: proposal.id,
            answeredCount: correct.count,
            totalCount: totalBlanks,
            correctCount: correct.values.filter { $0 }.count
        )
    }

    private func updateScore() async {
        var results: [QuestionResult] = []
        for q in allQuiz {
            for bi in q.blanks.indices {
                let k = BlankKey(quizIndex: q.index, blankIndex: bi)
                if let ua = selected[k], let c = correct[k] {
                    results.append(QuestionResult(
                        quizIndex: q.index, blankIndex: bi,
                        isCorrect: c, answer: q.blanks[bi].answer, userAnswer: ua
                    ))
                }
            }
        }
        results.sort { ($0.quizIndex, $0.blankIndex) < ($1.quizIndex, $1.blankIndex) }
        let score = ProposalScore(proposalId: proposal.id, questionResults: results)
        currentScore = score
        await store.saveScore(score)
    }
}
