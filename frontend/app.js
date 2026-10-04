// Clipboard-Gnome Frontend Controller

let uiMode = localStorage.getItem('uiMode') || 'list';
let previewTimeout = null;
let historyItems = [];
let selectedIndex = 0;
let activeFilter = 'all'; // 'all', 'pinned', 'text', 'image'
let searchQuery = '';
let isIncognito = false;
let currentPreviewId = null;

// DOM Elements
const searchInput = document.getElementById('search-input');
const btnClearSearch = document.getElementById('btn-clear-search');
const itemList = document.getElementById('item-list');
const itemCards = document.getElementById('item-cards');
const emptyState = document.getElementById('empty-state');
const btnIncognito = document.getElementById('btn-incognito');
const incognitoBanner = document.getElementById('incognito-banner');
const btnClear = document.getElementById('btn-clear');
const btnClose = document.getElementById('btn-close');
const filterPills = document.querySelectorAll('.filter-pill');
const toast = document.getElementById('toast');
const toastMessage = document.getElementById('toast-message');

const confirmModal = document.getElementById('confirm-modal');
const btnModalCancel = document.getElementById('btn-modal-cancel');
const btnModalConfirm = document.getElementById('btn-modal-confirm');

const btnSettings = document.getElementById('btn-settings');
const settingsModal = document.getElementById('settings-modal');
const btnSettingsCancel = document.getElementById('btn-settings-cancel');
const btnSettingsSave = document.getElementById('btn-settings-save');
const inputRetention = document.getElementById('set-retention');
const inputMaxSize = document.getElementById('set-max-size');
const inputKeybind = document.getElementById('set-keybind');
const inputDualTone = document.getElementById('set-dual-tone');
const inputThemeColor = document.getElementById('set-theme-color');
const inputRounding = document.getElementById('set-rounding');
const inputUIMode = document.getElementById('set-ui-mode');
const previewPopup = document.getElementById('preview-popup');
const previewType = document.getElementById('preview-type');
const previewTime = document.getElementById('preview-time');
const previewContent = document.getElementById('preview-content');
const btnPreviewDelete = document.getElementById('btn-preview-delete');
const listContainer = document.querySelector('.list-container');

document.addEventListener('DOMContentLoaded', () => {
    initApp();
    
    inputDualTone.addEventListener('change', () => {
        updateThemeDropdown(inputThemeColor.value);
    });
});

async function initApp() {
    setupEventListeners();
    setupWailsEventListeners();
    await checkIncognitoState();
    await applySavedTheme();
    applyUIMode();
    await loadHistory();
    focusSearch();
}

function applyUIMode() {
    if (uiMode === 'list') {
        listContainer.classList.add('list-mode');
        itemList.classList.add('list-mode');
        itemList.classList.remove('hidden');
        itemCards.classList.add('hidden');
    } else {
        listContainer.classList.remove('list-mode');
        itemList.classList.remove('list-mode');
        itemList.classList.add('hidden');
        itemCards.classList.remove('hidden');
        hidePreview();
    }
}

async function applySavedTheme() {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            const s = await window.go.main.App.GetSettings();
            if (s) {
                applyThemeToDOM(s);
            }
        }
    } catch (e) {
        console.error("Error loading theme on startup:", e);
    }
}

