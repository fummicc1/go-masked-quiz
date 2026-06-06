import SwiftUI

/// One proposal's quizzes, each answerable inline, with a progress header and
/// reset action.
struct ProposalQuizView: View {
    @StateObject private var viewModel: QuizViewModel

    init(proposal: Proposal, store: ScoreStore) {
        _viewModel = StateObject(wrappedValue: QuizViewModel(proposal: proposal, store: store))
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                QuizProgressView(progress: viewModel.progress)
                ForEach(viewModel.allQuiz) { quiz in
                    QuizCardView(quiz: quiz, viewModel: viewModel)
                }
            }
            .padding()
        }
        .navigationTitle(viewModel.proposal.title)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button("リセット") { viewModel.isShowingResetAlert = true }
                    .disabled(viewModel.isCorrect.isEmpty)
            }
        }
        .alert("スコアをリセットしますか？", isPresented: $viewModel.isShowingResetAlert) {
            Button("リセット", role: .destructive) { Task { await viewModel.resetQuiz() } }
            Button("キャンセル", role: .cancel) {}
        }
        .task { await viewModel.configure() }
    }
}
