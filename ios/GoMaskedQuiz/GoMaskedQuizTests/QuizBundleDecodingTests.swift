import XCTest
@testable import GoMaskedQuiz

final class QuizBundleDecodingTests: XCTestCase {
    private func loadGolden() throws -> QuizBundle {
        let url = try XCTUnwrap(
            Bundle(for: Self.self).url(forResource: "quizzes-seed42", withExtension: "json"),
            "golden fixture not bundled"
        )
        let data = try Data(contentsOf: url)
        return try JSONDecoder.quiz.decode(QuizBundle.self, from: data)
    }

    func testDecodesGoldenBundle() throws {
        let b = try loadGolden()
        XCTAssertEqual(b.version, 2)
        XCTAssertFalse(b.proposals.isEmpty)
        XCTAssertTrue(b.proposals.allSatisfy { !$0.quizzes.isEmpty })
    }

    func testEveryQuizHasExactlyOneMask() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            let masks = q.blocks.filter { $0 == .mask }.count
            XCTAssertEqual(masks, 1, "quiz \(q.id) should have exactly one mask")
        }
    }

    func testChoicesContainAnswer() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            XCTAssertEqual(q.choices.count, 4, "quiz \(q.id)")
            XCTAssertTrue(q.choices.contains(q.answer), "quiz \(q.id) choices missing answer")
        }
    }

    func testBlockKindsMatchQuizKind() throws {
        for q in try loadGolden().proposals.flatMap(\.quizzes) {
            for blk in q.blocks {
                switch (q.kind, blk) {
                case (.prose, .codeBlock):
                    XCTFail("prose quiz \(q.id) must not contain code_block")
                case (.code, .text), (.code, .inlineCode):
                    XCTFail("code quiz \(q.id) must not contain text/inline_code")
                default:
                    break
                }
            }
        }
    }
}
