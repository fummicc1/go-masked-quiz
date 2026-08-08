import XCTest
@testable import GoMaskedQuiz

final class QuizLoaderTests: XCTestCase {
    /// A cache path of its own per test, so a bundle the app cached earlier can't
    /// satisfy the cache tier and mask the tier under test.
    private func unusedCacheURL() -> URL {
        FileManager.default.temporaryDirectory
            .appendingPathComponent("quiz-loader-test-\(UUID().uuidString).json")
    }

    func testBundleFallbackLoadsContent() async {
        // url:nil skips the remote tier; with no cache the loader must reach the
        // bundled fixture.
        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "quizzes-seed42",
            cacheURL: unusedCacheURL()
        )
        let (bundle, source) = await loader.load()
        XCTAssertEqual(source, .bundle)
        XCTAssertFalse(bundle.proposals.isEmpty)
    }

    func testMissingBundleResourceReturnsEmpty() async {
        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "does-not-exist",
            cacheURL: unusedCacheURL()
        )
        let (bundle, source) = await loader.load()
        XCTAssertEqual(source, .bundle)
        XCTAssertTrue(bundle.proposals.isEmpty)
    }

    /// A cached bundle is preferred over the bundled copy when the remote tier is
    /// unavailable.
    func testCacheTierIsUsedWhenPresent() async throws {
        let cacheURL = unusedCacheURL()
        let fixture = try XCTUnwrap(
            Bundle(for: Self.self).url(forResource: "quizzes-seed42", withExtension: "json")
        )
        try Data(contentsOf: fixture).write(to: cacheURL)
        defer { try? FileManager.default.removeItem(at: cacheURL) }

        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "does-not-exist",
            cacheURL: cacheURL
        )
        let (bundle, source) = await loader.load()
        XCTAssertEqual(source, .cache)
        XCTAssertFalse(bundle.proposals.isEmpty)
    }

    /// A bundle whose version is outside the accepted range is rejected, so the
    /// loader falls through instead of showing content it can't render.
    func testCacheWithUnacceptedVersionIsRejected() async throws {
        let cacheURL = unusedCacheURL()
        let json = """
        {"version": 99, "generated_at": "2026-05-18T00:00:00Z", "source_repo": "",
         "source_fork": "", "source_license": "", "source_license_url": "", "proposals": []}
        """
        try Data(json.utf8).write(to: cacheURL)
        defer { try? FileManager.default.removeItem(at: cacheURL) }

        let loader = QuizLoader(
            url: nil,
            bundle: Bundle(for: Self.self),
            resourceName: "quizzes-seed42",
            cacheURL: cacheURL
        )
        let (_, source) = await loader.load()
        XCTAssertEqual(source, .bundle, "an unaccepted version must not be served from cache")
    }
}
