import Foundation

enum Configuration {
    /// CDN location of the static quizzes.json. Optional: when nil or
    /// unreachable, the loader falls back to cache then the bundled copy, so the
    /// demo runs fully offline. The CDN is not deployed yet, so this is nil and
    /// the app uses the bundled copy. Set this once the CDN is live, e.g.:
    ///   URL(string: "https://go-masked-quiz-data.pages.dev/v3/quizzes.json")
    static let quizDataURL: URL? = nil

    /// Bundle versions this build understands (current schema is v3).
    static let acceptedVersions: ClosedRange<Int> = 3...3
}
