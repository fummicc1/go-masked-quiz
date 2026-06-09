import XCTest
import SwiftUI
@testable import GoMaskedQuiz

@MainActor
final class BlockRenderingTests: XCTestCase {
    func testProseComposesWithBlankMarkers() {
        let view = ProseBlocksView(
            blocks: [.text("use "), .mask(blankIndex: 0), .text(" and "), .mask(blankIndex: 1)],
            displays: [
                0: BlankDisplay(text: "①", color: .yellow),
                1: BlankDisplay(text: "②", color: .yellow),
            ]
        )
        let s = String(view.attributed.characters)
        XCTAssertTrue(s.contains("use "))
        XCTAssertTrue(s.contains("①"))
        XCTAssertTrue(s.contains("②"))
    }

    func testProseMaskShowsAnswerWhenProvided() {
        let view = ProseBlocksView(
            blocks: [.mask(blankIndex: 0)],
            displays: [0: BlankDisplay(text: "range", color: .green)]
        )
        XCTAssertTrue(String(view.attributed.characters).contains("range"))
    }

    func testProseIgnoresCodeBlock() {
        let view = ProseBlocksView(
            blocks: [.text("a"), .codeBlock("nope"), .mask(blankIndex: 0)],
            displays: [:]
        )
        XCTAssertFalse(String(view.attributed.characters).contains("nope"))
    }

    func testCodeComposesAroundMask() {
        let view = CodeBlocksView(
            blocks: [.codeBlock("package "), .mask(blankIndex: 0), .codeBlock("\n")],
            displays: [0: BlankDisplay(text: "main", color: .green)]
        )
        let s = String(view.attributed.characters)
        XCTAssertTrue(s.contains("package "))
        XCTAssertTrue(s.contains("main"))
    }

    func testBlankMarker() {
        XCTAssertEqual(blankMarker(0), "①")
        XCTAssertEqual(blankMarker(1), "②")
        XCTAssertEqual(blankMarker(2), "③")
    }
}
