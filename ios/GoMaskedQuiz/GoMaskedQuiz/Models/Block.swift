import Foundation

/// One fragment of a quiz body. `mask` carries the index of the blank it fills
/// (`blank_index`). Unknown types decode to `.unknown` rather than throwing.
enum Block: Decodable, Sendable, Equatable, Hashable {
    case text(String)
    case inlineCode(String)
    case codeBlock(String)
    case mask(blankIndex: Int)
    case unknown

    private enum CodingKeys: String, CodingKey {
        case type, value
        case blankIndex = "blank_index"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        switch try c.decode(String.self, forKey: .type) {
        case "text":        self = .text(try c.decode(String.self, forKey: .value))
        case "inline_code": self = .inlineCode(try c.decode(String.self, forKey: .value))
        case "code_block":  self = .codeBlock(try c.decode(String.self, forKey: .value))
        case "mask":        self = .mask(blankIndex: try c.decode(Int.self, forKey: .blankIndex))
        default:            self = .unknown
        }
    }
}

extension JSONDecoder {
    /// Decoder configured for quizzes.json (ISO-8601 dates; explicit CodingKeys).
    static var quiz: JSONDecoder {
        let d = JSONDecoder()
        d.dateDecodingStrategy = .iso8601
        return d
    }
}
