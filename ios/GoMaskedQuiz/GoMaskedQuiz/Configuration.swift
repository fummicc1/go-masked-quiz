import Foundation

enum Configuration {
    /// CDN location of the static quizzes.json, served by jsDelivr straight from
    /// the repo's cdn/ (no extra hosting to run). Optional: when nil or
    /// unreachable, the loader falls back to cache then the bundled copy, so the
    /// demo runs fully offline.
    ///
    /// jsDelivr caches a branch ref for up to ~12h, which is fine for a
    /// daily-refreshed bundle. Swap to a tag/commit ref if you ever need
    /// immediate propagation.
    static let quizDataURL: URL? = URL(
        string: "https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v1/quizzes.json"
    )

    /// Bundle versions this build understands (current schema is v1).
    static let acceptedVersions: ClosedRange<Int> = 1...1
}
