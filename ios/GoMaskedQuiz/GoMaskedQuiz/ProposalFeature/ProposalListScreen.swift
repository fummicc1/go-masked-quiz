import SwiftUI

/// Top-level list of proposals as cards, each with a proposal-number badge and a
/// blank-based progress rail.
struct ProposalListScreen: View {
    let proposals: [Proposal]
    let store: ScoreStore
    var sourceLabel: String? = nil

    @State private var scores: [String: ProposalScore] = [:]

    var body: some View {
        NavigationStack {
            ScrollView {
                LazyVStack(spacing: 12) {
                    ForEach(proposals) { proposal in
                        NavigationLink(value: proposal) {
                            ProposalRow(proposal: proposal, progress: progress(for: proposal))
                        }
                        .buttonStyle(.plain)
                    }
                }
                .padding(16)
            }
            .background(Theme.bg)
            .navigationTitle("Go Proposals")
            .navigationDestination(for: Proposal.self) { proposal in
                ProposalQuizView(proposal: proposal, store: store)
            }
            .toolbarBackground(Theme.surface, for: .navigationBar)
            .toolbarBackground(.visible, for: .navigationBar)
            .safeAreaInset(edge: .bottom) {
                if let sourceLabel {
                    Text(sourceLabel)
                        .font(Theme.mono(10))
                        .foregroundStyle(Theme.textFaint)
                        .padding(.vertical, 6)
                        .frame(maxWidth: .infinity)
                        .background(Theme.bg)
                }
            }
            .onAppear { Task { scores = await store.getAllScores() } }
        }
        .tint(Theme.accent)
    }

    private func blankCount(_ p: Proposal) -> Int {
        p.quizzes.reduce(0) { $0 + $1.blanks.count }
    }

    private func progress(for proposal: Proposal) -> ProposalProgress {
        let score = scores[proposal.id]
        return ProposalProgress(
            proposalId: proposal.id,
            answeredCount: score?.totalCount ?? 0,
            totalCount: blankCount(proposal),
            correctCount: score?.correctCount ?? 0
        )
    }
}

private struct ProposalRow: View {
    let proposal: Proposal
    let progress: ProposalProgress

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Text(number)
                    .font(Theme.mono(12, .semibold))
                    .foregroundStyle(Theme.accent)
                    .padding(.horizontal, 7).padding(.vertical, 3)
                    .background(Theme.accentSoft)
                    .clipShape(Capsule())
                Spacer()
                Text("\(progress.totalCount) blanks")
                    .font(Theme.mono(11))
                    .foregroundStyle(Theme.textFaint)
                if progress.status == .completed {
                    Image(systemName: "checkmark.seal.fill").foregroundStyle(Theme.success).font(.caption)
                }
            }
            Text(title)
                .font(Theme.display(17))
                .foregroundStyle(Theme.textPrimary)
                .lineLimit(2)
                .frame(maxWidth: .infinity, alignment: .leading)
            ProgressRail(rate: progress.progressRate, done: progress.status == .completed)
        }
        .padding(16)
        .background(Theme.surface)
        .overlay(RoundedRectangle(cornerRadius: 14).stroke(Theme.border, lineWidth: 1))
        .clipShape(RoundedRectangle(cornerRadius: 14))
    }

    // "design-61405-range-over-func" -> "61405"
    private var number: String {
        let trimmed = proposal.id.replacingOccurrences(of: "design-", with: "")
        let digits = trimmed.prefix { $0.isNumber }
        return digits.isEmpty ? "GO" : String(digits)
    }

    // strip a leading "Proposal:" for a cleaner card title
    private var title: String {
        let t = proposal.title
        if let r = t.range(of: "Proposal:") {
            return String(t[r.upperBound...]).trimmingCharacters(in: .whitespaces)
        }
        return t
    }
}
