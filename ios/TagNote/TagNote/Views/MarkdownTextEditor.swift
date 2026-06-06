import SwiftUI
import UIKit

/// Computes how a newline should continue a markdown list / quote line, mirroring
/// the behaviour of the web editor (CodeMirror): Enter on a list item starts the
/// next item; Enter on an empty item ends the list.
enum ListContinuation: Equatable {
    case none
    case exit(markerRange: NSRange)   // empty item: remove the marker on this line
    case insert(String)               // continue with this marker on a new line

    /// `line` is the text of the caret's line (its `lineRange`); `lineStart` is
    /// that line's location in the document.
    static func evaluate(line rawLine: String, lineStart: Int) -> ListContinuation {
        let line = rawLine.hasSuffix("\n") ? String(rawLine.dropLast()) : rawLine
        let chars = Array(line)
        var i = 0
        while i < chars.count, chars[i] == " " || chars[i] == "\t" { i += 1 }
        let indent = String(chars[0..<i])
        let rest = String(chars[i...])

        func result(marker: String, content: String) -> ListContinuation {
            let isEmpty = content.trimmingCharacters(in: .whitespaces).isEmpty
            if isEmpty {
                let markerLength = (indent + marker) as NSString
                return .exit(markerRange: NSRange(location: lineStart, length: markerLength.length))
            }
            return .insert("\n" + indent + marker)
        }

        // Unordered list: -, *, +
        if let first = rest.first, "-*+".contains(first), rest.dropFirst().first == " " {
            return result(marker: "\(first) ", content: String(rest.dropFirst(2)))
        }
        // Blockquote: >
        if rest.hasPrefix("> ") {
            return result(marker: "> ", content: String(rest.dropFirst(2)))
        }
        // Ordered list: 1. 2. ...
        if let match = orderedPrefix(rest) {
            let next = match.number + 1
            let content = String(rest.dropFirst(match.length))
            if content.trimmingCharacters(in: .whitespaces).isEmpty {
                let markerLength = (indent + "\(match.number). ") as NSString
                return .exit(markerRange: NSRange(location: lineStart, length: markerLength.length))
            }
            return .insert("\n" + indent + "\(next). ")
        }
        return .none
    }

    private static func orderedPrefix(_ s: String) -> (number: Int, length: Int)? {
        var digits = ""
        for ch in s {
            if ch.isNumber { digits.append(ch) } else { break }
        }
        guard !digits.isEmpty, let number = Int(digits) else { return nil }
        let after = s.dropFirst(digits.count)
        guard after.first == ".", after.dropFirst().first == " " else { return nil }
        return (number, digits.count + 2)
    }
}

/// Commands the underlying UITextView for the formatting toolbar so buttons act
/// on the real selection / caret — things SwiftUI's `TextEditor` can't express.
final class MarkdownEditorController {
    weak var textView: UITextView?

    /// Wraps the selection with `prefix`/`suffix`, or unwraps it if it is already
    /// wrapped (so Bold toggles off). With no selection, inserts the markers and
    /// places the caret between them.
    func toggleWrap(prefix: String, suffix: String) {
        guard let textView else { return }
        let nsText = textView.text as NSString
        let sel = textView.selectedRange
        let pLen = (prefix as NSString).length
        let sLen = (suffix as NSString).length

        // Markers sit just outside the selection → unwrap.
        if sel.location >= pLen, sel.location + sel.length + sLen <= nsText.length,
           nsText.substring(with: NSRange(location: sel.location - pLen, length: pLen)) == prefix,
           nsText.substring(with: NSRange(location: sel.location + sel.length, length: sLen)) == suffix {
            let outer = NSRange(location: sel.location - pLen, length: pLen + sel.length + sLen)
            let inner = nsText.substring(with: sel)
            replace(textView, outer, with: inner)
            setSelectedRange(textView, NSRange(location: outer.location, length: (inner as NSString).length))
            notifyChanged(textView)
            return
        }
        // Markers are inside the selection (e.g. "**hi**" selected) → unwrap.
        if sel.length >= pLen + sLen {
            let selText = nsText.substring(with: sel) as NSString
            if selText.hasPrefix(prefix), selText.hasSuffix(suffix) {
                let inner = selText.substring(with: NSRange(location: pLen, length: selText.length - pLen - sLen))
                replace(textView, sel, with: inner)
                setSelectedRange(textView, NSRange(location: sel.location, length: (inner as NSString).length))
                notifyChanged(textView)
                return
            }
        }
        wrap(prefix: prefix, suffix: suffix)
    }

