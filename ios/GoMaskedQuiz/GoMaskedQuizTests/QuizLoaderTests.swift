import XCTest
@testable import GoMaskedQuiz

final class QuizLoaderTests: XCTestCase {
    func testBundleFallbackLoadsContent() async {
        // url:nil forces the remote tier to be skipped; with no cache the loader
        // must reach the bundled fixture.
        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "quizzes-seed42"
        )
        let (bundle, source) = await loader.load()
        XCTAssertEqual(source, .bundle)
        XCTAssertFalse(bundle.proposals.isEmpty)
    }

    func testMissingBundleResourceReturnsEmpty() async {
        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "does-not-exist"
        )
        let (bundle, source) = await loader.load()
        XCTAssertEqual(source, .bundle)
        XCTAssertTrue(bundle.proposals.isEmpty)
    }
}
