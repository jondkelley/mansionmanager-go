function headers() {
  const h = { 'Content-Type': 'application/json' };
  if (AUTH_HEADER) h['Authorization'] = AUTH_HEADER;
  return h;
}

/** Authorization only (for binary GETs that should not send JSON Content-Type). */
function authHeaders() {
  const h = {};
  if (AUTH_HEADER) h['Authorization'] = AUTH_HEADER;
  return h;
}

function showLogin() {
  $('passwordGate').classList.remove('visible');
  $('loginScreen').classList.add('visible');
  $('loginUser').focus();
}
function hideLogin() {
  $('loginScreen').classList.remove('visible');
}

function showPasswordGate() {
  $('loginScreen').classList.remove('visible');
  $('passwordGate').classList.add('visible');
  $('passwordGateError').textContent = '';
  $('passwordGateCurrent').value = '';
  $('passwordGateNew').value = '';
  $('passwordGateNew2').value = '';
  $('passwordGateCurrent').focus();
}

function hidePasswordGate() {
  $('passwordGate').classList.remove('visible');
}

function applySessionUI() {
  const admin = SESSION && SESSION.role === 'admin';
  ['navUpdate', 'navNginx', 'navUsers', 'btnNewPalace'].forEach(id => {
    const el = document.getElementById(id);
    if (el) el.style.display = admin ? '' : 'none';
  });
  if (admin) silentUpdateCheck();
}

async function refreshSession() {
  const res = await fetch('/api/session', { headers: headers() });
  if (!res.ok) return false;
  SESSION = await res.json();
  applySessionUI();
  return true;
}

async function afterSessionData(data) {
  SESSION = data;
  applySessionUI();
  if (data.mustChangePassword) {
    showPasswordGate();
    return;
  }
  hidePasswordGate();
  loadPalaces();
}

async function submitPasswordChange() {
  $('passwordGateError').textContent = '';
  const current = $('passwordGateCurrent').value;
  const a = $('passwordGateNew').value;
  const b = $('passwordGateNew2').value;
  if (!current) { $('passwordGateError').textContent = 'Enter your current password.'; return; }
  if (a.length < 10) { $('passwordGateError').textContent = 'New password must be at least 10 characters.'; return; }
  if (a !== b) { $('passwordGateError').textContent = 'New passwords do not match.'; return; }
  try {
    const res = await fetch('/api/session/password', {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ current, new: a }),
    });
    if (res.status === 401) {
      $('passwordGateError').textContent = 'Current password incorrect.';
      return;
    }
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      $('passwordGateError').textContent = err.error || ('HTTP ' + res.status);
      return;
    }
    hidePasswordGate();
    await refreshSession();
    loadPalaces();
  } catch (e) {
    $('passwordGateError').textContent = 'Connection error: ' + e.message;
  }
}

async function doLogin() {
  const user = $('loginUser').value.trim();
  const pass = $('loginPass').value;
  $('loginError').textContent = '';
  if (!user || !pass) { $('loginError').textContent = 'Username and password required.'; return; }

  const encoded = btoa(user + ':' + pass);
  const testHeaders = { 'Authorization': 'Basic ' + encoded, 'Content-Type': 'application/json' };

  try {
    const res = await fetch('/api/session', { headers: testHeaders });
    if (res.status === 401) {
      $('loginError').textContent = 'Invalid username or password.';
      return;
    }
    if (!res.ok) {
      $('loginError').textContent = 'HTTP ' + res.status;
      return;
    }
    const data = await res.json();
    AUTH_HEADER = 'Basic ' + encoded;
    sessionStorage.setItem('pm_auth', AUTH_HEADER);
    hideLogin();
    await afterSessionData(data);
  } catch(e) {
    $('loginError').textContent = 'Connection error: ' + e.message;
  }
}

function doLogout() {
  AUTH_HEADER = '';
  SESSION = null;
  sessionStorage.removeItem('pm_auth');
  hidePasswordGate();
  showLogin();
}

// Enter key submits login or password gate.
document.addEventListener('keydown', e => {
  if (e.key !== 'Enter') return;
  if ($('loginScreen').classList.contains('visible')) doLogin();
  else if ($('passwordGate').classList.contains('visible')) submitPasswordChange();
});
