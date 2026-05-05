'use strict';

const AVATAR_COLORS = ['#10b981','#06b6d4','#6366f1','#f59e0b','#ec4899','#8b5cf6','#14b8a6','#f97316'];

function avatarColor(name) {
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) | 0;
  return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

function initIcons() {
  document.querySelectorAll('.item-avatar[data-name]').forEach(el => {
    const name = el.dataset.name || '?';
    const icon = el.dataset.icon || '';
    const color = avatarColor(name);
    if (icon && icon.length <= 4 && !/https?:/.test(icon)) {
      el.style.background = color + '18';
      el.textContent = icon;
    } else {
      el.style.background = color + '22';
      el.style.color = color;
      el.textContent = name.charAt(0).toUpperCase();
    }
  });
}

// ── Search ───────────────────────────────────────────

function initSearch() {
  const input = document.getElementById('search');
  if (!input) return;
  input.addEventListener('input', () => {
    const q = input.value.toLowerCase().trim();
    document.querySelectorAll('.item-row').forEach(row => {
      const text = (row.dataset.name + ' ' + row.dataset.url + ' ' + (row.dataset.desc || '')).toLowerCase();
      row.style.display = (!q || text.includes(q)) ? '' : 'none';
    });
    updateCounts();
  });
}

function updateCounts() {
  const sc = document.getElementById('count-services');
  const bc = document.getElementById('count-bookmarks');
  if (sc) sc.textContent = document.querySelectorAll('#grid-services [data-id]').length;
  if (bc) bc.textContent = document.querySelectorAll('#grid-bookmarks [data-id]').length;
}

// ── Modal ────────────────────────────────────────────

let editingId = null;

const overlay      = () => document.getElementById('modalOverlay');
const modal        = () => document.getElementById('modal');
const titleEl      = () => document.getElementById('modalTitle');
const nameInput    = () => document.getElementById('mName');
const urlInput     = () => document.getElementById('mUrl');
const iconInput    = () => document.getElementById('mIcon');
const categoryInput= () => document.getElementById('mCategory');
const descInput    = () => document.getElementById('mDesc');

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

  nameInput().value     = item?.name     || '';
  urlInput().value      = item?.url      || '';
  iconInput().value     = item?.icon     || '';
  categoryInput().value = item?.category || '';
  descInput().value     = item?.desc     || '';

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
    category:    categoryInput()?.value.trim() ?? '',
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
  if (es) es.style.display = document.querySelectorAll('#grid-services [data-id]').length ? 'none' : '';
  if (eb) eb.style.display = document.querySelectorAll('#grid-bookmarks [data-id]').length ? 'none' : '';
}

// ── Greeting + clock ─────────────────────────────────

function greeting() {
  const h = new Date().getHours();
  if (h >= 5  && h < 12) return 'Good morning!';
  if (h >= 12 && h < 17) return 'Good afternoon!';
  if (h >= 17 && h < 22) return 'Good evening!';
  return 'Good night!';
}

