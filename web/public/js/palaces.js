// ===== Palaces =====
function syncPalaceSettingsMode() {
  const raw = $('palaceSettingsModeRaw').checked;
  $('palaceSettingsStructured').style.display = raw ? 'none' : '';
  $('palaceSettingsRawWrap').style.display = raw ? '' : 'none';
}

function populatePrefsFormFromDTO(f) {
  if (!f) return;
  $('psfServerName').value = f.serverName || '';
  $('psfSysop').value = f.sysop || '';
  $('psfURL').value = f.url || '';
  $('psfWebsite').value = f.website || '';
  $('psfMOTD').value = f.motd || '';
  $('psfBlurb').value = f.blurb || '';
  $('psfAnnouncement').value = f.announcement || '';
  $('psfDeathPenalty').value = f.deathPenalty != null && f.deathPenalty !== '' ? String(f.deathPenalty) : '';
  $('psfMaxOcc').value = f.maxOccupancy != null && f.maxOccupancy !== '' ? String(f.maxOccupancy) : '';
  $('psfRoomOcc').value = f.roomOccupancy != null && f.roomOccupancy !== '' ? String(f.roomOccupancy) : '';
  $('psfMinFlood').value = f.minFloodEvents != null && f.minFloodEvents !== '' ? String(f.minFloodEvents) : '';
  $('psfPurgeDays').value = f.purgePropDays != null && f.purgePropDays !== '' ? String(f.purgePropDays) : '';
  $('psfRecycleLimit').value = f.recycleLimit != null && f.recycleLimit !== '' ? String(f.recycleLimit) : '';
  $('psfChatTypes').value = f.chatLogTypes || '';
  $('psfChatFile').value = f.chatLogFile || '';
  const cf = (f.chatLogFormat || '').toLowerCase();
  $('psfChatFormat').value = cf === 'csv' ? 'csv' : cf === 'json' ? 'json' : '';
  $('psfChatNoWarn').checked = !!f.chatLogNoWarn;
}

function collectPrefsFormDTO() {
  const num = id => {
    const v = parseInt($(id).value, 10);
    return Number.isFinite(v) ? v : 0;
  };
  return {
    serverName: $('psfServerName').value,
    sysop: $('psfSysop').value,
    url: $('psfURL').value,
    website: $('psfWebsite').value,
    motd: $('psfMOTD').value,
    blurb: $('psfBlurb').value,
    announcement: $('psfAnnouncement').value,
    deathPenalty: num('psfDeathPenalty'),
    maxOccupancy: num('psfMaxOcc'),
    roomOccupancy: num('psfRoomOcc'),
    minFloodEvents: num('psfMinFlood'),
    purgePropDays: num('psfPurgeDays'),
    recycleLimit: num('psfRecycleLimit'),
    chatLogTypes: $('psfChatTypes').value,
    chatLogFile: $('psfChatFile').value,
    chatLogFormat: $('psfChatFormat').value || '',
    chatLogNoWarn: $('psfChatNoWarn').checked,
  };
}

async function openPalaceSettingsModal(name) {
  SETTINGS_PALACE = name;
  SETTINGS_RAW_SNAPSHOT = '';
  $('palaceSettingsError').textContent = '';
  $('palaceSettingsSaveBtn').disabled = false;
  $('palaceSettingsTitle').textContent = 'Palace Preferences — ' + name;
  $('palaceSettingsLead').textContent = '';
  $('palaceSettingsContent').value = '';
  $('palaceUnknownTail').value = '';
  $('palaceSettingsWarnings').style.display = 'none';
  $('palaceSettingsWarnings').textContent = '';
  $('palaceSettingsModeForm').checked = true;
  syncPalaceSettingsMode();

  $('palaceSettingsModal').classList.add('open');

  try {
    const [pres, pform, fres] = await Promise.all([
      fetch(`/api/palaces/${encodeURIComponent(name)}`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/prefs-form`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/server-files/pserver.prefs`, { headers: headers() }),
    ]);
    const pd = await pres.json().catch(() => ({}));
    const formData = await pform.json().catch(() => ({}));
    const rawFile = await fres.json().catch(() => ({}));

    if (!pres.ok) {
      $('palaceSettingsError').textContent = pd.error || ('HTTP ' + pres.status);
      return;
    }
    const dd = pd.dataDir || '';
    $('palaceSettingsLead').textContent = dd ? ('Data directory: ' + dd) : '';
    if (pform.ok && formData.form) {
      populatePrefsFormFromDTO(formData.form);
      $('palaceUnknownTail').value = formData.unknownTail || '';
      if (Array.isArray(formData.warnings) && formData.warnings.length) {
        $('palaceSettingsWarnings').style.display = '';
        $('palaceSettingsWarnings').textContent = 'Parse notes: ' + formData.warnings.map(esc).join(' · ');
      }
    } else if (!pform.ok) {
      $('palaceSettingsError').textContent = formData.error || ('prefs-form HTTP ' + pform.status);
    }

    if (fres.ok && rawFile.content !== undefined && rawFile.content !== null) {
      SETTINGS_RAW_SNAPSHOT = typeof rawFile.content === 'string' ? rawFile.content : '';
      $('palaceSettingsContent').value = SETTINGS_RAW_SNAPSHOT;
    } else if (fres.status === 404) {
      SETTINGS_RAW_SNAPSHOT = '; pserver.prefs — save to create on disk\n';
      $('palaceSettingsContent').value = SETTINGS_RAW_SNAPSHOT;
    } else if (!fres.ok && pform.ok) {
      $('palaceSettingsContent').value = SETTINGS_RAW_SNAPSHOT;
    }
  } catch (e) {
    $('palaceSettingsError').textContent = e.message || String(e);
  }
}

