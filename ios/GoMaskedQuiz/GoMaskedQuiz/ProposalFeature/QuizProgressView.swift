import SwiftUI

/// Reusable progress bar for a proposal's quizzes. (Ported from se-masked-quiz.)
struct QuizProgressView: View {
    let progress: ProposalProgress

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 8) {
                ProgressView(value: progress.progressRate).tint(progressColor)
                Text("\(Int(progress.progressPercentage))%")
                    .font(.caption).foregroundStyle(.secondary).monospacedDigit()
            }
            HStack(spacing: 12) {
                Text("\(progress.answeredCount)/\(progress.totalCount)問")
                    .font(.caption2).foregroundStyle(.secondary)
                if progress.answeredCount > 0 {
                    Text("正解率: \(Int(progress.accuracyPercentage))%")
                        .font(.caption2).foregroundStyle(.secondary)
                }
            }
        }
    }

    private var progressColor: Color {
        switch progress.status {
        case .notStarted: return .gray
        case .inProgress: return .blue
        case .completed: return .green
        }
    }
}
