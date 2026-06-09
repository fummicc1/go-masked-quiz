import Foundation

/// Top-level v3 quizzes.json document. Mirrors `internal/quiz/model.go`.
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

    static let empty = QuizBundle(
        version: 3,
        generatedAt: Date(timeIntervalSince1970: 0),
        sourceRepo: "", sourceFork: "", sourceCommit: nil,
        sourceLicense: "", sourceLicenseURL: "", proposals: []
    )
}

struct Proposal: Decodable, Identifiable, Sendable, Hashable {
    let id: String
    let title: String
    let url: String
    let quizzes: [Quiz]
}

/// One quiz built from a single unit (a prose paragraph or a code block). Each
/// mask block in `blocks` points into `blanks`; there is at least one blank.
struct Quiz: Decodable, Identifiable, Sendable, Equatable, Hashable {
    let id: String
    let kind: Kind
    let index: Int
    let blocks: [Block]
    let blanks: [Blank]

    enum Kind: String, Decodable, Sendable, Hashable {
        case prose
        case code

        init(from decoder: Decoder) throws {
            let raw = try decoder.singleValueContainer().decode(String.self)
            self = Kind(rawValue: raw) ?? .prose
        }
    }
}

/// One fill-in target: its answer and the multiple choices (which include the
/// answer). A blank may be referenced by several mask blocks (repeats).
struct Blank: Decodable, Sendable, Equatable, Hashable {
    let answer: String
    let choices: [String]
}