function closePalaceSettingsModal() {
  $('palaceSettingsModal').classList.remove('open');
  SETTINGS_PALACE = null;
  SETTINGS_RAW_SNAPSHOT = '';
}

async function savePalaceSettings() {
  const name = SETTINGS_PALACE;
  if (!name) return;
  $('palaceSettingsError').textContent = '';
  const rawMode = $('palaceSettingsModeRaw').checked;

  let body;
  if (rawMode) {
    body = {
      mode: 'raw',
      content: $('palaceSettingsContent').value,
    };
  } else {
    body = {
      mode: 'form',
      form: collectPrefsFormDTO(),
      unknownTail: $('palaceUnknownTail').value,
    };
  }
  const btn = $('palaceSettingsSaveBtn');
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/server-prefs`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify(body),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceSettingsError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    closePalaceSettingsModal();
    loadPalaces();
  } catch (e) {
    $('palaceSettingsError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

function palaceStatusDot(status) {
  if (status === 'active') {
    return { dotClass: 'status-dot-running', title: 'Service running' };
  }
  if (status === 'inactive') {
    return { dotClass: 'status-dot-stopped', title: 'Service stopped' };
  }
  if (status === 'failed') {
    return { dotClass: 'status-dot-stopped', title: 'Service failed' };
  }
  return { dotClass: 'status-dot-warn', title: 'Status unknown' };
}

/** Stop / Start / Restart — runs immediately (no confirmation). */
function palaceServiceControlButtonsHTML(nameJson) {
  return `<button type="button" onclick='void palaceAction(${nameJson},"stop")'>Stop</button>` +
    `<button type="button" onclick='void palaceAction(${nameJson},"start")'>Start</button>` +
    `<button type="button" onclick='void palaceAction(${nameJson},"restart")'>Restart</button>`;
}

const PROVISION_TCP_RANGE = [9990, 10990];
const PROVISION_HTTP_RANGE = [6000, 7000];
const PALACE_EXPANDED = new Set();

async function fetchUsedPalacePorts() {
  const used = new Set();
  const res = await fetch('/api/palaces', { headers: headers() });
  if (!res.ok) return used;
  const list = await res.json().catch(() => []);
  if (!Array.isArray(list)) return used;
  list.forEach(p => {
    const tcp = parseInt(p && p.tcpPort, 10);
    const http = parseInt(p && p.httpPort, 10);
    if (Number.isFinite(tcp) && tcp > 0) used.add(tcp);
    if (Number.isFinite(http) && http > 0) used.add(http);
  });
  return used;
}

function pickRangePort(start, end, used) {
  for (let port = start; port <= end; port += 1) {
    if (!used.has(port)) return port;
  }
  return 0;
}

async function suggestProvisionPorts() {
  const tcpInput = $('pTCP');
  const httpInput = $('pHTTP');
  const tcpAuto = $('pTCPAuto') && $('pTCPAuto').checked;
  const httpAuto = $('pHTTPAuto') && $('pHTTPAuto').checked;
  if (!tcpInput || !httpInput || (!tcpAuto && !httpAuto)) return;
  try {
    const used = await fetchUsedPalacePorts();
    if (tcpAuto) {
      const tcp = pickRangePort(PROVISION_TCP_RANGE[0], PROVISION_TCP_RANGE[1], used);
      if (tcp) {
        tcpInput.value = String(tcp);
        used.add(tcp);
      } else {
        tcpInput.value = '';
      }
    }
    if (httpAuto) {
      const http = pickRangePort(PROVISION_HTTP_RANGE[0], PROVISION_HTTP_RANGE[1], used);
      if (http) {
        httpInput.value = String(http);
        used.add(http);
      } else {
        httpInput.value = '';
      }
    }
  } catch (_) {}
}

function syncProvisionPortMode() {
  const tcpAuto = $('pTCPAuto') && $('pTCPAuto').checked;
  const httpAuto = $('pHTTPAuto') && $('pHTTPAuto').checked;
  if ($('pTCP')) $('pTCP').disabled = !!tcpAuto;
  if ($('pHTTP')) $('pHTTP').disabled = !!httpAuto;
  if (tcpAuto || httpAuto) {
    suggestProvisionPorts();
  }
}

function togglePalaceAccordion(name) {
  if (!name || !SESSION || SESSION.role !== 'admin') return;
  if (PALACE_EXPANDED.has(name)) {
    PALACE_EXPANDED.delete(name);
  } else {
    PALACE_EXPANDED.add(name);
  }
  loadPalaces();
}

async function loadPalaces() {
  const tbody = $('palaceBody');
  const unregPanel = $('unregisteredPalacesPanel');
  const unregTbody = $('unregisteredPalaceBody');
  try {
    const res = await fetch('/api/palaces', { headers: headers() });
    if (res.status === 403) {
      const d = await res.json().catch(() => ({}));
      if (d.code === 'password_change_required') {
        SESSION = SESSION || {};
        SESSION.mustChangePassword = true;
        showPasswordGate();
        return;
      }
    }
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty">Could not load palaces (HTTP ${res.status})</td></tr>`;
      if (unregPanel) unregPanel.style.display = 'none';
      return;
    }
    const data = await res.json();
    const canAdmin = SESSION && SESSION.role === 'admin';
    const orphans = canAdmin && Array.isArray(data) ? data.filter(p => p.registered === false) : [];
    const mainList = canAdmin && Array.isArray(data)
      ? data.filter(p => p.registered !== false)
      : (Array.isArray(data) ? data : []);

    if (canAdmin && orphans.length > 0) {
      unregPanel.style.display = '';
      unregTbody.innerHTML = orphans.map(p => {
        const { dotClass, title } = palaceStatusDot(p.status);
        const nm = JSON.stringify(p.name);
        const removeBtn = `<button type="button" class="danger" onclick='openRemovePalaceModal(${nm})'>Remove</button>`;
        const orphanSettingsBtn = `<button type="button" onclick='openPalaceSettingsModal(${nm})'>Settings</button>`;
        const controlBtns = palaceServiceControlButtonsHTML(nm);
        return `
      <tr>
        <td><strong>${esc(p.name)}</strong> <span class="badge badge-unregistered" title="Not in registry yet">Not registered</span></td>
        <td><span class="palace-status"><span class="status-dot ${dotClass}" title="${esc(title)}" aria-hidden="true"></span><span class="badge badge-${esc(p.status)}">${esc(p.status)}</span></span></td>
        <td>${p.tcpPort || '—'}</td>
        <td>${p.httpPort || '—'}</td>
        <td>
          <div class="actions unregistered-palace-actions">
            <div class="palace-detail-block" style="margin:0;">
              <span class="palace-detail-label">Control</span>
              <div class="palace-detail-actions" style="justify-content:flex-start;">${controlBtns}</div>
            </div>
            <div style="display:flex;flex-wrap:wrap;gap:6px;">
              <button type="button" class="primary" onclick='openRegisterPalaceModal(${nm})'>Register…</button>
              ${orphanSettingsBtn}
              ${removeBtn}
            </div>
          </div>
        </td>
      </tr>`;
      }).join('');
      if (SCROLL_UNREGISTER_PANEL) {
        SCROLL_UNREGISTER_PANEL = false;
        requestAnimationFrame(() => {
          unregPanel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        });
      }
    } else {
      unregPanel.style.display = 'none';
      unregTbody.innerHTML = '';
    }

    if (!Array.isArray(data) || data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">No palaces found. Provision one to get started.</td></tr>';
      return;
    }
    if (mainList.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">No registered palaces in the manager. Unregistered instances are listed below.</td></tr>';
      return;
    }
    const isAdmin = !!(SESSION && SESSION.role === 'admin');
    const isTenant = !!(SESSION && SESSION.role === 'tenant');
    const expandedList = mainList.filter(p => p.httpPort && (isTenant || PALACE_EXPANDED.has(p.name)));
    tbody.innerHTML = mainList.map(p => {
      const { dotClass, title } = palaceStatusDot(p.status);
      const nm = JSON.stringify(p.name);
      const expanded = isTenant || PALACE_EXPANDED.has(p.name);
      const expandGlyph = expanded ? '&#9662;' : '&#9656;';
      const pserv = `<code>${esc(p.pserverVersion || 'latest')}</code>`;
      const removeBtn = isAdmin
        ? `<button type="button" class="danger" onclick='openRemovePalaceModal(${nm})'>Remove</button>`
        : '';
      const logsBtn = `<button type="button" onclick='viewLogs(${nm})'>Logs</button>`;
      const settingsBtn = `<button type="button" onclick='openPalaceSettingsModal(${nm})'>Settings</button>`;
      const mediaBtn = `<button type="button" onclick='openPalaceMediaModal(${nm})' title="Media folder on disk (systemd -m)">Media</button>`;
      const backupsBtn = `<button type="button" onclick='openPalaceBackupsModal(${nm})' title="Config snapshots and full-home download">Backups</button>`;
      const filesBtn = `<button type="button" onclick='openServerFilesModal(${nm})'>Files</button>`;
      const usersBtn = p.httpPort
        ? `<button type="button" onclick='openPalaceUsersModal(${nm})'>Users</button>`
        : '';
      const summaryClass = isAdmin ? 'palace-row-summary' : '';
      const sid = palaceStatId(p.name);
      const controlBtns = palaceServiceControlButtonsHTML(nm);
      return `
      <tr class="${summaryClass}${expanded ? ' palace-row-open' : ''}"${isAdmin ? ` onclick='togglePalaceAccordion(${nm})'` : ''}>
        <td>
          <span class="palace-name-cell">
            <span class="palace-expander">${isAdmin ? expandGlyph : '&nbsp;'}</span>
            <strong>${esc(p.name)}</strong>
          </span>
        </td>
        <td><span class="palace-status"><span class="status-dot ${dotClass}" title="${esc(title)}" aria-hidden="true"></span><span class="badge badge-${esc(p.status)}">${esc(p.status)}</span></span></td>
        <td>${p.tcpPort || '—'}</td>
        <td>${p.httpPort || '—'}</td>
        <td>${pserv}</td>
      </tr>
      <tr class="palace-details-row" style="display:${expanded ? '' : 'none'};">
        <td colspan="5">
          <div class="palace-details-wrap">
            <div class="palace-detail-block">
              <span class="palace-detail-label">Service user</span>
              <span class="palace-detail-value"><code>${esc(p.user || p.name)}</code></span>
            </div>
            <div class="palace-details-side">
              <div class="palace-detail-block">
                <span class="palace-detail-label">Control</span>
                <div class="palace-detail-actions">${controlBtns}</div>
              </div>
              <div class="palace-detail-block">
                <span class="palace-detail-label">Actions</span>
                <div class="palace-detail-actions">
                  ${mediaBtn}
                  ${backupsBtn}
                  ${filesBtn}
                  ${settingsBtn}
                  ${logsBtn}
                  ${usersBtn}
                  ${removeBtn}
                </div>
              </div>
            </div>
          </div>
          ${p.httpPort ? `
          <div class="palace-stats-strip" id="${sid}">
            <div class="palace-stats-grid">
              <div class="palace-stat-item" id="${sid}-rooms"><span class="palace-stat-value">—</span><span class="palace-stat-label">Rooms</span></div>
              <div class="palace-stat-item" id="${sid}-uptime"><span class="palace-stat-value">—</span><span class="palace-stat-label">Uptime</span></div>
              <div class="palace-stat-item" id="${sid}-online"><span class="palace-stat-value">—</span><span class="palace-stat-label">Online</span></div>
              <div class="palace-stat-item" id="${sid}-max"><span class="palace-stat-value">—</span><span class="palace-stat-label">Max Users</span></div>
              <div class="palace-stat-item" id="${sid}-today"><span class="palace-stat-value">—</span><span class="palace-stat-label">Today</span></div>
              <div class="palace-stat-item" id="${sid}-week"><span class="palace-stat-value">—</span><span class="palace-stat-label">This Week</span></div>
              <div class="palace-stat-item" id="${sid}-ops"><span class="palace-stat-value">—</span><span class="palace-stat-label">Operators</span></div>
              <div class="palace-stat-item" id="${sid}-gods"><span class="palace-stat-value">—</span><span class="palace-stat-label">Gods</span></div>
              <div class="palace-stat-item" id="${sid}-owners"><span class="palace-stat-value">—</span><span class="palace-stat-label">Owners</span></div>
            </div>
          </div>` : ''}
        </td> 
      </tr>`;
    }).join('');
    syncPalaceStatsPolling(expandedList);
  } catch(e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">Error: ${esc(e.message)}</td></tr>`;
    if (unregPanel) unregPanel.style.display = 'none';
  }
}

