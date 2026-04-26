// Pages — per-palace modal (send system pages to wizard/god users + view page log).

let _palacePagesName = null;
let _palacePagesTimer = null;

function setPalacePagesAlert(msg, type) {
  const el = $('palacePagesAlert');
  if (!el) return;
  el.textContent = msg || '';
  el.className = 'bans-alert' + (type ? ' ' + type : '');
  el.style.display = type ? 'block' : 'none';
}

async function openPalacePagesModal(name) {
  if (_palacePagesTimer) {
    clearInterval(_palacePagesTimer);
    _palacePagesTimer = null;
  }
  _palacePagesName = name;
  $('palacePagesModalTitle').textContent = `Pages - ${name}`;
  $('palacePagesCompose').value = '';
  $('palacePagesCount').textContent = '';
  $('palacePagesFeed').innerHTML = '<div class="empty">Loading...</div>';
  setPalacePagesAlert('');
  $('palacePagesModal').classList.add('open');
  await loadPalacePages(name);
  _palacePagesTimer = setInterval(() => {
    if (_palacePagesName === name) {
      loadPalacePages(name);
    }
  }, 5000);
}

function closePalacePagesModal() {
  if (_palacePagesTimer) {
    clearInterval(_palacePagesTimer);
    _palacePagesTimer = null;
  }
  _palacePagesName = null;
  closePalacePagesGmsgConfirm();
  $('palacePagesModal').classList.remove('open');
}

function palacePagesRowHTML(entry) {
  const unix = Number(entry && entry.unix);
  const ts = Number.isFinite(unix) && unix > 0
    ? new Date(unix * 1000).toLocaleString()
    : '';
  const text = entry && entry.text ? entry.text : '';
  const isGmsg = /^\s*\[gmsg\b/i.test(String(text || ''));
  const rowClass = isGmsg ? 'pages-line pages-line-gmsg' : 'pages-line';
  const txtClass = isGmsg ? 'pages-txt pages-txt-gmsg' : 'pages-txt';
  return `<div class="${rowClass}"><span class="pages-ts">${esc(ts)}</span><span class="${txtClass}">${esc(text)}</span></div>`;
}

async function loadPalacePages(name) {
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/pages`, { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palacePagesCount').textContent = '';
      $('palacePagesFeed').innerHTML = '<div class="empty">Could not load pages</div>';
      setPalacePagesAlert(data.error || ('HTTP ' + res.status), 'error');
      return;
    }
    const entries = Array.isArray(data.entries) ? data.entries : [];
    if (entries.length === 0) {
      $('palacePagesCount').textContent = '';
      $('palacePagesFeed').innerHTML = '<div class="empty">No pages yet</div>';
      return;
    }
    $('palacePagesCount').textContent = `(${entries.length} in memory)`;
    const newestFirst = entries.slice().reverse();
    $('palacePagesFeed').innerHTML = newestFirst.map(palacePagesRowHTML).join('');
    $('palacePagesFeed').scrollTop = 0;
  } catch (e) {
    $('palacePagesCount').textContent = '';
    $('palacePagesFeed').innerHTML = `<div class="empty">Error: ${esc(e.message)}</div>`;
    setPalacePagesAlert('Network error: ' + e.message, 'error');
  }
}

async function sendPalacePage() {
  const name = _palacePagesName;
  if (!name) return;
  const message = $('palacePagesCompose').value;
  if (!String(message || '').trim()) {
    setPalacePagesAlert('Enter a message to send.', 'error');
    return;
  }
  const btn = $('palacePagesSendBtn');
  btn.disabled = true;
  setPalacePagesAlert('');
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/pages/send`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ message }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      setPalacePagesAlert(data.error || data.message || ('HTTP ' + res.status), 'error');
      return;
    }
    $('palacePagesCompose').value = '';
    setPalacePagesAlert(data.message || 'Sent.', 'success');
    await loadPalacePages(name);
  } catch (e) {
    setPalacePagesAlert('Network error: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
  }
}

function openPalacePagesGmsgConfirm() {
  $('palacePagesGmsgConfirmError').textContent = '';
  $('palacePagesGmsgConfirmBtn').disabled = false;
  $('palacePagesGmsgConfirmModal').classList.add('open');
}

function closePalacePagesGmsgConfirm() {
  $('palacePagesGmsgConfirmModal').classList.remove('open');
}

function openPalaceGmsgPrompt() {
  const message = $('palacePagesCompose').value;
  if (!String(message || '').trim()) {
    setPalacePagesAlert('Enter a message to send.', 'error');
    return;
  }
  setPalacePagesAlert('');
  openPalacePagesGmsgConfirm();
}

async function confirmPalaceGmsgSend() {
  const name = _palacePagesName;
  if (!name) return;
  const message = $('palacePagesCompose').value;
  if (!String(message || '').trim()) {
    $('palacePagesGmsgConfirmError').textContent = 'Enter a message to send.';
    return;
  }
  const btn = $('palacePagesGmsgConfirmBtn');
  btn.disabled = true;
  $('palacePagesGmsgConfirmError').textContent = '';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/pages/gmsg`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ message }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palacePagesGmsgConfirmError').textContent = data.error || data.message || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    $('palacePagesCompose').value = '';
    closePalacePagesGmsgConfirm();
    setPalacePagesAlert(data.message || 'Global message sent.', 'success');
    await loadPalacePages(name);
  } catch (e) {
    $('palacePagesGmsgConfirmError').textContent = 'Network error: ' + e.message;
    btn.disabled = false;
  }
}

$('palacePagesRefreshBtn').addEventListener('click', function () {
  if (_palacePagesName) {
    loadPalacePages(_palacePagesName);
  }
});

$('palacePagesSendBtn').addEventListener('click', sendPalacePage);
$('palacePagesGmsgBtn').addEventListener('click', openPalaceGmsgPrompt);
$('palacePagesCompose').addEventListener('keydown', function (ev) {
  if ((ev.key === 'Enter' || ev.keyCode === 13) && (ev.metaKey || ev.ctrlKey)) {
    ev.preventDefault();
    sendPalacePage();
  }
});