const singleToneThemes = {
    midnight:    { bg: 'rgba(15, 23, 42, 0.94)',  card: 'rgba(15, 23, 42, 0.6)',  solidCard: '#1e293b', popupHeader: '#0f172a', accent: '#6366f1', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(15, 23, 42, 0.2)' },
    pitch:       { bg: 'rgba(0, 0, 0, 0.94)',     card: 'rgba(0, 0, 0, 0.6)',     solidCard: '#121212', popupHeader: '#1a1a1a', accent: '#3b82f6', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(0, 0, 0, 0.2)' },
    charcoal:    { bg: 'rgba(28, 25, 23, 0.94)',  card: 'rgba(28, 25, 23, 0.6)',  solidCard: '#292524', popupHeader: '#1c1917', accent: '#f59e0b', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(28, 25, 23, 0.2)' },
    'light-cream': { bg: 'rgba(253, 251, 247, 0.94)', card: 'rgba(255, 255, 255, 0.7)', solidCard: '#ffffff', popupHeader: '#f1ece1', accent: '#6366f1', text: '#1e293b', textSec: '#475569', overlay: 'rgba(0, 0, 0, 0.05)' },
    'light-beige': { bg: 'rgba(245, 245, 240, 0.94)', card: 'rgba(250, 250, 248, 0.7)', solidCard: '#ffffff', popupHeader: '#e8e8e0', accent: '#d97706', text: '#333333', textSec: '#5c5c5c', overlay: 'rgba(0, 0, 0, 0.05)' }
};

const dualToneThemes = {
    tokyo:       { bg: 'rgba(26, 27, 38, 0.94)',  card: 'rgba(36, 40, 59, 0.7)',  solidCard: '#24283b', popupHeader: '#16161e', accent: '#7aa2f7', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(15, 23, 42, 0.2)' },
    dracula:     { bg: 'rgba(40, 42, 54, 0.94)',  card: 'rgba(68, 71, 90, 0.7)',  solidCard: '#44475a', popupHeader: '#21222c', accent: '#bd93f9', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(15, 23, 42, 0.2)' },
    nord:        { bg: 'rgba(46, 52, 64, 0.94)',  card: 'rgba(59, 66, 82, 0.7)',  solidCard: '#3b4252', popupHeader: '#272c36', accent: '#88c0d0', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(15, 23, 42, 0.2)' },
    slate:       { bg: 'rgba(15, 23, 42, 0.94)',  card: 'rgba(30, 41, 59, 0.7)',  solidCard: '#1e293b', popupHeader: '#0f172a', accent: '#6366f1', text: '#f8fafc', textSec: '#94a3b8', overlay: 'rgba(15, 23, 42, 0.2)' },
    'light-cream': { bg: 'rgba(240, 235, 225, 0.94)', card: 'rgba(253, 251, 247, 0.7)', solidCard: '#ffffff', popupHeader: '#e5dfd3', accent: '#6366f1', text: '#1e293b', textSec: '#475569', overlay: 'rgba(0, 0, 0, 0.08)' },
    'light-beige': { bg: 'rgba(235, 230, 220, 0.94)', card: 'rgba(245, 245, 240, 0.7)', solidCard: '#ffffff', popupHeader: '#dfd9cd', accent: '#d97706', text: '#333333', textSec: '#5c5c5c', overlay: 'rgba(0, 0, 0, 0.08)' }
};

function updateThemeDropdown(selectedValue) {
    inputThemeColor.innerHTML = '';
    const themes = inputDualTone.checked ? dualToneThemes : singleToneThemes;
    
    let found = false;
    for (const key of Object.keys(themes)) {
        const opt = document.createElement('option');
        opt.value = key;
        opt.textContent = key.charAt(0).toUpperCase() + key.slice(1);
        inputThemeColor.appendChild(opt);
        if (key === selectedValue) found = true;
    }
    
    if (found) {
        inputThemeColor.value = selectedValue;
    } else {
        inputThemeColor.selectedIndex = 0;
    }
}

function applyThemeToDOM(s) {
    const root = document.documentElement;
    
    // Apply Border Radius
    root.style.setProperty('--radius-md', `${s.border_radius}px`);
    root.style.setProperty('--radius-sm', `${Math.max(2, s.border_radius - 4)}px`);
    root.style.setProperty('--radius-lg', `${s.border_radius + 4}px`);

    const themes = s.is_dual_tone ? dualToneThemes : singleToneThemes;
    const t = themes[s.theme_color] || Object.values(themes)[0];
    
    root.style.setProperty('--bg-main', t.bg);
    root.style.setProperty('--bg-card', t.card);
    root.style.setProperty('--bg-solid', t.solidCard || '#1e293b');
    root.style.setProperty('--bg-popup-header', t.popupHeader || '#0f172a');
    root.style.setProperty('--accent-indigo', t.accent);
    root.style.setProperty('--text-primary', t.text);
    root.style.setProperty('--text-secondary', t.textSec);
    root.style.setProperty('--text-muted', t.textSec);
    
    // Header/app background sync
    const appContainer = document.querySelector('.app-container');
    const header = document.querySelector('.header');
    const footer = document.querySelector('.footer');
    
    if (appContainer) appContainer.style.background = t.bg;
    if (header) header.style.background = s.is_dual_tone ? t.overlay : 'transparent';
    if (footer) footer.style.background = s.is_dual_tone ? t.overlay : 'transparent';
}

function setupEventListeners() {
    // Search input
    searchInput.addEventListener('input', (e) => {
        searchQuery = e.target.value;
        if (searchQuery.length > 0) {
            btnClearSearch.classList.remove('hidden');
        } else {
            btnClearSearch.classList.add('hidden');
        }
        loadHistory();
    });

    btnClearSearch.addEventListener('click', () => {
        searchInput.value = '';
        searchQuery = '';
        btnClearSearch.classList.add('hidden');
        loadHistory();
        focusSearch();
    });

    // Category Filter Pills
    filterPills.forEach(pill => {
        pill.addEventListener('click', () => {
            filterPills.forEach(p => p.classList.remove('active'));
            pill.classList.add('active');
            activeFilter = pill.dataset.filter;
            loadHistory();
        });
    });

    // Header Action Buttons
    btnIncognito.addEventListener('click', toggleIncognito);
    btnClear.addEventListener('click', clearHistory);
    btnSettings.addEventListener('click', openSettings);
    btnClose.addEventListener('click', () => {
        if (window.go && window.go.main && window.go.main.App) {
            window.go.main.App.HideWindow();
        }
    });

    // Modal Actions
    btnModalCancel.addEventListener('click', () => {
        confirmModal.classList.add('hidden');
    });

    btnModalConfirm.addEventListener('click', async () => {
        confirmModal.classList.add('hidden');
        try {
            if (window.go && window.go.main && window.go.main.App) {
                await window.go.main.App.ClearHistory();
                showToast("History cleared");
                await loadHistory();
            }
        } catch (e) {
            console.error("Clear history error:", e);
        }
    });

    // Settings Modal
    btnSettingsCancel.addEventListener('click', () => {
        settingsModal.classList.add('hidden');
    });

    btnSettingsSave.addEventListener('click', async () => {
        settingsModal.classList.add('hidden');
        try {
            if (window.go && window.go.main && window.go.main.App) {
                const s = {
                    retention_days: parseInt(inputRetention.value) || 30,
                    max_item_size_mb: parseInt(inputMaxSize.value) || 10,
                    keybinding: inputKeybind.value || "<Super>c",
                    is_dual_tone: inputDualTone.checked,
                    theme_color: inputThemeColor.value || "indigo",
                    border_radius: parseInt(inputRounding.value) || 10
                };
                
                uiMode = inputUIMode.value;
                localStorage.setItem('uiMode', uiMode);
                applyUIMode();

                await window.go.main.App.SaveSettings(s);
                applyThemeToDOM(s);
                showToast("Settings saved");
                renderItems();
            }
        } catch (e) {
            console.error("Save settings error:", e);
        }
    });

    // Event Delegation for list items
    listContainer.addEventListener('click', (e) => {
        const card = e.target.closest('.item-card');
        if (!card) return;
        const id = parseInt(card.dataset.id);
        const actionBtn = e.target.closest('[data-action]');
        
        if (actionBtn) {
            const action = actionBtn.dataset.action;
            if (action === 'pin') togglePinItem(id);
            else if (action === 'delete') deleteItem(id);
            e.stopPropagation();
            return;
        }
        selectAndPaste(id);
    });

    listContainer.addEventListener('mouseover', (e) => {
        const card = e.target.closest('.item-card');
        if (card && uiMode === 'list') {
            if (!e.relatedTarget || !card.contains(e.relatedTarget)) {
                const id = parseInt(card.dataset.id);
                const item = historyItems.find(i => i.id === id);
                if (item) {
                    hidePreview();
                    clearTimeout(hideTimeout);
                    clearTimeout(previewTimeout);
                    previewTimeout = setTimeout(() => showPreview(item, card), 800);
                }
            } else {
                clearTimeout(hideTimeout);
            }
        }
    });

    listContainer.addEventListener('mouseout', (e) => {
        const card = e.target.closest('.item-card');
        if (card && uiMode === 'list') {
            if (!e.relatedTarget || !card.contains(e.relatedTarget)) {
                clearTimeout(previewTimeout);
                hideTimeout = setTimeout(() => hidePreview(), 50);
            }
        }
    });

    // Keyboard Shortcuts
    document.addEventListener('keydown', handleGlobalKeydown);

    // Auto-hide window when focus is lost to another application
    window.addEventListener('blur', () => {
        hidePreview();
        if (window.go && window.go.main && window.go.main.App) {
            window.go.main.App.HideWindow();
        }
    });
}

function setupWailsEventListeners() {
    if (window.runtime) {
        window.runtime.EventsOn("clipboard:changed", (newItem) => {
            showToast("New item copied");
            loadHistory();
        });

        window.runtime.EventsOn("ui:focus_search", () => {
            focusSearch();
            loadHistory();
        });
    }
}

function focusSearch() {
    setTimeout(() => {
        searchInput.focus();
        searchInput.select();
    }, 50);
}

async function checkIncognitoState() {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            isIncognito = await window.go.main.App.IsIncognito();
            updateIncognitoUI();
        }
    } catch (e) {
        console.error("Error checking incognito state:", e);
    }
}

async function toggleIncognito() {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            isIncognito = await window.go.main.App.ToggleIncognito();
            updateIncognitoUI();
            showToast(isIncognito ? "Incognito Mode Enabled" : "Incognito Mode Disabled");
        }
    } catch (e) {
        console.error("Error toggling incognito:", e);
    }
}

function updateIncognitoUI() {
    const offIcon = btnIncognito.querySelector('.incognito-off-icon');
    const onIcon = btnIncognito.querySelector('.incognito-on-icon');

    if (isIncognito) {
        offIcon.classList.add('hidden');
        onIcon.classList.remove('hidden');
        btnIncognito.classList.add('active');
        incognitoBanner.classList.remove('hidden');
    } else {
        offIcon.classList.remove('hidden');
        onIcon.classList.add('hidden');
        btnIncognito.classList.remove('active');
        incognitoBanner.classList.add('hidden');
    }
}

async function loadHistory(targetItemId = null) {
    try {
        let pinnedOnly = (activeFilter === 'pinned');
        let tagFilter = '';
        
        if (window.go && window.go.main && window.go.main.App) {
            historyItems = await window.go.main.App.GetHistory(searchQuery, tagFilter, pinnedOnly, 100, 0) || [];
        } else {
            // Demo fallback data if running outside Wails
            historyItems = [
                { id: 1, type: "text", content: "git commit -m 'Implement technical spec'", created_at: "2026-08-09 06:10:00", pinned: true, tags: ["git"] },
                { id: 2, type: "text", content: "https://wails.io/docs/gettingstarted/installation", created_at: "2026-08-09 06:05:00", pinned: false, tags: ["url"] },
                { id: 3, type: "text", content: "export PATH=$PATH:/home/surya/go/bin", created_at: "2026-08-09 05:45:00", pinned: false, tags: ["bash"] }
            ];
        }

        // Apply type filters if needed
        if (activeFilter === 'text') {
            historyItems = historyItems.filter(i => i.type === 'text');
        } else if (activeFilter === 'image') {
            historyItems = historyItems.filter(i => i.type === 'image');
        }

        if (targetItemId !== null) {
            let idx = historyItems.findIndex(i => i.id === targetItemId);
            if (idx !== -1) {
                selectedIndex = idx;
            } else {
                let firstUnpinned = historyItems.findIndex(i => !i.pinned);
                selectedIndex = firstUnpinned !== -1 ? firstUnpinned : 0;
            }
        } else {
            // Default: highlight the first unpinned item (most recently copied item)
            let firstUnpinned = historyItems.findIndex(i => !i.pinned);
            selectedIndex = firstUnpinned !== -1 ? firstUnpinned : 0;
        }

        renderItems();
    } catch (e) {
        console.error("Error loading history:", e);
    }
}

function renderItems() {
    if (historyItems.length === 0) {
        itemList.innerHTML = '';
        itemCards.innerHTML = '';
        emptyState.classList.remove('hidden');
        return;
    }

    emptyState.classList.add('hidden');

    const existingListCards = Array.from(itemList.querySelectorAll('.item-card'));
    const listCardsMap = new Map(existingListCards.map(c => [c.dataset.id, c]));
    const listFragment = document.createDocumentFragment();

    const existingCardsCards = Array.from(itemCards.querySelectorAll('.item-card'));
    const cardsCardsMap = new Map(existingCardsCards.map(c => [c.dataset.id, c]));
    const cardsFragment = document.createDocumentFragment();

    historyItems.forEach((item, index) => {
        const isCode = isCodeSnippet(item.content);
        const timeAgo = formatTimeAgo(item.created_at);

        // List Mode Card
        let listCard = listCardsMap.get(item.id.toString());
        if (!listCard) {
            listCard = document.createElement('div');
            listCard.dataset.id = item.id;
        }
        listCard.className = `item-card ${index === selectedIndex ? 'selected' : ''}`;
        listCard.dataset.index = index;

        let listCardInner = `
            <div class="list-item-icon">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    ${item.type === 'text' 
                        ? '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline>' 
                        : '<rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline>'}
                </svg>
            </div>
            ${item.type === 'image' 
                ? `<img src="${item.content}" class="item-image-preview" alt="Image">`
                : `<div class="item-content">${escapeHtml(item.content)}</div>`
            }
            <div class="list-item-actions">
                <button class="action-btn star-btn ${item.pinned ? 'pinned' : ''}" title="${item.pinned ? 'Unpin' : 'Pin'}" data-action="pin">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="${item.pinned ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="2">
                        <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon>
                    </svg>
                </button>
            </div>
        `;
        listCard.innerHTML = listCardInner;
        listFragment.appendChild(listCard);
        listCardsMap.delete(item.id.toString());

        // Cards Mode Card
        let cardCard = cardsCardsMap.get(item.id.toString());
        if (!cardCard) {
            cardCard = document.createElement('div');
            cardCard.dataset.id = item.id;
        }
        cardCard.className = `item-card ${index === selectedIndex ? 'selected' : ''}`;
        cardCard.dataset.index = index;

        let cardCardInner = `
            <div class="item-header">
                <div class="item-meta">
                    <span class="badge ${item.type === 'image' ? 'badge-image' : 'badge-text'}">${item.type}</span>
                    <span class="item-time">${timeAgo}</span>
                </div>
                <div class="item-actions">
                    <button class="action-btn star-btn ${item.pinned ? 'pinned' : ''}" title="${item.pinned ? 'Unpin' : 'Pin'}" data-action="pin">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="${item.pinned ? 'currentColor' : 'none'}" stroke="currentColor" stroke-width="2">
                            <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon>
                        </svg>
                    </button>
                    <button class="action-btn" title="Delete" data-action="delete">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <line x1="18" y1="6" x2="6" y2="18"></line>
                            <line x1="6" y1="6" x2="18" y2="18"></line>
                        </svg>
                    </button>
                </div>
            </div>
            
            ${item.type === 'image' 
                ? `<img src="${item.content}" class="item-image-preview" alt="Clipboard Image">`
                : `<div class="item-content ${isCode ? 'code-snippet' : ''}">${escapeHtml(item.content)}</div>`
            }

            ${item.tags && item.tags.length > 0 ? `
                <div class="item-tags">
                    ${item.tags.map(t => `<span class="tag-pill">#${escapeHtml(t)}</span>`).join('')}
                </div>
            ` : ''}
        `;
        cardCard.innerHTML = cardCardInner;
        cardsFragment.appendChild(cardCard);
        cardsCardsMap.delete(item.id.toString());
    });

    listCardsMap.forEach(card => card.remove());
    cardsCardsMap.forEach(card => card.remove());

    itemList.appendChild(listFragment);
    itemCards.appendChild(cardsFragment);

    scrollToSelected();
    if (uiMode === 'list' && historyItems[selectedIndex]) {
        hidePreview();
        clearTimeout(hideTimeout);
        clearTimeout(previewTimeout);
        previewTimeout = setTimeout(() => showPreview(historyItems[selectedIndex]), 800);
    }
}

let hideTimeout = null;

function setupPopupInteraction() {
    if (previewPopup && !previewPopup.dataset.eventsBound) {
        previewPopup.addEventListener('mouseenter', () => {
            clearTimeout(hideTimeout);
        });
        previewPopup.addEventListener('mouseleave', () => {
            hideTimeout = setTimeout(() => hidePreview(), 50);
        });
        const btnPreviewCopy = document.getElementById('btn-preview-copy');
        const btnPreviewPin = document.getElementById('btn-preview-pin');
        const btnPreviewDelete = document.getElementById('btn-preview-delete');

        if (btnPreviewCopy) {
            btnPreviewCopy.addEventListener('click', (e) => {
                e.stopPropagation();
                if (currentPreviewId !== null) {
                    selectAndPaste(currentPreviewId);
                    hidePreview();
                }
            });
        }
        if (btnPreviewPin) {
            btnPreviewPin.addEventListener('click', (e) => {
                e.stopPropagation();
                if (currentPreviewId !== null) {
                    togglePinItem(currentPreviewId);
                }
            });
        }
        if (btnPreviewDelete) {
            btnPreviewDelete.addEventListener('click', (e) => {
                e.stopPropagation();
                if (currentPreviewId !== null) {
                    deleteItem(currentPreviewId);
                    hidePreview();
                }
            });
        }
        previewPopup.dataset.eventsBound = "true";
    }
}

function showPreview(item, cardElement) {
    if (!cardElement) {
        cardElement = document.querySelector(`.item-card[data-id="${item.id}"]`);
    }

    currentPreviewId = item.id;
    document.querySelectorAll('.item-card.preview-active').forEach(c => c.classList.remove('preview-active'));
    if (cardElement) {
        cardElement.classList.add('preview-active');
    }

    previewType.textContent = item.type;
    previewType.className = `badge ${item.type === 'image' ? 'badge-image' : 'badge-text'}`;
    previewTime.textContent = formatTimeAgo(item.created_at);

    const btnPreviewPin = document.getElementById('btn-preview-pin');
    if (btnPreviewPin) {
        if (item.pinned) {
            btnPreviewPin.classList.add('active');
            btnPreviewPin.title = "Unpin item";
        } else {
            btnPreviewPin.classList.remove('active');
            btnPreviewPin.title = "Pin item";
        }
    }
    
    if (item.type === 'image') {
        previewContent.innerHTML = `<img src="${item.content}" alt="Preview">`;
    } else {
        const isCode = isCodeSnippet(item.content);
        previewContent.innerHTML = isCode 
            ? `<pre><code>${escapeHtml(item.content)}</code></pre>` 
            : escapeHtml(item.content);
    }
    
    previewPopup.classList.add('visible');
    previewPopup.classList.remove('hidden');

    setupPopupInteraction();

    if (cardElement) {
        previewPopup.style.maxHeight = '300px';
        const cardRect = cardElement.getBoundingClientRect();
        const popupRect = previewPopup.getBoundingClientRect();
        const bodyHeight = document.body.clientHeight;

        let top = cardRect.bottom;
        let availableBelow = bodyHeight - 10 - top;
        let availableAbove = cardRect.top - 60;

        if (availableBelow >= popupRect.height) {
            previewPopup.style.top = `${top}px`;
        } else if (availableAbove >= popupRect.height) {
            top = cardRect.top - popupRect.height;
            previewPopup.style.top = `${top}px`;
        } else {
            // Doesn't fit, pick side with most space and shrink
            if (availableBelow > availableAbove) {
                previewPopup.style.top = `${top}px`;
                previewPopup.style.maxHeight = `${availableBelow}px`;
            } else {
                previewPopup.style.maxHeight = `${availableAbove}px`;
                top = cardRect.top - availableAbove;
                previewPopup.style.top = `${top}px`;
            }
        }

        previewPopup.style.left = '50%';
        previewPopup.style.transform = 'translateX(-50%)';
    }
}

function hidePreview() {
    previewPopup.classList.remove('visible');
    previewPopup.classList.add('hidden');
    document.querySelectorAll('.item-card.preview-active').forEach(c => c.classList.remove('preview-active'));
}

function handleGlobalKeydown(e) {
    if (historyItems.length === 0) return;

    switch (e.key) {
        case 'ArrowDown':
            e.preventDefault();
            if (selectedIndex < historyItems.length - 1) {
                selectedIndex++;
                updateSelectionUI();
            }
            break;

        case 'ArrowUp':
            e.preventDefault();
            if (selectedIndex > 0) {
                selectedIndex--;
                updateSelectionUI();
            }
            break;

        case 'Enter':
            e.preventDefault();
            if (historyItems[selectedIndex]) {
                selectAndPaste(historyItems[selectedIndex].id);
            }
            break;

        case 'Escape':
            e.preventDefault();
            if (searchQuery.length > 0) {
                searchInput.value = '';
                searchQuery = '';
                btnClearSearch.classList.add('hidden');
                loadHistory();
            } else if (window.go && window.go.main && window.go.main.App) {
                window.go.main.App.HideWindow();
            }
            break;

        case 'Delete':
            if (document.activeElement !== searchInput && historyItems[selectedIndex]) {
                e.preventDefault();
                deleteItem(historyItems[selectedIndex].id);
            }
            break;

        case 'p':
        case 'P':
            if (document.activeElement !== searchInput && historyItems[selectedIndex]) {
                e.preventDefault();
                togglePinItem(historyItems[selectedIndex].id);
            }
            break;
    }
}

function updateSelectionUI() {
    const listCards = itemList.querySelectorAll('.item-card');
    const cardsCards = itemCards.querySelectorAll('.item-card');
    
    const updateCard = (card, idx) => {
        if (idx === selectedIndex) {
            card.classList.add('selected');
        } else {
            card.classList.remove('selected');
        }
    };
    
    listCards.forEach(updateCard);
    cardsCards.forEach(updateCard);
    
    scrollToSelected();

    if (uiMode === 'list' && historyItems[selectedIndex]) {
        hidePreview();
        clearTimeout(hideTimeout);
        clearTimeout(previewTimeout);
        previewTimeout = setTimeout(() => showPreview(historyItems[selectedIndex]), 800);
    }
}

function scrollToSelected() {
    const activeList = uiMode === 'list' ? itemList : itemCards;
    const selectedCard = activeList.querySelector('.item-card.selected');
    if (selectedCard) {
        selectedCard.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
}

async function selectAndPaste(id) {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            await window.go.main.App.TriggerPaste(id);
            showToast("Pasted to active window");
        } else {
            showToast("Pasted (Demo mode)");
        }
    } catch (e) {
        console.error("Paste error:", e);
        showToast("Error executing paste");
    }
}

async function togglePinItem(id) {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            const pinned = await window.go.main.App.TogglePin(id);
            showToast(pinned ? "Item pinned" : "Item unpinned");
            await loadHistory(id);
        }
    } catch (e) {
        console.error("Toggle pin error:", e);
    }
}

