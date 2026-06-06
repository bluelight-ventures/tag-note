import UIKit
import XCTest
@testable import TagNote

@MainActor
final class MarkdownEditorControllerTests: XCTestCase {
    private func make(_ text: String, selection: NSRange) -> (MarkdownEditorController, UITextView) {
        let textView = UITextView()
        textView.text = text
        textView.selectedRange = selection
        let controller = MarkdownEditorController()
        controller.textView = textView
        return (controller, textView)
    }

    func testBoldWrapsTheSelectedText() {
        let (controller, textView) = make("hello world", selection: NSRange(location: 0, length: 5))
        controller.wrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "**hello** world")
    }

    func testItalicWrapsTheSelectedText() {
        let (controller, textView) = make("hello world", selection: NSRange(location: 6, length: 5))
        controller.wrap(prefix: "*", suffix: "*")
        XCTAssertEqual(textView.text, "hello *world*")
    }

    func testWrapWithNoSelectionInsertsMarkersWithCaretBetween() {
        let (controller, textView) = make("ab", selection: NSRange(location: 1, length: 0))
        controller.wrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "a****b")
        XCTAssertEqual(textView.selectedRange, NSRange(location: 3, length: 0))
    }

    func testPrefixLineInsertsAtTheStartOfTheCaretLine() {
        let (controller, textView) = make("line one\nline two", selection: NSRange(location: 12, length: 0))
        controller.prefixLine("## ")
        XCTAssertEqual(textView.text, "line one\n## line two")
    }

    func testInsertPutsTextAtTheCaret() {
        let (controller, textView) = make("ac", selection: NSRange(location: 1, length: 0))
        controller.insert("b")
        XCTAssertEqual(textView.text, "abc")
    }
}
