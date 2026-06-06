import Foundation

/// Top-level v2 quizzes.json document produced by quizgen and consumed here.
/// Mirrors `internal/quiz/model.go` (the schema's source of truth).
struct QuizBundle: Decodable, Sendable {
    let version: Int
    let generatedAt: Date
    let sourceRepo: String
    let sourceFork: String
    let sourceCommit: String?
    let sourceLicense: String
    let sourceLicenseURL: String
    let proposals: [Proposal]

    enum CodingKeys: String, CodingKey {
        case version, proposals
        case generatedAt = "generated_at"
        case sourceRepo = "source_repo"
        case sourceFork = "source_fork"
        case sourceCommit = "source_commit"
        case sourceLicense = "source_license"
        case sourceLicenseURL = "source_license_url"
    }

    /// Last-resort value so the loader never returns nil even if the bundled
    /// JSON is missing or unreadable.
    static let empty = QuizBundle(
        version: 2,
        generatedAt: Date(timeIntervalSince1970: 0),
        sourceRepo: "", sourceFork: "", sourceCommit: nil,
        sourceLicense: "", sourceLicenseURL: "", proposals: []
    )
}

/// One design/*.md proposal and the quizzes generated from it.
struct Proposal: Decodable, Identifiable, Sendable, Hashable {
    let id: String
    let title: String
    let url: String
    let quizzes: [Quiz]
}

/// A single fill-in-the-blank question. Exactly one of `blocks` is `.mask`,
/// and `choices` always contains `answer`.
struct Quiz: Decodable, Identifiable, Sendable, Equatable, Hashable {
    let id: String
    let kind: Kind
    let index: Int
    let blocks: [Block]
    let answer: String
    let choices: [String]

    enum Kind: String, Decodable, Sendable, Hashable {
        case prose
        case code

        /// Unknown kinds fall back to `.prose` so a future schema addition does
        /// not break decoding of the whole bundle.
        init(from decoder: Decoder) throws {
            let raw = try decoder.singleValueContainer().decode(String.self)
            self = Kind(rawValue: raw) ?? .prose
        }
    }
}
