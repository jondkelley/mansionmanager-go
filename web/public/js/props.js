function formatPropsMB(bytes) {
  const n = Number(bytes);
  if (!Number.isFinite(n) || n < 0) return '—';
  return (n / (1024 * 1024)).toFixed(2) + ' MB';
}

function setPalacePropsAlert(msg, type) {
  const el = $('palacePropsAlert');
  if (!el) return;
  el.textContent = msg || '';
  el.className = 'bans-alert' + (type ? ' ' + type : '');
  el.style.display = type ? 'block' : 'none';
}

function setPalacePropsButtonsDisabled(disabled) {
  ['palacePropsPurgeBtn', 'palacePropsLimitBtn', 'palacePropsAutoBtn', 'palacePropsRefreshBtn'].forEach((id) => {
    const el = $(id);
    if (el) el.disabled = disabled;
  });
}

async function openPalacePropsModal(name) {
  PROPS_PALACE = name;
  $('palacePropsModalTitle').textContent = `Props - ${name}`;
  $('palacePropsUsage').textContent = '';
  $('palacePropsCount').textContent = '—';
  $('palacePropsStorage').textContent = '—';
  $('palacePropsPurgeLimit').textContent = '—';
  $('palacePropsAutoPurge').textContent = '—';
  setPalacePropsAlert('');
  $('palacePropsModal').classList.add('open');
  await loadPalaceProps(name);
}

function closePalacePropsModal() {
  PROPS_PALACE = '';
  $('palacePropsModal').classList.remove('open');
}

async function loadPalaceProps(name) {
  if (!name) return;
  try {
    setPalacePropsButtonsDisabled(true);
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/props`, { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      setPalacePropsAlert(data.error || ('HTTP ' + res.status), 'error');
      return;
    }
    $('palacePropsCount').textContent = String(data.prop_count ?? 0);
    $('palacePropsStorage').textContent = formatPropsMB(data.storage_bytes);
    $('palacePropsPurgeLimit').textContent = data.purge_inactive_props
      ? `${data.purge_prop_days || 0} day(s)`
      : 'Disabled';
    $('palacePropsAutoPurge').textContent = data.auto_purge ? 'On' : 'Off';
    $('palacePropsAutoToggle').checked = !!data.auto_purge;
    $('palacePropsPurgeDays').value = String((data.purge_prop_days && data.purge_prop_days > 0) ? data.purge_prop_days : 30);
    $('palacePropsLimitDays').value = String(data.purge_inactive_props ? (data.purge_prop_days || 30) : 0);
    $('palacePropsUsage').textContent = `${formatPropsMB(data.storage_bytes)} in use`;
  } catch (e) {
    setPalacePropsAlert('Network error: ' + e.message, 'error');
  } finally {
    setPalacePropsButtonsDisabled(false);
  }
}

async function sendPalacePropsCommand(body) {
  const name = PROPS_PALACE;
  if (!name) return null;
  const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/props/command`, {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.message || data.error || ('HTTP ' + res.status));
  }
  return data;
}

async function runPalacePropsPurge() {
  const days = parseInt(($('palacePropsPurgeDays').value || '').trim(), 10);
  if (!Number.isFinite(days) || days <= 0) {
    setPalacePropsAlert('Enter days > 0 for purgeprops.', 'error');
    return;
  }
  try {
    setPalacePropsButtonsDisabled(true);
    setPalacePropsAlert('');
    const out = await sendPalacePropsCommand({ action: 'purgeprops', days });
    setPalacePropsAlert(out.message || 'Purge complete.', 'success');
    await loadPalaceProps(PROPS_PALACE);
  } catch (e) {
    setPalacePropsAlert(e.message, 'error');
  } finally {
    setPalacePropsButtonsDisabled(false);
  }
}

async function setPalacePropsLimit() {
  const days = parseInt(($('palacePropsLimitDays').value || '').trim(), 10);
  if (!Number.isFinite(days) || days < 0) {
    setPalacePropsAlert('Enter days >= 0 for purgelimit.', 'error');
    return;
  }
  try {
    setPalacePropsButtonsDisabled(true);
    setPalacePropsAlert('');
    const out = await sendPalacePropsCommand({ action: 'purgelimit', days });
    setPalacePropsAlert(out.message || 'Purgelimit updated.', 'success');
    await loadPalaceProps(PROPS_PALACE);
  } catch (e) {
    setPalacePropsAlert(e.message, 'error');
  } finally {
    setPalacePropsButtonsDisabled(false);
  }
}

async function setPalacePropsAuto() {
  try {
    setPalacePropsButtonsDisabled(true);
    setPalacePropsAlert('');
    const enabled = !!$('palacePropsAutoToggle').checked;
    const out = await sendPalacePropsCommand({ action: 'autopurge', enabled });
    setPalacePropsAlert(out.message || 'Autopurge updated.', 'success');
    await loadPalaceProps(PROPS_PALACE);
  } catch (e) {
    setPalacePropsAlert(e.message, 'error');
  } finally {
    setPalacePropsButtonsDisabled(false);
  }
}
