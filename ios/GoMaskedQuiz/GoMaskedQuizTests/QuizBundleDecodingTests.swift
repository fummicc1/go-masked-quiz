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
        XCTAssertEqual(b.version, 1)
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

    func testDecodesLLMKind() throws {
        let json = """
        {
          "id": "issue-1-q01", "kind": "llm", "index": 0,
          "blocks": [
            {"type": "text", "value": "An "},
            {"type": "mask", "blank_index": 0},
            {"type": "text", "value": " is passed a yield callback."}
          ],
          "blanks": [
            {"answer": "iterator", "choices": ["iterator", "generator", "function", "callback"]}
          ]
        }
        """
        let quiz = try JSONDecoder.quiz.decode(Quiz.self, from: Data(json.utf8))
        XCTAssertEqual(quiz.kind, .llm, "\"llm\" must not silently fall back to .prose")
    }

    /// The generated summary and issue metadata must survive decoding — without
    /// them the app silently drops the LLM-written overview it ships.
    func testDecodesSummaryAndIssueMetadata() throws {
        let json = """
        {
          "id": "issue-73787",
          "title": "simd: add architecture-specific intrinsics",
          "url": "https://github.com/golang/go/issues/73787",
          "summary": "Introduces SIMD intrinsics under GOEXPERIMENT=simd.",
          "status": "accepted",
          "source_kind": "github-issues",
          "issue_number": 73787,
          "quizzes": []
        }
        """
        let p = try JSONDecoder.quiz.decode(Proposal.self, from: Data(json.utf8))
        XCTAssertEqual(p.summary, "Introduces SIMD intrinsics under GOEXPERIMENT=simd.")
        XCTAssertEqual(p.status, "accepted")
        XCTAssertEqual(p.sourceKind, "github-issues")
        XCTAssertEqual(p.issueNumber, 73787)
    }

    /// A proposal without the optional fields still decodes (design docs carry
    /// no issue number, and most proposals have no summary yet).
    func testDecodesProposalWithoutOptionalMetadata() throws {
        let json = """
        {"id": "design-61405-range-over-func", "title": "range over func",
         "url": "https://example.invalid", "quizzes": []}
        """
        let p = try JSONDecoder.quiz.decode(Proposal.self, from: Data(json.utf8))
        XCTAssertNil(p.summary)
        XCTAssertNil(p.issueNumber)
    }

    func testDisplayNumberUsesIssueAndDesignNumbers() throws {
        func proposal(id: String, issueNumber: Int?) throws -> Proposal {
            let issue = issueNumber.map { "\"issue_number\": \($0)," } ?? ""
            let json = """
            {"id": "\(id)", "title": "t", "url": "u", \(issue) "quizzes": []}
            """
            return try JSONDecoder.quiz.decode(Proposal.self, from: Data(json.utf8))
        }
        // issue-sourced proposals used to fall back to "GO" because the id was
        // only stripped of a "design-" prefix.
        XCTAssertEqual(try proposal(id: "issue-73787", issueNumber: 73787).displayNumber, "73787")
        XCTAssertEqual(try proposal(id: "issue-73787", issueNumber: nil).displayNumber, "73787")
        XCTAssertEqual(try proposal(id: "design-61405-range-over-func", issueNumber: nil).displayNumber, "61405")
        XCTAssertEqual(try proposal(id: "design-securitypolicy", issueNumber: nil).displayNumber, "GO")
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
