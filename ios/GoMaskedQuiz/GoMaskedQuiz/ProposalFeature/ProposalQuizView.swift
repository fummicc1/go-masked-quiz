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
