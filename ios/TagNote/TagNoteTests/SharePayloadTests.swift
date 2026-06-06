import UniformTypeIdentifiers
import XCTest
@testable import TagNote

final class SharePayloadTests: XCTestCase {
    func testExtractsSafariPageResult() async {
        let results: [String: Any] = [
            "title": "Apple unveils the M5 chip",
            "url": "https://apple.com/newsroom/m5",
            "selection": "highlighted text",
            "articleText": "The full article body."
        ]
        let provider = NSItemProvider(
            item: [NSExtensionJavaScriptPreprocessingResultsKey: results] as NSDictionary,
            typeIdentifier: UTType.propertyList.identifier
        )

        let payload = await SharePayload.extract(from: [item(with: provider)])

        XCTAssertEqual(payload.kind, .webPage)
        XCTAssertEqual(payload.url, URL(string: "https://apple.com/newsroom/m5"))
        XCTAssertEqual(payload.pageTitle, "Apple unveils the M5 chip")
        XCTAssertEqual(payload.selection, "highlighted text")
        XCTAssertTrue(payload.hasArticleText)
    }

    func testExtractsBareWebURL() async {
        let provider = NSItemProvider(
            item: URL(string: "https://example.com/post")! as NSURL,
            typeIdentifier: UTType.url.identifier
        )

        let payload = await SharePayload.extract(from: [item(with: provider)])

        XCTAssertEqual(payload.kind, .webPage)
        XCTAssertEqual(payload.url, URL(string: "https://example.com/post"))
        XCTAssertFalse(payload.hasArticleText)
    }

    func testExtractsPlainText() async {
        let provider = NSItemProvider(
            item: "a shared thought" as NSString,
            typeIdentifier: UTType.plainText.identifier
        )

        let payload = await SharePayload.extract(from: [item(with: provider)])

        XCTAssertEqual(payload.kind, .text)
        XCTAssertEqual(payload.text, "a shared thought")
    }

    func testIgnoresNonWebURL() async {
        let provider = NSItemProvider(
            item: URL(string: "file:///tmp/a.txt")! as NSURL,
            typeIdentifier: UTType.url.identifier
        )

        let payload = await SharePayload.extract(from: [item(with: provider)])

        XCTAssertNil(payload.url)
        XCTAssertEqual(payload.kind, .empty)
    }

    private func item(with provider: NSItemProvider) -> NSExtensionItem {
        let item = NSExtensionItem()
        item.attachments = [provider]
        return item
    }
}
