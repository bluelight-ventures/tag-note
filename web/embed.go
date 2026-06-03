package web

import "embed"

//go:embed index.html landing.html privacy.html terms.html support.html admin.html app.js style.css manifest.json sw.js favicon.svg icon-192.svg icon-192.png icon-512.svg icon-512.png icon-120.png og-image.svg og-image.png robots.txt sitemap.xml easymde.min.js easymde.min.css
var Assets embed.FS
