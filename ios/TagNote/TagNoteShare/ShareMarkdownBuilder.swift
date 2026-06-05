import Foundation

/// Whether a shared web page is captured as a compact link or as the page's
/// full readable text.
enum ShareContentMode {
    case link
    case fullPage
}

/// Pure helpers that assemble the markdown body for a shared item. Kept free of
/// UIKit/networking so it is unit-testable. The output matches how TagNote cards
/// render: the first `#` heading becomes the card title; `[text](url)` links are
/// tappable; `>` is a blockquote; images are `![](path)`.
enum ShareMarkdownBuilder {
    /// Markdown for a shared web page. `title`/`articleText`/`selection` may be
    /// empty (e.g. a bare URL shared from a non-Safari app).
    static func webPage(
        title: String?,
        url: URL,
        selection: String?,
        articleText: String?,
        mode: ShareContentMode
    ) -> String {
        let heading = trimmedNonEmpty(title) ?? displayHost(url)
        var blocks: [String] = ["# \(heading)", "[\(linkLabel(url))](\(url.absoluteString))"]

        switch mode {
        case .link:
            if let selection = trimmedNonEmpty(selection) {
                blocks.append(blockquote(selection))
            }
        case .fullPage:
            if let article = trimmedNonEmpty(articleText) {
                blocks.append(normalizeParagraphs(article))
            } else if let selection = trimmedNonEmpty(selection) {
                blocks.append(blockquote(selection))
            }
        }
        return blocks.joined(separator: "\n\n")
    }

    /// Markdown for shared plain text — the text becomes the body verbatim.
    static func plainText(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// Appends an uploaded image reference below any existing content.
    static func appendingImage(path: String, to content: String) -> String {
        let image = "![](\(path))"
        let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? image : "\(trimmed)\n\n\(image)"
    }

    // MARK: - Helpers

    /// A clean, human-readable link label: host + path, without the scheme, a
    /// leading `www.`, or a trailing slash (e.g. `apple.com/newsroom/m5`).
    static func linkLabel(_ url: URL) -> String {
        var host = url.host ?? ""
        if host.hasPrefix("www.") { host.removeFirst(4) }
        var label = host + url.path
        while label.count > 1 && label.hasSuffix("/") { label.removeLast() }
        return label.isEmpty ? url.absoluteString : label
    }

    private static func displayHost(_ url: URL) -> String {
        var host = url.host ?? url.absoluteString
        if host.hasPrefix("www.") { host.removeFirst(4) }
        return host
    }

    private static func blockquote(_ text: String) -> String {
        text
            .split(separator: "\n", omittingEmptySubsequences: false)
            .map { $0.isEmpty ? ">" : "> \($0)" }
            .joined(separator: "\n")
    }

    /// Collapses runs of 3+ newlines into a single blank line and trims, so a
    /// page's `innerText` reads as tidy paragraphs.
    private static func normalizeParagraphs(_ text: String) -> String {
        let lines = text.split(separator: "\n", omittingEmptySubsequences: false)
            .map { $0.trimmingCharacters(in: .whitespaces) }
        var result: [String] = []
        var blankRun = 0
        for line in lines {
            if line.isEmpty {
                blankRun += 1
                if blankRun <= 1 { result.append("") }
            } else {
                blankRun = 0
                result.append(line)
            }
        }
        return result.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private static func trimmedNonEmpty(_ value: String?) -> String? {
        guard let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines),
              !trimmed.isEmpty else { return nil }
        return trimmed
    }
}
