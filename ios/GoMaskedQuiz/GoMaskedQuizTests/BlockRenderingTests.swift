import XCTest
import SwiftUI
@testable import GoMaskedQuiz

@MainActor
final class BlockRenderingTests: XCTestCase {
    func testProseComposesTextAndMask() {
        let view = ProseBlocksView(
            blocks: [.text("use "), .mask, .text(" here")],
            mask: "______", tint: .yellow
        )
        XCTAssertEqual(String(view.attributed.characters), "use ______ here")
    }

    func testProseKeepsInlineCodeAndIgnoresCodeBlock() {
        let view = ProseBlocksView(
            blocks: [.text("the "), .inlineCode("range"), .text(" "), .mask, .codeBlock("ignored")],
            mask: "X", tint: .yellow
        )
        let s = String(view.attributed.characters)
        XCTAssertEqual(s, "the range X")
        XCTAssertFalse(s.contains("ignored"))
    }

    func testCodeComposesAroundMask() {
        let view = CodeBlocksView(
            blocks: [.codeBlock("package "), .mask, .codeBlock("\n")],
            mask: "____", tint: .green
        )
        XCTAssertEqual(String(view.attributed.characters), "package ____\n")
    }

    func testCodeIgnoresProseBlocks() {
        let view = CodeBlocksView(
            blocks: [.text("nope"), .codeBlock("ok"), .mask],
            mask: "_", tint: .green
        )
        let s = String(view.attributed.characters)
        XCTAssertEqual(s, "ok_")
    }
}
