// ----- Users (admin UI) -----
function toggleUserPalacesField() {
  const admin = $('userRole').value === 'admin';
  $('userPalacesRow').style.display = admin ? 'none' : '';
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
    $('userPalaces').value = (record.palaces || []).join(', ');
  } else {
    $('userRole').value = 'tenant';
    $('userPalaces').value = '';
  }
  toggleUserPalacesField();
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
  const raw = $('userPalaces').value.trim();
  const palaces = raw === '' ? [] : raw.split(',').map(s => s.trim()).filter(Boolean);
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
      return `<tr>
        <td><strong>${esc(u.username)}</strong></td>
        <td><code>${esc(u.role)}</code></td>
        <td style="max-width:280px;font-size:12px;">${esc((u.palaces || []).join(', ') || (u.role === 'admin' ? '—' : ''))}</td>
        <td>${u.mustChangePassword ? 'yes' : 'no'}</td>
        <td><div class="actions">
          <button type="button" onclick='openUserModal(${pj})'>Edit</button>
          <button type="button" class="danger" onclick='openDeleteUserModal(${JSON.stringify(u.username)})'>Delete</button>
        </div></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}