function clockTick() {
  const el = document.getElementById('heroClock');
  if (!el) return;
  const now = new Date();
  el.textContent = '· ' + now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function initClock() {
  const g = document.getElementById('greeting');
  if (g) g.textContent = greeting();
  clockTick();
  setInterval(clockTick, 10000);
}

// ── Weather ───────────────────────────────────────────

const WMO = {
  0:'☀️', 1:'🌤', 2:'⛅', 3:'☁️',
  45:'🌫', 48:'🌫',
  51:'🌦', 53:'🌦', 55:'🌦', 56:'🌦', 57:'🌦',
  61:'🌧', 63:'🌧', 65:'🌧', 66:'🌧', 67:'🌧',
  71:'🌨', 73:'🌨', 75:'🌨', 77:'🌨',
  80:'🌧', 81:'🌧', 82:'🌧',
  85:'❄️', 86:'❄️',
  95:'⛈', 96:'⛈', 99:'⛈',
};

function wmoEmoji(code) { return WMO[code] || '🌡'; }

async function fetchWeather(lat, lon) {
  try {
    const res = await fetch(`https://api.open-meteo.com/v1/forecast?latitude=${lat}&longitude=${lon}&current=temperature_2m,weather_code&temperature_unit=fahrenheit&forecast_days=1`);
    if (!res.ok) return null;
    return (await res.json()).current;
  } catch { return null; }
}

async function searchCities(query) {
  try {
    const res = await fetch(`https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(query)}&count=6`);
    if (!res.ok) return [];
    return (await res.json()).results ?? [];
  } catch { return []; }
}

function setWeatherDisplay(city, temp, code) {
  const el = document.getElementById('weatherDisplay');
  if (!el) return;
  if (!city) {
    el.textContent = 'Select city';
    el.className = 'weather-empty';
  } else {
    el.textContent = `${wmoEmoji(code)} ${Math.round(temp)}°F  ${city}`;
    el.className = '';
  }
}

async function loadWeather() {
  try {
    const res = await fetch('/api/settings');
    if (!res.ok) return;
    const s = await res.json();
    if (!s.weather_lat || !s.weather_city) { setWeatherDisplay('', 0, 0); return; }
    const current = await fetchWeather(s.weather_lat, s.weather_lon);
    if (current) setWeatherDisplay(s.weather_city, current.temperature_2m, current.weather_code);
  } catch {}
}

let _weatherDebounce = null;

function initWeather() {
  loadWeather();

  const trigger  = document.getElementById('weatherTrigger');
  const popup    = document.getElementById('weatherPopup');
  const input    = document.getElementById('weatherInput');
  const resultEl = document.getElementById('weatherResults');
  if (!trigger || !popup || !input || !resultEl) return;

  const openPopup = () => {
    popup.hidden = false;
    setTimeout(() => input.focus(), 20);
  };
  const closePopup = () => {
    popup.hidden = true;
    input.value = '';
    resultEl.hidden = true;
    resultEl.innerHTML = '';
  };

  trigger.addEventListener('click', e => {
    e.stopPropagation();
    popup.hidden ? openPopup() : closePopup();
  });

  document.addEventListener('click', e => {
    if (!document.getElementById('weatherWidget')?.contains(e.target)) closePopup();
  });

  input.addEventListener('keydown', e => { if (e.key === 'Escape') closePopup(); });

  input.addEventListener('input', () => {
    clearTimeout(_weatherDebounce);
    const q = input.value.trim();
    if (!q) { resultEl.hidden = true; resultEl.innerHTML = ''; return; }

    _weatherDebounce = setTimeout(async () => {
      const cities = await searchCities(q);
      resultEl.innerHTML = '';
      if (!cities.length) { resultEl.hidden = true; return; }

      cities.forEach(c => {
        const li = document.createElement('li');
        li.className = 'weather-result-item';
        const name = document.createTextNode(c.name);
        const country = document.createElement('span');
        country.className = 'weather-result-country';
        country.textContent = [c.admin1, c.country].filter(Boolean).join(', ');
        li.appendChild(name);
        li.appendChild(country);
        li.addEventListener('click', async () => {
          closePopup();
          await fetch('/api/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ weather_lat: String(c.latitude), weather_lon: String(c.longitude), weather_city: c.name }),
          });
          const current = await fetchWeather(c.latitude, c.longitude);
          if (current) setWeatherDisplay(c.name, current.temperature_2m, current.weather_code);
        });
        resultEl.appendChild(li);
      });
      resultEl.hidden = false;
    }, 280);
  });
}

// ── Drag and drop ────────────────────────────────────

let _dragCard = null;
let _dragH = 0;
let _placeholder = null;
let _lastRef = undefined;

function collapseCard(card) {
  card.style.height = '0';
  card.style.minHeight = '0';
  card.style.paddingTop = '0';
  card.style.paddingBottom = '0';
  card.style.overflow = 'hidden';
  card.style.opacity = '0';
  card.style.pointerEvents = 'none';
}

function restoreCard(card) {
  card.style.height = '';
  card.style.minHeight = '';
  card.style.paddingTop = '';
  card.style.paddingBottom = '';
  card.style.overflow = '';
  card.style.opacity = '';
  card.style.pointerEvents = '';
  card.classList.remove('dragging');
}

function getPlaceholder() {
  if (!_placeholder) {
    _placeholder = document.createElement('div');
    _placeholder.className = 'drag-placeholder';
    _placeholder.setAttribute('draggable', 'false');
  }
  _placeholder.style.height = _dragH + 'px';
  return _placeholder;
}

function removePlaceholder() {
  _placeholder?.remove();
}

