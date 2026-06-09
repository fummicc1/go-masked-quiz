import SwiftUI

/// Top-level list of proposals with per-proposal progress badges (over blanks).
struct ProposalListScreen: View {
    let proposals: [Proposal]
    let store: ScoreStore
    var sourceLabel: String? = nil

    @State private var scores: [String: ProposalScore] = [:]

    var body: some View {
        NavigationStack {
            List(proposals) { proposal in
                NavigationLink(value: proposal) {
                    VStack(alignment: .leading, spacing: 6) {
                        Text(proposal.title).font(.headline)
                        Text("\(blankCount(proposal))問")
                            .font(.caption2).foregroundStyle(.secondary)
                        QuizProgressView(progress: progress(for: proposal))
                    }
                    .padding(.vertical, 4)
                }
            }
            .navigationTitle("Go Proposals")
            .navigationDestination(for: Proposal.self) { proposal in
                ProposalQuizView(proposal: proposal, store: store)
            }
            .safeAreaInset(edge: .bottom) {
                if let sourceLabel {
                    Text(sourceLabel)
                        .font(.caption2)
                        .foregroundStyle(.secondary)
                        .padding(6)
                }
            }
            .onAppear { Task { scores = await store.getAllScores() } }
        }
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
