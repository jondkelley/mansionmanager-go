// Bans tab — load banlist per palace, unban with in-world page notification.

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
    const line = escHtml(e.line || '');
    const id = escHtml(e.id || '');
    return `<tr class="${rowCls}">
      <td class="ban-id" title="${id}">${id.length > 12 ? id.slice(0, 12) + '…' : id}</td>
      <td class="${kindCls}">${escHtml(e.kind || '—')}</td>
      <td>${e.active ? '✓' : '—'}</td>
      <td>${by}</td>
      <td>${reason}</td>
      <td class="ban-line" title="${line}">${line}</td>
      <td><button class="btn-unban" data-id="${id}" title="Unban this record">Unban</button></td>
    </tr>`;
  }).join('');
}

// Delegate click on the table body for unban buttons.
document.addEventListener('click', async function (ev) {
  const btn = ev.target.closest('.btn-unban');
  if (!btn) return;
  const id = btn.dataset.id;
  if (!id || !_bansPalace) return;

  btn.disabled = true;
  btn.textContent = '…';
  setBansAlert('');

  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(_bansPalace)}/banlist/unban`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ id }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      setBansAlert(data.error || data.message || 'Unban failed (HTTP ' + res.status + ')', 'error');
      btn.disabled = false;
      btn.textContent = 'Unban';
      return;
    }
    setBansAlert(data.message || 'Record removed.', 'success');
    // Reload the list to reflect the change.
    await loadBansTab();
  } catch (e) {
    setBansAlert('Network error: ' + e.message, 'error');
    btn.disabled = false;
    btn.textContent = 'Unban';
  }
});

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
