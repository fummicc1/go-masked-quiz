import XCTest
@testable import GoMaskedQuiz

@MainActor
final class QuizViewModelTests: XCTestCase {
    private func makeStore() -> ScoreStore {
        ScoreStore(userDefaults: UserDefaults(suiteName: "test-\(UUID().uuidString)")!)
    }

    private func makeProposal() -> Proposal {
        Proposal(id: "design-x", title: "X", url: "https://example.com", quizzes: [
            Quiz(id: "q1", kind: .prose, index: 0,
                 blocks: [.text("use "), .mask, .text(" here")],
                 answer: "foo", choices: ["foo", "bar", "baz", "qux"]),
            Quiz(id: "q2", kind: .code, index: 1,
                 blocks: [.codeBlock("package "), .mask],
                 answer: "main", choices: ["main", "fmt", "go", "func"]),
        ])
    }

    func testCorrectAnswerMarksCorrect() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("foo", for: p.quizzes[0])
        XCTAssertEqual(vm.isCorrect[0], true)
        XCTAssertEqual(vm.currentScore?.correctCount, 1)
    }

    func testWrongAnswerMarksIncorrect() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("bar", for: p.quizzes[0])
        XCTAssertEqual(vm.isCorrect[0], false)
        XCTAssertEqual(vm.currentScore?.correctCount, 0)
    }

    func testDoubleAnswerIgnored() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("foo", for: p.quizzes[0])
        await vm.selectAnswer("bar", for: p.quizzes[0]) // ignored
        XCTAssertEqual(vm.selectedAnswer[0], "foo")
        XCTAssertEqual(vm.isCorrect[0], true)
    }

    func testScorePersistsAndRestores() async {
        let store = makeStore()
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: store)
        await vm.configure()
        await vm.selectAnswer("foo", for: p.quizzes[0])

        let restored = QuizViewModel(proposal: makeProposal(), store: store)
        await restored.configure()
        XCTAssertEqual(restored.isCorrect[0], true)
        XCTAssertEqual(restored.selectedAnswer[0], "foo")
    }

    func testResetClearsScore() async {
        let store = makeStore()
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: store)
        await vm.configure()
        await vm.selectAnswer("foo", for: p.quizzes[0])
        await vm.resetQuiz()
        XCTAssertNil(vm.currentScore)
        XCTAssertTrue(vm.isCorrect.isEmpty)

        let reloaded = QuizViewModel(proposal: makeProposal(), store: store)
        await reloaded.configure()
        XCTAssertTrue(reloaded.isCorrect.isEmpty)
    }

    func testProgress() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        XCTAssertEqual(vm.progress.status, .notStarted)
        await vm.selectAnswer("foo", for: p.quizzes[0])
        XCTAssertEqual(vm.progress.answeredCount, 1)
        XCTAssertEqual(vm.progress.totalCount, 2)
        XCTAssertEqual(vm.progress.status, .inProgress)
    }
}
