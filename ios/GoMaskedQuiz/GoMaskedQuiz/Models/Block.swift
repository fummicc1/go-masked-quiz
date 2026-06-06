import Foundation

/// One fragment of a quiz body. The v2 schema ships these pre-parsed so the
/// client renders by iterating the slice — no Markdown re-parsing.
///
/// `mask` carries no value (the JSON omits the key). Unknown types decode to
/// `.unknown` rather than throwing, so the bundle survives schema additions.
enum Block: Decodable, Sendable, Equatable, Hashable {
    case text(String)
    case inlineCode(String)
    case codeBlock(String)
    case mask
    case unknown

    private enum CodingKeys: String, CodingKey {
        case type, value
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        switch try c.decode(String.self, forKey: .type) {
        case "text":        self = .text(try c.decode(String.self, forKey: .value))
        case "inline_code": self = .inlineCode(try c.decode(String.self, forKey: .value))
        case "code_block":  self = .codeBlock(try c.decode(String.self, forKey: .value))
        case "mask":        self = .mask
        default:            self = .unknown
        }
    }
}

extension JSONDecoder {
    /// Decoder configured for quizzes.json (ISO-8601 dates; explicit CodingKeys
    /// are used instead of key conversion to avoid `URL`-acronym key clashes).
    static var quiz: JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }
}
