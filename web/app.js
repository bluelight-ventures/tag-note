const API = '/api/v1';

// --- Guest Mode Storage Engine ---
const GUEST_NOTES_KEY = 'tagnote_guest_notes';
const GUEST_TRASH_KEY = 'tagnote_guest_trash';
const GUEST_TAGS_KEY = 'tagnote_guest_tags';
const GUEST_SETTINGS_KEY = 'tagnote_guest_settings';
const GUEST_ACTIVE_KEY = 'tagnote_guest_active';
const GUEST_SEEDED_KEY = 'tagnote_guest_seeded';
const GUEST_NOTE_LIMIT = 5;

function isGuestMode() {
    return !getToken() && localStorage.getItem(GUEST_ACTIVE_KEY) === 'true';
}

function enterGuestMode() {
    localStorage.setItem(GUEST_ACTIVE_KEY, 'true');
    if (!localStorage.getItem(GUEST_SEEDED_KEY)) {
        seedGuestContent();
        localStorage.setItem(GUEST_SEEDED_KEY, 'true');
    }
}

function exitGuestMode() {
    localStorage.removeItem(GUEST_ACTIVE_KEY);
    localStorage.removeItem(GUEST_SEEDED_KEY);
    localStorage.removeItem(GUEST_NOTES_KEY);
    localStorage.removeItem(GUEST_TRASH_KEY);
    localStorage.removeItem(GUEST_TAGS_KEY);
    localStorage.removeItem(GUEST_SETTINGS_KEY);
}

function generateGuestId() {
    // Simple ULID-like ID: timestamp + random
    const t = Date.now().toString(36);
    const r = Math.random().toString(36).substring(2, 10);
    return (t + r).toUpperCase();
}

// Seed notes embedded from backend
const SEED_NOTE_WELCOME = `# Welcome to TagNote!

TagNote organizes your notes with **tags** instead of folders. Here's what makes it different:

- **Tag freely** — every note can have multiple tags, so nothing gets lost in a single folder
- **Filter by tags** — click tags in the sidebar to see only matching notes
- **Combine tags** — filter by multiple tags at once to zoom in on exactly what you need
- **Search everything** — full-text search works alongside tag filters

## Quick start

1. Click **New note** in the sidebar to create your first note
2. Write in Markdown (this editor supports bold, lists, headings, images, and more)
3. Add tags in the tag field above the editor — type and press Enter
4. Click any tag in the sidebar to filter your notes

Take a look at the other example notes to see tags and priorities in action. When you're ready, feel free to delete these notes and start fresh!`;

const SEED_NOTE_TAGS = `# How tags work

Every note in TagNote gets one or more tags. Unlike folders, a note can belong to many categories at once.

## Filtering

- Click a tag in the **sidebar tag cloud** to filter notes
- Click additional tags to narrow down further (AND logic)
- Click an active tag again to remove the filter
- Use the **search bar** to search within your filtered results

## Managing tags

Open the **Tags** tab in the sidebar to:

- Approve or rename tags
- Set importance and urgency (see the priority note)
- Delete tags you no longer need

Tags are created automatically when you add them to a note — no setup needed.`;

const SEED_NOTE_PRIORITY = `# The priority system

TagNote uses an **Eisenhower-style** priority system based on two axes:

- **Importance** (0–100): How much does this matter?
- **Urgency** (0–100): How soon does it need attention?

## How to set priorities

1. Go to the **Tags** tab in the sidebar
2. Click on a tag to expand its settings
3. Adjust the importance and urgency sliders

## Color coding

Notes are color-coded based on the highest-priority tag they carry:

- **Red border** — high importance + high urgency (do first)
- **Amber border** — high on one axis (plan or delegate)
- **No border** — low priority (do later or drop)

Try adjusting the priority on the \`tips\` tag in the Tags tab to see the colors change on these example notes.`;

const SEED_NOTE_MARKDOWN = `# Writing with Markdown

TagNote uses a rich Markdown editor. Here are some things you can do:

## Formatting

- **Bold** with \`**text**\`
- *Italic* with \`*text*\`
- \`Code\` with backticks
- [Links](https://example.com) with \`[text](url)\`

## Lists and structure

1. Numbered lists
2. Like this one

- Bullet lists
- Like this one

> Blockquotes for callouts

## Images

Paste an image from your clipboard, drag and drop a file, or use the toolbar button. Images are uploaded and stored alongside your notes.

## Themes

Click the theme button (sun icon) in the sidebar to cycle through five themes: Light, Dark, Nord, Solarized, and Rosé Pine.`;

function seedGuestContent() {
    const now = Date.now();
    const notes = [
        { content: SEED_NOTE_MARKDOWN, tags: ['tips', 'markdown'], pinned: false },
        { content: SEED_NOTE_PRIORITY, tags: ['tips', 'priority'], pinned: false },
        { content: SEED_NOTE_TAGS, tags: ['tips', 'tags'], pinned: false },
        { content: SEED_NOTE_WELCOME, tags: ['welcome', 'getting-started'], pinned: true }
    ];

    const guestNotes = notes.map((n, i) => {
        const ts = new Date(now + i * 1000).toISOString();
        return {
            short_id: generateGuestId(),
            content: n.content,
            tags: n.tags,
            pinned: n.pinned,
            created_at: ts,
            updated_at: ts
        };
    });

    localStorage.setItem(GUEST_NOTES_KEY, JSON.stringify(guestNotes));
    localStorage.setItem(GUEST_TRASH_KEY, JSON.stringify([]));

    // Set up tag metadata with priorities
    const tagMeta = {
        'welcome': { importance: 80, urgency: 80, status: 'approved', note_count: 1 },
        'getting-started': { importance: 80, urgency: 80, status: 'approved', note_count: 1 },
        'tips': { importance: 60, urgency: 30, status: 'approved', note_count: 3 },
        'markdown': { importance: 50, urgency: 50, status: 'approved', note_count: 1 },
        'priority': { importance: 50, urgency: 50, status: 'approved', note_count: 1 },
        'tags': { importance: 50, urgency: 50, status: 'approved', note_count: 1 }
    };
    localStorage.setItem(GUEST_TAGS_KEY, JSON.stringify(tagMeta));
}

function guestGetNotes() {
    try {
        return JSON.parse(localStorage.getItem(GUEST_NOTES_KEY)) || [];
    } catch { return []; }
}

function guestSetNotes(notes) {
    localStorage.setItem(GUEST_NOTES_KEY, JSON.stringify(notes));
}

function guestGetTrash() {
    try {
        return JSON.parse(localStorage.getItem(GUEST_TRASH_KEY)) || [];
    } catch { return []; }
}

function guestSetTrash(trash) {
    localStorage.setItem(GUEST_TRASH_KEY, JSON.stringify(trash));
}

function guestGetTagMeta() {
    try {
        return JSON.parse(localStorage.getItem(GUEST_TAGS_KEY)) || {};
    } catch { return {}; }
}

function guestSetTagMeta(meta) {
    localStorage.setItem(GUEST_TAGS_KEY, JSON.stringify(meta));
}

function guestUpdateTagCounts() {
    const notes = guestGetNotes();
    const meta = guestGetTagMeta();
    // Reset counts
    for (var t in meta) {
        meta[t].note_count = 0;
    }
    // Count tags
    notes.forEach(function(n) {
        (n.tags || []).forEach(function(tag) {
            if (!meta[tag]) {
                meta[tag] = { importance: 50, urgency: 50, status: 'pending', note_count: 0 };
            }
            meta[tag].note_count++;
        });
    });
    guestSetTagMeta(meta);
}

// --- GuestStore CRUD ---
function guestCreateNote(content, tags) {
    var notes = guestGetNotes();
    var now = new Date().toISOString();
    var note = {
        short_id: generateGuestId(),
        content: content,
        tags: tags || [],
        pinned: false,
        created_at: now,
        updated_at: now
    };
    notes.push(note);
    guestSetNotes(notes);
    // Ensure tags exist in meta
    var meta = guestGetTagMeta();
    (tags || []).forEach(function(tag) {
        if (!meta[tag]) {
            meta[tag] = { importance: 50, urgency: 50, status: 'pending', note_count: 0 };
        }
    });
    guestSetTagMeta(meta);
    guestUpdateTagCounts();
    return { short_id: note.short_id };
}

function guestListNotes(tags, query, sort, limit, offset) {
    var notes = guestGetNotes();
    // Filter by tags
    if (tags && tags.length > 0) {
        notes = notes.filter(function(n) {
            return tags.every(function(t) { return (n.tags || []).includes(t); });
        });
    }
    // Filter by query
    if (query) {
        var q = query.toLowerCase();
        notes = notes.filter(function(n) {
            return (n.content || '').toLowerCase().includes(q);
        });
    }
    // Sort
    if (sort === 'updated') {
        notes.sort(function(a, b) { return new Date(b.updated_at) - new Date(a.updated_at); });
    } else {
        notes.sort(function(a, b) { return new Date(b.created_at) - new Date(a.created_at); });
    }
    // Pinned first
    notes.sort(function(a, b) { return (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0); });
    // Pagination
    var total = notes.length;
    notes = notes.slice(offset || 0, (offset || 0) + (limit || 50));
    return { notes: notes, total: total };
}

function guestGetNote(id) {
    var notes = guestGetNotes();
    return notes.find(function(n) { return n.short_id === id; }) || null;
}

function guestUpdateNote(id, content, tags) {
    var notes = guestGetNotes();
    var idx = notes.findIndex(function(n) { return n.short_id === id; });
    if (idx === -1) return null;
    notes[idx].content = content;
    notes[idx].tags = tags || [];
    notes[idx].updated_at = new Date().toISOString();
    guestSetNotes(notes);
    // Ensure tags exist
    var meta = guestGetTagMeta();
    (tags || []).forEach(function(tag) {
        if (!meta[tag]) {
            meta[tag] = { importance: 50, urgency: 50, status: 'pending', note_count: 0 };
        }
    });
    guestSetTagMeta(meta);
    guestUpdateTagCounts();
    return notes[idx];
}

function guestDeleteNote(id) {
    var notes = guestGetNotes();
    var idx = notes.findIndex(function(n) { return n.short_id === id; });
    if (idx === -1) return false;
    var note = notes.splice(idx, 1)[0];
    note.deleted_at = new Date().toISOString();
    guestSetNotes(notes);
    var trash = guestGetTrash();
    trash.push(note);
    guestSetTrash(trash);
    guestUpdateTagCounts();
    return true;
}

function guestTogglePin(id) {
    var notes = guestGetNotes();
    var idx = notes.findIndex(function(n) { return n.short_id === id; });
    if (idx === -1) return false;
    notes[idx].pinned = !notes[idx].pinned;
    notes[idx].updated_at = new Date().toISOString();
    guestSetNotes(notes);
    return notes[idx].pinned;
}

function guestListTrashed() {
    return guestGetTrash();
}

function guestRestoreNote(id) {
    var trash = guestGetTrash();
    var idx = trash.findIndex(function(n) { return n.short_id === id; });
    if (idx === -1) return false;
    var note = trash.splice(idx, 1)[0];
    delete note.deleted_at;
    guestSetTrash(trash);
    var notes = guestGetNotes();
    notes.push(note);
    guestSetNotes(notes);
    guestUpdateTagCounts();
    return true;
}

function guestPurgeNote(id) {
    var trash = guestGetTrash();
    var idx = trash.findIndex(function(n) { return n.short_id === id; });
    if (idx === -1) return false;
    trash.splice(idx, 1);
    guestSetTrash(trash);
    return true;
}

// --- GuestStore Tag Operations ---
function guestListTags() {
    var meta = guestGetTagMeta();
    return Object.keys(meta).sort();
}

function guestListTagsDetailed() {
    var meta = guestGetTagMeta();
    return Object.keys(meta).map(function(name) {
        return {
            name: name,
            note_count: meta[name].note_count || 0,
            importance: meta[name].importance || 50,
            urgency: meta[name].urgency || 50,
            status: meta[name].status || 'pending'
        };
    }).sort(function(a, b) { return a.name.localeCompare(b.name); });
}

function guestAutocompleteTags(q, limit) {
    var meta = guestGetTagMeta();
    var tags = Object.keys(meta);
    if (q) {
        var ql = q.toLowerCase();
        tags = tags.filter(function(t) { return t.toLowerCase().startsWith(ql); });
    }
    return tags.slice(0, limit || 10);
}

function guestApproveTag(name) {
    var meta = guestGetTagMeta();
    if (meta[name]) {
        meta[name].status = 'approved';
        guestSetTagMeta(meta);
        return true;
    }
    return false;
}

function guestApproveAllTags() {
    var meta = guestGetTagMeta();
    for (var name in meta) {
        meta[name].status = 'approved';
    }
    guestSetTagMeta(meta);
    return true;
}

function guestRenameTag(oldName, newName) {
    var notes = guestGetNotes();
    notes.forEach(function(n) {
        var idx = (n.tags || []).indexOf(oldName);
        if (idx !== -1) {
            n.tags[idx] = newName;
        }
    });
    guestSetNotes(notes);
    var meta = guestGetTagMeta();
    if (meta[oldName]) {
        meta[newName] = meta[oldName];
        delete meta[oldName];
        guestSetTagMeta(meta);
    }
    guestUpdateTagCounts();
    return true;
}

function guestDeleteTag(name) {
    var notes = guestGetNotes();
    notes.forEach(function(n) {
        n.tags = (n.tags || []).filter(function(t) { return t !== name; });
    });
    guestSetNotes(notes);
    var meta = guestGetTagMeta();
    delete meta[name];
    guestSetTagMeta(meta);
    return true;
}

function guestUpdateTagPriority(name, importance, urgency) {
    var meta = guestGetTagMeta();
    if (!meta[name]) {
        meta[name] = { importance: 50, urgency: 50, status: 'pending', note_count: 0 };
    }
    meta[name].importance = importance;
    meta[name].urgency = urgency;
    guestSetTagMeta(meta);
    return true;
}

function guestGetSettings() {
    try {
        return JSON.parse(localStorage.getItem(GUEST_SETTINGS_KEY)) || {};
    } catch { return {}; }
}

function guestSaveSettings(settings) {
    localStorage.setItem(GUEST_SETTINGS_KEY, JSON.stringify(settings));
}

function guestGetAllNotes() {
    return guestGetNotes();
}

function guestGetNoteCount() {
    return guestGetNotes().length;
}

// --- Guest Mode UI Functions ---
function showGuestBanner() {
    if (!isGuestMode()) return;
    if (sessionStorage.getItem('guest_banner_dismissed') === 'true') return;
    var banner = document.getElementById('guest-banner');
    if (banner) {
        banner.style.display = 'flex';
    }
}

function hideGuestBanner() {
    var banner = document.getElementById('guest-banner');
    if (banner) {
        banner.style.display = 'none';
    }
    sessionStorage.setItem('guest_banner_dismissed', 'true');
}

function showGuestLimitModal() {
    var modal = document.getElementById('guest-limit-modal');
    if (modal) {
        modal.style.display = 'flex';
    }
}

function hideGuestLimitModal() {
    var modal = document.getElementById('guest-limit-modal');
    if (modal) {
        modal.style.display = 'none';
    }
}

function initGuestModeHandlers() {
    // Guest banner dismiss
    var dismissBtn = document.getElementById('guest-banner-dismiss');
    if (dismissBtn) {
        dismissBtn.addEventListener('click', hideGuestBanner);
    }

    // Guest banner CTA - go to register
    var ctaBtn = document.getElementById('guest-banner-cta');
    if (ctaBtn) {
        ctaBtn.addEventListener('click', function() {
            sessionStorage.setItem('guest_converting', 'true');
            showAuthView();
            // Switch to register tab
            switchAuthTab('register');
        });
    }

    // Guest limit modal - Maybe later
    var laterBtn = document.getElementById('guest-limit-later');
    if (laterBtn) {
        laterBtn.addEventListener('click', hideGuestLimitModal);
    }

    // Guest limit modal - Create account
    var createBtn = document.getElementById('guest-limit-create');
    if (createBtn) {
        createBtn.addEventListener('click', function() {
            hideGuestLimitModal();
            sessionStorage.setItem('guest_converting', 'true');
            showAuthView();
            switchAuthTab('register');
        });
    }

    // Click outside to close guest limit modal
    var modal = document.getElementById('guest-limit-modal');
    if (modal) {
        modal.addEventListener('click', function(e) {
            if (e.target === modal) {
                hideGuestLimitModal();
            }
        });
    }
}