async function palaceAction(name, action) {
  try {
    await fetch(`/api/palaces/${encodeURIComponent(name)}/${action}`, { method: 'POST', headers: headers() });
  } catch (_) {
    /* no UI feedback — row refresh still picks up real state */
  }
  setTimeout(loadPalaces, 800);
  if (action === 'start' || action === 'restart') {
    loadNginxStatus();
    setTimeout(loadNginxStatus, 2500);
  }
}

async function downloadPalaceHomeBackup(name) {
  if (!name) return;
  const url = `/api/palaces/${encodeURIComponent(name)}/home-backup`;
  try {
    const res = await fetch(url, { headers: authHeaders() });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    const blob = await res.blob();
    const cd = res.headers.get('Content-Disposition') || '';
    const stamp = new Date().toISOString().replace(/[:.]/g, '-');
    let fname = `${name}-home-backup-${stamp}.tar.gz`;
    const m = /filename="([^"]+)"/.exec(cd);
    if (m) fname = m[1];
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = href;
    a.download = fname;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(href);
  } catch (e) {
    alert(e.message);
  }
}

async function openRegisterPalaceModal(name) {
  REGISTER_PALACE_NAME = name;
  $('registerPalaceTitleName').textContent = name;
  $('registerPalaceError').textContent = '';
  $('registerTcp').value = '';
  $('registerHttp').value = '';
  $('registerLinuxUser').value = '';
  $('registerDataDir').value = '';
  $('registerYPHost').value = '';
  $('registerYPPort').value = '';
  $('registerEnableNow').checked = true;
  $('registerPalaceSubmit').disabled = false;
  $('registerPalaceModal').classList.add('open');
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/discover`, { headers: headers() });
    const d = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('registerPalaceError').textContent = d.error || ('HTTP ' + res.status);
      return;
    }
    if (d.tcpPort) $('registerTcp').value = String(d.tcpPort);
    if (d.httpPort) $('registerHttp').value = String(d.httpPort);
    if (d.linuxUser) $('registerLinuxUser').value = d.linuxUser;
    if (d.dataDir) $('registerDataDir').value = d.dataDir;
  } catch (e) {
    $('registerPalaceError').textContent = e.message;
  }
}

function closeRegisterPalaceModal() {
  $('registerPalaceModal').classList.remove('open');
  REGISTER_PALACE_NAME = null;
}

async function confirmRegisterPalace() {
  const name = REGISTER_PALACE_NAME;
  if (!name) return;
  $('registerPalaceError').textContent = '';
  const btn = $('registerPalaceSubmit');
  btn.disabled = true;
  try {
    const tcp = parseInt($('registerTcp').value, 10);
    const httpPort = parseInt($('registerHttp').value, 10);
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/register`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({
        tcpPort: Number.isFinite(tcp) ? tcp : 0,
        httpPort: Number.isFinite(httpPort) ? httpPort : 0,
        linuxUser: $('registerLinuxUser').value.trim(),
        dataDir: $('registerDataDir').value.trim(),
        enableNow: $('registerEnableNow').checked,
        ypHost: $('registerYPHost').value.trim(),
        ypPort: (function(){ const n = parseInt($('registerYPPort').value, 10); return Number.isFinite(n) ? n : 0; })(),
      }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('registerPalaceError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    if (out.enableWarning) {
      alert('Palace registered, but systemd reported: ' + out.enableWarning);
    }
    closeRegisterPalaceModal();
    loadPalaces();
  } catch (e) {
    $('registerPalaceError').textContent = e.message;
    btn.disabled = false;
  }
}

