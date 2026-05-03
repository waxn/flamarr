'use strict';

function initIcons() {
  // Intentionally empty: the UI is text-only now.
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
  setTimeout(() => nameInput().focus(), 50);
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
  document.getElementById('saveBtn')?.addEventListener('keydown', e => {
    if (e.key === ' ' || e.code === 'Space') {
      e.preventDefault();
    }
  });

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
    if (e.key === 'Enter' && overlay().classList.contains('open')) {
      e.preventDefault();
      saveItem();
      return;
    }
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

  });
}

document.readyState === 'loading'
  ? document.addEventListener('DOMContentLoaded', init)
  : init();