function switchAuthTab(tab) {
    var tabs = document.querySelectorAll('.auth-tab');
    tabs.forEach(function(t) {
        t.classList.toggle('active', t.dataset.tab === tab);
    });
    var loginBtn = document.getElementById('login-btn');
    var registerBtn = document.getElementById('register-btn');
    var displayNameGroup = document.getElementById('display-name-group');
    var passwordStrength = document.getElementById('password-strength');
    var switchToRegister = document.getElementById('auth-switch-to-register');
    var switchToLogin = document.getElementById('auth-switch-to-login');
    var forgotLink = document.getElementById('auth-forgot-link');

    if (tab === 'register') {
        loginBtn.style.display = 'none';
        registerBtn.style.display = 'block';
        displayNameGroup.style.display = 'block';
        passwordStrength.style.display = 'block';
        switchToRegister.style.display = 'none';
        switchToLogin.style.display = 'block';
        forgotLink.style.display = 'none';
    } else {
        loginBtn.style.display = 'block';
        registerBtn.style.display = 'none';
        displayNameGroup.style.display = 'none';
        passwordStrength.style.display = 'none';
        switchToRegister.style.display = 'block';
        switchToLogin.style.display = 'none';
        forgotLink.style.display = 'block';
    }
}

// --- Toast Notification System ---
function showToast(message, type) {
    type = type || 'info';
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast toast-' + type;
    const msg = document.createElement('span');
    msg.textContent = message;
    toast.appendChild(msg);
    const dismiss = document.createElement('button');
    dismiss.className = 'toast-dismiss';
    dismiss.innerHTML = '&times;';
    dismiss.addEventListener('click', function() { removeToast(toast); });
    toast.appendChild(dismiss);
    container.appendChild(toast);
    setTimeout(function() { removeToast(toast); }, 4500);
}

function removeToast(toast) {
    if (!toast.parentNode) return;
    toast.style.animation = 'toast-out 0.2s ease forwards';
    setTimeout(function() { if (toast.parentNode) toast.remove(); }, 200);
}

function showUndoToast(message, onUndo) {
    var container = document.getElementById('toast-container');
    var toast = document.createElement('div');
    toast.className = 'toast toast-info';
    var msg = document.createElement('span');
    msg.textContent = message;
    toast.appendChild(msg);
    var undoBtn = document.createElement('button');
    undoBtn.className = 'toast-undo';
    undoBtn.textContent = 'Undo';
    undoBtn.addEventListener('click', function() {
        removeToast(toast);
        if (onUndo) onUndo();
    });
    toast.appendChild(undoBtn);
    var dismiss = document.createElement('button');
    dismiss.className = 'toast-dismiss';
    dismiss.innerHTML = '&times;';
    dismiss.addEventListener('click', function() { removeToast(toast); });
    toast.appendChild(dismiss);
    container.appendChild(toast);
    setTimeout(function() { removeToast(toast); }, 8000);
}

// --- Modal Dialog System ---
function showModal(options) {
    return new Promise(function(resolve) {
        var overlay = document.getElementById('modal-overlay');
        var body = document.getElementById('modal-body');
        var inputWrap = document.getElementById('modal-input-wrap');
        var input = document.getElementById('modal-input');
        var cancelBtn = document.getElementById('modal-cancel');
        var confirmBtn = document.getElementById('modal-confirm');

        body.textContent = options.message || '';

        if (options.prompt) {
            inputWrap.style.display = '';
            input.value = options.defaultValue || '';
            input.placeholder = '';
        } else {
            inputWrap.style.display = 'none';
        }

        confirmBtn.textContent = options.confirmText || 'OK';
        confirmBtn.className = 'btn btn-sm ' + (options.danger ? 'btn-danger' : 'btn-primary');
        cancelBtn.textContent = options.cancelText || 'Cancel';

        overlay.style.display = '';
        trapFocus(overlay);

        if (options.prompt) {
            setTimeout(function() { input.focus(); input.select(); }, 50);
        } else {
            setTimeout(function() { confirmBtn.focus(); }, 50);
        }

        function cleanup() {
            overlay.style.display = 'none';
            releaseFocus();
            confirmBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
            document.removeEventListener('keydown', onKey);
        }

        function onConfirm() {
            cleanup();
            if (options.prompt) {
                resolve(input.value);
            } else {
                resolve(true);
            }
        }

        function onCancel() {
            cleanup();
            resolve(options.prompt ? null : false);
        }

        function onKey(e) {
            if (e.key === 'Escape') { onCancel(); }
            if (e.key === 'Enter' && options.prompt) { onConfirm(); }
        }

        confirmBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);
        document.addEventListener('keydown', onKey);
    });
}

// --- Auth helpers ---
function getToken() { return localStorage.getItem('tagnote_token'); }

// --- Focus Trap ---
var _focusTrapPrev = null;
var _focusTrapHandler = null;

function trapFocus(container) {
    _focusTrapPrev = document.activeElement;
    var appContent = document.getElementById('app-content');
    if (appContent) appContent.setAttribute('aria-hidden', 'true');

    function handler(e) {
        if (e.key !== 'Tab') return;
        var focusable = container.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
        var items = Array.prototype.filter.call(focusable, function(el) {
            return el.offsetParent !== null && !el.disabled;
        });
        if (items.length === 0) return;
        var first = items[0];
        var last = items[items.length - 1];
        if (e.shiftKey) {
            if (document.activeElement === first) { e.preventDefault(); last.focus(); }
        } else {
            if (document.activeElement === last) { e.preventDefault(); first.focus(); }
        }
    }
    _focusTrapHandler = handler;
    document.addEventListener('keydown', handler);
}

function releaseFocus() {
    if (_focusTrapHandler) {
        document.removeEventListener('keydown', _focusTrapHandler);
        _focusTrapHandler = null;
    }
    var appContent = document.getElementById('app-content');
    if (appContent) appContent.removeAttribute('aria-hidden');
    if (_focusTrapPrev && _focusTrapPrev.focus) {
        _focusTrapPrev.focus();
        _focusTrapPrev = null;
    }
}
function setToken(t) { localStorage.setItem('tagnote_token', t); }
function clearAuth() { localStorage.removeItem('tagnote_token'); localStorage.removeItem('tagnote_user'); }
function getUser() { const u = localStorage.getItem('tagnote_user'); return u ? JSON.parse(u) : null; }
function setUser(u) { localStorage.setItem('tagnote_user', JSON.stringify(u)); }
function isLoggedIn() { return !!getToken(); }

// --- EasyMDE editor instances ---
let focusEditor = null;

function getPreviewMode() {
    return localStorage.getItem('tagnote_preview_mode') || 'plain';
}
function setPreviewMode(mode) {
    localStorage.setItem('tagnote_preview_mode', mode);
}

// --- Theme System ---
const THEME_KEY = 'tagnote_theme';
const DEFAULT_THEME = 'everforest-light';
const THEMES = ['everforest-light', 'everforest-dark', 'solarized-light', 'solarized-dark', 'gruvbox-light', 'gruvbox-dark', 'nord-light', 'nord-dark'];
const THEME_LABELS = {
    'everforest-light': 'Everforest Light',
    'everforest-dark': 'Everforest Dark',
    'solarized-light': 'Solarized Light',
    'solarized-dark': 'Solarized Dark',
    'gruvbox-light': 'Gruvbox Light',
    'gruvbox-dark': 'Gruvbox Dark',
    'nord-light': 'Nord Light',
    'nord-dark': 'Nord Dark'
};
const THEME_META_COLORS = {
    'everforest-light': '#f3ead3',
    'everforest-dark': '#272e33',
    'solarized-light': '#eee8d5',
    'solarized-dark': '#002b36',
    'gruvbox-light': '#f2e5bc',
    'gruvbox-dark': '#282828',
    'nord-light': '#e5e9f0',
    'nord-dark': '#2e3440'
};

function getStoredTheme() {
    return localStorage.getItem(THEME_KEY) || '';
}

function getThemeType() {
    const theme = document.documentElement.getAttribute('data-theme');
    if (theme && theme.endsWith('-light')) return 'light';
    if (!theme) return 'light';
    return 'dark';
}

function applyTheme(theme) {
    if (!THEMES.includes(theme)) theme = DEFAULT_THEME;
    document.documentElement.setAttribute('data-theme', theme);
    const metaThemeColor = document.querySelector('meta[name="theme-color"]');
    if (metaThemeColor) {
        metaThemeColor.setAttribute('content', THEME_META_COLORS[theme]);
    }
    const appleStatusBar = document.querySelector('meta[name="apple-mobile-web-app-status-bar-style"]');
    if (appleStatusBar) {
        appleStatusBar.setAttribute('content', theme.endsWith('-light') ? 'default' : 'black-translucent');
    }
    localStorage.setItem(THEME_KEY, theme);
    const btn = document.getElementById('theme-toggle');
    if (btn) btn.title = THEME_LABELS[theme] || theme;
    if (lastNotes && lastNotes.length > 0) {
        renderFeed(lastNotes);
    }
}

function cycleTheme() {
    const current = getStoredTheme() || DEFAULT_THEME;
    const idx = THEMES.indexOf(current);
    const next = THEMES[(idx + 1) % THEMES.length];
    applyTheme(next);
    return next;
}

function initTheme() {
    let theme = getStoredTheme();
    if (!theme || !THEMES.includes(theme)) {
        if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
            theme = 'everforest-dark';
        } else {
            theme = DEFAULT_THEME;
        }
    }
    applyTheme(theme);
}

if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (!getStoredTheme()) {
            applyTheme(e.matches ? 'everforest-dark' : DEFAULT_THEME);
        }
    });
}

function compressImage(file, maxDimension, quality) {
    return new Promise((resolve, reject) => {
        if (file.type === 'image/gif') { resolve(file); return; }
        if (file.size < 200 * 1024) { resolve(file); return; }
        const img = new Image();
        const url = URL.createObjectURL(file);
        img.onload = () => {
            URL.revokeObjectURL(url);
            let { width, height } = img;
            if (width > maxDimension || height > maxDimension) {
                if (width > height) {
                    height = Math.round(height * maxDimension / width);
                    width = maxDimension;
                } else {
                    width = Math.round(width * maxDimension / height);
                    height = maxDimension;
                }
            }
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            const ctx = canvas.getContext('2d');
            ctx.drawImage(img, 0, 0, width, height);
            const outputType = file.type === 'image/png' ? 'image/png' : 'image/jpeg';
            const outputQuality = outputType === 'image/jpeg' ? quality : undefined;
            canvas.toBlob(blob => {
                if (!blob) { resolve(file); return; }
                if (blob.size < file.size) {
                    const ext = outputType === 'image/jpeg' ? '.jpg' : '.png';
                    resolve(new File([blob], file.name.replace(/\.[^.]+$/, ext), { type: outputType }));
                } else {
                    resolve(file);
                }
            }, outputType, outputQuality);
        };
        img.onerror = () => { URL.revokeObjectURL(url); resolve(file); };
        img.src = url;
    });
}

function uploadImage(file, onSuccess, onError) {
    compressImage(file, 1920, 0.85).then(compressedFile => {
        const formData = new FormData();
        formData.append('file', compressedFile, compressedFile.name || file.name);
        const token = getToken();
        fetch(API + '/images', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + token },
            body: formData,
        })
        .then(resp => {
            if (resp.status === 401) {
                clearAuth();
                showAuthView();
                onError('Session expired. Please log in again.');
                return;
            }
            return resp.json();
        })
        .then(data => {
            if (data && data.data && data.data.filePath) {
                onSuccess(data.data.filePath);
            } else if (data && data.error) {
                onError(data.error);
            }
        })
        .catch(err => onError(err.message || 'Upload failed'));
    }).catch(err => onError('Compression failed: ' + err.message));
}

function createEasyMDE(textareaEl, opts) {
    const forcePlain = opts && opts.forcePlain;
    const mode = forcePlain ? 'plain' : getPreviewMode();
    const editor = new EasyMDE({
        element: textareaEl,
        spellChecker: false,
        autofocus: false,
        status: false,
        placeholder: textareaEl.getAttribute('placeholder') || 'Write your note...',
        imageMaxSize: 5 * 1024 * 1024,
        imageAccept: 'image/png, image/jpeg, image/gif, image/webp',
        imageUploadFunction: uploadImage,
        imagePathAbsolute: true,
        sideBySideFullscreen: false,
        toolbar: [
            'bold', 'italic', 'strikethrough', 'heading', '|',
            'code', 'quote', 'unordered-list', 'ordered-list', '|',
            'link', 'upload-image', 'table', 'horizontal-rule', '|',
            'preview', 'side-by-side', 'fullscreen', '|',
            'guide',
        ],
    });

    if (mode === 'side-by-side') {
        editor.toggleSideBySide();
    } else if (mode === 'preview') {
        editor.togglePreview();
    }
    // 'plain' mode: no toggle needed, single-column editor

    const cm = editor.codemirror;
    const wrapper = cm.getWrapperElement();
    const editorContainer = wrapper.closest('.EasyMDEContainer');
    const observer = new MutationObserver(() => {
        const preview = editorContainer ? editorContainer.querySelector('.editor-preview') : null;
        const isSideBySide = wrapper.classList.contains('CodeMirror-sided');
        const isPreview = preview && preview.classList.contains('editor-preview-active');
        if (isSideBySide) {
            setPreviewMode('side-by-side');
        } else if (isPreview) {
            setPreviewMode('preview');
        } else {
            setPreviewMode('plain');
        }
    });
    observer.observe(wrapper, { attributes: true, attributeFilter: ['class'] });
    if (editorContainer) {
        const previewEl = editorContainer.querySelector('.editor-preview');
        if (previewEl) {
            observer.observe(previewEl, { attributes: true, attributeFilter: ['class'] });
        }
    }

    if (opts && opts.autofocus) {
        setTimeout(() => cm.focus(), 100);
    }

    return editor;
}

// --- Views ---
var currentAuthTab = 'login';

function showAuthView() {
    document.getElementById('app-content').style.display = 'none';
    document.getElementById('auth-view').style.display = '';
    document.getElementById('auth-error').style.display = 'none';
    switchAuthTab('login');
}

function switchAuthTab(tab) {
    currentAuthTab = tab;
    var tabs = document.querySelectorAll('.auth-tab');
    tabs.forEach(function(t) {
        t.classList.toggle('active', t.dataset.tab === tab);
    });

    var displayNameGroup = document.getElementById('display-name-group');
    var loginBtn = document.getElementById('login-btn');
    var registerBtn = document.getElementById('register-btn');
    var switchToRegister = document.getElementById('auth-switch-to-register');
    var switchToLogin = document.getElementById('auth-switch-to-login');
    var passwordInput = document.getElementById('auth-password');
    var passwordStrength = document.getElementById('password-strength');

    if (tab === 'login') {
        displayNameGroup.style.display = 'none';
        loginBtn.style.display = '';
        registerBtn.style.display = 'none';
        switchToRegister.style.display = '';
        switchToLogin.style.display = 'none';
        passwordInput.setAttribute('autocomplete', 'current-password');
        passwordStrength.style.display = 'none';
    } else {
        displayNameGroup.style.display = '';
        loginBtn.style.display = 'none';
        registerBtn.style.display = '';
        switchToRegister.style.display = 'none';
        switchToLogin.style.display = '';
        passwordInput.setAttribute('autocomplete', 'new-password');
        if (passwordInput.value) {
            passwordInput.dispatchEvent(new Event('input'));
        }
    }

    document.getElementById('auth-error').style.display = 'none';
}

function showAppContent() {
    document.getElementById('auth-view').style.display = 'none';
    document.getElementById('app-content').style.display = '';
    const user = getUser();
    if (user) {
        document.getElementById('user-display').textContent = user.display_name || user.email;
    }
    // Reset UI elements that guest mode may have hidden
    if (!isGuestMode()) {
        document.getElementById('sidebar-filters').style.display = '';
        document.getElementById('export-btn').style.display = '';
        document.getElementById('import-btn').style.display = '';
    }
    // Load settings from server and apply if no local override
    loadServerSettings();
}

function loadServerSettings() {
    api('GET', '/settings').then(function(settings) {
        if (settings && settings.theme && !getStoredTheme()) {
            applyTheme(settings.theme);
        } else if (settings && settings.theme && getStoredTheme() !== settings.theme) {
            // Server has a setting — sync local to server if they differ
            // Prefer server-side setting on fresh login
        }
        if (settings && settings.preview_mode && !getPreviewMode()) {
            setPreviewMode(settings.preview_mode);
        }
    }).catch(function() {});
}

function showAuthError(msg) {
    const el = document.getElementById('auth-error');
    el.textContent = msg;
    el.style.display = '';
}