    /// Wraps the current selection with `prefix`/`suffix` (no toggle).
    func wrap(prefix: String, suffix: String) {
        guard let textView, let range = targetRange(in: textView) else { return }
        let selected = textView.text(in: range) ?? ""
        textView.replace(range, withText: prefix + selected + suffix)
        if selected.isEmpty, let caret = textView.selectedTextRange,
           let inner = textView.position(from: caret.start, offset: -(suffix as NSString).length) {
            textView.selectedTextRange = textView.textRange(from: inner, to: inner)
        }
        notifyChanged(textView)
    }

    /// Adds a line prefix (heading `## `, list `- `, quote `> `) at the start of
    /// the caret's line, or removes it if already present (toggle).
    func toggleLinePrefix(_ prefix: String) {
        guard let textView else { return }
        let nsText = textView.text as NSString
        let caret = min(max(textView.selectedRange.location, 0), nsText.length)
        let lineRange = nsText.lineRange(for: NSRange(location: caret, length: 0))
        let line = nsText.substring(with: lineRange) as NSString
        if line.hasPrefix(prefix) {
            replace(textView, NSRange(location: lineRange.location, length: (prefix as NSString).length), with: "")
        } else {
            replace(textView, NSRange(location: lineRange.location, length: 0), with: prefix)
        }
        notifyChanged(textView)
    }

    /// Toggles an ordered-list marker (`1. `) at the start of the caret's line.
    func toggleOrderedList() {
        guard let textView else { return }
        let nsText = textView.text as NSString
        let caret = min(max(textView.selectedRange.location, 0), nsText.length)
        let lineRange = nsText.lineRange(for: NSRange(location: caret, length: 0))
        let line = nsText.substring(with: lineRange)
        if let existing = orderedMarkerLength(line) {
            replace(textView, NSRange(location: lineRange.location, length: existing), with: "")
        } else {
            replace(textView, NSRange(location: lineRange.location, length: 0), with: "1. ")
        }
        notifyChanged(textView)
    }

    /// Inserts a markdown link. The selection becomes the link text and the caret
    /// lands on the `url` placeholder (selected) so the user can paste/type it.
    func insertLink() {
        guard let textView, let range = targetRange(in: textView) else { return }
        let selected = textView.text(in: range) ?? ""
        let label = selected.isEmpty ? "text" : selected
        let placeholder = selected.isEmpty ? "text" : "url"
        let inserted = "[\(label)](url)" as NSString
        textView.replace(range, withText: inserted as String)

        if let caretEnd = textView.selectedTextRange?.end {
            let phRange = inserted.range(of: placeholder, options: .backwards)
            let backFromEnd = inserted.length - phRange.location
            if let start = textView.position(from: caretEnd, offset: -backFromEnd),
               let end = textView.position(from: start, offset: phRange.length) {
                textView.selectedTextRange = textView.textRange(from: start, to: end)
            }
        }
        notifyChanged(textView)
    }

    /// Inserts text at the caret (used by the quick-symbols row).
    func insert(_ text: String) {
        guard let textView, let range = targetRange(in: textView) else { return }
        textView.replace(range, withText: text)
        notifyChanged(textView)
    }

    func resignFocus() {
        textView?.resignFirstResponder()
    }

    // MARK: - Internals

    private func orderedMarkerLength(_ line: String) -> Int? {
        var digits = ""
        for ch in line {
            if ch.isNumber { digits.append(ch) } else { break }
        }
        guard !digits.isEmpty else { return nil }
        let after = line.dropFirst(digits.count)
        guard after.first == ".", after.dropFirst().first == " " else { return nil }
        return digits.count + 2
    }

    private func targetRange(in textView: UITextView) -> UITextRange? {
        textView.selectedTextRange ?? textView.textRange(from: textView.endOfDocument, to: textView.endOfDocument)
    }

