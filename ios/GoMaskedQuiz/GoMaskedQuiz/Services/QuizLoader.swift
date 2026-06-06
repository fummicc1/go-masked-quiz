import Foundation

/// Loads the quiz bundle with a three-tier fallback: remote CDN → local cache →
/// app-bundled copy. `load()` never throws — the bundled JSON is the demo's
/// guaranteed floor, so the app always starts with content.
actor QuizLoader {
    enum Source: Sendable, Equatable {
        case remote, cache, bundle
    }

    private let url: URL?
    private let accepted: ClosedRange<Int>
    private let bundle: Bundle
    private let resourceName: String
    private let cacheURL: URL
    private let session: URLSession

    init(
        url: URL? = Configuration.quizDataURL,
        accepted: ClosedRange<Int> = Configuration.acceptedVersions,
        bundle: Bundle = .main,
        resourceName: String = "quizzes"
    ) {
        self.url = url
        self.accepted = accepted
        self.bundle = bundle
        self.resourceName = resourceName
        let caches = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
        self.cacheURL = caches.appendingPathComponent("quizzes.json")
        let cfg = URLSessionConfiguration.ephemeral
        cfg.timeoutIntervalForRequest = 10
        cfg.timeoutIntervalForResource = 30
        self.session = URLSession(configuration: cfg)
    }

    func load() async -> (bundle: QuizBundle, source: Source) {
        if let url, let fetched = await fetchRemote(url) {
            try? fetched.data.write(to: cacheURL, options: .atomic)
            return (fetched.bundle, .remote)
        }
        if let data = try? Data(contentsOf: cacheURL), let decoded = decode(data) {
            return (decoded, .cache)
        }
        return (loadBundled(), .bundle)
    }

    private func fetchRemote(_ url: URL) async -> (bundle: QuizBundle, data: Data)? {
        guard let (data, resp) = try? await session.data(from: url),
              let http = resp as? HTTPURLResponse, http.statusCode == 200,
              let decoded = decode(data) else { return nil }
        return (decoded, data)
    }

    private func decode(_ data: Data) -> QuizBundle? {
        guard let b = try? JSONDecoder.quiz.decode(QuizBundle.self, from: data),
              accepted.contains(b.version) else { return nil }
        return b
    }

    private func loadBundled() -> QuizBundle {
        guard let u = bundle.url(forResource: resourceName, withExtension: "json"),
              let data = try? Data(contentsOf: u),
              let b = try? JSONDecoder.quiz.decode(QuizBundle.self, from: data)
        else { return .empty }
        return b
    }
}
