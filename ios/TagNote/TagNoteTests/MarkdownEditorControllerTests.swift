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
        controller.toggleWrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "**hello** world")
    }

    func testBoldTogglesOffWhenAlreadyBold() {
        // Caret-selection of "hello" with the ** markers just outside it.
        let (controller, textView) = make("**hello** world", selection: NSRange(location: 2, length: 5))
        controller.toggleWrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "hello world")
    }

    func testBoldTogglesOffWhenMarkersAreInsideSelection() {
        let (controller, textView) = make("**hello** world", selection: NSRange(location: 0, length: 9))
        controller.toggleWrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "hello world")
    }

    func testWrapWithNoSelectionInsertsMarkersWithCaretBetween() {
        let (controller, textView) = make("ab", selection: NSRange(location: 1, length: 0))
        controller.toggleWrap(prefix: "**", suffix: "**")
        XCTAssertEqual(textView.text, "a****b")
        XCTAssertEqual(textView.selectedRange, NSRange(location: 3, length: 0))
    }

    func testToggleLinePrefixAddsThenRemoves() {
        let (controller, textView) = make("title", selection: NSRange(location: 2, length: 0))
        controller.toggleLinePrefix("## ")
        XCTAssertEqual(textView.text, "## title")
        controller.toggleLinePrefix("## ")
        XCTAssertEqual(textView.text, "title")
    }

    func testToggleOrderedListAddsThenRemoves() {
        let (controller, textView) = make("item", selection: NSRange(location: 0, length: 0))
        controller.toggleOrderedList()
        XCTAssertEqual(textView.text, "1. item")
        controller.toggleOrderedList()
        XCTAssertEqual(textView.text, "item")
    }

    func testInsertLinkWrapsSelectionAndSelectsURL() {
        let (controller, textView) = make("docs", selection: NSRange(location: 0, length: 4))
        controller.insertLink()
        XCTAssertEqual(textView.text, "[docs](url)")
        // The "url" placeholder is selected so it can be replaced by typing.
        XCTAssertEqual(textView.text(in: textView.selectedTextRange!), "url")
    }

    func testInsertLinkWithNoSelectionSelectsText() {
        let (controller, textView) = make("", selection: NSRange(location: 0, length: 0))
        controller.insertLink()
        XCTAssertEqual(textView.text, "[text](url)")
        XCTAssertEqual(textView.text(in: textView.selectedTextRange!), "text")
    }

    func testInsertPutsTextAtTheCaret() {
        let (controller, textView) = make("ac", selection: NSRange(location: 1, length: 0))
        controller.insert("b")
        XCTAssertEqual(textView.text, "abc")
    }

    // MARK: - List continuation

    func testListContinuationContinuesBullets() {
        XCTAssertEqual(ListContinuation.evaluate(line: "- first", lineStart: 0), .insert("\n- "))
    }

    func testListContinuationIncrementsOrdered() {
        XCTAssertEqual(ListContinuation.evaluate(line: "3. third", lineStart: 0), .insert("\n4. "))
    }

    func testListContinuationContinuesQuotes() {
        XCTAssertEqual(ListContinuation.evaluate(line: "> quoted", lineStart: 0), .insert("\n> "))
    }

    func testListContinuationExitsOnEmptyItem() {
        XCTAssertEqual(ListContinuation.evaluate(line: "- ", lineStart: 4),
                       .exit(markerRange: NSRange(location: 4, length: 2)))
    }

    func testListContinuationPreservesIndent() {
        XCTAssertEqual(ListContinuation.evaluate(line: "  - nested", lineStart: 0), .insert("\n  - "))
    }

    func testListContinuationIgnoresPlainText() {
        XCTAssertEqual(ListContinuation.evaluate(line: "just text", lineStart: 0), .none)
    }
}
