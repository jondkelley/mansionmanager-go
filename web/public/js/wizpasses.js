function wizPassToggleUsername() {
  const scope = $('wizPassScope').value;
  $('wizPassUserRow').style.display = scope === 'user' ? '' : 'none';
}

async function loadWizPasses() {
  const pathEl = $('wizPassPath');
  const globalsEl = $('wizPassGlobalCount');
  const usersBody = $('wizPassUsersBody');
  pathEl.textContent = 'Loading...';
  globalsEl.textContent = '...';
  usersBody.innerHTML = '<tr><td class="empty">Loading...</td></tr>';
  try {
    const res = await fetch('/api/wizpasses', { headers: headers() });
    if (!res.ok) {
      pathEl.textContent = 'error';
      globalsEl.textContent = '0';
      usersBody.innerHTML = `<tr><td class="empty">HTTP ${res.status}</td></tr>`;
      return;
    }
    const data = await res.json();
    pathEl.textContent = data.path || '/etc/palacehostpass';
    globalsEl.textContent = String(data.globalCount || 0);
    const users = Array.isArray(data.users) ? data.users : [];
    if (users.length === 0) {
      usersBody.innerHTML = '<tr><td class="empty">No user-specific host passes.</td></tr>';
      return;
    }
    usersBody.innerHTML = users.map(u => `<tr><td><code>${esc(u)}</code></td></tr>`).join('');
  } catch (e) {
    pathEl.textContent = 'error';
    globalsEl.textContent = '0';
    usersBody.innerHTML = `<tr><td class="empty">Error: ${esc(e.message)}</td></tr>`;
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
