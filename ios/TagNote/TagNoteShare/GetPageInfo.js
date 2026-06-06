// Safari runs this in the shared page before presenting the Share Extension.
// It returns the page title, canonical URL, current text selection, and the
// page's readable text (used for the "Full page" capture mode). The result is
// delivered to the extension as a property-list item under
// NSExtensionJavaScriptPreprocessingResultsKey.
var GetPageInfo = function() {};

GetPageInfo.prototype = {
    run: function(args) {
        var canonical = document.querySelector("link[rel=canonical]");
        args.completionFunction({
            title: document.title || "",
            url: (canonical && canonical.href) || document.location.href,
            selection: (window.getSelection ? window.getSelection().toString() : "") || "",
            articleText: (document.body ? document.body.innerText : "") || ""
        });
    },
    finalize: function(args) {}
};

var ExtensionPreprocessingJS = new GetPageInfo();