function openRemovePalaceModal(name) {
  REMOVE_PALACE_NAME = name;
  $('removePalaceNameDisplay').textContent = name;
  $('removePalaceError').textContent = '';
  document.querySelector('input[name="removePalacePurge"][value="false"]').checked = true;
  syncRemovePalaceSubmitStyle();
  $('removePalaceSubmit').disabled = false;
  $('removePalaceModal').classList.add('open');
}

function closeRemovePalaceModal() {
  $('removePalaceModal').classList.remove('open');
  $('removePalaceSpinner').style.display = 'none';
  $('removePalaceFooter').style.display = '';
  REMOVE_PALACE_NAME = null;
}

function syncRemovePalaceSubmitStyle() {
  const purge = document.querySelector('input[name="removePalacePurge"]:checked').value === 'true';
  const btn = $('removePalaceSubmit');
  btn.classList.toggle('danger', purge);
  btn.classList.toggle('primary', !purge);
  btn.textContent = purge ? 'Remove & delete account' : 'Remove palace';
}

async function confirmRemovePalace() {
  const name = REMOVE_PALACE_NAME;
  if (!name) return;
  const purge = document.querySelector('input[name="removePalacePurge"]:checked').value === 'true';
  $('removePalaceError').textContent = '';
  const btn = $('removePalaceSubmit');
  btn.disabled = true;
  $('removePalaceSpinner').style.display = '';
  $('removePalaceFooter').style.display = 'none';
  try {
    const q = purge ? '?purge=true' : '';
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}${q}`, {
      method: 'DELETE',
      headers: headers(),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('removePalaceSpinner').style.display = 'none';
      $('removePalaceFooter').style.display = '';
      $('removePalaceError').textContent = body.error || ('HTTP ' + res.status);
      btn.disabled = false;
      syncRemovePalaceSubmitStyle();
      return;
    }
    if (!purge) {
      SCROLL_UNREGISTER_PANEL = true;
    }
    closeRemovePalaceModal();
    loadPalaces();
  } catch (e) {
    $('removePalaceSpinner').style.display = 'none';
    $('removePalaceFooter').style.display = '';
    $('removePalaceError').textContent = e.message;
    btn.disabled = false;
    syncRemovePalaceSubmitStyle();
  }
}

// Provision modal
function openProvisionModal() {
  $('provisionModal').classList.add('open');
  $('provisionStream').innerHTML = '';
  $('provisionResult').className = '';
  $('provisionResult').innerHTML = '';
  $('provisionResult').style.display = 'none';
  $('provisionForm').style.display = '';
  $('provisionFooter').innerHTML =
    `<button id="provisionCancelBtn" onclick="closeProvisionModal()">Cancel</button>` +
    `<button id="provisionBtn" class="primary" onclick="doProvision()">Provision</button>`;
  ['pName','pTCP','pHTTP'].forEach(id => { $(id).value = ''; });
  if ($('pTCPAuto')) $('pTCPAuto').checked = true;
  if ($('pHTTPAuto')) $('pHTTPAuto').checked = true;
  syncProvisionPortMode();
}
function closeProvisionModal() {
  $('provisionModal').classList.remove('open');
  loadPalaces();
}

function _setProvisionRunning(running) {
  ['pName','pYPHost','pYPPort','pTCPAuto','pHTTPAuto'].forEach(id => { const el = $(id); if (el) el.disabled = running; });
  if (!running) syncProvisionPortMode();
  if (running) {
    ['pTCP','pHTTP'].forEach(id => { const el = $(id); if (el) el.disabled = true; });
  }
  const btn = $('provisionBtn');
  if (btn) { btn.disabled = running; btn.textContent = running ? 'Provisioning…' : 'Provision'; }
}

async function doProvision() {
  const name = $('pName').value.trim();
  let tcpPort = parseInt($('pTCP').value, 10);
  let httpPort = parseInt($('pHTTP').value, 10);
  if (($('pTCPAuto') && $('pTCPAuto').checked) || ($('pHTTPAuto') && $('pHTTPAuto').checked)) {
    if (!Number.isFinite(tcpPort) || !Number.isFinite(httpPort)) {
      await suggestProvisionPorts();
      tcpPort = parseInt($('pTCP').value, 10);
      httpPort = parseInt($('pHTTP').value, 10);
    }
  }
  if (!name || !tcpPort || !httpPort) { alert('All fields required'); return; }

  const stream = $('provisionStream');
  stream.textContent = '';
  stream.innerHTML = '';
  $('provisionResult').style.display = 'none';

  _setProvisionRunning(true);

  const ypHost = $('pYPHost').value.trim();
  let ypPort = parseInt($('pYPPort').value, 10);
  if (!Number.isFinite(ypPort)) ypPort = 0;
  const res = await fetch('/api/palaces', {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ name, tcpPort, httpPort, ypHost, ypPort })
  });

  if (res.status === 412) {
    const data = await res.json().catch(() => ({ error: 'pserver template not ready' }));
    stream.innerHTML =
      `<span style="color:var(--yellow);font-weight:600;">⚠ Setup required</span>\n\n` +
      `<span style="color:var(--text);">${esc(data.error)}</span>\n\n` +
      `<span style="color:var(--muted);">→ <a href="#" onclick="gotoUpdateTab();return false" style="color:var(--accent);">Go to Updates</a> and click <strong>Updates</strong> to download the pserver template, then come back here.</span>`;
    _setProvisionRunning(false);
    return;
  }

  await streamSSE(res, stream, (okObj) => {
    if (okObj) {
      _showProvisionSuccess(okObj.name || name);
    } else {
      _showProvisionFailure();
    }
  });
}

function _showProvisionSuccess(palName) {
  loadPalaces();
  loadNginxStatus();
  setTimeout(loadNginxStatus, 3000);
  const el = $('provisionResult');
  el.className = 'success';
  el.innerHTML =
    `<div class="res-title" style="color:var(--green);">✓ Palace "${esc(palName)}" is ready!</div>` +
    `<div class="res-body">Your new palace has been provisioned. <strong>Click the button below to close this window</strong> — then hit <strong>Start</strong> next to <em>${esc(palName)}</em> in the Palaces list to launch it.</div>` +
    `<div class="res-note">Before starting, make sure your pserver.pat and media assets are in place:<br>` +
    `<code>/home/${esc(palName)}/palace/pserver.pat</code><br>` +
    `<code>/home/${esc(palName)}/palace/media/</code></div>`;
  el.style.display = 'block';
  $('provisionFooter').innerHTML =
    `<button class="primary" onclick="closeProvisionModal()">Done — Go to Palaces ›</button>`;
}

function _showProvisionFailure() {
  const el = $('provisionResult');
  el.className = 'failure';
  el.innerHTML =
    `<div class="res-title" style="color:var(--red);">✗ Provisioning failed</div>` +
    `<div class="res-body">Check the log above for details. You can fix the issue and try again, or <a href="#" onclick="closeProvisionModal();return false">close</a> this window.</div>`;
  el.style.display = 'block';
  _setProvisionRunning(false);
}

function gotoUpdateTab() {
  closeProvisionModal();
  const btn = Array.from(document.querySelectorAll('nav button')).find(b => b.textContent.trim() === 'Updates');
  if (btn) showTab('update', btn);
}

// Log modal (polls while open — near–real-time tail of pserver.log)
let logLiveTimer = null;
let logLiveName = null;
let logActiveFile = 'pserver.log';
let logAllLines = [];

function _renderLogContent() {
  const el = $('logContent');
  const searchEl = $('logSearch');
  const term = searchEl ? searchEl.value.toLowerCase().trim() : '';
  const fromBottom = el.scrollHeight - el.scrollTop;
  const stickBottom = fromBottom <= el.clientHeight + 80;
  const lines = term ? logAllLines.filter(l => l.toLowerCase().includes(term)) : logAllLines;
  el.textContent = lines.join('\n');
  if (stickBottom) {
    el.scrollTop = el.scrollHeight;
  } else {
    el.scrollTop = Math.max(0, el.scrollHeight - fromBottom);
  }
}

function _applyLogText(text) {
  logAllLines = text ? text.split('\n') : [];
  _renderLogContent();
}

function applyLogSearch() {
  _renderLogContent();
}

async function fetchPalaceLogs(name) {
  if (!$('logModal').classList.contains('open') || name !== logLiveName) return;
  if (logActiveFile !== 'pserver.log') return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/logs?lines=500`, { headers: headers() });
    if (!res.ok) {
      _applyLogText(`Error: HTTP ${res.status}`);
      return;
    }
    const data = await res.json();
    _applyLogText((data.lines || []).join('\n'));
  } catch (e) {
    _applyLogText(`Error: ${e.message}`);
  }
}