// --- Auth events ---
document.querySelectorAll('.auth-tab').forEach(function(tab) {
    tab.addEventListener('click', function() {
        switchAuthTab(this.dataset.tab);
    });
});

document.getElementById('switch-to-register').addEventListener('click', function(e) {
    e.preventDefault();
    switchAuthTab('register');
});

document.getElementById('switch-to-login').addEventListener('click', function(e) {
    e.preventDefault();
    switchAuthTab('login');
});

document.getElementById('password-toggle').addEventListener('click', function() {
    var input = document.getElementById('auth-password');
    var eyeOpen = this.querySelector('.eye-open');
    var eyeClosed = this.querySelector('.eye-closed');
    if (input.type === 'password') {
        input.type = 'text';
        eyeOpen.style.display = 'none';
        eyeClosed.style.display = '';
        this.title = 'Hide password';
    } else {
        input.type = 'password';
        eyeOpen.style.display = '';
        eyeClosed.style.display = 'none';
        this.title = 'Show password';
    }
});

function setButtonLoading(btn, loading) {
    if (loading) {
        btn.classList.add('btn-loading');
        btn.disabled = true;
    } else {
        btn.classList.remove('btn-loading');
        btn.disabled = false;
    }
}

document.getElementById('login-btn').addEventListener('click', async () => {
    const btn = document.getElementById('login-btn');
    const email = document.getElementById('auth-email').value.trim();
    const password = document.getElementById('auth-password').value;
    if (!email || !password) { showAuthError('Email and password are required.'); return; }
    setButtonLoading(btn, true);
    try {
        const resp = await fetch(API + '/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });
        const data = await resp.json();
        if (!resp.ok) { showAuthError(data.error || 'Login failed'); setButtonLoading(btn, false); return; }
        if (data.pending_verify) {
            showVerifyPendingView(data.pending_verify_email || email);
            setButtonLoading(btn, false);
            return;
        }
        setToken(data.token);
        setUser(data.user);
        showAppContent();
        await migrateGuestNotes();
        loadTags();
        updateBadge();
        loadFeed();
    } catch (e) {
        showAuthError('Login failed: ' + e.message);
    }
    setButtonLoading(btn, false);
});

// --- Auth View Helpers ---
function showVerifyPendingView(email) {
    document.getElementById('auth-form-main').style.display = 'none';
    document.getElementById('auth-verify-pending').style.display = 'block';
    document.getElementById('auth-forgot').style.display = 'none';
    document.getElementById('auth-reset').style.display = 'none';
    document.getElementById('verify-email-display').textContent = email;
    // Store email for resend
    window.pendingVerifyEmail = email;
}

function showMainAuthForm() {
    document.getElementById('auth-form-main').style.display = 'block';
    document.getElementById('auth-verify-pending').style.display = 'none';
    document.getElementById('auth-forgot').style.display = 'none';
    document.getElementById('auth-reset').style.display = 'none';
    clearAuthError();
}

function showForgotPasswordView() {
    document.getElementById('auth-form-main').style.display = 'none';
    document.getElementById('auth-verify-pending').style.display = 'none';
    document.getElementById('auth-forgot').style.display = 'block';
    document.getElementById('auth-reset').style.display = 'none';
    document.getElementById('forgot-error').style.display = 'none';
    document.getElementById('forgot-success').style.display = 'none';
    document.getElementById('forgot-email').value = document.getElementById('auth-email').value;
}

function showResetPasswordView() {
    document.getElementById('auth-form-main').style.display = 'none';
    document.getElementById('auth-verify-pending').style.display = 'none';
    document.getElementById('auth-forgot').style.display = 'none';
    document.getElementById('auth-reset').style.display = 'block';
    document.getElementById('reset-error').style.display = 'none';
    document.getElementById('reset-success').style.display = 'none';
}

// --- Forgot Password Link ---
document.getElementById('forgot-password-link').addEventListener('click', (e) => {
    e.preventDefault();
    showForgotPasswordView();
});

// --- Magic Link Login ---
var magicLinkMode = false;

function setMagicLinkMode(enabled) {
    magicLinkMode = enabled;
    var passwordGroup = document.querySelector('.input-group:has(#auth-password)');
    var passwordStrength = document.getElementById('password-strength');
    var magicLinkBtn = document.getElementById('magic-link-btn');
    var loginBtn = document.getElementById('login-btn');
    var forgotLink = document.getElementById('auth-forgot-link');

    if (enabled) {
        if (passwordGroup) passwordGroup.style.display = 'none';
        if (passwordStrength) passwordStrength.style.display = 'none';
        if (forgotLink) forgotLink.style.display = 'none';
        magicLinkBtn.textContent = 'Login with password';
        loginBtn.textContent = 'Send login link';
    } else {
        if (passwordGroup) passwordGroup.style.display = '';
        magicLinkBtn.textContent = 'Login without password';
        loginBtn.textContent = 'Login';
        if (forgotLink) forgotLink.style.display = '';
    }
}

document.getElementById('magic-link-btn').addEventListener('click', function(e) {
    e.preventDefault();
    setMagicLinkMode(!magicLinkMode);
});

// Update login button to handle magic link mode
var originalLoginHandler = null;
document.getElementById('login-btn').addEventListener('click', async function(e) {
    if (!magicLinkMode) return; // Let original handler work

    e.preventDefault();
    e.stopPropagation();

    var btn = document.getElementById('login-btn');
    var email = document.getElementById('auth-email').value.trim();
    if (!email) {
        showAuthError('Email is required');
        return;
    }

    setButtonLoading(btn, true);
    try {
        var resp = await fetch(API + '/auth/magic-link', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email: email })
        });
        var data = await resp.json();
        if (!resp.ok) {
            showAuthError(data.error || 'Failed to send login link');
            setButtonLoading(btn, false);
            return;
        }
        showToast('Login link sent! Check your email.', 'success');
        setButtonLoading(btn, false);
    } catch (err) {
        showAuthError('Failed to send login link: ' + err.message);
        setButtonLoading(btn, false);
    }
}, true);

// --- Back to Login Links ---
document.getElementById('back-to-login').addEventListener('click', (e) => {
    e.preventDefault();
    showMainAuthForm();
});

document.getElementById('forgot-back-to-login').addEventListener('click', (e) => {
    e.preventDefault();
    showMainAuthForm();
});

document.getElementById('reset-back-to-login').addEventListener('click', (e) => {
    e.preventDefault();
    showMainAuthForm();
});

// --- Resend Verification ---
document.getElementById('resend-verification-btn').addEventListener('click', async () => {
    const btn = document.getElementById('resend-verification-btn');
    const email = window.pendingVerifyEmail;
    if (!email) return;
    setButtonLoading(btn, true);
    try {
        const resp = await fetch(API + '/auth/resend-verification', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email })
        });
        const data = await resp.json();
        if (!resp.ok) {
            showToast(data.error || 'Failed to resend email', 'error');
        } else {
            showToast('Verification email sent!', 'success');
        }
    } catch (e) {
        showToast('Failed to resend email: ' + e.message, 'error');
    }
    setButtonLoading(btn, false);
});

// --- Forgot Password Submit ---
document.getElementById('forgot-submit-btn').addEventListener('click', async () => {
    const btn = document.getElementById('forgot-submit-btn');
    const email = document.getElementById('forgot-email').value.trim();
    const errorEl = document.getElementById('forgot-error');
    const successEl = document.getElementById('forgot-success');

    errorEl.style.display = 'none';
    successEl.style.display = 'none';

    if (!email) {
        errorEl.textContent = 'Email is required';
        errorEl.style.display = 'block';
        return;
    }

    setButtonLoading(btn, true);
    try {
        const resp = await fetch(API + '/auth/forgot-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email })
        });
        const data = await resp.json();
        if (!resp.ok) {
            errorEl.textContent = data.error || 'Failed to send reset email';
            errorEl.style.display = 'block';
        } else {
            successEl.textContent = 'If an account exists with this email, a password reset link has been sent.';
            successEl.style.display = 'block';
        }
    } catch (e) {
        errorEl.textContent = 'Failed to send reset email: ' + e.message;
        errorEl.style.display = 'block';
    }
    setButtonLoading(btn, false);
});

// --- Reset Password Submit ---
document.getElementById('reset-submit-btn').addEventListener('click', async () => {
    const btn = document.getElementById('reset-submit-btn');
    const password = document.getElementById('reset-password').value;
    const token = window.resetToken;
    const errorEl = document.getElementById('reset-error');
    const successEl = document.getElementById('reset-success');

    errorEl.style.display = 'none';
    successEl.style.display = 'none';

    if (!password) {
        errorEl.textContent = 'Password is required';
        errorEl.style.display = 'block';
        return;
    }

    if (password.length < 8) {
        errorEl.textContent = 'Password must be at least 8 characters';
        errorEl.style.display = 'block';
        return;
    }

    if (!token) {
        errorEl.textContent = 'Invalid reset link';
        errorEl.style.display = 'block';
        return;
    }

    setButtonLoading(btn, true);
    try {
        const resp = await fetch(API + '/auth/reset-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token, new_password: password })
        });
        const data = await resp.json();
        if (!resp.ok) {
            errorEl.textContent = data.error || 'Failed to reset password';
            errorEl.style.display = 'block';
        } else {
            successEl.textContent = 'Password reset successfully! You can now login with your new password.';
            successEl.style.display = 'block';
            window.resetToken = null;
            // Clear URL params
            window.history.replaceState({}, document.title, window.location.pathname);
            setTimeout(() => {
                showMainAuthForm();
            }, 2000);
        }
    } catch (e) {
        errorEl.textContent = 'Failed to reset password: ' + e.message;
        errorEl.style.display = 'block';
    }
    setButtonLoading(btn, false);
});

// --- Reset Password Toggle ---
const resetPasswordToggle = document.getElementById('reset-password-toggle');
if (resetPasswordToggle) {
    resetPasswordToggle.addEventListener('click', () => {
        const input = document.getElementById('reset-password');
        const eyeOpen = resetPasswordToggle.querySelector('.eye-open');
        const eyeClosed = resetPasswordToggle.querySelector('.eye-closed');
        if (input.type === 'password') {
            input.type = 'text';
            eyeOpen.style.display = 'none';
            eyeClosed.style.display = 'block';
        } else {
            input.type = 'password';
            eyeOpen.style.display = 'block';
            eyeClosed.style.display = 'none';
        }
    });
}

// --- Reset Password Strength Meter ---
const resetPasswordInput = document.getElementById('reset-password');
if (resetPasswordInput) {
    resetPasswordInput.addEventListener('input', () => {
        const strengthFill = document.getElementById('reset-strength-fill');
        const strengthLabel = document.getElementById('reset-strength-label');
        const password = resetPasswordInput.value;
        const { score, label, color } = calculatePasswordStrength(password);
        if (strengthFill && strengthLabel) {
            strengthFill.style.width = (score / 4 * 100) + '%';
            strengthFill.style.backgroundColor = color;
            strengthLabel.textContent = label;
            strengthLabel.style.color = color;
        }
    });
}

// --- Google Sign-In ---
document.getElementById('google-signin-btn').addEventListener('click', async () => {
    // This is now just a fallback - the real Google button is rendered inside
    const btn = document.getElementById('google-signin-btn');
    if (typeof google === 'undefined' || !google.accounts) {
        showAuthError('Google Sign-In is not available. Please use email/password.');
        return;
    }
    // Trigger One Tap prompt as fallback
    google.accounts.id.prompt();
});

// --- Google ID Token Handler ---
async function handleGoogleCredentialResponse(response) {
    try {
        const resp = await fetch(API + '/auth/google', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id_token: response.credential })
        });
        const data = await resp.json();
        if (!resp.ok) {
            showAuthError(data.error || 'Google login failed');
            return;
        }
        setToken(data.token);
        setUser(data.user);
        showAppContent();
        await migrateGuestNotes();
        loadTags();
        updateBadge();
        loadFeed();
    } catch (e) {
        showAuthError('Google login failed: ' + e.message);
    }
}

// --- Initialize Google Identity Services ---
function initGoogleSignIn() {
    // Only initialize if Google client ID is configured
    const googleClientId = window.GOOGLE_CLIENT_ID;
    if (!googleClientId) {
        // Hide Google button if not configured
        hideGoogleButton();
        return;
    }

    // Check if Google library is loaded
    if (typeof google !== 'undefined' && google.accounts) {
        setupGoogleSignIn(googleClientId);
    } else {
        // Wait for Google script to load (it's loaded async)
        // Check periodically for up to 5 seconds
        let attempts = 0;
        const maxAttempts = 50;
        const checkInterval = setInterval(() => {
            attempts++;
            if (typeof google !== 'undefined' && google.accounts) {
                clearInterval(checkInterval);
                setupGoogleSignIn(googleClientId);
            } else if (attempts >= maxAttempts) {
                clearInterval(checkInterval);
                console.warn('Google Sign-In library failed to load');
                hideGoogleButton();
            }
        }, 100);
    }
}

function setupGoogleSignIn(clientId) {
    google.accounts.id.initialize({
        client_id: clientId,
        callback: handleGoogleCredentialResponse,
        auto_select: false,
        cancel_on_tap_outside: true
    });

    // Render the official Google button inside our custom button container
    // This avoids popup blocking issues in Safari
    const buttonContainer = document.getElementById('google-signin-btn');
    if (buttonContainer) {
        // Clear the button content and render Google's official button
        buttonContainer.innerHTML = '';
        buttonContainer.style.padding = '0';
        buttonContainer.style.border = 'none';
        buttonContainer.style.background = 'transparent';
        buttonContainer.style.height = 'auto';
        buttonContainer.style.minHeight = '44px';

        google.accounts.id.renderButton(buttonContainer, {
            type: 'standard',
            theme: 'outline',
            size: 'large',
            text: 'continue_with',
            shape: 'rectangular',
            width: buttonContainer.offsetWidth || 300
        });
    }
}

function hideGoogleButton() {
    const googleBtn = document.getElementById('google-signin-btn');
    if (googleBtn) googleBtn.style.display = 'none';
    const divider = document.querySelector('.auth-divider');
    if (divider) divider.style.display = 'none';
}

// --- Handle URL Parameters for Auth ---
function handleAuthUrlParams() {
    const params = new URLSearchParams(window.location.search);

    // Handle email verification
    const verifyToken = params.get('verify');
    if (verifyToken) {
        verifyEmailToken(verifyToken);
        return true;
    }

    // Handle password reset
    const resetToken = params.get('reset');
    if (resetToken) {
        window.resetToken = resetToken;
        showAuthView();
        showResetPasswordView();
        return true;
    }

    // Handle magic link login
    const magicToken = params.get('magic');
    if (magicToken) {
        verifyMagicLinkToken(magicToken);
        return true;
    }

    return false;
}

async function verifyMagicLinkToken(token) {
    showAuthView();
    showToast('Logging you in...', 'info');
    try {
        const resp = await fetch(API + '/auth/verify-magic-link', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token })
        });
        const data = await resp.json();
        if (!resp.ok) {
            showAuthError(data.error || 'Login link is invalid or expired');
            window.history.replaceState({}, document.title, window.location.pathname);
            return;
        }
        setToken(data.token);
        setUser(data.user);
        window.history.replaceState({}, document.title, window.location.pathname);
        showToast('Welcome back!', 'success');
        showAppContent();
        await migrateGuestNotes();
        loadTags();
        updateBadge();
        loadFeed();
    } catch (e) {
        showAuthError('Login failed: ' + e.message);
        window.history.replaceState({}, document.title, window.location.pathname);
    }
}

async function verifyEmailToken(token) {
    showAuthView();
    showToast('Verifying email...', 'info');
    try {
        const resp = await fetch(API + '/auth/verify-email', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ token })
        });
        const data = await resp.json();
        if (!resp.ok) {
            showAuthError(data.error || 'Email verification failed');
            // Clear URL params
            window.history.replaceState({}, document.title, window.location.pathname);
            return;
        }
        // Clear URL params
        window.history.replaceState({}, document.title, window.location.pathname);
        setToken(data.token);
        setUser(data.user);
        showAppContent();
        loadTags();
        updateBadge();
        loadFeed();
        showToast('Email verified successfully!', 'success');
    } catch (e) {
        showAuthError('Email verification failed: ' + e.message);
        window.history.replaceState({}, document.title, window.location.pathname);
    }
}

