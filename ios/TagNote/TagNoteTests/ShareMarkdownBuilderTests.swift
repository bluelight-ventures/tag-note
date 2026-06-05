import XCTest
@testable import TagNote

final class ShareMarkdownBuilderTests: XCTestCase {
    private let url = URL(string: "https://www.apple.com/newsroom/m5/")!

    func testWebLinkModeUsesTitleHeadingAndCleanLink() {
        let md = ShareMarkdownBuilder.webPage(
            title: "Apple unveils the M5 chip",
            url: url,
            selection: nil,
            articleText: nil,
            mode: .link
        )
        XCTAssertEqual(md, """
        # Apple unveils the M5 chip

        [apple.com/newsroom/m5](https://www.apple.com/newsroom/m5/)
        """)
    }

    func testWebLinkModeAppendsSelectionAsBlockquote() {
        let md = ShareMarkdownBuilder.webPage(
            title: "Apple unveils the M5 chip",
            url: url,
            selection: "The M5 delivers 2x faster GPU\nand better efficiency.",
            articleText: "ignored in link mode",
            mode: .link
        )
        XCTAssertEqual(md, """
        # Apple unveils the M5 chip

        [apple.com/newsroom/m5](https://www.apple.com/newsroom/m5/)

        > The M5 delivers 2x faster GPU
        > and better efficiency.
        """)
    }

    func testWebFullPageModeUsesArticleText() {
        let md = ShareMarkdownBuilder.webPage(
            title: "Apple unveils the M5 chip",
            url: url,
            selection: "a selection",
            articleText: "Apple today announced the M5.\n\n\n\nIt is fast.",
            mode: .fullPage
        )
        XCTAssertEqual(md, """
        # Apple unveils the M5 chip

        [apple.com/newsroom/m5](https://www.apple.com/newsroom/m5/)

        Apple today announced the M5.

        It is fast.
        """)
    }

    func testFullPageFallsBackToSelectionWhenNoArticle() {
        let md = ShareMarkdownBuilder.webPage(
            title: "Title",
            url: url,
            selection: "highlighted bit",
            articleText: "   ",
            mode: .fullPage
        )
        XCTAssertTrue(md.contains("> highlighted bit"))
    }

    func testEmptyTitleFallsBackToHost() {
        let md = ShareMarkdownBuilder.webPage(
            title: "  ",
            url: url,
            selection: nil,
            articleText: nil,
            mode: .link
        )
        XCTAssertTrue(md.hasPrefix("# apple.com\n"))
    }

    func testPlainTextIsTrimmedBody() {
        XCTAssertEqual(ShareMarkdownBuilder.plainText("  hello world \n"), "hello world")
    }

    func testAppendingImageToEmptyContent() {
        XCTAssertEqual(ShareMarkdownBuilder.appendingImage(path: "/uploads/a.jpg", to: "   "),
                       "![](/uploads/a.jpg)")
    }

    func testAppendingImageBelowExistingContent() {
        XCTAssertEqual(ShareMarkdownBuilder.appendingImage(path: "/uploads/a.jpg", to: "note body"),
                       "note body\n\n![](/uploads/a.jpg)")
    }

    func testLinkLabelStripsWwwAndTrailingSlash() {
        XCTAssertEqual(ShareMarkdownBuilder.linkLabel(URL(string: "https://www.example.com/a/b/")!),
                       "example.com/a/b")
        XCTAssertEqual(ShareMarkdownBuilder.linkLabel(URL(string: "https://example.com")!),
                       "example.com")
    }
}
