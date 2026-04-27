// ----- Users (admin UI) -----
/** Last successful GET /api/users payload (admin); used for client-side filters. */
let USERS_LIST_CACHE = [];
let USERS_FILTER_TMR = null;

/** Palace names to pre-check when (re)building the tenant picker; updated when hiding the tenant row. */
let USER_MODAL_INITIAL_PALACES = [];

function getSelectedUserPalaces() {
  return [...document.querySelectorAll('#userPalacesList input.user-palace-cb:checked')].map(cb => cb.value);
}

function userPalacesSelectAll(checked) {
  document.querySelectorAll('#userPalacesList input.user-palace-cb').forEach(cb => { cb.checked = checked; });
}

async function refreshUserPalacesPicker(selectedNames) {
  const container = $('userPalacesList');
  if (!container) return;
  const selected = new Set(Array.isArray(selectedNames) ? selectedNames : []);
  container.innerHTML = '<span style="color:var(--muted);">Loading…</span>';
  try {
    const res = await fetch('/api/palaces', { headers: headers() });
    if (!res.ok) {
      container.innerHTML = `<span style="color:var(--red);">Could not load palaces (HTTP ${res.status})</span>`;
      return;
    }
    const data = await res.json();
    const names = (Array.isArray(data) ? data : [])
      .filter(p => p && p.registered !== false && p.name)
      .map(p => p.name)
      .sort((a, b) => a.localeCompare(b));
    container.innerHTML = '';
    if (names.length === 0) {
      container.innerHTML = '<span style="color:var(--muted);">No registered palaces yet.</span>';
      return;
    }
    for (const name of names) {
      const label = document.createElement('label');
      label.style.cssText = 'display:flex;align-items:center;gap:8px;cursor:pointer;user-select:none;';
      const cb = document.createElement('input');
      cb.type = 'checkbox';
      cb.className = 'user-palace-cb';
      cb.value = name;
      cb.checked = selected.has(name);
      const span = document.createElement('span');
      span.textContent = name;
      label.appendChild(cb);
      label.appendChild(span);
      container.appendChild(label);
    }
  } catch (e) {
    container.innerHTML = `<span style="color:var(--red);">${esc(e.message)}</span>`;
  }
}

/** Show/hide palace picker and refresh checkboxes from USER_MODAL_INITIAL_PALACES (does not read DOM). */
function applyUserPalacesRowVisibility() {
  const admin = $('userRole').value === 'admin';
  const row = $('userPalacesRow');
  row.style.display = admin ? 'none' : '';
  if (!admin) {
    void refreshUserPalacesPicker(USER_MODAL_INITIAL_PALACES);
  }
}

/** Persist checked palaces when switching tenant → admin so a later switch back restores them. */
function onUserRoleChange() {
  const row = $('userPalacesRow');
  const switchingToAdmin = $('userRole').value === 'admin';
  if (switchingToAdmin && row.style.display !== 'none') {
    USER_MODAL_INITIAL_PALACES = getSelectedUserPalaces();
  }
  applyUserPalacesRowVisibility();
}

function openUserModal(record) {
  EDIT_USER = record ? record.username : null;
  $('userModalError').textContent = '';
  $('userModalTitle').textContent = EDIT_USER ? ('Edit user — ' + EDIT_USER) : 'Add user';
  $('userName').value = EDIT_USER || '';
  $('userName').disabled = !!EDIT_USER;
  $('userPass').value = '';
  $('userPassLabel').textContent = EDIT_USER ? 'New password (optional)' : 'Password';
  if (record) {
    $('userRole').value = record.role || 'tenant';
    USER_MODAL_INITIAL_PALACES = record.role === 'tenant' ? [...(record.palaces || [])] : [];
  } else {
    $('userRole').value = 'tenant';
    USER_MODAL_INITIAL_PALACES = [];
  }
  applyUserPalacesRowVisibility();
  $('userModal').classList.add('open');
}

function closeUserModal() {
  $('userModal').classList.remove('open');
  EDIT_USER = null;
}