function onLogAutoUpdateChange() {
  const checked = $('logAutoUpdate') && $('logAutoUpdate').checked;
  if (checked && logLiveName && logActiveFile === 'pserver.log') {
    if (!logLiveTimer) {
      logLiveTimer = setInterval(() => fetchPalaceLogs(logLiveName), 2000);
    }
  } else {
    if (logLiveTimer) {
      clearInterval(logLiveTimer);
      logLiveTimer = null;
    }
  }
}

async function _loadArchivedLog(name, fileName) {
  $('logContent').textContent = 'Loading…';
  logAllLines = [];
  try {
    const res = await fetch(
      `/api/palaces/${encodeURIComponent(name)}/server-files/${encodeURIComponent(fileName)}`,
      { headers: headers() }
    );
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      _applyLogText(`Error: ${data.error || ('HTTP ' + res.status)}`);
      return;
    }
    if (data.encoding === 'base64') {
      try {
        const binary = Uint8Array.from(atob(data.content), c => c.charCodeAt(0));
        const ds = new DecompressionStream('gzip');
        const decompressed = await new Response(
          new Blob([binary]).stream().pipeThrough(ds)
        ).text();
        _applyLogText(decompressed);
      } catch (e) {
        _applyLogText(`Error decompressing ${fileName}: ${e.message}`);
      }
    } else {
      _applyLogText(typeof data.content === 'string' ? data.content : '');
    }
  } catch (e) {
    _applyLogText(`Error: ${e.message}`);
  }
}

