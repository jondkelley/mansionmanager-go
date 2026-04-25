let BACKUPS_MODAL_NAME = '';
let BACKUPS_ACTIVE_TAB = 'pat';
let BACKUPS_RESTORE_PENDING = null; // { palaceName, filename } or null

function openPalaceBackupsModal(palaceName) {
  BACKUPS_MODAL_NAME = palaceName;
  const t = $('backupsModalTitle');
  if (t) t.textContent = 'Backups — ' + palaceName;
  const note = $('backupsSnapshotNote');
  if (note) note.textContent = '';
  $('backupsModal').classList.add('open');
  setBackupTab('pat');
  refreshBackupsModal();
}

function closeBackupsModal() {
  $('backupsModal').classList.remove('open');
  BACKUPS_MODAL_NAME = '';
  hideBackupsRestoreConfirm();
  hideBackupsRestoreOverlay();
}

function showBackupsRestoreOverlay() {
  const o = $('backupsRestoreOverlay');
  if (o) o.classList.add('open');
}

function hideBackupsRestoreOverlay() {
  const o = $('backupsRestoreOverlay');
  if (o) o.classList.remove('open');
}

function hideBackupsRestoreConfirm() {
  const o = $('backupsRestoreConfirmOverlay');
  if (o) {
    o.classList.remove('open');
    o.setAttribute('aria-hidden', 'true');
  }
  BACKUPS_RESTORE_PENDING = null;
  const btn = $('backupsRestoreConfirmOk');
  if (btn) btn.disabled = false;
}

function showBackupsRestoreConfirm(palaceName, filename) {
  BACKUPS_RESTORE_PENDING = { palaceName, filename };
  const body = $('backupsRestoreConfirmBody');
  if (body) {
    body.textContent =
      'Restore ' + filename + '? The palace service will stop, the live file will be replaced from the backup, then the service will start again.';
  }
  const o = $('backupsRestoreConfirmOverlay');
  if (o) {
    o.classList.add('open');
    o.setAttribute('aria-hidden', 'false');
  }
  const ok = $('backupsRestoreConfirmOk');
  if (ok) ok.focus();
}

async function refreshBackupsModal() {
  const name = BACKUPS_MODAL_NAME;
  if (!name) return;
  const tbody = $('backupsTableBody');
  if (tbody) tbody.innerHTML = '<tr><td colspan="5" class="empty">Loading…</td></tr>';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/config-backups`, { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="empty">${esc(data.error || ('HTTP ' + res.status))}</td></tr>`;
      return;
    }
    window.BACKUPS_LIST_DATA = data;
    const hint = $('backupsDirHint');
    if (hint) hint.textContent = data.backupDir ? `Backup folder: ${data.backupDir}` : '';
    renderBackupsTableForTab(BACKUPS_ACTIVE_TAB);
  } catch (e) {
    if (tbody) tbody.innerHTML = `<tr><td colspan="5" class="empty">${esc(e.message)}</td></tr>`;
  }
}

function setBackupTab(kind) {
  BACKUPS_ACTIVE_TAB = kind;
  document.querySelectorAll('.backups-tab').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.kind === kind);
  });
  renderBackupsTableForTab(kind);
}

function formatBackupSize(n) {
  if (typeof n !== 'number') return '—';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function renderBackupsTableForTab(kind) {
  const data = window.BACKUPS_LIST_DATA;
  const tbody = $('backupsTableBody');
  if (!tbody || !data || !Array.isArray(data.kinds)) return;
  const grp = data.kinds.find(k => k.id === kind);
  const items = grp && grp.items ? grp.items : [];
  if (items.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty">No backups yet for this file.</td></tr>';
    return;
  }
  const nm = JSON.stringify(BACKUPS_MODAL_NAME);
  tbody.innerHTML = items.map(it => {
    const fn = JSON.stringify(it.filename);
    const sz = formatBackupSize(it.size);
    const dt = it.modTime ? esc(it.modTime) : '—';
    const dateTag = esc(it.dateTag || '');
    return `<tr><td><code>${esc(it.filename)}</code></td><td>${dateTag}</td><td>${sz}</td><td style="font-size:11px;color:var(--muted);">${dt}</td><td><button type="button" onclick='openBackupsRestoreConfirm(${nm},${fn})'>Restore backup</button></td></tr>`;
  }).join('');
}

async function runBackupsSnapshotNow() {
  const name = BACKUPS_MODAL_NAME;
  if (!name) return;
  const btn = $('takeBackupNowBtn');
  const note = $('backupsSnapshotNote');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Taking backup…';
  }
  if (note) note.textContent = '';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/config-backups/snapshot`, {
      method: 'POST',
      headers: headers(),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    const created = Array.isArray(data.created) ? data.created : [];
    if (note) {
      note.textContent = created.length
        ? `Wrote ${created.length} file(s) into backups/: ${created.join(', ')}`
        : 'No live files to snapshot (pserver.pat, pserver.prefs, and serverprefs.json were all missing).';
    }
    await refreshBackupsModal();
  } catch (e) {
    alert(e.message);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = 'Take backup now';
    }
  }
}

function downloadPalaceBackupFromBackupsModal() {
  if (!BACKUPS_MODAL_NAME) return;
  downloadPalaceHomeBackup(BACKUPS_MODAL_NAME);
}

function openBackupsRestoreConfirm(palaceName, filename) {
  showBackupsRestoreConfirm(palaceName, filename);
}

async function executeRestoreConfigBackup() {
  const pending = BACKUPS_RESTORE_PENDING;
  if (!pending) return;
  const { palaceName, filename } = pending;
  hideBackupsRestoreConfirm();
  showBackupsRestoreOverlay();
  const ac = new AbortController();
  const tid = setTimeout(() => ac.abort(), 180000);
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palaceName)}/config-backups/restore`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ filename }),
      signal: ac.signal,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    await refreshBackupsModal();
    if (typeof loadPalaces === 'function') loadPalaces();
    if (typeof loadNginxStatus === 'function') {
      loadNginxStatus();
      setTimeout(loadNginxStatus, 2500);
    }
  } catch (e) {
    if (e.name === 'AbortError') alert('Restore timed out after three minutes — check palace status manually.');
    else alert(e.message);
  } finally {
    clearTimeout(tid);
    hideBackupsRestoreOverlay();
  }
}
