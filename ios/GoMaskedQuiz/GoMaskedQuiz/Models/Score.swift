import Foundation

/// One answered blank's result. (Ported from se-masked-quiz.)
struct QuestionResult: Codable, Equatable {
    let index: Int
    let isCorrect: Bool
    let answer: String
    let userAnswer: String
}

/// A proposal's saved score, persisted in UserDefaults.
struct ProposalScore: Codable, Equatable {
    let proposalId: String
    let questionResults: [QuestionResult]
    let timestamp: Date

    var correctCount: Int { questionResults.filter(\.isCorrect).count }
    var totalCount: Int { questionResults.count }
    var percentage: Double {
        guard totalCount > 0 else { return 0 }
        return Double(correctCount) / Double(totalCount) * 100
    }

    init(proposalId: String, questionResults: [QuestionResult], timestamp: Date = Date()) {
        self.proposalId = proposalId
        self.questionResults = questionResults
        self.timestamp = timestamp
    }
}

/// Derived progress for a proposal, used for list badges.
struct ProposalProgress: Equatable {
    let proposalId: String
    let answeredCount: Int
    let totalCount: Int
    let correctCount: Int

    var progressRate: Double {
        guard totalCount > 0 else { return 0 }
        return Double(answeredCount) / Double(totalCount)
    }
    var progressPercentage: Double { progressRate * 100 }
    var accuracyPercentage: Double {
        guard answeredCount > 0 else { return 0 }
        return Double(correctCount) / Double(answeredCount) * 100
    }
    var status: ProgressStatus {
        if answeredCount == 0 { return .notStarted }
        if answeredCount >= totalCount { return .completed }
        return .inProgress
    }
}

enum ProgressStatus {
    case notStarted, inProgress, completed
}
