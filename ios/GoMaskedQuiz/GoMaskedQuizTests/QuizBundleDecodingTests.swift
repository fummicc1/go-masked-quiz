import XCTest
@testable import GoMaskedQuiz

final class QuizBundleDecodingTests: XCTestCase {
    private func loadGolden() throws -> QuizBundle {
        let url = try XCTUnwrap(
            Bundle(for: Self.self).url(forResource: "quizzes-seed42", withExtension: "json"),
            "golden fixture not bundled"
        )
        return try JSONDecoder.quiz.decode(QuizBundle.self, from: Data(contentsOf: url))
    }

    func testDecodesGoldenBundle() throws {
        let b = try loadGolden()
        XCTAssertEqual(b.version, 3)
        XCTAssertFalse(b.proposals.isEmpty)
        XCTAssertTrue(b.proposals.allSatisfy { !$0.quizzes.isEmpty })
    }

    func testEveryQuizHasBlanksAndValidMasks() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            XCTAssertFalse(q.blanks.isEmpty, "\(q.id) has no blanks")
            var referenced = Array(repeating: false, count: q.blanks.count)
            var maskCount = 0
            for block in q.blocks {
                if case .mask(let bi) = block {
                    maskCount += 1
                    if bi >= 0 && bi < q.blanks.count {
                        referenced[bi] = true
                    } else {
                        XCTFail("\(q.id): blank_index \(bi) out of range")
                    }
                }
            }
            XCTAssertGreaterThanOrEqual(maskCount, 1, "\(q.id) has no mask")
            XCTAssertTrue(referenced.allSatisfy { $0 }, "\(q.id) has an unreferenced blank")
        }
    }

    func testChoicesContainAnswer() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            for bl in q.blanks {
                XCTAssertEqual(bl.choices.count, 4, "\(q.id)")
                XCTAssertTrue(bl.choices.contains(bl.answer), "\(q.id): \(bl.answer)")
            }
        }
    }

    func testBlockKindsMatchQuizKind() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            for block in q.blocks {
                switch (q.kind, block) {
                case (.prose, .codeBlock):
                    XCTFail("prose quiz \(q.id) has code_block")
                case (.code, .text), (.code, .inlineCode):
                    XCTFail("code quiz \(q.id) has text/inline_code")
                default:
                    break
                }
            }
        }
    }
}