document.getElementById('register-btn').addEventListener('click', async () => {
    const btn = document.getElementById('register-btn');
    const email = document.getElementById('auth-email').value.trim();
    const password = document.getElementById('auth-password').value;
    const displayName = document.getElementById('auth-display-name').value.trim();
    if (!email || !password) { showAuthError('Email and password are required.'); return; }
    setButtonLoading(btn, true);
    try {
        const resp = await fetch(API + '/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password, display_name: displayName })
        });
        const data = await resp.json();
        if (!resp.ok) { showAuthError(data.error || 'Registration failed'); setButtonLoading(btn, false); return; }
        if (data.pending_verify) {
            showVerifyPendingView(data.pending_verify_email || email);
            setButtonLoading(btn, false);
            return;
        }
        setToken(data.token);
        setUser(data.user);
        showAppContent();
        await migrateGuestNotes();
        loadTags();
        updateBadge();
        loadFeed();
    } catch (e) {
        showAuthError('Registration failed: ' + e.message);
    }
    setButtonLoading(btn, false);
});

document.getElementById('auth-display-name').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        document.getElementById('auth-password').focus();
    }
});

document.getElementById('auth-password').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        if (currentAuthTab === 'login') {
            document.getElementById('login-btn').click();
        } else {
            document.getElementById('register-btn').click();
        }
    }
});

// --- Password strength indicator ---
document.getElementById('auth-password').addEventListener('input', function() {
    var pw = this.value;
    var el = document.getElementById('password-strength');
    var fill = document.getElementById('strength-fill');
    var label = document.getElementById('strength-label');
    if (!pw || currentAuthTab !== 'register') { el.style.display = 'none'; return; }
    el.style.display = '';
    var score = 0;
    if (pw.length >= 8) score++;
    if (pw.length >= 12) score++;
    if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
    if (/[0-9]/.test(pw)) score++;
    if (/[^a-zA-Z0-9]/.test(pw)) score++;
    var levels = [
        { max: 1, width: '20%', color: '#dc2626', text: 'Weak' },
        { max: 2, width: '40%', color: '#f59e0b', text: 'Fair' },
        { max: 3, width: '70%', color: '#16a34a', text: 'Good' },
        { max: 5, width: '100%', color: '#059669', text: 'Strong' }
    ];
    var level = levels[0];
    for (var i = 0; i < levels.length; i++) {
        if (score <= levels[i].max) { level = levels[i]; break; }
        level = levels[i];
    }
    fill.style.width = level.width;
    fill.style.background = level.color;
    label.textContent = level.text;
    label.style.color = level.color;
});

document.getElementById('logout-btn').addEventListener('click', () => {
    if (isGuestMode()) {
        exitGuestMode();
    }
    clearAuth();
    // Remove guest parameter from URL
    if (window.location.search.includes('guest')) {
        var url = new URL(window.location.href);
        url.searchParams.delete('guest');
        window.history.replaceState({}, '', url.pathname);
    }
    showAuthView();
});

document.getElementById('export-btn').addEventListener('click', async () => {
    try {
        const resp = await fetch(API + '/notes/export', {
            headers: { 'Authorization': 'Bearer ' + getToken() }
        });
        if (!resp.ok) throw new Error('Export failed');
        const data = await resp.json();
        // Ensure client-side settings are included in the export
        if (!data.settings) data.settings = {};
        if (!data.settings.theme) data.settings.theme = getStoredTheme() || '';
        if (!data.settings.preview_mode) data.settings.preview_mode = getPreviewMode() || '';
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'tagnote-export.json';
        a.click();
        URL.revokeObjectURL(url);
        var counts = [];
        if (data.notes && data.notes.length) counts.push(data.notes.length + ' note' + (data.notes.length !== 1 ? 's' : ''));
        if (data.trash && data.trash.length) counts.push(data.trash.length + ' trashed');
        if (data.tags && data.tags.length) counts.push(data.tags.length + ' tag' + (data.tags.length !== 1 ? 's' : ''));
        showToast('Exported ' + (counts.length ? counts.join(', ') : 'data'), 'success');
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
});

// --- Import Notes ---
document.getElementById('import-btn').addEventListener('click', () => {
    document.getElementById('import-file-input').value = '';
    document.getElementById('import-file-input').click();
});

document.getElementById('import-file-input').addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file) return;

    let parsed;
    try {
        const text = await file.text();
        parsed = JSON.parse(text);
    } catch (err) {
        showToast('Could not read file: ' + err.message, 'error');
        return;
    }

    // Detect format: full export (has version field), legacy array, or legacy {notes:[...]}
    var isFullExport = parsed && typeof parsed === 'object' && !Array.isArray(parsed) && parsed.version > 0;

    if (isFullExport) {
        // Full format import
        var importPayload = {
            version: parsed.version,
            notes: (parsed.notes || []).map(function(n) {
                var note = { content: n.content || '', tags: n.tags || [], pinned: !!n.pinned };
                if (n.created_at) note.created_at = n.created_at;
                if (n.updated_at) note.updated_at = n.updated_at;
                return note;
            }),
            trash: (parsed.trash || []).map(function(n) {
                var note = { content: n.content || '', tags: n.tags || [], pinned: !!n.pinned };
                if (n.created_at) note.created_at = n.created_at;
                if (n.updated_at) note.updated_at = n.updated_at;
                return note;
            }),
            tags: (parsed.tags || []).map(function(t) {
                return { name: t.name || '', status: t.status || 'approved', note_count: 0, importance: t.importance != null ? t.importance : 50, urgency: t.urgency != null ? t.urgency : 50 };
            }),
            settings: parsed.settings || null,
            dry_run: true
        };

        var totalItems = importPayload.notes.length + importPayload.trash.length + importPayload.tags.length;
        if (totalItems === 0) {
            showToast('No data found in the export file.', 'error');
            return;
        }

        openImportPreview();
        var importContent = document.getElementById('import-content');
        importContent.innerHTML = '<div class="loading-spinner"></div>';
        document.getElementById('import-confirm').style.display = 'none';

        try {
            var preview = await api('POST', '/notes/import', importPayload);
            renderFullImportPreview(preview, importPayload);
        } catch (err) {
            importContent.innerHTML = '<p class="error-msg">' + esc(err.message) + '</p>';
        }
    } else {
        // Legacy format
        var importedNotes;
        if (Array.isArray(parsed)) {
            importedNotes = parsed;
        } else if (parsed && parsed.notes && Array.isArray(parsed.notes)) {
            importedNotes = parsed.notes;
        } else {
            showToast('Invalid file format. Expected a TagNote export file.', 'error');
            return;
        }

        if (importedNotes.length === 0) {
            showToast('No notes found in the file.', 'error');
            return;
        }

        var notesToImport = importedNotes.map(function(n) {
            var note = { content: n.content || '', tags: n.tags || [], pinned: !!n.pinned };
            if (n.created_at) note.created_at = n.created_at;
            if (n.updated_at) note.updated_at = n.updated_at;
            return note;
        });

        openImportPreview();
        var importContent2 = document.getElementById('import-content');
        importContent2.innerHTML = '<div class="loading-spinner"></div>';
        document.getElementById('import-confirm').style.display = 'none';

        try {
            var preview2 = await api('POST', '/notes/import', { notes: notesToImport, dry_run: true });
            renderImportPreview(preview2, notesToImport);
        } catch (err) {
            importContent2.innerHTML = '<p class="error-msg">' + esc(err.message) + '</p>';
        }
    }
});

function openImportPreview() {
    var overlay = document.getElementById('import-overlay');
    overlay.style.display = '';
    trapFocus(overlay);
}

function closeImportPreview() {
    document.getElementById('import-overlay').style.display = 'none';
    releaseFocus();
}

document.getElementById('import-close').addEventListener('click', closeImportPreview);

// --- Full format import preview ---
function renderFullImportPreview(preview, importPayload) {
    var importContent = document.getElementById('import-content');
    var confirmBtn = document.getElementById('import-confirm');
    var titleEl = document.getElementById('import-title');

    var newNoteCount = preview.new_notes ? preview.new_notes.length : 0;
    var dupNoteCount = preview.duplicate_notes ? preview.duplicate_notes.length : 0;
    var newTrashCount = preview.new_trash ? preview.new_trash.length : 0;
    var dupTrashCount = preview.duplicate_trash ? preview.duplicate_trash.length : 0;
    var newTagCount = preview.new_tags ? preview.new_tags.length : 0;
    var updatedTagCount = preview.updated_tags ? preview.updated_tags.length : 0;
    var hasSettings = preview.settings && (preview.settings.theme || preview.settings.preview_mode);

    var totalNew = newNoteCount + newTrashCount + newTagCount + updatedTagCount + (hasSettings ? 1 : 0);
    titleEl.textContent = 'Import Data';

    var html = '';

    // --- Notes section ---
    html += '<div class="import-section">';
    html += '<h3 class="import-section-title import-section-new">' + newNoteCount + ' new note' + (newNoteCount !== 1 ? 's' : '') + ' will be imported</h3>';
    if (newNoteCount > 0) {
        html += '<div class="import-note-list">';
        for (var i = 0; i < preview.new_notes.length; i++) {
            html += renderImportNoteCard(preview.new_notes[i], false);
        }
        html += '</div>';
    }
    if (dupNoteCount > 0) {
        html += '<h3 class="import-section-title import-section-dup">' + dupNoteCount + ' duplicate note' + (dupNoteCount !== 1 ? 's' : '') + ' will be skipped</h3>';
        html += '<div class="import-note-list">';
        for (var j = 0; j < preview.duplicate_notes.length; j++) {
            html += renderImportNoteCard(preview.duplicate_notes[j], true);
        }
        html += '</div>';
    }
    html += '</div>';

    // --- Trash section ---
    if (newTrashCount > 0 || dupTrashCount > 0) {
        html += '<div class="import-section">';
        if (newTrashCount > 0) {
            html += '<h3 class="import-section-title import-section-new">' + newTrashCount + ' trashed note' + (newTrashCount !== 1 ? 's' : '') + ' will be imported</h3>';
            html += '<div class="import-note-list">';
            for (var ti = 0; ti < preview.new_trash.length; ti++) {
                html += renderImportNoteCard(preview.new_trash[ti], false);
            }
            html += '</div>';
        }
        if (dupTrashCount > 0) {
            html += '<h3 class="import-section-title import-section-dup">' + dupTrashCount + ' duplicate trashed note' + (dupTrashCount !== 1 ? 's' : '') + ' will be skipped</h3>';
            html += '<div class="import-note-list">';
            for (var tj = 0; tj < preview.duplicate_trash.length; tj++) {
                html += renderImportNoteCard(preview.duplicate_trash[tj], true);
            }
            html += '</div>';
        }
        html += '</div>';
    }

    // --- Tags section ---
    if (newTagCount > 0 || updatedTagCount > 0) {
        html += '<div class="import-section">';
        if (newTagCount > 0) {
            html += '<h3 class="import-section-title import-section-new">' + newTagCount + ' new tag' + (newTagCount !== 1 ? 's' : '') + '</h3>';
            html += '<div class="import-tag-list">';
            for (var tk = 0; tk < preview.new_tags.length; tk++) {
                html += renderImportTagCard(preview.new_tags[tk]);
            }
            html += '</div>';
        }
        if (updatedTagCount > 0) {
            html += '<h3 class="import-section-title import-section-new">' + updatedTagCount + ' tag' + (updatedTagCount !== 1 ? 's' : '') + ' will be updated</h3>';
            html += '<div class="import-tag-list">';
            for (var tl = 0; tl < preview.updated_tags.length; tl++) {
                html += renderImportTagCard(preview.updated_tags[tl]);
            }
            html += '</div>';
        }
        html += '</div>';
    }

    // --- Settings section ---
    if (hasSettings) {
        html += '<div class="import-section">';
        html += '<h3 class="import-section-title import-section-new">Settings will be applied</h3>';
        html += '<div class="import-settings-info">';
        if (preview.settings.theme) html += '<span class="import-setting-item">Theme: <strong>' + esc(preview.settings.theme) + '</strong></span> ';
        if (preview.settings.preview_mode) html += '<span class="import-setting-item">Preview mode: <strong>' + esc(preview.settings.preview_mode) + '</strong></span>';
        html += '</div>';
        html += '</div>';
    }

    importContent.innerHTML = html;

    if (totalNew > 0) {
        var parts = [];
        if (newNoteCount > 0) parts.push(newNoteCount + ' note' + (newNoteCount !== 1 ? 's' : ''));
        if (newTrashCount > 0) parts.push(newTrashCount + ' trashed');
        if (newTagCount + updatedTagCount > 0) parts.push((newTagCount + updatedTagCount) + ' tag' + ((newTagCount + updatedTagCount) !== 1 ? 's' : ''));
        if (hasSettings) parts.push('settings');
        confirmBtn.textContent = 'Import ' + parts.join(', ');
        confirmBtn.style.display = '';
        var newBtn = confirmBtn.cloneNode(true);
        confirmBtn.parentNode.replaceChild(newBtn, confirmBtn);
        newBtn.addEventListener('click', async function() {
            newBtn.classList.add('btn-loading');
            try {
                importPayload.dry_run = false;
                var result = await api('POST', '/notes/import', importPayload);
                closeImportPreview();
                var msgs = [];
                if (result.imported_notes > 0) msgs.push(result.imported_notes + ' note' + (result.imported_notes !== 1 ? 's' : ''));
                if (result.imported_trash > 0) msgs.push(result.imported_trash + ' trashed note' + (result.imported_trash !== 1 ? 's' : ''));
                if (result.imported_tags > 0) msgs.push(result.imported_tags + ' tag' + (result.imported_tags !== 1 ? 's' : ''));
                if (result.settings_applied) {
                    msgs.push('settings');
                    // Apply imported settings locally
                    if (importPayload.settings) {
                        if (importPayload.settings.theme) {
                            applyTheme(importPayload.settings.theme);
                        }
                        if (importPayload.settings.preview_mode) {
                            setPreviewMode(importPayload.settings.preview_mode);
                        }
                    }
                }
                showToast('Imported ' + (msgs.length ? msgs.join(', ') : 'data'), 'success');
                refresh();
            } catch (err) {
                showToast('Import failed: ' + err.message, 'error');
            } finally {
                newBtn.classList.remove('btn-loading');
            }
        });
    } else {
        confirmBtn.style.display = 'none';
    }
}

function renderImportTagCard(tag) {
    var html = '<div class="import-note-card">';
    html += '<span class="tag">#' + esc(tag.name) + '</span>';
    if (tag.importance !== 50 || tag.urgency !== 50) {
        html += ' <span class="import-tag-priority">Importance: ' + tag.importance + ', Urgency: ' + tag.urgency + '</span>';
    }
    html += '</div>';
    return html;
}

// --- Legacy import preview (backward compatible) ---
function renderImportPreview(preview, allNotes) {
    var importContent = document.getElementById('import-content');
    var confirmBtn = document.getElementById('import-confirm');
    var titleEl = document.getElementById('import-title');

    var newCount = preview.new ? preview.new.length : 0;
    var dupCount = preview.duplicates ? preview.duplicates.length : 0;

    titleEl.textContent = 'Import Notes (' + allNotes.length + ' total)';

    var html = '';

    html += '<div class="import-section">';
    html += '<h3 class="import-section-title import-section-new">' + newCount + ' new note' + (newCount !== 1 ? 's' : '') + ' will be imported</h3>';
    if (newCount > 0) {
        html += '<div class="import-note-list">';
        for (var i = 0; i < preview.new.length; i++) {
            html += renderImportNoteCard(preview.new[i], false);
        }
        html += '</div>';
    } else {
        html += '<p class="placeholder">All notes already exist.</p>';
    }
    html += '</div>';

    if (dupCount > 0) {
        html += '<div class="import-section">';
        html += '<h3 class="import-section-title import-section-dup">' + dupCount + ' duplicate' + (dupCount !== 1 ? 's' : '') + ' will be skipped</h3>';
        html += '<div class="import-note-list">';
        for (var j = 0; j < preview.duplicates.length; j++) {
            html += renderImportNoteCard(preview.duplicates[j], true);
        }
        html += '</div>';
        html += '</div>';
    }

    importContent.innerHTML = html;

    if (newCount > 0) {
        confirmBtn.textContent = 'Import ' + newCount + ' note' + (newCount !== 1 ? 's' : '');
        confirmBtn.style.display = '';
        var newBtn = confirmBtn.cloneNode(true);
        confirmBtn.parentNode.replaceChild(newBtn, confirmBtn);
        newBtn.addEventListener('click', async function() {
            newBtn.classList.add('btn-loading');
            try {
                var result = await api('POST', '/notes/import', { notes: preview.new, dry_run: false });
                closeImportPreview();
                showToast(result.imported + ' note' + (result.imported !== 1 ? 's' : '') + ' imported', 'success');
                refresh();
            } catch (err) {
                showToast('Import failed: ' + err.message, 'error');
            } finally {
                newBtn.classList.remove('btn-loading');
            }
        });
    } else {
        confirmBtn.style.display = 'none';
    }
}

