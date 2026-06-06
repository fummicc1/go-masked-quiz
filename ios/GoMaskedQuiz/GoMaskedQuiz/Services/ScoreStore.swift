import Foundation

/// Persists per-proposal quiz scores in UserDefaults. (Ported from
/// se-masked-quiz's QuizRepository — score portion only; no network/LLM.)
actor ScoreStore {
    private let userDefaults: UserDefaults
    private static let scoreKey = "proposal_scores"

    init(userDefaults: UserDefaults = .standard) {
        self.userDefaults = userDefaults
    }

    func saveScore(_ score: ProposalScore) {
        var scores = allScores()
        scores[score.proposalId] = score
        if let encoded = try? JSONEncoder().encode(scores) {
            userDefaults.set(encoded, forKey: Self.scoreKey)
        }
    }

    func getAllScores() -> [String: ProposalScore] { allScores() }

    func getScore(for proposalId: String) -> ProposalScore? { allScores()[proposalId] }

    func resetScore(for proposalId: String) {
        var scores = allScores()
        scores.removeValue(forKey: proposalId)
        if let encoded = try? JSONEncoder().encode(scores) {
            userDefaults.set(encoded, forKey: Self.scoreKey)
        }
    }

    private func allScores() -> [String: ProposalScore] {
        guard let data = userDefaults.data(forKey: Self.scoreKey),
              let scores = try? JSONDecoder().decode([String: ProposalScore].self, from: data)
        else { return [:] }
        return scores
    }
}

extension UserDefaults: @retroactive @unchecked Sendable {}