async function deleteItem(id) {
    try {
        if (window.go && window.go.main && window.go.main.App) {
            await window.go.main.App.DeleteItem(id);
            showToast("Item deleted");
            await loadHistory();
        }
    } catch (e) {
        console.error("Delete item error:", e);
    }
}

async function clearHistory() {
    hidePreview();
    confirmModal.classList.remove('hidden');
}

async function openSettings() {
    hidePreview();
    try {
        if (window.go && window.go.main && window.go.main.App) {
            const s = await window.go.main.App.GetSettings();
            if (s) {
                inputRetention.value = s.retention_days;
                inputMaxSize.value = s.max_item_size_mb;
                inputKeybind.value = s.keybinding;
                inputDualTone.checked = s.is_dual_tone;
                updateThemeDropdown(s.theme_color);
                inputThemeColor.value = s.theme_color;
                inputRounding.value = s.border_radius;
                inputUIMode.value = uiMode;
            }
        }
    } catch (e) {
        console.error("Get settings error:", e);
    }
    settingsModal.classList.remove('hidden');
}

function isCodeSnippet(str) {
    if (!str) return false;
    const codePatterns = [/function\s+/i, /const\s+/i, /let\s+/i, /var\s+/i, /import\s+/i, /export\s+/i, /def\s+/i, /class\s+/i, /package\s+/i, /func\s+/i, /SELECT\s+/i, /INSERT\s+/i, /[{};=<>]/];
    return codePatterns.some(p => p.test(str)) || str.includes('\n');
}

function formatTimeAgo(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr.replace(' ', 'T'));
    const now = new Date();
    const diffSec = Math.floor((now - date) / 1000);

    if (diffSec < 60) return 'Just now';
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
    return `${Math.floor(diffSec / 86400)}d ago`;
}

function escapeHtml(str) {
    return str
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function showToast(msg) {
    toastMessage.textContent = msg;
    toast.classList.remove('hidden');
    setTimeout(() => {
        toast.classList.add('hidden');
    }, 2000);
}
