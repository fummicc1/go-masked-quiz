import SwiftUI

/// Progress summary for a proposal's blanks: a cyan rail with percent and
/// answered/accuracy counts.
struct QuizProgressView: View {
    let progress: ProposalProgress

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 8) {
                ProgressRail(rate: progress.progressRate, done: progress.status == .completed)
                Text("\(Int(progress.progressPercentage))%")
                    .font(Theme.mono(12, .medium))
                    .foregroundStyle(Theme.textSecondary)
                    .monospacedDigit()
            }
            HStack(spacing: 10) {
                Text("\(progress.answeredCount)/\(progress.totalCount) blanks")
                    .font(Theme.mono(11))
                    .foregroundStyle(Theme.textFaint)
                if progress.answeredCount > 0 {
                    Text("· \(Int(progress.accuracyPercentage))% correct")
                        .font(Theme.mono(11))
                        .foregroundStyle(Theme.textFaint)
                }
            }
        }
    }
}
