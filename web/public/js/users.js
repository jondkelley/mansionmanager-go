// ----- Users (admin UI) -----
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

async function loadUsers() {
  const tbody = $('usersBody');
  try {
    const res = await fetch('/api/users', { headers: headers() });
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty">HTTP ${res.status}</td></tr>`;
      return;
    }
    const rows = await res.json();
    if (!Array.isArray(rows) || rows.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">No users.</td></tr>';
      return;
    }
    tbody.innerHTML = rows.map(u => {
      const pj = JSON.stringify(u).replace(/</g, '\\u003c');
      const palCol = u.role === 'subaccount'
        ? esc((u.parentTenant || '') + ' · ' + JSON.stringify(u.palacePerms || {}))
        : esc((u.palaces || []).join(', ') || (u.role === 'admin' ? '—' : ''));
      const editBtn = u.role === 'subaccount'
        ? ''
        : `<button type="button" onclick='openUserModal(${pj})'>Edit</button>`;
      return `<tr>
        <td><strong>${esc(u.username)}</strong></td>
        <td><code>${esc(u.role)}</code></td>
        <td style="max-width:280px;font-size:12px;">${palCol}</td>
        <td>${u.mustChangePassword ? 'yes' : 'no'}</td>
        <td><div class="actions">
          ${editBtn}
          <button type="button" class="danger" onclick='openDeleteUserModal(${JSON.stringify(u.username)})'>Delete</button>
        </div></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}
