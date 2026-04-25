function wizPassToggleUsername() {
  const scope = $('wizPassScope').value;
  $('wizPassUserRow').style.display = scope === 'user' ? '' : 'none';
}

async function loadWizPasses() {
  const pathEl = $('wizPassPath');
  const globalsEl = $('wizPassGlobalCount');
  const entriesBody = $('wizPassEntriesBody');
  pathEl.textContent = 'Loading...';
  globalsEl.textContent = '...';
  entriesBody.innerHTML = '<tr><td colspan="4" class="empty">Loading...</td></tr>';
  try {
    const res = await fetch('/api/wizpasses', { headers: headers() });
    if (!res.ok) {
      pathEl.textContent = 'error';
      globalsEl.textContent = '0';
      entriesBody.innerHTML = `<tr><td colspan="4" class="empty">HTTP ${res.status}</td></tr>`;
      return;
    }
    const data = await res.json();
    pathEl.textContent = data.path || '/etc/palacehostpass';
    globalsEl.textContent = String(data.globalCount || 0);
    const entries = Array.isArray(data.entries) ? data.entries : [];
    if (entries.length === 0) {
      entriesBody.innerHTML = '<tr><td colspan="4" class="empty">No host pass entries.</td></tr>';
      return;
    }
    entriesBody.innerHTML = entries.map(e => {
      const scope = e.scope === 'user' ? 'User-specific' : 'Global';
      const user = e.scope === 'user' ? `<code>${esc(e.username || '')}</code>` : '—';
      const desc = e.scope === 'user'
        ? ('user ' + (e.username || '') + ' entry')
        : 'global entry';
      return `<tr>
        <td>${e.line}</td>
        <td>${scope}</td>
        <td>${user}</td>
        <td><button type="button" class="danger" onclick='deleteWizPass(${Number(e.line) || 0}, ${JSON.stringify(desc)})'>Delete</button></td>
      </tr>`;
    }).join('');
  } catch (e) {
    pathEl.textContent = 'error';
    globalsEl.textContent = '0';
    entriesBody.innerHTML = `<tr><td colspan="4" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

async function addWizPass() {
  $('wizPassError').textContent = '';
  $('wizPassSuccess').textContent = '';
  const scope = $('wizPassScope').value;
  const username = $('wizPassUser').value.trim();
  const password = $('wizPassPassword').value;

  if (!password) {
    $('wizPassError').textContent = 'Password is required.';
    return;
  }
  if (scope === 'user' && !username) {
    $('wizPassError').textContent = 'Username is required for user scope.';
    return;
  }

  const btn = $('wizPassAddBtn');
  btn.disabled = true;
  try {
    const res = await fetch('/api/wizpasses', {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ scope, username, password }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('wizPassError').textContent = body.error || ('HTTP ' + res.status);
      return;
    }
    $('wizPassPassword').value = '';
    $('wizPassSuccess').textContent = scope === 'global'
      ? 'Added shared host password hash.'
      : ('Added host password hash for user ' + username + '.');
    await loadWizPasses();
  } catch (e) {
    $('wizPassError').textContent = e.message;
  } finally {
    btn.disabled = false;
  }
}

async function deleteWizPass(line, description) {
  $('wizPassError').textContent = '';
  $('wizPassSuccess').textContent = '';
  if (!line || line < 1) {
    $('wizPassError').textContent = 'Invalid entry line.';
    return;
  }
  if (!confirm('Delete ' + description + ' from host pass file?')) {
    return;
  }
  try {
    const res = await fetch('/api/wizpasses', {
      method: 'DELETE',
      headers: headers(),
      body: JSON.stringify({ line }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('wizPassError').textContent = body.error || ('HTTP ' + res.status);
      return;
    }
    $('wizPassSuccess').textContent = 'Deleted host pass entry on line ' + line + '.';
    await loadWizPasses();
  } catch (e) {
    $('wizPassError').textContent = e.message;
  }
}
