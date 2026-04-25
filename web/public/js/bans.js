// Bans — per-palace modal (from palace listing), with in-world page notification on unban.

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
      <td><button class="btn-unban" data-id="${id}" title="Unban this record">Unban</button></td>
    </tr>`;
  }).join('');
}

document.addEventListener('click', async function (ev) {
  const btn = ev.target.closest('.btn-unban');
  if (!btn) return;

  const id = btn.dataset.id;
  const palace = _palaceBansName;
  if (!id || !palace) return;

  btn.disabled = true;
  btn.textContent = '…';

  setPalaceBansAlert('');

  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palace)}/banlist/unban`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ id }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      setPalaceBansAlert(data.error || data.message || 'Unban failed (HTTP ' + res.status + ')', 'error');
      btn.disabled = false;
      btn.textContent = 'Unban';
      return;
    }
    setPalaceBansAlert(data.message || 'Record removed.', 'success');
    await fetchPalaceBans(palace);
  } catch (e) {
    setPalaceBansAlert('Network error: ' + e.message, 'error');
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
