'use strict';

// ── Avatar generation ────────────────────────────────

const AVATAR_COLORS = [
  '#10b981','#06b6d4','#6366f1','#f59e0b',
  '#ec4899','#8b5cf6','#14b8a6','#f97316',
];

function avatarColor(name) {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) | 0;
  return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

function isEmoji(str) {
  // rough check: short string with no http and looks like emoji/symbol
  return str.length <= 4 && !/https?:\/\//.test(str);
}

function renderIcon(el, name, icon) {
  if (!el) return;
  const color = avatarColor(name);
  if (!icon) {
    el.style.background = color + '22';
    el.innerHTML = `<span style="color:${color};font-size:inherit;font-weight:700;line-height:1">${name.charAt(0).toUpperCase()}</span>`;
  } else if (isEmoji(icon)) {
    el.style.background = color + '18';
    el.innerHTML = `<span style="font-size:inherit;line-height:1">${icon}</span>`;
  } else {
    el.style.background = 'transparent';
    el.innerHTML = `<img src="${icon}" alt="" loading="lazy" onerror="this.parentElement.innerHTML='<span style=color:${color};font-weight:700>${name.charAt(0).toUpperCase()}</span>';this.parentElement.style.background='${color}22'" />`;
  }
}

function initIcons() {
  document.querySelectorAll('.service-icon[data-name]').forEach(el => {
    renderIcon(el, el.dataset.name, el.dataset.icon || '');
  });
  document.querySelectorAll('.bookmark-icon[data-name]').forEach(el => {
    renderIcon(el, el.dataset.name, el.dataset.icon || '');
  });
}

// ── Search ───────────────────────────────────────────

function initSearch() {
  const input = document.getElementById('search');
  if (!input) return;
  input.addEventListener('input', () => {
    const q = input.value.toLowerCase().trim();
    document.querySelectorAll('.service-card, .bookmark-card').forEach(card => {
      const text = (card.dataset.name + ' ' + card.dataset.url + ' ' + (card.dataset.desc || '')).toLowerCase();
      card.style.display = (!q || text.includes(q)) ? '' : 'none';
    });
    updateCounts();
  });
}

function updateCounts() {
  const sCount = document.querySelectorAll('.service-card:not([style*="display: none"])').length;
  const bCount = document.querySelectorAll('.bookmark-card:not([style*="display: none"])').length;
  const sc = document.getElementById('count-services');
  const bc = document.getElementById('count-bookmarks');
  if (sc) sc.textContent = sCount;
  if (bc) bc.textContent = bCount;
}

// ── Modal ────────────────────────────────────────────

let editingId = null;

const overlay   = () => document.getElementById('modalOverlay');
const modal     = () => document.getElementById('modal');
const titleEl   = () => document.getElementById('modalTitle');
const nameInput = () => document.getElementById('mName');
const urlInput  = () => document.getElementById('mUrl');
const iconInput = () => document.getElementById('mIcon');
const descInput = () => document.getElementById('mDesc');

function selectedType() {
  return document.querySelector('.type-btn.active')?.dataset.type || 'service';
}

function openModal(item = null) {
  editingId = item ? item.id : null;
  titleEl().textContent = item ? 'Edit item' : 'Add item';

  // reset type buttons
  document.querySelectorAll('.type-btn').forEach(b => b.classList.remove('active'));
  const type = item ? item.type : 'service';
  document.querySelector(`.type-btn[data-type="${type}"]`).classList.add('active');

  nameInput().value = item?.name || '';
  urlInput().value  = item?.url  || '';
  iconInput().value = item?.icon || '';
  descInput().value = item?.desc || '';

  overlay().classList.add('open');
  setupFaviconFetch();
  setTimeout(() => nameInput().focus(), 50);
}

// Try to fetch favicon from a URL and return as data URL
async function fetchFavicon(url) {
  try {
    const baseUrl = new URL(url).origin;
    const faviconUrl = baseUrl + '/favicon.ico';
    const response = await fetch(faviconUrl, { mode: 'no-cors' });
    if (response.ok) {
      const blob = await response.blob();
      return new Promise(resolve => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.readAsDataURL(blob);
      });
    }
  } catch (e) {
    // favicon fetch failed, that's ok
  }
  return null;
}