function insertionPoint(grid, x, y) {
  let nearest = null, nearestDist = Infinity;
  for (const card of grid.querySelectorAll('[data-id]:not(.dragging)')) {
    const r = card.getBoundingClientRect();
    if (r.height === 0) continue;
    const dist = Math.hypot(x - (r.left + r.width / 2), y - (r.top + r.height / 2));
    if (dist < nearestDist) { nearestDist = dist; nearest = card; }
  }
  if (!nearest) return { card: null, before: false };
  const r = nearest.getBoundingClientRect();
  return { card: nearest, before: x < r.left + r.width / 2 };
}

async function saveOrder() {
  const items = [];
  document.querySelectorAll('#grid-services .category-group').forEach(group => {
    const cat = group.dataset.category || '';
    group.querySelectorAll('[data-id]').forEach((el, i) => {
      items.push({ id: parseInt(el.dataset.id), position: i, type: 'service', category: cat });
    });
  });
  document.querySelectorAll('#grid-bookmarks .category-group').forEach(group => {
    const cat = group.dataset.category || '';
    group.querySelectorAll('[data-id]').forEach((el, i) => {
      items.push({ id: parseInt(el.dataset.id), position: i, type: 'bookmark', category: cat });
    });
  });
  await fetch('/api/items/reorder', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(items),
  });
}

function initDrag() {
  document.addEventListener('dragstart', e => {
    const row = e.target.closest('[data-id]');
    if (!row || e.target.closest('button')) { if (e.target.closest('button')) e.preventDefault(); return; }
    _dragCard = row;
    _dragH = row.offsetHeight;
    e.dataTransfer.effectAllowed = 'move';
    requestAnimationFrame(() => collapseCard(row));
  });

  document.addEventListener('dragend', () => {
    if (_dragCard) restoreCard(_dragCard);
    removePlaceholder();
    _dragCard = null;
    _lastRef = undefined;
  });

  // Use event delegation on each top-level grid
  for (const gridId of ['grid-services', 'grid-bookmarks']) {
    const grid = document.getElementById(gridId);
    if (!grid) continue;
    const newType = gridId === 'grid-services' ? 'service' : 'bookmark';

    grid.addEventListener('dragover', e => {
      e.preventDefault();
      if (!_dragCard) return;
      // Find the category-items container under the cursor
      const container = e.target.closest('.category-items');
      if (!container) return;
      const { card, before } = insertionPoint(container, e.clientX, e.clientY);
      const ref = card ? (before ? card : card.nextSibling) : null;
      if (ref === _lastRef && _placeholder?.parentNode === container) return;
      _lastRef = ref;
      container.insertBefore(getPlaceholder(), ref);
    });

    grid.addEventListener('drop', e => {
      e.preventDefault();
      if (!_dragCard) return;
      const ph = _placeholder;
      const container = ph?.parentNode;
      if (container?.classList.contains('category-items')) {
        container.insertBefore(_dragCard, ph);
        // update data-category to match new group
        const newCat = container.closest('.category-group')?.dataset.category || '';
        _dragCard.dataset.category = newCat;
      }
      if (_dragCard.dataset.type !== newType) {
        _dragCard.dataset.type = newType;
      }
      refreshEmpty();
      updateCounts();
      saveOrder();
    });
  }
}

// ── Event wiring ─────────────────────────────────────

function init() {
  initIcons();
  initSearch();
  initClock();
  initWeather();
  initDrag();

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

  // Item clicks: edit, delete, navigate
  document.addEventListener('click', e => {
    const editBtn = e.target.closest('.edit-btn');
    if (editBtn) {
      const card = editBtn.closest('[data-id]');
      openModal({
        id:       card.dataset.id,
        name:     card.dataset.name,
        url:      card.dataset.url,
        icon:     card.dataset.icon,
        category: card.dataset.category ?? '',
        desc:     card.dataset.desc,
        type:     card.dataset.type,
      });
      return;
    }
    const delBtn = e.target.closest('.delete-btn');
    if (delBtn) {
      const card = delBtn.closest('[data-id]');
      deleteItem(card.dataset.id);
      return;
    }
    // Navigate on row click
    const row = e.target.closest('.item-row[data-url]');
    if (row) window.open(row.dataset.url, '_blank', 'noopener');
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
