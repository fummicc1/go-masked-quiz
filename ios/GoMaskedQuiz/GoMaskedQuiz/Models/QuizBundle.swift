import Foundation

/// Top-level v1 quizzes.json document. Mirrors `internal/quiz/model.go`.
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
        version: 1,
        generatedAt: Date(timeIntervalSince1970: 0),
        sourceRepo: "", sourceFork: "", sourceCommit: nil,
        sourceLicense: "", sourceLicenseURL: "", proposals: []
    )
}

struct Proposal: Decodable, Identifiable, Sendable, Hashable {
    let id: String
    let title: String
    let url: String
    /// LLM-written overview of the proposal. Present only where one has been
    /// generated, so the UI must treat it as optional enrichment.
    let summary: String?
    /// Where the proposal stands: "accepted" or "active" (issue-sourced only).
    let status: String?
    /// Which upstream it came from: "design-docs" or "github-issues".
    let sourceKind: String?
    /// golang/go issue number, for proposals taken from the issue tracker.
    let issueNumber: Int?
    let quizzes: [Quiz]

    enum CodingKeys: String, CodingKey {
        case id, title, url, summary, status, quizzes
        case sourceKind = "source_kind"
        case issueNumber = "issue_number"
    }

    /// The enrichment fields default to nil so callers only supply what a
    /// proposal actually has.
    init(
        id: String,
        title: String,
        url: String,
        summary: String? = nil,
        status: String? = nil,
        sourceKind: String? = nil,
        issueNumber: Int? = nil,
        quizzes: [Quiz]
    ) {
        self.id = id
        self.title = title
        self.url = url
        self.summary = summary
        self.status = status
        self.sourceKind = sourceKind
        self.issueNumber = issueNumber
        self.quizzes = quizzes
    }
}

extension Proposal {
    /// Short label for the card badge: the issue number, else the number embedded
    /// in the id ("design-61405-range-over-func" → "61405", "issue-73787" →
    /// "73787"). Skipping the prefix keeps issue-sourced proposals from all
    /// collapsing onto the placeholder.
    var displayNumber: String {
        if let issueNumber { return String(issueNumber) }
        let digits = id.drop { !$0.isNumber }.prefix { $0.isNumber }
        return digits.isEmpty ? "GO" : String(digits)
    }
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
        case llm

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