async function saveUserModal() {
  $('userModalError').textContent = '';
  const name = $('userName').value.trim();
  const role = $('userRole').value;
  const palaces = role === 'tenant' ? getSelectedUserPalaces() : [];
  const pass = $('userPass').value;

  try {
    if (!EDIT_USER) {
      if (!name || !pass) {
        $('userModalError').textContent = 'Username and password required.';
        return;
      }
      if (role === 'tenant' && palaces.length === 0) {
        $('userModalError').textContent = 'Tenant needs at least one palace.';
        return;
      }
      const res = await fetch('/api/users', {
        method: 'POST',
        headers: headers(),
        body: JSON.stringify({ username: name, password: pass, role, palaces }),
      });
      const errBody = await res.json().catch(() => ({}));
      if (!res.ok) {
        $('userModalError').textContent = errBody.error || ('HTTP ' + res.status);
        return;
      }
    } else {
      const body = { role };
      if (role === 'tenant') body.palaces = palaces;
      if (pass) body.password = pass;
      const res = await fetch('/api/users/' + encodeURIComponent(EDIT_USER), {
        method: 'PATCH',
        headers: headers(),
        body: JSON.stringify(body),
      });
      const errBody = await res.json().catch(() => ({}));
      if (!res.ok) {
        $('userModalError').textContent = errBody.error || ('HTTP ' + res.status);
        return;
      }
    }
    closeUserModal();
    loadUsers();
  } catch (e) {
    $('userModalError').textContent = e.message;
  }
}

function openDeleteUserModal(username) {
  DELETE_USER_NAME = username;
  $('deleteUserNameDisplay').textContent = username;
  $('deleteUserError').textContent = '';
  $('deleteUserSubmit').disabled = false;
  $('deleteUserModal').classList.add('open');
}

function closeDeleteUserModal() {
  $('deleteUserModal').classList.remove('open');
  DELETE_USER_NAME = null;
}

async function confirmDeleteUser() {
  const username = DELETE_USER_NAME;
  if (!username) return;
  $('deleteUserError').textContent = '';
  const btn = $('deleteUserSubmit');
  btn.disabled = true;
  try {
    const res = await fetch('/api/users/' + encodeURIComponent(username), {
      method: 'DELETE',
      headers: headers(),
    });
    const errBody = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('deleteUserError').textContent = errBody.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    closeDeleteUserModal();
    loadUsers();
  } catch (e) {
    $('deleteUserError').textContent = e.message;
    btn.disabled = false;
  }
}

function scheduleUsersTableFilter() {
  clearTimeout(USERS_FILTER_TMR);
  USERS_FILTER_TMR = setTimeout(() => renderUsersTable(), 220);
}

function resetUsersFilters() {
  const s = $('usersFilterSearch');
  if (s) s.value = '';
  const p = $('usersFilterPalaceSelect');
  if (p) p.value = '';
  const t = $('usersFilterTenantSelect');
  if (t) t.value = '';
  const r = $('usersFilterRole');
  if (r) r.value = '';
  const pw = $('usersFilterPwd');
  if (pw) pw.value = '';
  renderUsersTable();
}

function updateUsersStatCards(rows) {
  const ids = ['usersStatTotal', 'usersStatAdmins', 'usersStatTenants', 'usersStatSubs'];
  const set = (id, v) => {
    const el = $(id);
    if (el) el.textContent = String(v);
  };
  if (!Array.isArray(rows) || rows.length === 0) {
    ids.forEach(id => set(id, '0'));
    return;
  }
  let admins = 0;
  let tenants = 0;
  let subs = 0;
  for (const u of rows) {
    if (u.role === 'admin') admins++;
    else if (u.role === 'tenant') tenants++;
    else if (u.role === 'subaccount') subs++;
  }
  set('usersStatTotal', rows.length);
  set('usersStatAdmins', admins);
  set('usersStatTenants', tenants);
  set('usersStatSubs', subs);
}

function populateUsersAdminFilterDropdowns(rows) {
  const palaceSet = new Set();
  const tenantSet = new Set();
  if (Array.isArray(rows)) {
    for (const u of rows) {
      if (u.role === 'tenant') {
        tenantSet.add(u.username);
        for (const p of u.palaces || []) {
          if (p) palaceSet.add(p);
        }
      }
      if (u.role === 'subaccount') {
        if (u.parentTenant) tenantSet.add(u.parentTenant);
        if (u.palacePerms) {
          for (const k of Object.keys(u.palacePerms)) {
            if (k) palaceSet.add(k);
          }
        }
      }
    }
  }
  const ps = $('usersFilterPalaceSelect');
  const ts = $('usersFilterTenantSelect');
  const keepP = ps && ps.value;
  const keepT = ts && ts.value;
  if (ps) {
    ps.innerHTML = '<option value="">Any palace</option>';
    [...palaceSet].sort((a, b) => a.localeCompare(b)).forEach(name => {
      ps.appendChild(new Option(name, name));
    });
    if (keepP && palaceSet.has(keepP)) ps.value = keepP;
  }
  if (ts) {
    ts.innerHTML = '<option value="">Any tenant</option>';
    [...tenantSet].sort((a, b) => a.localeCompare(b)).forEach(name => {
      ts.appendChild(new Option(name, name));
    });
    if (keepT && tenantSet.has(keepT)) ts.value = keepT;
  }
}