    private func replace(_ textView: UITextView, _ nsRange: NSRange, with text: String) {
        guard let start = textView.position(from: textView.beginningOfDocument, offset: nsRange.location),
              let end = textView.position(from: start, offset: nsRange.length),
              let range = textView.textRange(from: start, to: end) else { return }
        textView.replace(range, withText: text)
    }

    private func setSelectedRange(_ textView: UITextView, _ nsRange: NSRange) {
        guard let start = textView.position(from: textView.beginningOfDocument, offset: nsRange.location),
              let end = textView.position(from: start, offset: nsRange.length) else { return }
        textView.selectedTextRange = textView.textRange(from: start, to: end)
    }

    private func notifyChanged(_ textView: UITextView) {
        textView.delegate?.textViewDidChange?(textView)
    }
}

/// A UITextView-backed plain-text editor so the formatting toolbar can wrap the
/// real selection and insert at the caret. Plain text (markdown source),
/// monospaced, Dynamic-Type aware, with smart list/quote continuation on Enter.
struct MarkdownTextView: UIViewRepresentable {
    @Binding var text: String
    @Binding var isFocused: Bool
    let controller: MarkdownEditorController
    var textColor: UIColor
    var tintColor: UIColor

    func makeUIView(context: Context) -> UITextView {
        let textView = UITextView()
        textView.delegate = context.coordinator
        textView.backgroundColor = .clear
        textView.textColor = textColor
        textView.tintColor = tintColor
        textView.autocapitalizationType = .sentences
        textView.autocorrectionType = .default
        textView.keyboardType = .default
        textView.smartQuotesType = .no
        textView.smartDashesType = .no
        textView.textContainerInset = UIEdgeInsets(top: 12, left: 12, bottom: 12, right: 12)
        let base = UIFont.monospacedSystemFont(ofSize: 17, weight: .regular)
        textView.font = UIFontMetrics(forTextStyle: .body).scaledFont(for: base)
        textView.adjustsFontForContentSizeCategory = true
        textView.text = text
        textView.accessibilityIdentifier = "note-content-editor"
        controller.textView = textView
        return textView
    }

    func updateUIView(_ textView: UITextView, context: Context) {
        if textView.text != text {
            textView.text = text
        }
        textView.textColor = textColor
        textView.tintColor = tintColor
        controller.textView = textView
    }

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    final class Coordinator: NSObject, UITextViewDelegate {
        private let parent: MarkdownTextView

        init(_ parent: MarkdownTextView) { self.parent = parent }

        func textViewDidChange(_ textView: UITextView) {
            parent.text = textView.text
        }

        // Continue list/quote markers when Enter is pressed (web-editor parity).
        func textView(_ textView: UITextView, shouldChangeTextIn range: NSRange, replacementText text: String) -> Bool {
            guard text == "\n", range.length == 0 else { return true }
            let nsText = textView.text as NSString
            let lineRange = nsText.lineRange(for: NSRange(location: range.location, length: 0))
            // Only act when the caret is at the end of the line's content.
            guard range.location == lineRange.location + lineRange.length
                    || range.location == lineRange.location + lineRange.length - 1 else { return true }
            let line = nsText.substring(with: lineRange)

            switch ListContinuation.evaluate(line: line, lineStart: lineRange.location) {
            case .none:
                return true
            case .insert(let marker):
                replace(textView, NSRange(location: range.location, length: 0), with: marker)
                textViewDidChange(textView)
                return false
            case .exit(let markerRange):
                replace(textView, markerRange, with: "")
                textViewDidChange(textView)
                return false
            }
        }

        func textViewDidBeginEditing(_ textView: UITextView) { setFocused(true) }
        func textViewDidEndEditing(_ textView: UITextView) { setFocused(false) }

        private func replace(_ textView: UITextView, _ nsRange: NSRange, with text: String) {
            guard let start = textView.position(from: textView.beginningOfDocument, offset: nsRange.location),
                  let end = textView.position(from: start, offset: nsRange.length),
                  let range = textView.textRange(from: start, to: end) else { return }
            textView.replace(range, withText: text)
        }

        private func setFocused(_ value: Bool) {
            DispatchQueue.main.async { [parent] in
                if parent.isFocused != value { parent.isFocused = value }
            }
        }
    }
}
