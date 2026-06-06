import SwiftUI
import UIKit

/// Commands the underlying UITextView for the formatting toolbar. Held by the
/// SwiftUI editor and handed a weak reference to the text view by the
/// representable, so toolbar buttons can act on the real selection/cursor.
final class MarkdownEditorController {
    weak var textView: UITextView?

    /// Wraps the current selection with `prefix`/`suffix` (e.g. `**` for bold).
    /// With no selection, inserts the markers and places the caret between them.
    func wrap(prefix: String, suffix: String) {
        guard let textView, let range = targetRange(in: textView) else { return }
        let selected = textView.text(in: range) ?? ""
        textView.replace(range, withText: prefix + selected + suffix)
        if selected.isEmpty, let caret = textView.selectedTextRange,
           let inner = textView.position(from: caret.start, offset: -suffix.count) {
            textView.selectedTextRange = textView.textRange(from: inner, to: inner)
        }
        notifyChanged(textView)
    }

    /// Inserts a line prefix (heading `## `, list `- `, quote `> `) at the start
    /// of the line the caret is on.
    func prefixLine(_ prefix: String) {
        guard let textView else { return }
        let nsText = textView.text as NSString
        let caret = textView.selectedRange.location
        let safeCaret = min(max(caret, 0), nsText.length)
        let lineStart = nsText.lineRange(for: NSRange(location: safeCaret, length: 0)).location
        guard let startPos = textView.position(from: textView.beginningOfDocument, offset: lineStart),
              let startRange = textView.textRange(from: startPos, to: startPos) else { return }
        textView.replace(startRange, withText: prefix)
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

    private func targetRange(in textView: UITextView) -> UITextRange? {
        textView.selectedTextRange ?? textView.textRange(from: textView.endOfDocument, to: textView.endOfDocument)
    }

    private func notifyChanged(_ textView: UITextView) {
        textView.delegate?.textViewDidChange?(textView)
    }
}

/// A UITextView-backed plain-text editor so the formatting toolbar can wrap the
/// real selection and insert at the caret — things SwiftUI's `TextEditor` can't
/// express. Plain text (markdown source), monospaced, Dynamic-Type aware.
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

        func textViewDidBeginEditing(_ textView: UITextView) {
            setFocused(true)
        }

        func textViewDidEndEditing(_ textView: UITextView) {
            setFocused(false)
        }

        private func setFocused(_ value: Bool) {
            // Avoid mutating SwiftUI state during a view update.
            DispatchQueue.main.async { [parent] in
                if parent.isFocused != value { parent.isFocused = value }
            }
        }
    }
}