function renderImportNoteCard(note, isDuplicate) {
    var snippet = (note.content || '').substring(0, 200);
    if ((note.content || '').length > 200) snippet += '...';

    var tagsHtml = '';
    if (note.tags && note.tags.length > 0) {
        for (var k = 0; k < note.tags.length; k++) {
            tagsHtml += '<span class="tag">#' + esc(note.tags[k]) + '</span> ';
        }
    }

    var cardClass = 'import-note-card' + (isDuplicate ? ' import-duplicate' : '');
    if (note.pinned) cardClass += ' pinned';

    return '<div class="' + cardClass + '">'
        + '<div class="import-note-tags">' + tagsHtml + '</div>'
        + '<div class="import-note-snippet">' + esc(snippet) + '</div>'
        + (note.pinned ? '<span class="import-pinned-badge">Pinned</span>' : '')
        + '</div>';
}

document.getElementById('theme-toggle').addEventListener('click', () => {
    var newTheme = cycleTheme();
    // Persist theme to server
    if (isLoggedIn()) {
        api('PUT', '/settings', { theme: newTheme, preview_mode: getPreviewMode() || '' }).catch(function() {});
    }
});

// --- Core ---
function parseTags(str) {
    return str.split(',').map(s => s.trim().toLowerCase()).filter(Boolean);
}

// --- Chip Input Component ---
function initChipInput(containerEl, inputEl, options) {
    options = options || {};
    var filterOnly = options.filterOnly || false;
    var onChange = options.onChange || null;
    const chips = []; // Array of {tag: string, el: HTMLElement}
    let lastChipHighlighted = false;
    let suppressChange = false;

    function notifyChange() {
        if (!suppressChange && onChange) onChange();
    }

    function createChip(tag) {
        tag = tag.trim().toLowerCase();
        if (!tag || chips.some(function(c) { return c.tag === tag; })) return;

        // In filter-only mode, reject tags not in tagCache
        if (filterOnly && !tagCache[tag]) return;

        var chipEl = document.createElement('span');
        chipEl.className = 'chip';

        // Visual encoding based on tagCache
        var info = tagCache[tag];
        if (info && info.status === 'approved') {
            var color = getPriorityColor([tag]);
            if (color) {
                chipEl.style.background = color.bg;
                chipEl.style.color = color.text;
                chipEl.style.border = '1px solid ' + color.border;
            }
        } else if (!filterOnly) {
            chipEl.classList.add('ghost');
        }

        // Ghost "+" prefix for new/unreviewed tags (not in filter mode)
        if (!filterOnly && (!info || info.status !== 'approved')) {
            var plusSpan = document.createElement('span');
            plusSpan.className = 'chip-new-icon';
            plusSpan.textContent = '+';
            chipEl.appendChild(plusSpan);
        }

        // Tag text
        var labelSpan = document.createElement('span');
        labelSpan.className = 'chip-label';
        labelSpan.textContent = tag;
        chipEl.appendChild(labelSpan);

        // Remove button
        var removeBtn = document.createElement('button');
        removeBtn.className = 'chip-remove';
        removeBtn.type = 'button';
        removeBtn.innerHTML = '&times;';
        removeBtn.setAttribute('aria-label', 'Remove ' + tag);
        removeBtn.addEventListener('mousedown', function(e) {
            e.preventDefault();
            removeChip(tag);
        });
        chipEl.appendChild(removeBtn);

        // Insert before the input element
        containerEl.insertBefore(chipEl, inputEl);
        chips.push({ tag: tag, el: chipEl });

        inputEl.value = '';
        if (!filterOnly) updateFocusPriorityPreview();
        notifyChange();
    }

    function removeChip(tag) {
        var idx = chips.findIndex(function(c) { return c.tag === tag; });
        if (idx === -1) return;
        chips[idx].el.remove();
        chips.splice(idx, 1);
        inputEl.focus();
        if (!filterOnly) updateFocusPriorityPreview();
        notifyChange();
    }

    function commitInput() {
        var val = inputEl.value.trim().toLowerCase();
        if (val) {
            createChip(val);
            inputEl.value = '';
        }
    }

    // Keyboard handling
    inputEl.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault();
            commitInput();
        } else if (e.key === ' ') {
            var val = inputEl.value.trim();
            if (val) {
                e.preventDefault();
                commitInput();
            }
        } else if (e.key === 'Backspace') {
            if (inputEl.selectionStart === 0 && inputEl.selectionEnd === 0 && inputEl.value === '') {
                if (chips.length === 0) return;
                if (lastChipHighlighted) {
                    var last = chips[chips.length - 1];
                    last.el.classList.remove('highlighted');
                    removeChip(last.tag);
                    lastChipHighlighted = false;
                } else {
                    chips[chips.length - 1].el.classList.add('highlighted');
                    lastChipHighlighted = true;
                }
                return;
            }
            lastChipHighlighted = false;
        } else {
            if (lastChipHighlighted && chips.length > 0) {
                chips[chips.length - 1].el.classList.remove('highlighted');
                lastChipHighlighted = false;
            }
        }
    });

    // Paste handling
    inputEl.addEventListener('paste', function(e) {
        e.preventDefault();
        var text = (e.clipboardData || window.clipboardData).getData('text');
        var tags = text.split(/[,;\s]+/).map(function(s) { return s.trim().toLowerCase(); }).filter(Boolean);
        for (var k = 0; k < tags.length; k++) {
            createChip(tags[k]);
        }
    });

    // Click container to focus input
    containerEl.addEventListener('click', function(e) {
        if (e.target === containerEl) {
            inputEl.focus();
        }
    });

    function getTags() {
        return chips.map(function(c) { return c.tag; });
    }

    function setTags(tagArray) {
        suppressChange = true;
        clearTags();
        for (var k = 0; k < (tagArray || []).length; k++) {
            createChip(tagArray[k]);
        }
        suppressChange = false;
    }

    function clearTags() {
        while (chips.length > 0) {
            chips[0].el.remove();
            chips.shift();
        }
        inputEl.value = '';
        lastChipHighlighted = false;
    }

    return { getTags: getTags, setTags: setTags, clearTags: clearTags, createChip: createChip, commitInput: commitInput };
}

// Initialize chip inputs (declared here so openFocus/submit/filterByTag/etc. can reference them)
var focusChips = null; // initialized after DOM-dependent code below
var filterChips = null;

function tagQuery(tags) {
    return tags.map(t => 'tag=' + encodeURIComponent(t)).join('&');
}

function buildFilterQuery(page) {
    let q = tagQuery(currentTags);
    if (currentQuery) {
        q += (q ? '&' : '') + 'q=' + encodeURIComponent(currentQuery);
    }
    if (currentSort === 'updated') {
        q += (q ? '&' : '') + 'sort=updated';
    }
    q += (q ? '&' : '') + 'limit=' + FEED_PAGE_SIZE;
    if (page > 0) {
        q += '&offset=' + (page * FEED_PAGE_SIZE);
    }
    return q;
}

async function api(method, path, body) {
    // Guest mode interception
    if (isGuestMode()) {
        return guestApiHandler(method, path, body);
    }

    const opts = { method, headers: {} };
    const token = getToken();
    if (token) {
        opts.headers['Authorization'] = 'Bearer ' + token;
    }
    if (body) {
        opts.headers['Content-Type'] = 'application/json';
        opts.body = JSON.stringify(body);
    }
    const resp = await fetch(API + path, opts);
    if (resp.status === 401) {
        clearAuth();
        showAuthView();
        throw new Error('Session expired. Please log in again.');
    }
    if (!resp.ok) {
        const err = await resp.text();
        throw new Error(err);
    }
    const ct = resp.headers.get('content-type') || '';
    if (ct.includes('json')) return resp.json();
    return resp.text();
}

