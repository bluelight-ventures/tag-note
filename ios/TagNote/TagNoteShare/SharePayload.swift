import Foundation
import ImageIO
import UniformTypeIdentifiers
import UIKit

/// The content extracted from a share invocation, normalized for the compose UI.
struct SharePayload {
    enum Kind {
        case webPage   // a URL, optionally with page title/selection/article text
        case text      // plain shared text
        case image     // an image to upload
        case empty     // nothing usable
    }

    var kind: Kind = .empty
    var url: URL?
    var pageTitle: String?
    var selection: String?
    var articleText: String?
    var text: String?
    var imageData: Data?
    var imageFileName: String?
    var imageMimeType: String?

    /// True when the page's readable text was captured (Safari only) — gates the
    /// Link / Full-page toggle in the compose UI.
    var hasArticleText: Bool {
        !(articleText ?? "").trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// The initial markdown shown in the compose editor for the given web mode.
    func markdown(mode: ShareContentMode) -> String {
        switch kind {
        case .webPage:
            guard let url else { return "" }
            return ShareMarkdownBuilder.webPage(
                title: pageTitle, url: url, selection: selection,
                articleText: articleText, mode: mode
            )
        case .text:
            return ShareMarkdownBuilder.plainText(text ?? "")
        case .image, .empty:
            return ""
        }
    }
}

extension SharePayload {
    /// Extracts a payload from the extension's input items. Precedence:
    /// Safari page result (web) → image → web URL → plain text.
    static func extract(from items: [NSExtensionItem]) async -> SharePayload {
        let providers = items.flatMap { $0.attachments ?? [] }
        var payload = SharePayload()

        // 1. Safari JavaScript preprocessing result (title / url / selection / text).
        for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.propertyList.identifier) {
            guard let dict = await loadPropertyList(provider),
                  let results = dict[NSExtensionJavaScriptPreprocessingResultsKey] as? [String: Any]
            else { continue }
            payload.kind = .webPage
            payload.pageTitle = results["title"] as? String
            payload.selection = results["selection"] as? String
            payload.articleText = results["articleText"] as? String
            if let urlString = results["url"] as? String, let url = URL(string: urlString) {
                payload.url = url
            }
            break
        }

        // 2. Image (checked before a bare URL so file-URL images aren't mistaken for web pages).
        if payload.kind == .empty {
            for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.image.identifier) {
                guard let image = await loadImage(provider) else { continue }
                payload.kind = .image
                payload.imageData = image.data
                payload.imageFileName = image.fileName
                payload.imageMimeType = image.mimeType
                break
            }
        }

        // 3. Web URL (http/https) without a captured page.
        if payload.url == nil && payload.kind != .image {
            for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
                guard let url = await loadWebURL(provider) else { continue }
                payload.url = url
                payload.kind = .webPage
                break
            }
        }

        // 4. Plain text.
        if payload.kind == .empty {
            for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
                guard let text = await loadText(provider) else { continue }
                payload.text = text
                payload.kind = .text
                break
            }
        }

        // 5. Fallback page title for non-Safari web shares (Chrome, Mail, etc.):
        // those don't run GetPageInfo.js, so the title arrives on the extension
        // item's content text / title instead. Only used when the page has no
        // title yet and the candidate isn't just the URL.
        if payload.kind == .webPage, isEmpty(payload.pageTitle), let title = itemProvidedTitle(items) {
            if title != payload.url?.absoluteString {
                payload.pageTitle = title
            }
        }

        return payload
    }

    private static func itemProvidedTitle(_ items: [NSExtensionItem]) -> String? {
        for item in items {
            if let title = item.attributedTitle?.string, !isEmpty(title) { return title }
            if let content = item.attributedContentText?.string, !isEmpty(content) { return content }
        }
        return nil
    }

    private static func isEmpty(_ value: String?) -> Bool {
        (value ?? "").trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    // MARK: - Loaders

    private static func loadPropertyList(_ provider: NSItemProvider) async -> [String: Any]? {
        await withCheckedContinuation { continuation in
            provider.loadItem(forTypeIdentifier: UTType.propertyList.identifier, options: nil) { item, _ in
                continuation.resume(returning: item as? [String: Any])
            }
        }
    }

    private static func loadWebURL(_ provider: NSItemProvider) async -> URL? {
        await withCheckedContinuation { continuation in
            provider.loadItem(forTypeIdentifier: UTType.url.identifier, options: nil) { item, _ in
                guard let url = item as? URL,
                      let scheme = url.scheme?.lowercased(),
                      scheme == "http" || scheme == "https" else {
                    continuation.resume(returning: nil)
                    return
                }
                continuation.resume(returning: url)
            }
        }
    }

    private static func loadText(_ provider: NSItemProvider) async -> String? {
        await withCheckedContinuation { continuation in
            provider.loadItem(forTypeIdentifier: UTType.plainText.identifier, options: nil) { item, _ in
                continuation.resume(returning: item as? String)
            }
        }
    }

    private static func loadImage(_ provider: NSItemProvider) async -> (data: Data, fileName: String, mimeType: String)? {
        let raw: Data? = await withCheckedContinuation { continuation in
            provider.loadDataRepresentation(forTypeIdentifier: UTType.image.identifier) { data, _ in
                continuation.resume(returning: data)
            }
        }
        guard let raw else { return nil }
        // Downscale + re-encode as JPEG to stay within the extension memory budget.
        let jpeg = downscaledJPEG(from: raw) ?? raw
        return (jpeg, "shared-image.jpg", "image/jpeg")
    }

    private static func downscaledJPEG(from data: Data, maxPixel: CGFloat = 2048) -> Data? {
        guard let source = CGImageSourceCreateWithData(data as CFData, nil) else { return nil }
        let options: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxPixel
        ]
        guard let thumbnail = CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary) else {
            return nil
        }
        return UIImage(cgImage: thumbnail).jpegData(compressionQuality: 0.85)
    }
}
