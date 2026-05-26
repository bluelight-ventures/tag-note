const CACHE_NAME = 'tagnote-v4';
const STATIC_ASSETS = [
    '/app.js',
    '/style.css',
    '/easymde.min.js',
    '/easymde.min.css',
    '/manifest.json',
    '/icon-192.svg',
    '/icon-512.svg'
];

// Pre-cache static assets on install
self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then(cache => cache.addAll(STATIC_ASSETS))
            .then(() => self.skipWaiting())
    );
});

// Clean up old caches on activate
self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys =>
            Promise.all(keys
                .filter(key => key !== CACHE_NAME)
                .map(key => caches.delete(key))
            )
        ).then(() => self.clients.claim())
    );
});

// Stale-while-revalidate for static assets, network-first for HTML
self.addEventListener('fetch', event => {
    const url = new URL(event.request.url);

    // Let landing page, API, and uploads bypass the service worker
    if (url.pathname === '/' ||
        url.pathname === '/landing.html' ||
        url.pathname.startsWith('/api/') ||
        url.pathname.startsWith('/uploads/')) {
        return;
    }

    // For /app (HTML), always fetch from network (it has dynamic config injected)
    if (url.pathname === '/app' || url.pathname === '/app/') {
        event.respondWith(
            fetch(event.request).catch(() => caches.match(event.request))
        );
        return;
    }

    // Static assets: stale-while-revalidate
    event.respondWith(
        caches.open(CACHE_NAME).then(cache =>
            cache.match(event.request).then(cached => {
                const fetchPromise = fetch(event.request).then(response => {
                    if (response.ok) {
                        cache.put(event.request, response.clone());
                    }
                    return response;
                }).catch(() => cached);

                return cached || fetchPromise;
            })
        )
    );
});
