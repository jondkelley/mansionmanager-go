// Bans — modal (per-palace) + standalone tab, with in-world page notification on unban.

// ===== Palace Bans Modal =====

let _palaceBansName = null;
let _palaceBansAll = [];      // full unfiltered list for the modal
let _palaceBansDebounce = null;

function onPalaceBansSearch() {
  clearTimeout(_palaceBansDebounce);
  _palaceBansDebounce = setTimeout(() => {
    _palaceBansDebounce = null;
    renderPalaceBansTable(_palaceBansAll);
  }, 180);
}

async function openPalaceBansModal(name) {
  _palaceBansName = name;
  _palaceBansAll = [];
  $('palaceBansModalTitle').textContent = `Banlist — ${name}`;
  $('palaceBansCount').textContent = '';
  $('palaceBansSearch').value = '';
  $('palaceBansTbody').innerHTML = '<tr><td colspan="7" class="empty">Loading…</td></tr>';
  setPalaceBansAlert('');
  $('palaceBansModal').classList.add('open');
  await fetchPalaceBans(name);
}

function closePalaceBansModal() {
  _palaceBansName = null;
  $('palaceBansModal').classList.remove('open');
}

async function fetchPalaceBans(name) {
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/banlist`, { headers: headers() });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      $('palaceBansTbody').innerHTML = `<tr><td colspan="7" class="empty">Error: ${esc(body.error || 'HTTP ' + res.status)}</td></tr>`;
      return;
    }
    const entries = await res.json();
    _palaceBansAll = Array.isArray(entries) ? entries : [];
    renderPalaceBansTable(_palaceBansAll);
  } catch (e) {
    $('palaceBansTbody').innerHTML = `<tr><td colspan="7" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

function renderPalaceBansTable(entries) {
  const q = ($('palaceBansSearch').value || '').toLowerCase().trim();
  const filtered = q
    ? entries.filter(e =>
        (e.id || '').toLowerCase().includes(q) ||
        (e.reason || '').toLowerCase().includes(q) ||
        (e.created_by || '').toLowerCase().includes(q) ||
        (e.line || '').toLowerCase().includes(q) ||
        (e.kind || '').toLowerCase().includes(q)
      )
    : entries;

  $('palaceBansCount').textContent = filtered.length
    ? `${filtered.length} record${filtered.length === 1 ? '' : 's'}`
    : '';

  if (filtered.length === 0) {
    $('palaceBansTbody').innerHTML = '<tr><td colspan="7" class="empty">No ban records found.</td></tr>';
    return;
  }

  $('palaceBansTbody').innerHTML = filtered.map(e => {
    const kindCls = e.kind === 'track' ? 'ban-kind-track'
                  : e.kind === 'timed-deny' ? 'ban-kind-timed'
                  : 'ban-kind-deny';
    const rowCls = e.active ? '' : ' ban-inactive';
    const id = escHtml(e.id || '');
    const lineRaw = e.line || '';
    const lineTitle = escHtml(lineRaw);
    const lineCell = escHtml(formatBanLineMultiline(lineRaw));
    return `<tr class="${rowCls}">
      <td class="ban-id" title="${id}">${id.length > 14 ? id.slice(0, 14) + '…' : id}</td>
      <td class="${kindCls}">${escHtml(e.kind || '—')}</td>
      <td>${e.active ? '✓' : '—'}</td>
      <td>${escHtml(e.created_by || '—')}</td>
      <td>${escHtml(e.reason || '—')}</td>
      <td class="ban-line" title="${lineTitle}">${lineCell}</td>
      <td><button class="btn-unban" data-id="${id}" data-ctx="modal" title="Unban this record">Unban</button></td>
    </tr>`;
  }).join('');
}

// Delegate unban clicks for both modal and standalone tab.
document.addEventListener('click', async function (ev) {
  const btn = ev.target.closest('.btn-unban');
  if (!btn) return;

  const id = btn.dataset.id;
  const ctx = btn.dataset.ctx || 'modal';
  const palace = ctx === 'modal' ? _palaceBansName : _bansPalace;
  if (!id || !palace) return;

  btn.disabled = true;
  btn.textContent = '…';

  const alertFn = ctx === 'modal' ? setPalaceBansAlert : setBansAlert;
  alertFn('');

  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palace)}/banlist/unban`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ id }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      alertFn(data.error || data.message || 'Unban failed (HTTP ' + res.status + ')', 'error');
      btn.disabled = false;
      btn.textContent = 'Unban';
      return;
    }
    alertFn(data.message || 'Record removed.', 'success');
    if (ctx === 'modal') {
      await fetchPalaceBans(palace);
    } else {
      await loadBansTab();
    }
  } catch (e) {
    alertFn('Network error: ' + e.message, 'error');
    btn.disabled = false;
    btn.textContent = 'Unban';
  }
});

function setPalaceBansAlert(msg, type) {
  const el = $('palaceBansAlert');
  if (!el) return;
  el.textContent = msg;
  el.className = 'bans-alert' + (type ? ' ' + type : '');
  if (!type) el.style.display = 'none';
}

// ===== Standalone Bans Tab =====

let _bansSearchDebounce = null;
let _bansAllEntries = [];   // full list from last fetch
let _bansPalace = '';       // currently selected palace

function onBansSearch() {
  clearTimeout(_bansSearchDebounce);
  _bansSearchDebounce = setTimeout(() => {
    _bansSearchDebounce = null;
    renderBansTable(_bansAllEntries);
  }, 200);
}

// Populate the palace dropdown from the global palace list (already loaded).
async function populateBansPalaceSelect() {
  setBansAlert('');
  try {
    const res = await fetch('/api/palaces', { headers: headers() });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const palaces = await res.json();
    const sel = $('bansPalaceSelect');
    const prev = sel.value;
    sel.innerHTML = '<option value="">— select a palace —</option>';
    (palaces || []).forEach(p => {
      const opt = document.createElement('option');
      opt.value = p.name;
      opt.textContent = p.name;
      sel.appendChild(opt);
    });
    // Restore previous selection if still present.
    if (prev && [...sel.options].some(o => o.value === prev)) {
      sel.value = prev;
    }
    if (sel.value) {
      _bansPalace = sel.value;
      await loadBansTab();
    }
  } catch (e) {
    setBansAlert('Could not load palace list: ' + e.message, 'error');
  }
}

async function loadBansTab() {
  const sel = $('bansPalaceSelect');
  const palace = sel ? sel.value : '';
  _bansPalace = palace;
  setBansAlert('');

  if (!palace) {
    $('bansLoading').style.display = 'none';
    $('bansTableWrap').style.display = 'none';
    $('bansEmpty').style.display = 'none';
    return;
  }

  $('bansLoading').style.display = 'block';
  $('bansTableWrap').style.display = 'none';
  $('bansEmpty').style.display = 'none';

  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palace)}/banlist`, { headers: headers() });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || 'HTTP ' + res.status);
    }
    const entries = await res.json();
    _bansAllEntries = Array.isArray(entries) ? entries : [];
    $('bansLoading').style.display = 'none';
    renderBansTable(_bansAllEntries);
  } catch (e) {
    $('bansLoading').style.display = 'none';
    $('bansTableWrap').style.display = 'none';
    $('bansEmpty').style.display = 'none';
    setBansAlert('Could not load banlist: ' + e.message, 'error');
  }
}

