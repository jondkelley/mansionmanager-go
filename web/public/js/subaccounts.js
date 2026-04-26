// ----- Subaccounts (tenant → delegated palace RBAC) -----
let EDIT_SUBACCOUNT = null;

const SUBACCOUNT_PERM_LABELS = {
  control: 'Control (start / stop / restart)',
  logs: 'Logs',
  users: 'In-server users',
  bans: 'Bans',
  media: 'Media',
  files: 'Files (server root)',
  settings: 'Settings (prefs, serverprefs, misc, ranks, reload)',
  props: 'Props',
  pages: 'Pages',
  backups: 'Backups (snapshots & home download)',
};

function subaccountPermLabel(key) {
  return SUBACCOUNT_PERM_LABELS[key] || key;
}

function renderSubaccountPermEditors(selectedByPalace) {
  const mount = $('subaccountPermsMount');
  const palaces = (SESSION && SESSION.palaces) || [];
  const permKeys = (SESSION && SESSION.validPalacePerms) || [];
  mount.innerHTML = '';
  if (palaces.length === 0) {
    mount.innerHTML = '<p class="empty">You have no palaces assigned.</p>';
    return;
  }
  for (const palace of palaces) {
    const box = document.createElement('div');
    box.style.cssText = 'margin-bottom:14px;padding:10px;border:1px solid var(--border);border-radius:6px;';
    const h = document.createElement('div');
    h.style.cssText = 'font-weight:600;margin-bottom:8px;';
    h.textContent = palace;
    box.appendChild(h);
    const cur = (selectedByPalace && selectedByPalace[palace]) || [];
    for (const p of permKeys) {
      const lab = document.createElement('label');
      lab.style.cssText = 'display:flex;align-items:center;gap:8px;cursor:pointer;font-size:13px;margin:4px 0;user-select:none;';
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.className = 'subaccount-palace-perm';
      cb.dataset.palace = palace;
      cb.dataset.perm = p;
      cb.checked = cur.indexOf(p) >= 0;
      lab.appendChild(cb);
      lab.appendChild(document.createTextNode(subaccountPermLabel(p)));
      box.appendChild(lab);
    }
    mount.appendChild(box);
  }
}

function gatherSubaccountPalacePerms() {
  const m = {};
  document.querySelectorAll('.subaccount-palace-perm:checked').forEach((cb) => {
    const palace = cb.dataset.palace;
    const perm = cb.dataset.perm;
    if (!palace || !perm) return;
    if (!m[palace]) m[palace] = [];
    m[palace].push(perm);
  });
  return m;
}

function openSubaccountModal(record) {
  if (!SESSION || SESSION.role !== 'tenant') return;
  EDIT_SUBACCOUNT = record ? record.username : null;
  $('subaccountModalError').textContent = '';
  $('subaccountModalTitle').textContent = EDIT_SUBACCOUNT ? ('Edit subaccount — ' + EDIT_SUBACCOUNT) : 'Add subaccount';
  $('subaccountName').value = EDIT_SUBACCOUNT || '';
  $('subaccountName').disabled = !!EDIT_SUBACCOUNT;
  $('subaccountPass').value = '';
  $('subaccountPassLabel').textContent = EDIT_SUBACCOUNT ? 'New password (optional)' : 'Password';
  const selected = {};
  if (record && record.palacePerms) {
    for (const k of Object.keys(record.palacePerms)) {
      selected[k] = (record.palacePerms[k] || []).slice();
    }
  }
  renderSubaccountPermEditors(selected);
  $('subaccountModal').classList.add('open');
}

function closeSubaccountModal() {
  $('subaccountModal').classList.remove('open');
  EDIT_SUBACCOUNT = null;
}

async function saveSubaccountModal() {
  $('subaccountModalError').textContent = '';
  const name = $('subaccountName').value.trim();
  const pass = $('subaccountPass').value;
  const palacePerms = gatherSubaccountPalacePerms();
  const keys = Object.keys(palacePerms);
  if (!EDIT_SUBACCOUNT) {
    if (!name || !pass) {
      $('subaccountModalError').textContent = 'Username and password required.';
      return;
    }
    if (keys.length === 0) {
      $('subaccountModalError').textContent = 'Select at least one permission on a palace.';
      return;
    }
    try {
      const res = await fetch('/api/subaccounts', {
        method: 'POST',
        headers: headers(),
        body: JSON.stringify({ username: name, password: pass, palacePerms }),
      });
      const errBody = await res.json().catch(() => ({}));
      if (!res.ok) {
        $('subaccountModalError').textContent = errBody.error || ('HTTP ' + res.status);
        return;
      }
      closeSubaccountModal();
      loadSubaccounts();
    } catch (e) {
      $('subaccountModalError').textContent = e.message;
    }
    return;
  }
  if (keys.length === 0) {
    $('subaccountModalError').textContent = 'Select at least one permission on a palace.';
    return;
  }
  const body = { palacePerms };
  if (pass) body.password = pass;
  try {
    const res = await fetch('/api/subaccounts/' + encodeURIComponent(EDIT_SUBACCOUNT), {
      method: 'PATCH',
      headers: headers(),
      body: JSON.stringify(body),
    });
    const errBody = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('subaccountModalError').textContent = errBody.error || ('HTTP ' + res.status);
      return;
    }
    closeSubaccountModal();
    loadSubaccounts();
  } catch (e) {
    $('subaccountModalError').textContent = e.message;
  }
}

async function deleteSubaccount(username) {
  if (!username || !confirm('Delete subaccount ' + username + '?')) return;
  try {
    const res = await fetch('/api/subaccounts/' + encodeURIComponent(username), {
      method: 'DELETE',
      headers: headers(),
    });
    const errBody = await res.json().catch(() => ({}));
    if (!res.ok) {
      alert(errBody.error || ('HTTP ' + res.status));
      return;
    }
    loadSubaccounts();
  } catch (e) {
    alert(e.message);
  }
}

async function loadSubaccounts() {
  const tbody = $('subaccountsBody');
  if (!tbody) return;
  try {
    const res = await fetch('/api/subaccounts', { headers: headers() });
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="4" class="empty">HTTP ${res.status}</td></tr>`;
      return;
    }
    const rows = await res.json();
    if (!Array.isArray(rows) || rows.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty">No subaccounts yet.</td></tr>';
      return;
    }
    tbody.innerHTML = rows.map((u) => {
      const pj = JSON.stringify(u).replace(/</g, '\\u003c');
      const permSummary = Object.keys(u.palacePerms || {})
        .map((pn) => {
          const arr = (u.palacePerms[pn] || []).join(', ');
          return `${pn}: [${arr}]`;
        })
        .join(' · ');
      return `<tr>
        <td><strong>${esc(u.username)}</strong></td>
        <td style="max-width:360px;font-size:12px;">${esc(permSummary)}</td>
        <td>${u.mustChangePassword ? 'yes' : 'no'}</td>
        <td><div class="actions">
          <button type="button" onclick='openSubaccountModal(${pj})'>Edit</button>
          <button type="button" class="danger" onclick='deleteSubaccount(${JSON.stringify(u.username)})'>Delete</button>
        </div></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="4" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}