function guestApiHandler(method, path, body) {
    // POST /notes - Create note (with limit check)
    if (method === 'POST' && path === '/notes') {
        if (guestGetNoteCount() >= GUEST_NOTE_LIMIT) {
            showGuestLimitModal();
            return Promise.reject(new Error('GUEST_LIMIT'));
        }
        return Promise.resolve(guestCreateNote(body.content, body.tags));
    }
    // GET /notes/trash - List trashed (must come before GET /notes/:id)
    if (method === 'GET' && path === '/notes/trash') {
        return Promise.resolve(guestListTrashed());
    }
    // POST /notes/trash/:id/restore - Restore note
    if (method === 'POST' && /^\/notes\/trash\/[^/]+\/restore$/.test(path)) {
        var id = path.split('/')[3];
        if (guestRestoreNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // DELETE /notes/trash/:id - Purge note
    if (method === 'DELETE' && /^\/notes\/trash\/[^/]+$/.test(path)) {
        var id = path.split('/')[3];
        if (guestPurgeNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // GET /notes/:id - Get note (must come after /notes/trash routes)
    if (method === 'GET' && /^\/notes\/[^/?]+$/.test(path.split('?')[0]) && path.split('/')[2] !== 'trash') {
        var id = path.split('/')[2].split('?')[0];
        var note = guestGetNote(id);
        if (!note) return Promise.reject(new Error('Note not found'));
        return Promise.resolve(note);
    }
    // GET /notes - List notes
    if (method === 'GET' && (path === '/notes' || path.startsWith('/notes?'))) {
        var params = new URLSearchParams(path.split('?')[1] || '');
        var tags = params.get('tags') ? params.get('tags').split(',') : [];
        var query = params.get('q') || '';
        var sort = params.get('sort') || 'newest';
        var limit = parseInt(params.get('limit')) || 50;
        var offset = parseInt(params.get('offset')) || 0;
        var result = guestListNotes(tags, query, sort, limit, offset);
        return Promise.resolve(result.notes);
    }
    // PUT /notes/:id - Update note
    if (method === 'PUT' && /^\/notes\/[^/]+$/.test(path) && !path.includes('/restore')) {
        var id = path.split('/')[2];
        var note = guestUpdateNote(id, body.content, body.tags);
        if (!note) return Promise.reject(new Error('Note not found'));
        return Promise.resolve(note);
    }
    // PUT /notes/:id/restore - Restore note from trash (undo delete)
    if (method === 'PUT' && /^\/notes\/[^/]+\/restore$/.test(path)) {
        var id = path.split('/')[2];
        if (guestRestoreNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // DELETE /notes/:id - Delete note
    if (method === 'DELETE' && /^\/notes\/[^/]+$/.test(path) && !path.includes('/trash/')) {
        var id = path.split('/')[2];
        if (guestDeleteNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // POST /notes/:id/pin - Toggle pin
    if (method === 'POST' && /^\/notes\/[^/]+\/pin$/.test(path)) {
        var id = path.split('/')[2];
        var pinned = guestTogglePin(id);
        return Promise.resolve({ pinned: pinned });
    }
    // GET /trash - List trashed
    if (method === 'GET' && path === '/trash') {
        return Promise.resolve(guestListTrashed());
    }
    // POST /trash/:id/restore - Restore note
    if (method === 'POST' && /^\/trash\/[^/]+\/restore$/.test(path)) {
        var id = path.split('/')[2];
        if (guestRestoreNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // DELETE /trash/:id - Purge note
    if (method === 'DELETE' && /^\/trash\/[^/]+$/.test(path)) {
        var id = path.split('/')[2];
        if (guestPurgeNote(id)) return Promise.resolve({});
        return Promise.reject(new Error('Note not found'));
    }
    // GET /tags - List tags
    if (method === 'GET' && (path === '/tags' || path.startsWith('/tags?'))) {
        return Promise.resolve(guestListTags());
    }
    // GET /tags/detailed - List tags detailed
    if (method === 'GET' && path === '/tags/detailed') {
        return Promise.resolve(guestListTagsDetailed());
    }
    // GET /tags/autocomplete - Autocomplete tags
    if (method === 'GET' && path.startsWith('/tags/autocomplete')) {
        var params = new URLSearchParams(path.split('?')[1] || '');
        var q = params.get('q') || '';
        var limit = parseInt(params.get('limit')) || 10;
        return Promise.resolve(guestAutocompleteTags(q, limit));
    }
    // POST /tags/:name/approve - Approve tag
    if (method === 'POST' && /^\/tags\/[^/]+\/approve$/.test(path)) {
        var name = decodeURIComponent(path.split('/')[2]);
        guestApproveTag(name);
        return Promise.resolve({});
    }
    // POST /tags/approve-all - Approve all tags
    if (method === 'POST' && path === '/tags/approve-all') {
        guestApproveAllTags();
        return Promise.resolve({});
    }
    // POST /tags/:name/rename - Rename tag
    if (method === 'POST' && /^\/tags\/[^/]+\/rename$/.test(path)) {
        var oldName = decodeURIComponent(path.split('/')[2]);
        guestRenameTag(oldName, body.new_name);
        return Promise.resolve({});
    }
    // DELETE /tags/:name - Delete tag
    if (method === 'DELETE' && /^\/tags\/[^/]+$/.test(path)) {
        var name = decodeURIComponent(path.split('/')[2]);
        guestDeleteTag(name);
        return Promise.resolve({});
    }
    // PUT /tags/:name/priority - Update tag priority
    if (method === 'PUT' && /^\/tags\/[^/]+\/priority$/.test(path)) {
        var name = decodeURIComponent(path.split('/')[2]);
        guestUpdateTagPriority(name, body.importance, body.urgency);
        return Promise.resolve({});
    }
    // GET /settings - Get settings (not implemented for guest)
    if (method === 'GET' && path === '/settings') {
        return Promise.resolve(guestGetSettings());
    }
    // PUT /settings - Save settings
    if (method === 'PUT' && path === '/settings') {
        guestSaveSettings(body);
        return Promise.resolve({});
    }
    // POST /notes/import - Import notes (not supported in guest mode)
    if (method === 'POST' && path === '/notes/import') {
        return Promise.reject(new Error('Import not supported in guest mode'));
    }
    // POST /notes/export - Export notes (not supported in guest mode)
    if (method === 'GET' && path === '/notes/export') {
        return Promise.reject(new Error('Export not supported in guest mode'));
    }
    // Fallback - not supported
    return Promise.reject(new Error('Operation not supported in guest mode'));
}

let currentTags = [];
let currentQuery = '';
let currentSort = 'newest';
let lastNotes = [];
let feedPage = 0;
let feedLoading = false;
let feedHasMore = true;
const FEED_PAGE_SIZE = 50;
let tagCache = {}; // {tagName: {importance, urgency, status, note_count}}

// --- Markdown ---
function simpleMarkdown(text) {
    return text
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
        .replace(/^### (.+)$/gm, '<h3>$1</h3>')
        .replace(/^## (.+)$/gm, '<h2>$1</h2>')
        .replace(/^# (.+)$/gm, '<h1>$1</h1>')
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/g, '<em>$1</em>')
        .replace(/`(.+?)`/g, '<code>$1</code>')
        .replace(/^---$/gm, '<hr>')
        .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" style="max-width:100%">')
        .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank">$1</a>')
        .replace(/\n\n/g, '</p><p>')
        .replace(/\n/g, '<br>')
        .replace(/^/, '<p>').replace(/$/, '</p>');
}

function renderMarkdown(text) {
    if (focusEditor && typeof focusEditor.markdown === 'function') {
        return focusEditor.markdown(text);
    }
    return simpleMarkdown(text);
}

function highlightText(html, query) {
    if (!query) return html;
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp('(?<=>)([^<]*?)(' + escaped + ')', 'gi');
    return html.replace(re, function(m, pre, match) {
        return '>' + pre + '<mark>' + match + '</mark>';
    }).replace(/^([^<]*?)(<mark>)/i, function(m, pre, tag) {
        return pre + tag;
    });
}

function esc(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}

// --- Priority Color System ---
async function refreshTagCache() {
    try {
        const tags = await api('GET', '/tags/detailed');
        tagCache = {};
        for (const t of tags) {
            tagCache[t.name] = {
                importance: t.importance,
                urgency: t.urgency,
                status: t.status,
                note_count: t.note_count
            };
        }
        return tags;
    } catch (e) {
        return [];
    }
}

// Smooth gradient color mapping using bilinear interpolation across I/U space.
// Corner hues: (lowI,lowU)=slate, (highI,lowU)=green, (lowI,highU)=amber, (highI,highU)=red.
// Returns {bg, border, text, label} with HSL color strings, or null for default.
function getPriorityColor(tags) {
    if (!tags || tags.length === 0) return null;
    let maxI = 0, maxU = 0;
    let found = false;
    for (const t of tags) {
        const info = tagCache[t];
        if (info) {
            found = true;
            if (info.importance > maxI) maxI = info.importance;
            if (info.urgency > maxU) maxU = info.urgency;
        }
    }
    if (!found) return null;

    // Normalize to 0-1
    const i = maxI / 100;
    const u = maxU / 100;

    // Distance from neutral center (0.5, 0.5) — determines color intensity
    const dx = i - 0.5, dy = u - 0.5;
    const dist = Math.min(1, Math.sqrt(dx * dx + dy * dy) / 0.707); // max dist is sqrt(0.5) ≈ 0.707

    // Below threshold, card stays default white
    if (dist < 0.12) return null;

    // Corner hues (in degrees)
    const hLL = 215; // low I, low U — slate/blue
    const hHL = 145; // high I, low U — green
    const hLH = 35;  // low I, high U — amber
    const hHH = 0;   // high I, high U — red

    // Bilinear interpolation of hue
    function lerpAngle(a, b, t) {
        // Shortest-path interpolation on the hue circle
        let delta = ((b - a + 540) % 360) - 180;
        return ((a + delta * t) + 360) % 360;
    }
    const topHue = lerpAngle(hLH, hHH, i);    // high U edge
    const botHue = lerpAngle(hLL, hHL, i);     // low U edge
    const hue = lerpAngle(botHue, topHue, u);

    // Saturation and lightness scale with distance from center
    const isDark = getThemeType() === 'dark';
    const sat = isDark
        ? 25 + dist * 40       // 25% → 65% (muted for dark surfaces)
        : 30 + dist * 55;      // 30% → 85%
    const bgLight = isDark
        ? 15 + dist * 8        // 15% → 23% (dark pastels)
        : 97 - dist * 8;       // 97% → 89% (light pastels)
    const borderLight = isDark
        ? 35 + dist * 15       // 35% → 50%
        : 55 - dist * 15;      // 55% → 40%
    const textLight = isDark
        ? 70 + dist * 10       // 70% → 80% (light text on dark)
        : 35 - dist * 10;      // 35% → 25% (dark text on light)

    // Generate label based on quadrant
    let label;
    if (i > 0.6 && u > 0.6) label = 'Critical';
    else if (i > 0.6) label = 'Strategic';
    else if (u > 0.6) label = 'Tactical';
    else if (i < 0.35 && u < 0.35) label = 'Archive';
    else label = 'Normal';

    return {
        bg: 'hsl(' + Math.round(hue) + ',' + Math.round(sat) + '%,' + Math.round(bgLight) + '%)',
        border: 'hsl(' + Math.round(hue) + ',' + Math.round(sat) + '%,' + Math.round(borderLight) + '%)',
        text: 'hsl(' + Math.round(hue) + ',' + Math.round(sat) + '%,' + Math.round(textLight) + '%)',
        label: label
    };
}

// Per-tag pill color for tag cloud and note card tag pills.
// Returns {bg, text} with inactive (subtle tint) or active (bold) variants, or null for default.
function getTagPillColor(tagName, isActive) {
    var info = tagCache[tagName];
    if (!info) return null;

    var i = info.importance / 100;
    var u = info.urgency / 100;

    var dx = i - 0.5, dy = u - 0.5;
    var dist = Math.min(1, Math.sqrt(dx * dx + dy * dy) / 0.707);

    if (dist < 0.12) return null;

    // Corner hues — same as getPriorityColor
    var hLL = 215, hHL = 145, hLH = 35, hHH = 0;
    function lerpAngle(a, b, t) {
        var delta = ((b - a + 540) % 360) - 180;
        return ((a + delta * t) + 360) % 360;
    }
    var topHue = lerpAngle(hLH, hHH, i);
    var botHue = lerpAngle(hLL, hHL, i);
    var hue = lerpAngle(botHue, topHue, u);

    var isDark = getThemeType() === 'dark';
    var sat, bgL, textL;

    if (isActive) {
        sat = isDark ? 40 + dist * 30 : 50 + dist * 35;
        bgL = isDark ? 25 + dist * 10 : 85 - dist * 15;
        textL = isDark ? 90 : 15;
    } else {
        sat = isDark ? 15 + dist * 20 : 20 + dist * 30;
        bgL = isDark ? 18 + dist * 5 : 93 - dist * 5;
        textL = isDark ? 75 + dist * 10 : 40 - dist * 10;
    }

    return {
        bg: 'hsl(' + Math.round(hue) + ',' + Math.round(sat) + '%,' + Math.round(bgL) + '%)',
        text: 'hsl(' + Math.round(hue) + ',' + Math.round(sat) + '%,' + Math.round(textL) + '%)'
    };
}

function formatTime(dateStr) {
    const d = new Date(dateStr);
    const now = new Date();
    const diffMs = now - d;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return diffMins + 'm ago';
    if (diffHours < 24) return diffHours + 'h ago';
    if (diffDays < 7) return diffDays + 'd ago';

    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    const month = months[d.getMonth()];
    const day = d.getDate();
    const hours = d.getHours();
    const mins = d.getMinutes().toString().padStart(2, '0');
    const ampm = hours >= 12 ? 'PM' : 'AM';
    const h12 = hours % 12 || 12;

    if (d.getFullYear() === now.getFullYear()) {
        return month + ' ' + day + ', ' + h12 + ':' + mins + ' ' + ampm;
    }
    return month + ' ' + day + ', ' + d.getFullYear();
}

// --- Focus Layout ---
let focusMode = null; // null | 'create' | 'edit'
let focusEditId = null;
let focusInitialContent = '';
let focusInitialTags = [];

function hasFocusChanges() {
    var content = focusEditor ? focusEditor.value().trim() : document.getElementById('focus-content').value.trim();
    var tags = focusChips ? focusChips.getTags() : [];
    if (content !== focusInitialContent) return true;
    if (tags.length !== focusInitialTags.length) return true;
    for (var i = 0; i < tags.length; i++) {
        if (tags[i] !== focusInitialTags[i]) return true;
    }
    return false;
}

async function confirmCloseFocus() {
    if (!hasFocusChanges()) {
        closeFocus();
        return;
    }
    var confirmed = await showModal({ message: 'You have unsaved changes. Discard them?', confirmText: 'Discard', danger: true });
    if (confirmed) closeFocus();
}

function openFocus(mode, note) {
    focusMode = mode;
    focusEditId = note ? note.short_id : null;

    const overlay = document.getElementById('focus-overlay');
    const title = document.getElementById('focus-title');
    const submitBtn = document.getElementById('focus-submit');
    const cancelBtn = document.getElementById('focus-cancel');

    if (mode === 'edit' && note) {
        title.textContent = 'Edit note';
        submitBtn.textContent = 'Save note';
        cancelBtn.textContent = 'Cancel';
        focusChips.setTags(note.tags || []);
    } else {
        title.textContent = 'New note';
        submitBtn.textContent = 'Save note';
        cancelBtn.textContent = 'Clear';
        focusChips.clearTags();
    }

    overlay.style.display = '';
    trapFocus(overlay);

    // Init EasyMDE lazily
    if (!focusEditor) {
        const el = document.getElementById('focus-content');
        if (el && typeof EasyMDE !== 'undefined') {
            focusEditor = createEasyMDE(el, { autofocus: true, forcePlain: true });
        }
    }

    // Set content
    if (mode === 'edit' && note) {
        if (focusEditor) {
            focusEditor.value(note.content);
        } else {
            document.getElementById('focus-content').value = note.content;
        }
    } else {
        if (focusEditor) {
            focusEditor.value('');
        } else {
            document.getElementById('focus-content').value = '';
        }
    }

    // Focus the editor
    if (focusEditor) {
        setTimeout(() => focusEditor.codemirror.focus(), 100);
    }

    // Update priority preview
    updateFocusPriorityPreview();

    // Save initial state for dirty check
    setTimeout(function() {
        focusInitialContent = focusEditor ? focusEditor.value().trim() : document.getElementById('focus-content').value.trim();
        focusInitialTags = focusChips ? focusChips.getTags().slice() : [];
    }, 150);
}

function updateFocusPriorityPreview() {
    const tags = focusChips ? focusChips.getTags() : [];
    const color = getPriorityColor(tags);

    let preview = document.getElementById('focus-priority-preview');
    if (!preview) return;

    if (color) {
        preview.style.background = color.bg;
        preview.style.color = color.text;
        preview.style.border = '1px solid ' + color.border;
        preview.textContent = color.label;
    } else {
        preview.style.background = 'var(--tag-bg)';
        preview.style.color = 'var(--text-muted)';
        preview.style.border = '1px solid transparent';
        preview.textContent = 'Normal';
    }
}

function closeFocus() {
    const overlay = document.getElementById('focus-overlay');
    overlay.style.display = 'none';
    focusMode = null;
    focusEditId = null;
    releaseFocus();
}

// --- Read Overlay ---
let readNote = null;

function openRead(note) {
    readNote = note;
    const overlay = document.getElementById('read-overlay');
    const title = document.getElementById('read-title');
    const content = document.getElementById('read-content');
    const tags = document.getElementById('read-tags');

    title.textContent = formatTime(note.created_at) + (note.updated_at ? ' (edited)' : '');
    content.innerHTML = renderMarkdown(note.content);

    tags.innerHTML = '';
    for (const t of (note.tags || [])) {
        const pill = document.createElement('span');
        pill.className = 'tag';
        pill.textContent = '#' + t;
        tags.appendChild(pill);
    }

    overlay.style.display = '';
    trapFocus(overlay);
}

function closeRead() {
    document.getElementById('read-overlay').style.display = 'none';
    readNote = null;
    releaseFocus();
}

document.getElementById('read-close').addEventListener('click', closeRead);
document.getElementById('read-edit').addEventListener('click', () => {
    if (readNote) {
        const note = readNote;
        const focusOverlay = document.getElementById('focus-overlay');
        focusOverlay.style.animation = 'none';
        openFocus('edit', note);
        closeRead();
        requestAnimationFrame(() => { focusOverlay.style.animation = ''; });
    }
});

// New note trigger (sidebar)
document.getElementById('sidebar-new-note').addEventListener('click', () => {
    openFocus('create');
    closeMobileSidebar();
});

// Focus close buttons
document.getElementById('focus-close').addEventListener('click', confirmCloseFocus);
document.getElementById('focus-cancel').addEventListener('click', () => {
    if (focusMode === 'edit') {
        confirmCloseFocus();
    } else {
        // In create mode, clear inputs but keep overlay open
        if (focusEditor) {
            focusEditor.value('');
        } else {
            document.getElementById('focus-content').value = '';
        }
        focusChips.clearTags();
    }
});

// Focus submit
document.getElementById('focus-submit').addEventListener('click', async () => {
    const content = focusEditor ? focusEditor.value().trim() : document.getElementById('focus-content').value.trim();
    focusChips.commitInput();
    const tags = focusChips.getTags();
    if (!content || tags.length === 0) {
        showToast('Content and at least one tag are required.', 'error');
        return;
    }
    const submitBtn = document.getElementById('focus-submit');
    submitBtn.classList.add('btn-loading');
    try {
        if (focusMode === 'edit' && focusEditId) {
            await api('PUT', '/notes/' + encodeURIComponent(focusEditId), { content, tags });
        } else {
            await api('POST', '/notes', { content, tags });
        }
        closeFocus();
        refresh();
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    } finally {
        submitBtn.classList.remove('btn-loading');
    }
});

// Escape key closes focus or read overlay, Ctrl/Cmd+N opens new note
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        if (document.getElementById('import-overlay').style.display !== 'none') {
            closeImportPreview();
        } else if (readNote) closeRead();
        else if (focusMode) confirmCloseFocus();
    }
    if ((e.metaKey || e.ctrlKey) && e.key === 'n') {
        if (!focusMode && !readNote && isLoggedIn()) {
            e.preventDefault();
            openFocus('create');
        }
    }
});

// --- Feed Rendering ---
async function loadFeed(append) {
    const feed = document.getElementById('feed');
    if (!append) {
        feedPage = 0;
        feedHasMore = true;
        lastNotes = [];
        feed.innerHTML = '<div class="loading-spinner"></div>';
    }
    if (feedLoading) return;
    feedLoading = true;
    try {
        await refreshTagCache();
        const notes = await api('GET', '/notes?' + buildFilterQuery(feedPage));
        if (notes.length < FEED_PAGE_SIZE) {
            feedHasMore = false;
        }
        if (append) {
            lastNotes = lastNotes.concat(notes);
        } else {
            lastNotes = notes;
        }
        renderFeed(lastNotes);
    } catch (e) {
        if (!append) {
            feed.innerHTML = '<p class="error">' + esc(e.message) + '</p>';
        }
    } finally {
        feedLoading = false;
    }
}

function getMasonryColumnCount() {
    const main = document.querySelector('.main-content');
    const style = getComputedStyle(main);
    const width = main.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
    const minCol = 280;
    const gap = 24; // 1.5rem
    return Math.max(1, Math.floor((width + gap) / (minCol + gap)));
}

function renderFeed(notes) {
    lastNotes = notes || [];
    const feed = document.getElementById('feed');

    if (!notes || notes.length === 0) {
        feed.innerHTML = '<div class="feed-empty">No notes yet. Click "New note" to get started.</div>';
        return;
    }

    const colCount = getMasonryColumnCount();
    feed.innerHTML = '';

    const columns = [];
    const colHeights = [];
    for (let i = 0; i < colCount; i++) {
        const col = document.createElement('div');
        col.className = 'feed-column';
        feed.appendChild(col);
        columns.push(col);
        colHeights.push(0);
    }

    for (const note of notes) {
        // Find the shortest column
        let shortest = 0;
        for (let i = 1; i < colCount; i++) {
            if (colHeights[i] < colHeights[shortest]) shortest = i;
        }
        const card = createNoteCard(note);
        columns[shortest].appendChild(card);
        // Estimate height: use a rough heuristic based on content length
        // Actual height will settle after render, but this distributes evenly enough
        const contentLen = (note.content || '').length;
        const estimatedHeight = Math.min(contentLen * 0.5, 300) + 80;
        colHeights[shortest] += estimatedHeight;
    }

    // Add infinite scroll sentinel
    if (feedHasMore) {
        const sentinel = document.createElement('div');
        sentinel.className = 'feed-sentinel';
        sentinel.id = 'feed-sentinel';
        feed.appendChild(sentinel);
        observeFeedSentinel();
    }
}

function createNoteCard(note) {
    const card = document.createElement('div');
    card.className = 'note-card' + (note.pinned ? ' pinned' : '');
    const color = getPriorityColor(note.tags);
    if (color) {
        card.style.background = color.bg;
        card.style.borderLeft = '3px solid ' + color.border;
        card.style.setProperty('--card-bg', color.bg);
    }
    card.dataset.id = note.short_id;

    // Header
    const header = document.createElement('div');
    header.className = 'note-card-header';

    const time = document.createElement('span');
    time.className = 'note-card-time';
    time.textContent = formatTime(note.created_at);
    if (note.updated_at) {
        time.title = 'Edited: ' + new Date(note.updated_at).toLocaleString();
        time.textContent += ' (edited)';
    }

    const actions = document.createElement('div');
    actions.className = 'note-card-actions';

    const editBtn = document.createElement('button');
    editBtn.className = 'action-edit';
    editBtn.title = 'Edit';
    editBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>';
    editBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        openFocus('edit', note);
    });

    const deleteBtn = document.createElement('button');
    deleteBtn.className = 'action-delete';
    deleteBtn.title = 'Delete';
    deleteBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>';
    deleteBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        try {
            await api('DELETE', '/notes/' + note.short_id);
            lastNotes = lastNotes.filter(function(n) { return n.short_id !== note.short_id; });
            renderFeed(lastNotes);
            updateTrashBadge();
            showUndoToast('Note deleted', async function() {
                try {
                    await api('PUT', '/notes/' + note.short_id + '/restore');
                    lastNotes.push(note);
                    lastNotes.sort(function(a, b) {
                        if (a.pinned !== b.pinned) return b.pinned ? 1 : -1;
                        return new Date(b.created_at) - new Date(a.created_at);
                    });
                    renderFeed(lastNotes);
                    updateTrashBadge();
                } catch (err) {
                    showToast('Could not restore: ' + err.message, 'error');
                }
            });
        } catch (err) {
            showToast('Error: ' + err.message, 'error');
        }
    });

    const expandBtn = document.createElement('button');
    expandBtn.className = 'action-expand';
    expandBtn.title = 'Full screen';
    expandBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>';
    expandBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        openRead(note);
    });

    const pinBtn = document.createElement('button');
    pinBtn.className = 'note-pin-btn' + (note.pinned ? ' pinned' : '');
    pinBtn.title = note.pinned ? 'Unpin' : 'Pin';
    pinBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="' + (note.pinned ? 'currentColor' : 'none') + '" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2l2.09 6.26L21 9.27l-5 4.87L17.18 21 12 17.77 6.82 21 8 14.14l-5-4.87 6.91-1.01z"/></svg>';
    pinBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        try {
            await api('PUT', '/notes/' + note.short_id + '/pin');
            var idx = lastNotes.findIndex(function(n) { return n.short_id === note.short_id; });
            if (idx !== -1) {
                lastNotes[idx].pinned = !lastNotes[idx].pinned;
                lastNotes.sort(function(a, b) {
                    if (a.pinned !== b.pinned) return b.pinned ? 1 : -1;
                    return new Date(b.created_at) - new Date(a.created_at);
                });
            }
            renderFeed(lastNotes);
        } catch (err) {
            showToast('Error: ' + err.message, 'error');
        }
    });

    actions.appendChild(expandBtn);
    actions.appendChild(editBtn);
    actions.appendChild(deleteBtn);
    header.appendChild(time);
    header.appendChild(actions);

    // Body
    const body = document.createElement('div');
    body.className = 'note-card-body';
    const content = document.createElement('div');
    content.className = 'markdown';

    let rendered = renderMarkdown(note.content);
    if (note.snippet && currentQuery) {
        rendered = highlightText(rendered, currentQuery);
    }
    content.innerHTML = rendered;
    body.appendChild(content);

    // Pin + Tags row (at top of card, before header and body)
    const topRow = document.createElement('div');
    topRow.className = 'note-card-top';
    topRow.appendChild(pinBtn);

    const tagsDiv = document.createElement('div');
    tagsDiv.className = 'note-card-tags';
    for (const t of (note.tags || [])) {
        const pill = document.createElement('span');
        pill.className = 'tag clickable';
        pill.textContent = '#' + t;
        const isActive = currentTags.includes(t);
        if (isActive) {
            pill.classList.add('active');
        }
        const pillColor = getTagPillColor(t, isActive);
        if (pillColor) {
            pill.style.background = pillColor.bg;
            pill.style.color = pillColor.text;
        }
        pill.addEventListener('click', () => {
            filterByTag(t);
        });
        tagsDiv.appendChild(pill);
    }
    topRow.appendChild(tagsDiv);
    card.appendChild(topRow);

    card.appendChild(header);
    card.appendChild(body);

    // Collapse long content after it's in the DOM
    requestAnimationFrame(() => {
        if (content.scrollHeight > 300) {
            body.classList.add('collapsed');
            const readMore = document.createElement('button');
            readMore.className = 'read-more-btn';
            readMore.textContent = 'Read more';
            readMore.addEventListener('click', () => {
                const collapsed = body.classList.toggle('collapsed');
                readMore.textContent = collapsed ? 'Read more' : 'Read less';
            });
            card.appendChild(readMore);
        }
    });

    return card;
}

// --- Filtering ---
function filterByTag(tag) {
    if (currentTags.includes(tag)) {
        currentTags = currentTags.filter(t => t !== tag);
    } else {
        currentTags.push(tag);
    }
    if (filterChips) filterChips.setTags(currentTags);
    updateTagCloudActive();
    refresh();
}

function updateTagCloudActive() {
    document.querySelectorAll('#tag-cloud .tag').forEach(el => {
        const tag = el.dataset.tag;
        const isActive = currentTags.includes(tag);
        if (isActive) {
            el.classList.add('active');
        } else {
            el.classList.remove('active');
        }
        const color = getTagPillColor(tag, isActive);
        if (color) {
            el.style.background = color.bg;
            el.style.color = color.text;
        } else {
            el.style.background = '';
            el.style.color = '';
        }
    });
}

function refresh() {
    loadFeed();
    loadTags();
    updateBadge();
}

// --- Tags ---
async function loadTags(limit) {
    if (limit === undefined) limit = 100;
    try {
        if (Object.keys(tagCache).length === 0) {
            await refreshTagCache();
        }
        const url = limit > 0 ? '/tags?limit=' + limit : '/tags';
        const tags = await api('GET', url);
        const cloud = document.getElementById('tag-cloud');
        let html = tags.map(t => {
            const isActive = currentTags.includes(t);
            const activeClass = isActive ? ' active' : '';
            const color = getTagPillColor(t, isActive);
            let style = '';
            if (color) {
                style = ' style="background:' + color.bg + ';color:' + color.text + '"';
            }
            return '<span class="tag clickable' + activeClass + '" data-tag="' + esc(t) + '"' + style + '>#' + esc(t) + '</span>';
        }).join('');
        if (limit > 0 && tags.length >= limit) {
            html += ' <button class="show-all-tags" id="show-all-tags-btn">Show all</button>';
        }
        cloud.innerHTML = html;
        cloud.querySelectorAll('.clickable').forEach(el => {
            el.addEventListener('click', () => {
                filterByTag(el.dataset.tag);
            });
        });
        const showAllBtn = document.getElementById('show-all-tags-btn');
        if (showAllBtn) {
            showAllBtn.addEventListener('click', (e) => {
                e.preventDefault();
                loadTags(0);
            });
        }
    } catch (e) {
        // Ignore errors on initial load
    }
}

// --- Tag cloud search ---
document.getElementById('tag-cloud-search').addEventListener('input', function() {
    const q = this.value.trim().toLowerCase();
    document.querySelectorAll('#tag-cloud .tag').forEach(function(el) {
        var tag = el.dataset.tag || '';
        el.style.display = (!q || tag.indexOf(q) !== -1) ? '' : 'none';
    });
    var showAllBtn = document.getElementById('show-all-tags-btn');
    if (showAllBtn) showAllBtn.style.display = q ? 'none' : '';
});

// --- Filter events (reactive, instant) ---
let filterDebounce = null;
function triggerFilter() {
    clearTimeout(filterDebounce);
    filterDebounce = setTimeout(() => {
        currentTags = filterChips ? filterChips.getTags() : [];
        currentQuery = document.getElementById('search-input').value.trim();
        updateTagCloudActive();
        loadFeed();
    }, 250);
}

document.getElementById('search-input').addEventListener('input', triggerFilter);

document.getElementById('sort-select').addEventListener('change', (e) => {
    currentSort = e.target.value;
    loadFeed();
});

document.getElementById('search-input').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        clearTimeout(filterDebounce);
        currentTags = filterChips ? filterChips.getTags() : [];
        currentQuery = document.getElementById('search-input').value.trim();
        updateTagCloudActive();
        loadFeed();
    }
});