function renderBansTable(entries) {
  const q = ($('bansSearch').value || '').toLowerCase().trim();
  const filtered = q
    ? entries.filter(e =>
        (e.id || '').toLowerCase().includes(q) ||
        (e.reason || '').toLowerCase().includes(q) ||
        (e.created_by || '').toLowerCase().includes(q) ||
        (e.line || '').toLowerCase().includes(q) ||
        (e.kind || '').toLowerCase().includes(q)
      )
    : entries;

  if (filtered.length === 0) {
    $('bansTableWrap').style.display = 'none';
    $('bansEmpty').style.display = 'block';
    return;
  }

  $('bansEmpty').style.display = 'none';
  $('bansTableWrap').style.display = 'block';

  $('bansTbody').innerHTML = filtered.map(e => {
    const kindCls = e.kind === 'track' ? 'ban-kind-track'
                  : e.kind === 'timed-deny' ? 'ban-kind-timed'
                  : 'ban-kind-deny';
    const rowCls = e.active ? '' : ' ban-inactive';
    const reason = escHtml(e.reason || '—');
    const by = escHtml(e.created_by || '—');
    const lineRaw = e.line || '';
    const lineTitle = escHtml(lineRaw);
    const lineCell = escHtml(formatBanLineMultiline(lineRaw));
    const id = escHtml(e.id || '');
    return `<tr class="${rowCls}">
      <td class="ban-id" title="${id}">${id.length > 12 ? id.slice(0, 12) + '…' : id}</td>
      <td class="${kindCls}">${escHtml(e.kind || '—')}</td>
      <td>${e.active ? '✓' : '—'}</td>
      <td>${by}</td>
      <td>${reason}</td>
      <td class="ban-line" title="${lineTitle}">${lineCell}</td>
      <td><button class="btn-unban" data-id="${id}" data-ctx="tab" title="Unban this record">Unban</button></td>
    </tr>`;
  }).join('');
}

// (unban click handling is consolidated in the modal section above)

function setBansAlert(msg, type) {
  const el = $('bansAlert');
  if (!el) return;
  el.textContent = msg;
  el.className = 'bans-alert' + (type ? ' ' + type : '');
  if (!type) el.style.display = 'none';
}

function escHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// Ban list "line" is space-separated key=value pairs; break before each key for readability.
function formatBanLineMultiline(line) {
  return String(line || '').replace(/ (?=[a-z][a-z_]*=)/gi, '\n');
}