async function mergeRegistryPalacesIntoUserFilter() {
  const ps = $('usersFilterPalaceSelect');
  if (!ps) return;
  try {
    const res = await fetch('/api/palaces', { headers: headers() });
    if (!res.ok) return;
    const data = await res.json();
    const names = (Array.isArray(data) ? data : [])
      .filter(p => p && p.name)
      .map(p => p.name)
      .sort((a, b) => a.localeCompare(b));
    const existing = new Set([...ps.options].map(o => o.value).filter(Boolean));
    for (const n of names) {
      if (!existing.has(n)) {
        ps.appendChild(new Option(n, n));
        existing.add(n);
      }
    }
  } catch (_) { /* ignore */ }
}

function userMatchesAdminFilters(u, palaceSel, tenantSel, roleQ, pwdQ, searchQ) {
  const sq = (searchQ || '').trim().toLowerCase();
  if (sq && !String(u.username).toLowerCase().includes(sq)) return false;

  const ps = (palaceSel || '').trim();
  if (ps) {
    if (u.role === 'tenant') {
      const pals = u.palaces || [];
      if (!pals.includes(ps)) return false;
    } else if (u.role === 'subaccount') {
      const perms = u.palacePerms || {};
      if (!Object.prototype.hasOwnProperty.call(perms, ps)) return false;
    } else {
      return false;
    }
  }

  const tt = (tenantSel || '').trim();
  if (tt) {
    if (u.role === 'tenant') {
      if (u.username !== tt) return false;
    } else if (u.role === 'subaccount') {
      if ((u.parentTenant || '') !== tt) return false;
    } else {
      return false;
    }
  }

  if (roleQ && u.role !== roleQ) return false;

  if (pwdQ === 'must' && !u.mustChangePassword) return false;
  if (pwdQ === 'ok' && u.mustChangePassword) return false;

  return true;
}

function userInitials(username) {
  const s = String(username || '').trim();
  if (!s) return '?';
  const parts = s.split(/[^a-zA-Z0-9]+/).filter(Boolean);
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase().slice(0, 2);
  if (s.length <= 2) return s.toUpperCase();
  return (s[0] + s[1]).toUpperCase();
}

function rolePillHTML(role) {
  const cls = role === 'admin' ? 'user-role-pill user-role-pill-admin'
    : role === 'tenant' ? 'user-role-pill user-role-pill-tenant'
      : 'user-role-pill user-role-pill-sub';
  const label = role === 'admin' ? 'Admin' : role === 'tenant' ? 'Tenant' : 'Subaccount';
  return `<span class="${cls}">${esc(label)}</span>`;
}

function avatarClassForRole(role) {
  if (role === 'admin') return 'user-avatar user-avatar-admin';
  if (role === 'tenant') return 'user-avatar user-avatar-tenant';
  return 'user-avatar user-avatar-sub';
}

function userSubtitle(u) {
  if (u.role === 'admin') return 'Full manager access';
  if (u.role === 'tenant') return 'Palace owner';
  return 'Delegated login';
}

function pwdBadgeHTML(u) {
  if (u.mustChangePassword) {
    return '<span class="users-pwd-badge users-pwd-must" title="User must set a new password at next sign-in">Must change</span>';
  }
  return '<span class="users-pwd-badge users-pwd-ok" title="Password rotation not required">OK</span>';
}

function userIdCellHTML(u) {
  const ini = esc(userInitials(u.username));
  const av = avatarClassForRole(u.role);
  return `<div class="user-cell-id"><div class="${av}" aria-hidden="true">${ini}</div><div class="user-cell-id-meta"><div class="uname">${esc(u.username)}</div><div class="uhandle">${esc(userSubtitle(u))}</div></div></div>`;
}

/** Subaccount: structured palace → permission chips. Tenant: palace list. Admin: — */
function formatPalacesAndDelegatedCell(u) {
  if (u.role === 'tenant') {
    const pals = u.palaces || [];
    if (!pals.length) return '<span style="color:var(--muted);">—</span>';
    return '<div class="users-scope-cell">' + esc(pals.join(', ')) + '</div>';
  }
  if (u.role !== 'subaccount') {
    return '<span style="color:var(--muted);">—</span>';
  }
  const perms = u.palacePerms || {};
  const palaces = Object.keys(perms).sort();
  if (!palaces.length) return '<span style="color:var(--muted);">—</span>';
  let html = '<div class="users-scope-cell">';
  for (const p of palaces) {
    const arr = Array.isArray(perms[p]) ? perms[p] : [];
    const chips = arr.map(x => '<span class="users-perm-chip">' + esc(x) + '</span>').join('');
    html += '<div class="users-perm-block"><div class="users-perm-palace">' + esc(p) + '</div><div>' + (chips || '<span style="color:var(--muted);">—</span>') + '</div></div>';
  }
  html += '</div>';
  return html;
}