// --- Tag autocomplete ---
function attachTagAutocomplete(inputEl, chipApiGetter) {
    let debounceTimer = null;
    let listEl = null;
    let activeIndex = -1;

    function getChipApi() {
        return chipApiGetter ? chipApiGetter() : null;
    }

    function getWrapper() {
        return inputEl.closest('.autocomplete-wrapper');
    }

    function closeList() {
        if (listEl) { listEl.remove(); listEl = null; }
        activeIndex = -1;
    }

    function getCurrentFragment() {
        if (getChipApi()) {
            // In chip mode, entire input value is the fragment
            return inputEl.value.trim().toLowerCase();
        }
        // Legacy comma-separated mode
        const val = inputEl.value;
        const cursor = inputEl.selectionStart;
        const before = val.slice(0, cursor);
        const lastComma = before.lastIndexOf(',');
        return before.slice(lastComma + 1).trim().toLowerCase();
    }

    function replaceCurrentFragment(tag) {
        var ca = getChipApi();
        if (ca) {
            ca.createChip(tag);
            inputEl.value = '';
            closeList();
            return;
        }
        // Legacy comma-separated mode
        const val = inputEl.value;
        const cursor = inputEl.selectionStart;
        const before = val.slice(0, cursor);
        const after = val.slice(cursor);
        const lastComma = before.lastIndexOf(',');
        const prefix = lastComma >= 0 ? before.slice(0, lastComma + 1) + ' ' : '';
        const afterComma = after.indexOf(',');
        const suffix = afterComma >= 0 ? after.slice(afterComma) : '';
        inputEl.value = prefix + tag + ', ' + suffix.replace(/^,\s*/, '');
        inputEl.focus();
        closeList();
    }

    function updateActive() {
        if (!listEl) return;
        const items = listEl.querySelectorAll('.autocomplete-item');
        items.forEach(function(it, i) {
            it.classList.toggle('active', i === activeIndex);
        });
        if (activeIndex >= 0 && items[activeIndex]) {
            items[activeIndex].scrollIntoView({ block: 'nearest' });
        }
    }

    inputEl.addEventListener('input', () => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(async () => {
            const fragment = getCurrentFragment();
            if (fragment.length < 1) { closeList(); return; }
            try {
                let tags = await api('GET', '/tags/autocomplete?q=' + encodeURIComponent(fragment) + '&limit=8');
                // Filter out already-selected chips
                var ca = getChipApi();
                if (ca) {
                    const existing = ca.getTags();
                    tags = tags.filter(function(t) { return existing.indexOf(t) === -1; });
                }
                if (tags.length === 0) { closeList(); return; }
                closeList();
                listEl = document.createElement('div');
                listEl.className = 'autocomplete-list';
                for (const t of tags) {
                    const item = document.createElement('div');
                    item.className = 'autocomplete-item';
                    item.dataset.tag = t;

                    // Color dot from priority color
                    const info = tagCache[t];
                    if (info) {
                        const color = getPriorityColor([t]);
                        if (color) {
                            const dot = document.createElement('span');
                            dot.className = 'autocomplete-dot';
                            dot.style.background = color.border;
                            item.appendChild(dot);
                        }
                    }

                    // Tag name
                    const nameSpan = document.createElement('span');
                    nameSpan.textContent = t;
                    item.appendChild(nameSpan);

                    // I/U scores
                    if (info) {
                        const meta = document.createElement('span');
                        meta.className = 'autocomplete-meta';
                        meta.textContent = 'I:' + info.importance + ' U:' + info.urgency;
                        item.appendChild(meta);
                    }

                    item.addEventListener('mousedown', (e) => {
                        e.preventDefault();
                        replaceCurrentFragment(t);
                    });
                    listEl.appendChild(item);
                }
                getWrapper().appendChild(listEl);
            } catch (e) {
                closeList();
            }
        }, 150);
    });

    inputEl.addEventListener('blur', () => {
        setTimeout(closeList, 200);
    });

    inputEl.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            if (listEl) {
                e.stopPropagation(); // prevent focus overlay from closing
                closeList();
                return;
            }
        }

        if (!listEl) return;
        const items = listEl.querySelectorAll('.autocomplete-item');
        if (items.length === 0) return;

        if (e.key === 'ArrowDown') {
            e.preventDefault();
            activeIndex = Math.min(activeIndex + 1, items.length - 1);
            updateActive();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            activeIndex = Math.max(activeIndex - 1, -1);
            updateActive();
        } else if (e.key === 'Enter' && activeIndex >= 0) {
            e.preventDefault();
            e.stopImmediatePropagation(); // prevent chip keydown from also firing
            const selectedTag = items[activeIndex].dataset.tag;
            replaceCurrentFragment(selectedTag);
        }
    });
}

attachTagAutocomplete(document.getElementById('focus-tag-input'), function() { return focusChips; });
attachTagAutocomplete(document.getElementById('filter-tags'), function() { return filterChips; });

// Initialize chip inputs (must be after attachTagAutocomplete for correct keydown order)
focusChips = initChipInput(
    document.getElementById('focus-chip-container'),
    document.getElementById('focus-tag-input')
);

filterChips = initChipInput(
    document.getElementById('filter-chip-container'),
    document.getElementById('filter-tags'),
    {
        filterOnly: true,
        onChange: function() {
            clearTimeout(filterDebounce);
            currentTags = filterChips.getTags();
            currentQuery = document.getElementById('search-input').value.trim();
            updateTagCloudActive();
            loadFeed();
        }
    }
);

// --- Tab switching ---
async function switchToTab(tabName) {
    if (focusMode && hasFocusChanges()) {
        var confirmed = await showModal({ message: 'You have unsaved changes. Discard them?', confirmText: 'Discard', danger: true });
        if (!confirmed) return;
    }
    closeFocus();
    document.querySelectorAll('.nav-item').forEach(b => {
        b.classList.toggle('active', b.dataset.tab === tabName);
    });
    document.getElementById('notes-tab').style.display = tabName === 'notes' ? '' : 'none';
    document.getElementById('tags-tab').style.display = tabName === 'tags' ? '' : 'none';
    document.getElementById('trash-tab').style.display = tabName === 'trash' ? '' : 'none';
    document.getElementById('sidebar-filters').style.display = (tabName === 'notes' && !isGuestMode()) ? '' : 'none';
    if (tabName === 'tags') loadTagsManagement();
    if (tabName === 'trash') loadTrash();
}

document.querySelectorAll('.nav-item[data-tab]').forEach(btn => {
    btn.addEventListener('click', () => {
        switchToTab(btn.dataset.tab);
        closeMobileSidebar();
    });
});

