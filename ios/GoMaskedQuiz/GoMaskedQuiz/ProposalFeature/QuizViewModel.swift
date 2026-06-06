import SwiftUI

/// Drives one proposal's quizzes: tracks selected answers, scores them, and
/// persists/restores progress. (Ported from se-masked-quiz, LLM removed; quizzes
/// come from the loaded Proposal, and every quiz is answerable inline rather than
/// one-at-a-time in a sheet.)
@MainActor
final class QuizViewModel: ObservableObject {
    @Published var allQuiz: [Quiz] = []
    @Published var selectedAnswer: [Int: String] = [:]
    @Published var isCorrect: [Int: Bool] = [:]
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
                selectedAnswer[r.index] = r.userAnswer
                isCorrect[r.index] = r.isCorrect
            }
        }
        isConfigured = true
    }

    /// Records the answer for `quiz`, scores it, and persists. Async so callers
    /// (and tests) can await the save — unlike se-masked-quiz's fire-and-forget.
    func selectAnswer(_ answer: String, for quiz: Quiz) async {
        guard isCorrect[quiz.index] == nil else { return } // ignore re-answers
        selectedAnswer[quiz.index] = answer
        isCorrect[quiz.index] = (answer == quiz.answer)
        await updateScore()
    }

    func isAnswered(_ quiz: Quiz) -> Bool { isCorrect[quiz.index] != nil }

    func resetQuiz() async {
        await store.resetScore(for: proposal.id)
        selectedAnswer = [:]
        isCorrect = [:]
        currentScore = nil
    }

    var progress: ProposalProgress {
        ProposalProgress(
            proposalId: proposal.id,
            answeredCount: isCorrect.count,
            totalCount: allQuiz.count,
            correctCount: isCorrect.values.filter { $0 }.count
        )
    }

    private func updateScore() async {
        let byIndex = Dictionary(uniqueKeysWithValues: allQuiz.map { ($0.index, $0) })
        let results = isCorrect.compactMap { index, correct -> QuestionResult? in
            guard let q = byIndex[index], let userAnswer = selectedAnswer[index] else { return nil }
            return QuestionResult(index: index, isCorrect: correct, answer: q.answer, userAnswer: userAnswer)
        }.sorted { $0.index < $1.index }

        let score = ProposalScore(proposalId: proposal.id, questionResults: results)
        currentScore = score
        await store.saveScore(score)
    }
}