function parentTenantCell(u) {
  if (u.role === 'subaccount') return '<code>' + esc(u.parentTenant || '') + '</code>';
  return '<span style="color:var(--muted);">—</span>';
}

function renderUsersTable() {
  const tbody = $('usersBody');
  const summary = $('usersFilterSummary');
  if (!tbody) return;
  const rows = USERS_LIST_CACHE;
  if (!Array.isArray(rows)) {
    tbody.innerHTML = '<tr><td colspan="6" class="empty">No data.</td></tr>';
    if (summary) summary.textContent = '';
    updateUsersStatCards([]);
    return;
  }
  updateUsersStatCards(rows);

  const palaceSel = ($('usersFilterPalaceSelect') && $('usersFilterPalaceSelect').value) || '';
  const tenantSel = ($('usersFilterTenantSelect') && $('usersFilterTenantSelect').value) || '';
  const roleQ = ($('usersFilterRole') && $('usersFilterRole').value) || '';
  const pwdQ = ($('usersFilterPwd') && $('usersFilterPwd').value) || '';
  const searchQ = ($('usersFilterSearch') && $('usersFilterSearch').value) || '';
  const filtered = rows.filter(u => userMatchesAdminFilters(u, palaceSel, tenantSel, roleQ, pwdQ, searchQ));

  if (summary) {
    const parts = [];
    if (palaceSel) parts.push('palace');
    if (tenantSel) parts.push('tenant');
    if (roleQ) parts.push('role');
    if (pwdQ) parts.push('password');
    if ((searchQ || '').trim()) parts.push('search');
    const filterNote = parts.length ? ` · filtered by ${parts.join(', ')}` : '';
    summary.textContent = filtered.length === rows.length
      ? `Showing all ${rows.length} user${rows.length === 1 ? '' : 's'}${filterNote}`
      : `Showing ${filtered.length} of ${rows.length} users${filterNote}`;
  }

  if (filtered.length === 0) {
    if (rows.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty">No users yet. Add a tenant or admin to get started.</td></tr>';
    } else {
      tbody.innerHTML = `<tr><td colspan="6"><div class="users-empty-illustration"><div class="big" aria-hidden="true">⌕</div><div style="font-weight:600;color:var(--text);">No one matches these filters</div><p style="margin-top:8px;font-size:12px;">Adjust the dropdowns or clear the search box.</p><p style="margin-top:12px;"><button type="button" onclick="resetUsersFilters()">Clear all filters</button></p></div></td></tr>`;
    }
    return;
  }

  tbody.innerHTML = filtered.map(u => {
    const pj = JSON.stringify(u).replace(/</g, '\\u003c');
    const editBtn = u.role === 'subaccount'
      ? ''
      : `<button type="button" onclick='openUserModal(${pj})'>Edit</button>`;
    return `<tr>
      <td>${userIdCellHTML(u)}</td>
      <td>${rolePillHTML(u.role)}</td>
      <td class="users-scope-cell">${parentTenantCell(u)}</td>
      <td>${formatPalacesAndDelegatedCell(u)}</td>
      <td>${pwdBadgeHTML(u)}</td>
      <td><div class="actions">
        ${editBtn}
        <button type="button" class="danger" onclick='openDeleteUserModal(${JSON.stringify(u.username)})'>Delete</button>
      </div></td>
    </tr>`;
  }).join('');
}

async function loadUsers() {
  const tbody = $('usersBody');
  const summary = $('usersFilterSummary');
  try {
    const res = await fetch('/api/users', { headers: headers() });
    if (!res.ok) {
      USERS_LIST_CACHE = [];
      tbody.innerHTML = `<tr><td colspan="6" class="empty">HTTP ${res.status}</td></tr>`;
      if (summary) summary.textContent = '';
      updateUsersStatCards([]);
      return;
    }
    const rows = await res.json();
    USERS_LIST_CACHE = Array.isArray(rows) ? rows : [];
    populateUsersAdminFilterDropdowns(USERS_LIST_CACHE);
    await mergeRegistryPalacesIntoUserFilter();
    renderUsersTable();
  } catch (e) {
    USERS_LIST_CACHE = [];
    tbody.innerHTML = `<tr><td colspan="6" class="empty">Error: ${esc(e.message)}</td></tr>`;
    if (summary) summary.textContent = '';
    updateUsersStatCards([]);
  }
}
