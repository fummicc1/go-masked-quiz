import SwiftUI

@main
struct GoMaskedQuizApp: App {
    private let store = ScoreStore()
    @State private var phase: Phase = .loading

    enum Phase {
        case loading
        case loaded([Proposal], QuizLoader.Source)
    }

    var body: some Scene {
        WindowGroup {
            switch phase {
            case .loading:
                ProgressView("読み込み中…")
                    .task { await load() }
            case .loaded(let proposals, let source):
                // Demo aid: launch straight into the first proposal's quiz when
                // DEMO_QUIZ is set (used for screenshots); otherwise the list.
                if ProcessInfo.processInfo.environment["DEMO_QUIZ"] != nil,
                   let first = proposals.first {
                    NavigationStack {
                        ProposalQuizView(proposal: first, store: store)
                    }
                } else {
                    ProposalListScreen(
                        proposals: proposals,
                        store: store,
                        sourceLabel: sourceLabel(source)
                    )
                }
            }
        }
    }

    private func load() async {
        let (bundle, source) = await QuizLoader().load()
        phase = .loaded(bundle.proposals, source)
    }

    private func sourceLabel(_ source: QuizLoader.Source) -> String {
        switch source {
        case .remote: return "データ: CDN"
        case .cache:  return "データ: キャッシュ"
        case .bundle: return "データ: アプリ同梱"
        }
    }
}