async function selectLogFile(name, fileName) {
  logActiveFile = fileName;

  // Update active state of tab buttons
  const sel = $('logFileSelector');
  for (const btn of sel.querySelectorAll('button')) {
    const btnFile = btn.dataset.file;
    btn.classList.toggle('active', btnFile === fileName);
  }

  // Stop live timer when viewing an archived file
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }

  if (fileName === 'pserver.log') {
    $('logContent').textContent = 'Loading…';
    await fetchPalaceLogs(name);
    const autoUpdate = $('logAutoUpdate');
    if (autoUpdate && autoUpdate.checked && !logLiveTimer) {
      logLiveTimer = setInterval(() => fetchPalaceLogs(name), 2000);
    }
  } else {
    await _loadArchivedLog(name, fileName);
  }
}

async function _loadLogFileSelector(name) {
  const sel = $('logFileSelector');
  sel.innerHTML = '<span style="color:var(--muted);font-size:11px;">Loading files…</span>';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/server-files`, { headers: headers() });
    if (!res.ok) { sel.innerHTML = ''; return; }
    const data = await res.json();
    const files = (data.files || []).filter(f => isPalaceServerLogFamily(f.name));
    // Sort: pserver.log first, then by name (pserver.log.1, pserver.log.1.gz, pserver.log.2, …)
    files.sort((a, b) => {
      if (a.name === 'pserver.log') return -1;
      if (b.name === 'pserver.log') return 1;
      return a.name.localeCompare(b.name);
    });
    sel.innerHTML = '';
    for (const f of files) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.dataset.file = f.name;
      btn.textContent = f.name === 'pserver.log' ? 'Current' : f.name;
      if (f.name === logActiveFile) btn.classList.add('active');
      btn.onclick = () => selectLogFile(name, f.name);
      sel.appendChild(btn);
    }
    if (files.length === 0) {
      sel.innerHTML = '<span style="color:var(--muted);font-size:11px;">No log files found.</span>';
    }
  } catch (_) {
    sel.innerHTML = '';
  }
}

async function viewLogs(name) {
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  logLiveName = name;
  logActiveFile = 'pserver.log';
  logAllLines = [];

  $('logModalTitle').textContent = `Logs — ${name}`;
  $('logContent').textContent = 'Loading…';
  $('logFileSelector').innerHTML = '';
  const searchEl = $('logSearch');
  if (searchEl) searchEl.value = '';
  const autoUpdate = $('logAutoUpdate');
  if (autoUpdate) autoUpdate.checked = true;

  $('logModal').classList.add('open');

  await Promise.all([
    _loadLogFileSelector(name),
    fetchPalaceLogs(name),
  ]);

  logLiveTimer = setInterval(() => fetchPalaceLogs(name), 2000);
}

function closeLogModal() {
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  logLiveName = null;
  logActiveFile = 'pserver.log';
  logAllLines = [];
  $('logModal').classList.remove('open');
}

// ===== Palace Server Stats (per-card polling) =====

// Map of palace name → { fetchTimer, uptimeTimer, startTime, lastData }
// lastData holds the most-recently-received stats payload so re-renders can
// restore values immediately without waiting for the next poll tick.
const PALACE_STAT_TIMERS = new Map();

// Returns a DOM-safe ID prefix for a palace's stats elements.
function palaceStatId(name) {
  return 'pstat-' + encodeURIComponent(name).replace(/[^a-zA-Z0-9_-]/g, '_');
}

function formatUptime(startIso) {
  if (!startIso) return '—';
  const elapsed = Math.max(0, Math.floor((Date.now() - new Date(startIso).getTime()) / 1000));
  const d = Math.floor(elapsed / 86400);
  const h = Math.floor((elapsed % 86400) / 3600);
  const m = Math.floor((elapsed % 3600) / 60);
  const s = elapsed % 60;
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  return `${m}m ${s}s`;
}

function setStatEl(sid, key, value) {
  const el = document.getElementById(`${sid}-${key}`);
  if (!el) return;
  const val = el.querySelector('.palace-stat-value');
  if (val) val.textContent = value;
}

// Write a full stats payload into the DOM elements for a given stat-strip ID.
function applyPalaceStats(sid, d) {
  setStatEl(sid, 'rooms',  d.room_count  ?? '—');
  setStatEl(sid, 'online', d.user_count  ?? '—');
  setStatEl(sid, 'max',    d.max_users   ?? '—');
  setStatEl(sid, 'today',  d.users_today ?? '—');
  setStatEl(sid, 'week',   d.users_week  ?? '—');
  setStatEl(sid, 'ops',    (d.operators ?? 0));
  setStatEl(sid, 'gods',   (d.gods ?? 0) + (d.hosts ?? 0));
  setStatEl(sid, 'owners', d.owners ?? '—');
  setStatEl(sid, 'uptime', formatUptime(d.start_time));
}

async function fetchPalaceStats(name) {
  const sid = palaceStatId(name);
  if (!document.getElementById(sid)) {
    // Stats strip not in DOM (palace collapsed) — stop polling.
    stopPalaceStatPolling(name);
    return;
  }
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/stats`, { headers: headers() });
    if (!res.ok) {
      setStatEl(sid, 'rooms', '—');
      setStatEl(sid, 'online', '—');
      return;
    }
    const d = await res.json();
    const entry = PALACE_STAT_TIMERS.get(name) || {};
    entry.startTime = d.start_time;
    entry.lastData  = d;
    PALACE_STAT_TIMERS.set(name, entry);

    applyPalaceStats(sid, d);
  } catch (_) {
    // Silently ignore fetch errors between polls.
  }
}