// Set up auto-fetch favicon when URL changes (for service type only)
function setupFaviconFetch() {
  urlInput().addEventListener('blur', async () => {
    if (selectedType() !== 'service') return;
    const url = urlInput().value.trim();
    if (!url || iconInput().value.trim()) return; // don't overwrite if already has icon
    const favicon = await fetchFavicon(url);
    if (favicon) {
      iconInput().value = favicon;
    }
  });
}

function closeModal() {
  overlay().classList.remove('open');
  editingId = null;
}

async function saveItem() {
  const name = nameInput().value.trim();
  let url  = urlInput().value.trim();
  if (!name || !url) {
    nameInput().focus();
    return;
  }
  // Ensure URL has a scheme so clicking opens the external site directly
  if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//.test(url)) {
    url = 'https://' + url;
  }
  const body = {
    name,
    url,
    icon:        iconInput().value.trim(),
    description: descInput().value.trim(),
    type:        selectedType(),
  };
  const res = editingId
    ? await fetch(`/api/items/${editingId}`, { method: 'PUT',    headers: {'Content-Type':'application/json'}, body: JSON.stringify(body) })
    : await fetch('/api/items',              { method: 'POST',   headers: {'Content-Type':'application/json'}, body: JSON.stringify(body) });
  if (!res.ok) return;
  closeModal();
  window.location.reload();
}

async function deleteItem(id) {
  const res = await fetch(`/api/items/${id}`, { method: 'DELETE' });
  if (!res.ok) return;
  // remove card from DOM without reload
  const card = document.querySelector(`[data-id="${id}"]`);
  if (card) {
    card.style.transition = 'opacity 0.2s, transform 0.2s';
    card.style.opacity = '0';
    card.style.transform = 'scale(0.95)';
    setTimeout(() => {
      card.remove();
      refreshEmpty();
      updateCounts();
    }, 200);
  }
}

function refreshEmpty() {
  const es = document.getElementById('empty-services');
  const eb = document.getElementById('empty-bookmarks');
  if (es) es.style.display = document.querySelectorAll('.service-card').length ? 'none' : '';
  if (eb) eb.style.display = document.querySelectorAll('.bookmark-card').length ? 'none' : '';
}

// ── Event wiring ─────────────────────────────────────

function init() {
  initIcons();
  initSearch();

  // Open button
  document.getElementById('openModal')?.addEventListener('click', () => openModal());
  document.getElementById('closeModal')?.addEventListener('click', closeModal);
  document.getElementById('saveBtn')?.addEventListener('click', saveItem);

  // Overlay click-outside
  overlay()?.addEventListener('click', e => { if (e.target === overlay()) closeModal(); });

  // Type toggle
  document.querySelectorAll('.type-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.type-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
    });
  });

  // Card edit / delete
  document.addEventListener('click', e => {
    const editBtn = e.target.closest('.edit-btn');
    if (editBtn) {
      e.preventDefault();
      const card = editBtn.closest('[data-id]');
      openModal({
        id:   card.dataset.id,
        name: card.dataset.name,
        url:  card.dataset.url,
        icon: card.dataset.icon,
        desc: card.dataset.desc,
        type: card.dataset.type,
      });
      return;
    }
    const delBtn = e.target.closest('.delete-btn');
    if (delBtn) {
      e.preventDefault();
      const card = delBtn.closest('[data-id]');
      deleteItem(card.dataset.id);
    }
  });

  // Keyboard shortcuts: S = add Service, B = add Bookmark, N/A = open (default Service)
  document.addEventListener('keydown', e => {
    if (e.key === 'Escape') { closeModal(); return; }
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    const k = e.key.toLowerCase();

    const setType = t => {
      document.querySelectorAll('.type-btn').forEach(b => b.classList.remove('active'));
      const btn = document.querySelector(`.type-btn[data-type="${t}"]`);
      if (btn) btn.classList.add('active');
    };

    if (k === 's') {
      e.preventDefault();
      openModal();
      setType('service');
      return;
    }
    if (k === 'b') {
      e.preventDefault();
      openModal();
      setType('bookmark');
      return;
    }
    if (k === 'n' || k === 'a') {
      e.preventDefault();
      openModal();
      setType('service');
      return;
    }

    if (e.key === 'Enter' && overlay().classList.contains('open')) {
      if (document.activeElement !== document.getElementById('saveBtn')) {
        e.preventDefault();
        saveItem();
      }
    }
  });
}

document.readyState === 'loading'
  ? document.addEventListener('DOMContentLoaded', init)
  : init();
