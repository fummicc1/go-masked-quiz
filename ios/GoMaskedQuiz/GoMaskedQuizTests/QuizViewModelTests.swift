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
                 blocks: [.text("use "), .mask(blankIndex: 0), .text(" and "), .mask(blankIndex: 1)],
                 blanks: [
                    Blank(answer: "foo", choices: ["foo", "bar", "baz", "qux"]),
                    Blank(answer: "go", choices: ["go", "rust", "c", "zig"]),
                 ]),
        ])
    }

    func testCorrectAnswer() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)
        XCTAssertEqual(vm.state(quiz: p.quizzes[0], blankIndex: 0).isCorrect, true)
        XCTAssertEqual(vm.currentScore?.correctCount, 1)
    }

    func testWrongAnswer() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("bar", quiz: p.quizzes[0], blankIndex: 0)
        XCTAssertEqual(vm.state(quiz: p.quizzes[0], blankIndex: 0).isCorrect, false)
        XCTAssertEqual(vm.currentScore?.correctCount, 0)
    }

    func testDoubleAnswerIgnored() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)
        await vm.selectAnswer("bar", quiz: p.quizzes[0], blankIndex: 0)
        XCTAssertEqual(vm.state(quiz: p.quizzes[0], blankIndex: 0).selected, "foo")
    }

    func testIndependentBlanks() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)
        await vm.selectAnswer("rust", quiz: p.quizzes[0], blankIndex: 1)
        XCTAssertEqual(vm.state(quiz: p.quizzes[0], blankIndex: 0).isCorrect, true)
        XCTAssertEqual(vm.state(quiz: p.quizzes[0], blankIndex: 1).isCorrect, false)
        XCTAssertEqual(vm.currentScore?.correctCount, 1)
    }

    func testPersistAndRestore() async {
        let store = makeStore()
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: store)
        await vm.configure()
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)

        let restored = QuizViewModel(proposal: makeProposal(), store: store)
        await restored.configure()
        XCTAssertEqual(restored.state(quiz: p.quizzes[0], blankIndex: 0).isCorrect, true)
        XCTAssertEqual(restored.state(quiz: p.quizzes[0], blankIndex: 0).selected, "foo")
    }

    func testReset() async {
        let store = makeStore()
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: store)
        await vm.configure()
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)
        await vm.resetQuiz()
        XCTAssertNil(vm.currentScore)
        XCTAssertTrue(vm.correct.isEmpty)
    }

    func testProgressCountsBlanks() async {
        let p = makeProposal()
        let vm = QuizViewModel(proposal: p, store: makeStore())
        await vm.configure()
        XCTAssertEqual(vm.progress.totalCount, 2)
        XCTAssertEqual(vm.progress.status, .notStarted)
        await vm.selectAnswer("foo", quiz: p.quizzes[0], blankIndex: 0)
        XCTAssertEqual(vm.progress.answeredCount, 1)
        XCTAssertEqual(vm.progress.status, .inProgress)
    }
}
