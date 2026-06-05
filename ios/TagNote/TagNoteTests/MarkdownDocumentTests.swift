import XCTest
@testable import TagNote

final class MarkdownDocumentTests: XCTestCase {
    func testParsesSeedStyleNoteIntoDocumentBlocks() {
        let markdown = """
        # Welcome to TagNote!

        TagNote organizes your notes with **tags** instead of folders.

        ## Here's what makes it different:

        - **Tag freely** — every note can have multiple tags
        - **Filter by tags** — click tags in the sidebar
        - **Search everything** — full-text search works alongside tag filters

        ### Quick start

        1. Click **New note**
        2. Write in Markdown
        3. Add tags

        > Keep the notes you need.

        ---

        ```
        tsn export
        ```
        """

        let document = MarkdownDocument.parse(markdown)

        XCTAssertEqual(document.blocks, [
            .heading(level: 1, text: "Welcome to TagNote!"),
            .paragraph("TagNote organizes your notes with **tags** instead of folders."),
            .heading(level: 2, text: "Here's what makes it different:"),
            .unorderedList([
                MarkdownListItem(blocks: [.paragraph("**Tag freely** — every note can have multiple tags")]),
                MarkdownListItem(blocks: [.paragraph("**Filter by tags** — click tags in the sidebar")]),
                MarkdownListItem(blocks: [.paragraph("**Search everything** — full-text search works alongside tag filters")])
            ]),
            .heading(level: 3, text: "Quick start"),
            .orderedList(start: 1, items: [
                MarkdownListItem(blocks: [.paragraph("Click **New note**")]),
                MarkdownListItem(blocks: [.paragraph("Write in Markdown")]),
                MarkdownListItem(blocks: [.paragraph("Add tags")])
            ]),
            .quote([.paragraph("Keep the notes you need.")]),
            .horizontalRule,
            .code("tsn export")
        ])
    }

    func testPreservesSingleLineBreaksInsideParagraphs() {
        let document = MarkdownDocument.parse("""
        First line
        second line
        third line
        """)

        XCTAssertEqual(document.blocks, [
            .paragraph("First line\nsecond line\nthird line")
        ])
    }

    func testNormalizesWindowsLineEndingsAndIgnoresExtraBlankLines() {
        let document = MarkdownDocument.parse("# Title\r\n\r\n\r\nBody\r\n\r\n- one\r\n- two")

        XCTAssertEqual(document.blocks, [
            .heading(level: 1, text: "Title"),
            .paragraph("Body"),
            .unorderedList([
                MarkdownListItem(blocks: [.paragraph("one")]),
                MarkdownListItem(blocks: [.paragraph("two")])
            ])
        ])
    }

    func testParsesStandaloneImage() {
        let document = MarkdownDocument.parse("![](/uploads/01kssytzsm1akjhhk0cma7hvzg.png)")

        XCTAssertEqual(document.blocks, [
            .image(source: "/uploads/01kssytzsm1akjhhk0cma7hvzg.png", alt: "")
        ])
    }

    func testParsesImageAltText() {
        let document = MarkdownDocument.parse("![A screenshot](/uploads/x.png)")

        XCTAssertEqual(document.blocks, [
            .image(source: "/uploads/x.png", alt: "A screenshot")
        ])
    }

    func testParsesMultipleImagesInOneParagraph() {
        // Regression: two uploaded images back to back must each become an image
        // block, not degrade to literal "!/uploads/..." text.
        let document = MarkdownDocument.parse("![](/uploads/a.png)![](/uploads/b.png)")

        XCTAssertEqual(document.blocks, [
            .image(source: "/uploads/a.png", alt: ""),
            .image(source: "/uploads/b.png", alt: "")
        ])
    }

    func testSplitsTextAndImageInSameParagraph() {
        let document = MarkdownDocument.parse("See ![shot](/uploads/x.png) here")

        XCTAssertEqual(document.blocks, [
            .paragraph("See"),
            .image(source: "/uploads/x.png", alt: "shot"),
            .paragraph("here")
        ])
    }

    func testParsesImageInsideListItem() {
        let document = MarkdownDocument.parse("- ![](/uploads/x.png)")

        XCTAssertEqual(document.blocks, [
            .unorderedList([
                MarkdownListItem(blocks: [.image(source: "/uploads/x.png", alt: "")])
            ])
        ])
    }
}