function tickPalaceUptime(name) {
  const sid = palaceStatId(name);
  if (!document.getElementById(sid)) return;
  const entry = PALACE_STAT_TIMERS.get(name);
  if (entry && entry.startTime) {
    setStatEl(sid, 'uptime', formatUptime(entry.startTime));
  }
}

function stopPalaceStatPolling(name) {
  const entry = PALACE_STAT_TIMERS.get(name);
  if (!entry) return;
  if (entry.fetchTimer)  clearInterval(entry.fetchTimer);
  if (entry.uptimeTimer) clearInterval(entry.uptimeTimer);
  PALACE_STAT_TIMERS.delete(name);
}

function startPalaceStatPolling(name) {
  // Already polling → keep running; timers survive the DOM re-render.
  if (PALACE_STAT_TIMERS.has(name)) return;
  fetchPalaceStats(name); // immediate first fetch
  const fetchTimer  = setInterval(() => fetchPalaceStats(name), 5000);
  const uptimeTimer = setInterval(() => tickPalaceUptime(name), 1000);
  PALACE_STAT_TIMERS.set(name, { fetchTimer, uptimeTimer, startTime: null, lastData: null });
}

// Called after loadPalaces() renders the DOM. Starts polls for newly-expanded
// palaces, stops polls for palaces no longer expanded/visible, and immediately
// restores any cached stat values so the DOM never flickers back to '—'.
function syncPalaceStatsPolling(expandedPalaces) {
  const activeNames = new Set(expandedPalaces.map(p => p.name));
  // Stop polls for palaces no longer visible.
  for (const name of PALACE_STAT_TIMERS.keys()) {
    if (!activeNames.has(name)) stopPalaceStatPolling(name);
  }
  for (const p of expandedPalaces) {
    const entry = PALACE_STAT_TIMERS.get(p.name);
    if (entry) {
      // Palace was already polling — DOM was just re-rendered with '—' placeholders.
      // Restore cached values immediately so there's no visible flicker.
      const sid = palaceStatId(p.name);
      if (entry.lastData)  applyPalaceStats(sid, entry.lastData);
      if (entry.startTime) setStatEl(sid, 'uptime', formatUptime(entry.startTime));
    } else {
      startPalaceStatPolling(p.name);
    }
  }
}