// --- Tag management ---
async function loadTagsManagement(sortByPriority) {
    const el = document.getElementById('tags-management');
    el.innerHTML = '<div class="loading-spinner"></div>';
    try {
        const tags = await api('GET', '/tags/detailed');
        if (tags.length === 0) {
            el.innerHTML = '<p class="placeholder">No tags yet. Tags are created automatically when you add them to notes.</p>';
            return;
        }
        if (sortByPriority) {
            tags.sort((a, b) => (b.importance + b.urgency) - (a.importance + a.urgency));
        }

        // Sort button with arrow icon
        const arrowIcon = sortByPriority
            ? '<svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 4l4 4 4-4"/></svg>'
            : '<svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 4l4 4 4-4"/></svg>';
        let html = '<div class="tag-mgmt-toolbar">';
        html += '<button class="sort-priority-btn' + (sortByPriority ? ' active' : '') + '" id="sort-priority-btn">Sort by Priority' + arrowIcon + '</button>';
        const unreviewedCount = tags.filter(function(t) { return t.status === 'unreviewed'; }).length;
        if (unreviewedCount > 0) {
            html += ' <button class="btn btn-secondary btn-sm" id="approve-all-btn">Approve all (' + unreviewedCount + ')</button>';
        }
        html += '</div>';

        // Table header - 6 columns: Tag, I/U Values, Importance & Urgency sliders, Status, Notes, Actions
        html += '<div class="tag-mgmt-header">'
            + '<span class="tag-mgmt-header-cell">Tag</span>'
            + '<span class="tag-mgmt-header-cell">I / U</span>'
            + '<span class="tag-mgmt-header-cell">Importance & Urgency</span>'
            + '<span class="tag-mgmt-header-cell">Status</span>'
            + '<span class="tag-mgmt-header-cell">Notes</span>'
            + '<span class="tag-mgmt-header-cell">Actions</span>'
            + '</div>';

        html += '<div class="tags-management-list">';
        for (const t of tags) {
            // Compute color preview using getPriorityColor
            const tempCacheEntry = { importance: t.importance, urgency: t.urgency, status: t.status };
            const originalCacheEntry = tagCache[t.name];
            tagCache[t.name] = tempCacheEntry;
            const colorInfo = getPriorityColor([t.name]);
            if (originalCacheEntry) tagCache[t.name] = originalCacheEntry;
            else delete tagCache[t.name];

            // Style for the tag pill (same as sidebar)
            const tagStyle = colorInfo
                ? 'background:' + colorInfo.bg + ';border-color:' + colorInfo.border + ';color:' + colorInfo.text
                : '';

            // Format priority values with padding
            const iVal = String(t.importance).padStart(2, ' ');
            const uVal = String(t.urgency).padStart(2, ' ');

            html += '<div class="tag-mgmt-row" data-tag="' + esc(t.name) + '">'
                + '<span class="tag-mgmt-name"><span class="tag"' + (tagStyle ? ' style="' + tagStyle + '"' : '') + '>#' + esc(t.name) + '</span></span>'
                + '<span class="tag-priority-values">I:' + iVal + ' U:' + uVal + '</span>'
                + '<div class="tag-priority-sliders">'
                + '<label>I<input type="range" min="0" max="100" value="' + t.importance + '" class="importance-slider" data-name="' + esc(t.name) + '"></label>'
                + '<label>U<input type="range" min="0" max="100" value="' + t.urgency + '" class="urgency-slider" data-name="' + esc(t.name) + '"></label>'
                + '</div>'
                + '<span class="tag-status ' + esc(t.status) + '">' + esc(t.status) + '</span>'
                + '<span class="tag-mgmt-count">' + t.note_count + '</span>'
                + '<div class="tag-mgmt-actions">';

            // Icon-based action buttons
            if (t.status === 'unreviewed') {
                html += '<button class="tag-action-btn approve-btn" data-name="' + esc(t.name) + '" title="Approve" aria-label="Approve tag">'
                    + '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 8l3 3 7-7"/></svg>'
                    + '</button>';
            }
            html += '<button class="tag-action-btn rename-btn" data-name="' + esc(t.name) + '" title="Rename" aria-label="Rename tag">'
                + '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><path d="M11.5 2.5l2 2M3 13l-1 1 2 0 8-8-2-2-8 8 0 2z"/></svg>'
                + '</button>';
            html += '<button class="tag-action-btn tag-del-btn" data-name="' + esc(t.name) + '" title="Delete" aria-label="Delete tag">'
                + '<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 4l8 8M12 4l-8 8"/></svg>'
                + '</button>';
            html += '</div></div>';
        }
        html += '</div>';
        el.innerHTML = html;

        // Sort by priority button
        document.getElementById('sort-priority-btn').addEventListener('click', () => {
            loadTagsManagement(!sortByPriority);
        });

        // Approve all button
        const approveAllBtn = document.getElementById('approve-all-btn');
        if (approveAllBtn) {
            approveAllBtn.addEventListener('click', async () => {
                try {
                    await api('PUT', '/tags/approve-all');
                    loadTagsManagement(sortByPriority);
                    updateBadge();
                    showToast('All tags approved', 'success');
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

// Slider handlers
        el.querySelectorAll('.importance-slider, .urgency-slider').forEach(slider => {
            let saveTimer = null;
            const row = slider.closest('.tag-mgmt-row');
            const valuesEl = row.querySelector('.tag-priority-values');
            const tagPillEl = row.querySelector('.tag-mgmt-name .tag');
            const tagName = slider.dataset.name;

slider.addEventListener('input', () => {
                const iSlider = row.querySelector('.importance-slider');
                const uSlider = row.querySelector('.urgency-slider');
                const importance = parseInt(iSlider.value);
                const urgency = parseInt(uSlider.value);

                // Update values display with padding
                const iVal = String(importance).padStart(2, ' ');
                const uVal = String(urgency).padStart(2, ' ');
                valuesEl.textContent = 'I:' + iVal + ' U:' + uVal;

                // Temporarily update tagCache for color calculation
                const origEntry = tagCache[tagName];
                tagCache[tagName] = { importance: importance, urgency: urgency, status: origEntry ? origEntry.status : 'approved' };
                const colorInfo = getPriorityColor([tagName]);
                if (origEntry) tagCache[tagName] = origEntry;
                else delete tagCache[tagName];

                // Update tag pill style in real-time
                if (colorInfo) {
                    tagPillEl.style.background = colorInfo.bg;
                    tagPillEl.style.borderColor = colorInfo.border;
                    tagPillEl.style.color = colorInfo.text;
                } else {
                    tagPillEl.style.background = '';
                    tagPillEl.style.borderColor = '';
                    tagPillEl.style.color = '';
                }
            });

            slider.addEventListener('change', () => {
                clearTimeout(saveTimer);
                saveTimer = setTimeout(async () => {
                    const iSlider = row.querySelector('.importance-slider');
                    const uSlider = row.querySelector('.urgency-slider');
                    const name = slider.dataset.name;
                    const importance = parseInt(iSlider.value);
                    const urgency = parseInt(uSlider.value);
                    try {
                        await api('PUT', '/tags/' + encodeURIComponent(name) + '/priority', {
                            importance: importance,
                            urgency: urgency
                        });
                        if (tagCache[name]) {
                            tagCache[name].importance = importance;
                            tagCache[name].urgency = urgency;
                        }
                    } catch (e) {
                        showToast('Error saving priority: ' + e.message, 'error');
                    }
                }, 300);
            });
        });

        el.querySelectorAll('.approve-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                await api('PUT', '/tags/' + encodeURIComponent(btn.dataset.name) + '/approve');
                loadTagsManagement(sortByPriority);
                updateBadge();
            });
        });

        el.querySelectorAll('.rename-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const newName = await showModal({ message: 'Rename "' + btn.dataset.name + '" to:', prompt: true, defaultValue: btn.dataset.name, confirmText: 'Rename' });
                if (!newName || newName.trim() === '' || newName.trim().toLowerCase() === btn.dataset.name) return;
                try {
                    await api('PUT', '/tags/' + encodeURIComponent(btn.dataset.name) + '/rename',
                        { new_name: newName.trim() });
                    loadTagsManagement(sortByPriority);
                    loadTags();
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        });

        el.querySelectorAll('.tag-del-btn').forEach(btn => {
            btn.addEventListener('click', async () => {
                const confirmed = await showModal({ message: 'Delete tag "' + btn.dataset.name + '"? Notes will not be deleted.', confirmText: 'Delete', danger: true });
                if (!confirmed) return;
                try {
                    await api('DELETE', '/tags/' + encodeURIComponent(btn.dataset.name));
                    loadTagsManagement(sortByPriority);
                    loadTags();
                    updateBadge();
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        });
    } catch (e) {
        el.innerHTML = '<p class="error">' + esc(e.message) + '</p>';
    }
}

// --- Unreviewed badge ---
async function updateBadge() {
    try {
        const tags = await refreshTagCache();
        const count = tags.filter(t => t.status === 'unreviewed').length;
        const badge = document.getElementById('unreviewed-badge');
        if (count > 0) {
            badge.textContent = count;
            badge.style.display = '';
        } else {
            badge.style.display = 'none';
        }
    } catch (e) {
        // Ignore
    }
    updateTrashBadge();
}

async function updateTrashBadge() {
    try {
        var notes = await api('GET', '/notes/trash');
        var badge = document.getElementById('trash-badge');
        if (notes && notes.length > 0) {
            badge.textContent = notes.length;
            badge.style.display = '';
        } else {
            badge.style.display = 'none';
        }
    } catch (e) {
        // Ignore
    }
}

async function loadTrash() {
    var el = document.getElementById('trash-list');
    el.innerHTML = '<div class="loading-spinner"></div>';
    try {
        var notes = await api('GET', '/notes/trash');
        if (!notes || notes.length === 0) {
            el.innerHTML = '<p class="placeholder">Trash is empty.</p>';
            return;
        }
        renderTrashFeed(el, notes);
    } catch (e) {
        el.innerHTML = '<p class="error">' + esc(e.message) + '</p>';
    }
}

function renderTrashFeed(container, notes) {
    var colCount = getMasonryColumnCount();
    container.innerHTML = '';

    var columns = [];
    var colHeights = [];
    for (var i = 0; i < colCount; i++) {
        var col = document.createElement('div');
        col.className = 'feed-column';
        container.appendChild(col);
        columns.push(col);
        colHeights.push(0);
    }

    for (var j = 0; j < notes.length; j++) {
        var shortest = 0;
        for (var k = 1; k < colCount; k++) {
            if (colHeights[k] < colHeights[shortest]) shortest = k;
        }
        var card = createTrashCard(notes[j]);
        columns[shortest].appendChild(card);
        var contentLen = (notes[j].content || '').length;
        colHeights[shortest] += Math.min(contentLen * 0.5, 300) + 80;
    }
}

function createTrashCard(note) {
    var card = document.createElement('div');
    card.className = 'note-card trash-card';

    // Tags
    var tagsDiv = document.createElement('div');
    tagsDiv.className = 'note-card-tags';
    for (var i = 0; i < (note.tags || []).length; i++) {
        var pill = document.createElement('span');
        pill.className = 'tag';
        pill.textContent = '#' + note.tags[i];
        tagsDiv.appendChild(pill);
    }
    if (tagsDiv.children.length > 0) {
        card.appendChild(tagsDiv);
    }

    // Header
    var header = document.createElement('div');
    header.className = 'note-card-header';
    var time = document.createElement('span');
    time.className = 'note-card-time';
    time.textContent = formatTime(note.created_at);
    var actions = document.createElement('div');
    actions.className = 'trash-card-actions';

    var restoreBtn = document.createElement('button');
    restoreBtn.className = 'btn btn-secondary btn-sm';
    restoreBtn.textContent = 'Restore';
    restoreBtn.addEventListener('click', async function() {
        try {
            await api('PUT', '/notes/' + encodeURIComponent(note.short_id) + '/restore');
            showToast('Note restored', 'success');
            loadTrash();
            updateTrashBadge();
            refresh();
        } catch (e) {
            showToast('Error: ' + e.message, 'error');
        }
    });

    var purgeBtn = document.createElement('button');
    purgeBtn.className = 'btn btn-danger btn-sm';
    purgeBtn.textContent = 'Delete';
    purgeBtn.addEventListener('click', async function() {
        var confirmed = await showModal({ message: 'Permanently delete this note? This cannot be undone.', confirmText: 'Delete forever', danger: true });
        if (!confirmed) return;
        try {
            await api('DELETE', '/notes/' + encodeURIComponent(note.short_id) + '/permanent');
            showToast('Note permanently deleted', 'success');
            loadTrash();
            updateTrashBadge();
        } catch (e) {
            showToast('Error: ' + e.message, 'error');
        }
    });

    actions.appendChild(restoreBtn);
    actions.appendChild(purgeBtn);
    header.appendChild(time);
    header.appendChild(actions);
    card.appendChild(header);

    // Body
    var body = document.createElement('div');
    body.className = 'note-card-body';
    var content = document.createElement('div');
    content.className = 'markdown';
    content.innerHTML = renderMarkdown(note.content);
    body.appendChild(content);
    card.appendChild(body);

    // Collapse long content
    requestAnimationFrame(function() {
        if (content.scrollHeight > 300) {
            body.classList.add('collapsed');
            var readMore = document.createElement('button');
            readMore.className = 'read-more-btn';
            readMore.textContent = 'Read more';
            readMore.addEventListener('click', function() {
                var collapsed = body.classList.toggle('collapsed');
                readMore.textContent = collapsed ? 'Read more' : 'Read less';
            });
            card.appendChild(readMore);
        }
    });

    return card;
}

// --- Infinite scroll ---
let feedObserver = null;
function observeFeedSentinel() {
    if (feedObserver) feedObserver.disconnect();
    const sentinel = document.getElementById('feed-sentinel');
    if (!sentinel) return;
    feedObserver = new IntersectionObserver(function(entries) {
        if (entries[0].isIntersecting && feedHasMore && !feedLoading) {
            feedPage++;
            loadFeed(true);
        }
    }, { rootMargin: '200px' });
    feedObserver.observe(sentinel);
}

// --- Mobile sidebar ---
function closeMobileSidebar() {
    document.getElementById('sidebar').classList.remove('open');
    const overlay = document.getElementById('sidebar-overlay');
    if (overlay) overlay.classList.remove('open');
}

(function initMobileMenu() {
    const overlay = document.createElement('div');
    overlay.className = 'sidebar-overlay';
    overlay.id = 'sidebar-overlay';
    overlay.addEventListener('click', closeMobileSidebar);
    document.body.appendChild(overlay);

    const menuBtn = document.getElementById('mobile-menu-btn');
    if (menuBtn) {
        menuBtn.addEventListener('click', () => {
            document.getElementById('sidebar').classList.toggle('open');
            overlay.classList.toggle('open');
        });
    }
})();

// --- Init ---
initTheme();
initGoogleSignIn();
initGuestModeHandlers();

// --- Guest Note Migration ---
async function migrateGuestNotes() {
    // Check if we were converting from guest mode
    var wasGuest = sessionStorage.getItem('guest_converting') === 'true' ||
                   localStorage.getItem(GUEST_ACTIVE_KEY) === 'true';
    if (!wasGuest) return;

    var notes = guestGetAllNotes();
    if (notes.length === 0) {
        exitGuestMode();
        sessionStorage.removeItem('guest_converting');
        return;
    }

    // Format notes for import API
    var importNotes = notes.map(function(n) {
        return {
            content: n.content,
            tags: n.tags || [],
            pinned: n.pinned || false,
            created_at: n.created_at,
            updated_at: n.updated_at
        };
    });

    try {
        var opts = {
            method: 'POST',
            headers: {
                'Authorization': 'Bearer ' + getToken(),
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ notes: importNotes })
        };
        var resp = await fetch(API + '/notes/import', opts);
        if (resp.ok) {
            exitGuestMode();
            sessionStorage.removeItem('guest_converting');
            showToast('Your notes have been saved to your account!', 'success');
        } else {
            showToast('Could not import guest notes. They remain in your browser.', 'error');
        }
    } catch (e) {
        showToast('Could not import guest notes. They remain in your browser.', 'error');
    }
}

// Handle URL params for auth (verify/reset tokens)
const authParamsHandled = handleAuthUrlParams();

let resizeTimer;
window.addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => {
        if (lastNotes.length > 0) renderFeed(lastNotes);
    }, 200);
});

// --- Warn on window close with unsaved changes ---
window.addEventListener('beforeunload', function(e) {
    if (focusMode && hasFocusChanges()) {
        e.preventDefault();
        e.returnValue = '';
    }
});

if (!authParamsHandled) {
    if (isLoggedIn()) {
        showAppContent();
        loadTags();
        updateBadge();
        loadFeed();
    } else if (isGuestMode() || new URLSearchParams(location.search).has('guest')) {
        enterGuestMode();
        showAppContent();
        loadTags();
        updateBadge();
        loadFeed();
        showGuestBanner();
        updateGuestUI();
    } else {
        showAuthView();
    }
}

// Guest mode button handler
document.getElementById('guest-mode-btn').addEventListener('click', function() {
    enterGuestMode();
    showAppContent();
    loadTags();
    updateBadge();
    loadFeed();
    showGuestBanner();
    updateGuestUI();
});

function updateGuestUI() {
    if (isGuestMode()) {
        document.getElementById('user-display').textContent = 'Guest';
        document.getElementById('export-btn').style.display = 'none';
        document.getElementById('import-btn').style.display = 'none';
        document.getElementById('sidebar-filters').style.display = 'none';
    }
}

// --- PWA (only on /app) ---
if (window.location.pathname === '/app' || window.location.pathname.startsWith('/app/')) {
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/sw.js', { scope: '/app' }).catch(() => {});
    }

    let deferredInstallPrompt = null;
    window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferredInstallPrompt = e;
        showInstallHint();
        // Show sidebar install button
        var sidebarInstall = document.getElementById('sidebar-install');
        if (sidebarInstall) sidebarInstall.style.display = '';
    });

    // Sidebar install button
    document.getElementById('sidebar-install-btn').addEventListener('click', async () => {
        if (deferredInstallPrompt) {
            deferredInstallPrompt.prompt();
            await deferredInstallPrompt.userChoice;
            deferredInstallPrompt = null;
        }
        var sidebarInstall = document.getElementById('sidebar-install');
        if (sidebarInstall) sidebarInstall.style.display = 'none';
        var hint = document.getElementById('install-hint');
        if (hint) hint.remove();
    });

    function showInstallHint() {
        if (document.getElementById('install-hint')) return;
        const hint = document.createElement('div');
        hint.id = 'install-hint';
        hint.className = 'install-hint';
        hint.innerHTML = '<div class="install-hint-icon"><svg width="36" height="36" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="var(--bg-on-accent)"/><path d="M8 10.5C8 9.67 8.67 9 9.5 9H17.59c.4 0 .78.16 1.06.44l5.91 5.91a1.5 1.5 0 010 2.12l-6.21 6.21a1.5 1.5 0 01-2.12 0l-5.91-5.91A1.5 1.5 0 019.88 17H9.5A1.5 1.5 0 018 15.5V10.5z" fill="none" stroke="var(--accent)" stroke-width="1.5"/><circle cx="12.5" cy="13" r="1.5" fill="var(--accent)"/></svg></div>'
            + '<div class="install-hint-text"><strong>Install TagNote</strong><span class="install-hint-sub">Add to your home screen for quick access</span></div>'
            + '<div class="install-hint-actions"><button id="install-btn" class="btn btn-sm">Install</button><button id="install-dismiss" class="btn btn-ghost btn-sm install-dismiss">Dismiss</button></div>';
        const main = document.querySelector('.main-content');
        if (main) {
            main.prepend(hint);
        } else {
            document.body.prepend(hint);
        }

        document.getElementById('install-btn').addEventListener('click', async () => {
            if (deferredInstallPrompt) {
                deferredInstallPrompt.prompt();
                await deferredInstallPrompt.userChoice;
                deferredInstallPrompt = null;
            }
            hint.remove();
            var si = document.getElementById('sidebar-install');
            if (si) si.style.display = 'none';
        });

        document.getElementById('install-dismiss').addEventListener('click', () => {
            hint.remove();
        });
    }
}
