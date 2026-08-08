import SwiftUI

/// One proposal's quizzes, each answerable inline, with a progress header and
/// reset action, on the dark editorial surface.
struct ProposalQuizView: View {
    @StateObject private var viewModel: QuizViewModel

    init(proposal: Proposal, store: ScoreStore) {
        _viewModel = StateObject(wrappedValue: QuizViewModel(proposal: proposal, store: store))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                // Read the gist before answering: proposals that have a generated
                // summary lead with it.
                if let summary = viewModel.proposal.summary, !summary.isEmpty {
                    SummaryCard(text: summary)
                }

                QuizProgressView(progress: viewModel.progress)
                    .padding(16)
                    .background(Theme.surface)
                    .overlay(RoundedRectangle(cornerRadius: 12).stroke(Theme.border, lineWidth: 1))
                    .clipShape(RoundedRectangle(cornerRadius: 12))

                ForEach(viewModel.allQuiz) { quiz in
                    QuizCardView(quiz: quiz, viewModel: viewModel)
                }
            }
            .padding(16)
        }
        .background(Theme.bg)
        .navigationTitle(title)
        .navigationBarTitleDisplayMode(.inline)
        .toolbarBackground(Theme.surface, for: .navigationBar)
        .toolbarBackground(.visible, for: .navigationBar)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("Reset") { viewModel.isShowingResetAlert = true }
                    .font(Theme.mono(13))
                    .disabled(viewModel.correct.isEmpty)
            }
        }
        .alert("Reset this proposal's score?", isPresented: $viewModel.isShowingResetAlert) {
            Button("Reset", role: .destructive) { Task { await viewModel.resetQuiz() } }
            Button("Cancel", role: .cancel) {}
        }
        .task { await viewModel.configure() }
    }

    private var title: String {
        let t = viewModel.proposal.title
        if let r = t.range(of: "Proposal:") {
            return String(t[r.upperBound...]).trimmingCharacters(in: .whitespaces)
        }
        return t
    }
}

/// The proposal's generated overview. Labelled as machine-written so readers can
/// weigh it against the proposal text the quizzes are drawn from.
private struct SummaryCard: View {
    let text: String

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("SUMMARY · AI GENERATED")
                .font(Theme.mono(10, .semibold))
                .foregroundStyle(Theme.textFaint)
            Text(text)
                .font(Theme.body(14))
                .foregroundStyle(Theme.textSecondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.surface)
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(Theme.border, lineWidth: 1))
        .clipShape(RoundedRectangle(cornerRadius: 12))
    }
}