// ===== Palace Users Modal =====

let palaceUsersLiveName = null;
let palaceUsersTimer = null;

function formatSignonTime(secs) {
  const s = Math.max(0, Math.floor(secs));
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

async function fetchPalaceUsers(name) {
  if (!$('palaceUsersModal').classList.contains('open') || name !== palaceUsersLiveName) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/palace-users`, { headers: headers() });
    if (!res.ok) {
      $('palaceUsersBody').innerHTML = `<tr><td colspan="13" class="empty">Error: HTTP ${res.status}</td></tr>`;
      return;
    }
    const users = await res.json();
    if (!Array.isArray(users) || users.length === 0) {
      $('palaceUsersCount').textContent = '0 users';
      $('palaceUsersBody').innerHTML = '<tr><td colspan="13" class="empty">No users connected</td></tr>';
      return;
    }
    $('palaceUsersCount').textContent = `${users.length} user${users.length === 1 ? '' : 's'}`;
    $('palaceUsersBody').innerHTML = users.map(u => `
      <tr>
        <td><code>${u.id}</code></td>
        <td>${esc(u.role)}</td>
        <td><strong>${esc(u.name)}</strong></td>
        <td><code>${esc(u.client_version)}</code></td>
        <td>${esc(u.os || '?')}</td>
        <td>${esc(u.room_name)}</td>
        <td><code>${esc(u.ip)}</code></td>
        <td><code style="font-size:10px;">${esc(u.uuid || '')}</code></td>
        <td><code>${u.puid_ctr || 0}</code></td>
        <td><code>${esc(u.crc)}</code></td>
        <td><code>${u.cnt || 0}</code></td>
        <td><code>${esc(u.wiz_key)}</code></td>
        <td>${formatSignonTime(u.signon_seconds)}</td>
      </tr>`).join('');
  } catch (e) {
    $('palaceUsersBody').innerHTML = `<tr><td colspan="13" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

async function openPalaceUsersModal(name) {
  if (palaceUsersTimer) {
    clearInterval(palaceUsersTimer);
    palaceUsersTimer = null;
  }
  palaceUsersLiveName = name;
  $('palaceUsersModalTitle').textContent = `Connected Users — ${name}`;
  $('palaceUsersCount').textContent = '';
  $('palaceUsersBody').innerHTML = '<tr><td colspan="13" class="empty">Loading…</td></tr>';
  $('palaceUsersModal').classList.add('open');
  await fetchPalaceUsers(name);
  palaceUsersTimer = setInterval(() => fetchPalaceUsers(name), 5000);
}

function closePalaceUsersModal() {
  if (palaceUsersTimer) {
    clearInterval(palaceUsersTimer);
    palaceUsersTimer = null;
  }
  palaceUsersLiveName = null;
  $('palaceUsersModal').classList.remove('open');
}
