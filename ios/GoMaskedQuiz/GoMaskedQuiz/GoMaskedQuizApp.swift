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
            ZStack {
                Theme.bg.ignoresSafeArea()
                switch phase {
                case .loading:
                    ProgressView()
                        .tint(Theme.accent)
                        .task { await load() }
                case .loaded(let proposals, let source):
                    if ProcessInfo.processInfo.environment["DEMO_QUIZ"] != nil,
                       let first = proposals.first {
                        NavigationStack { ProposalQuizView(proposal: first, store: store) }
                            .tint(Theme.accent)
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
    }

    private func load() async {
        let (bundle, source) = await QuizLoader().load()
        phase = .loaded(bundle.proposals, source)
    }

    private func sourceLabel(_ source: QuizLoader.Source) -> String {
        switch source {
        case .remote: return "source · cdn"
        case .cache:  return "source · cache"
        case .bundle: return "source · bundled"
        }
    }
}
