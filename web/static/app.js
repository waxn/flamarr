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
  0:'☀️',1:'🌤',2:'⛅',3:'☁️',
  45:'🌫',48:'🌫',
  51:'🌦',53:'🌦',55:'🌦',56:'🌦',57:'🌦',
  61:'🌧',63:'🌧',65:'🌧',66:'🌧',67:'🌧',
  71:'🌨',73:'🌨',75:'🌨',77:'🌨',
  80:'🌧',81:'🌧',82:'🌧',
  85:'❄️',86:'❄️',
  95:'⛈',96:'⛈',99:'⛈',
};

function wmoEmoji(code) {
  return WMO[code] || '🌡';
}

async function fetchWeather(lat, lon) {
  const url = `https://api.open-meteo.com/v1/forecast?latitude=${lat}&longitude=${lon}&current=temperature_2m,weather_code&temperature_unit=celsius&forecast_days=1`;
  const res = await fetch(url);
  if (!res.ok) return null;
  const data = await res.json();
  return data.current;
}

async function geocodeCity(city) {
  const res = await fetch(`https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(city)}&count=1`);
  if (!res.ok) return null;
  const data = await res.json();
  return data.results?.[0] ?? null;
}

function renderWeather(city, temp, code) {
  const el = document.getElementById('weatherDisplay');
  if (!el) return;
  el.textContent = `${wmoEmoji(code)} ${Math.round(temp)}°C ${city}`;
}

async function loadWeather() {
  const res = await fetch('/api/settings');
  if (!res.ok) return;
  const s = await res.json();
  if (!s.weather_lat || !s.weather_lon || !s.weather_city) return;
  const current = await fetchWeather(s.weather_lat, s.weather_lon);
  if (current) renderWeather(s.weather_city, current.temperature_2m, current.weather_code);
}

function initWeather() {
  loadWeather();

  const pen    = document.getElementById('weatherPen');
  const editor = document.getElementById('weatherEditor');
  const input  = document.getElementById('weatherInput');
  const save   = document.getElementById('weatherSave');
  if (!pen || !editor || !input || !save) return;

  pen.addEventListener('click', () => {
    editor.hidden = !editor.hidden;
    if (!editor.hidden) setTimeout(() => input.focus(), 30);
  });

  const doSave = async () => {
    const city = input.value.trim();
    if (!city) return;
    save.textContent = '…';
    const result = await geocodeCity(city);
    if (!result) { save.textContent = '✗'; setTimeout(() => { save.textContent = '→'; }, 1500); return; }
    await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ weather_lat: String(result.latitude), weather_lon: String(result.longitude), weather_city: result.name }),
    });
    save.textContent = '→';
    editor.hidden = true;
    input.value = '';
    const current = await fetchWeather(result.latitude, result.longitude);
    if (current) renderWeather(result.name, current.temperature_2m, current.weather_code);
  };

  save.addEventListener('click', doSave);
  input.addEventListener('keydown', e => { if (e.key === 'Enter') doSave(); if (e.key === 'Escape') { editor.hidden = true; } });
}

// ── Event wiring ─────────────────────────────────────

function init() {
  initIcons();
  initSearch();
  initClock();
  initWeather();

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
