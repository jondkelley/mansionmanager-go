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
        return `
      <tr>
        <td><strong>${esc(p.name)}</strong> <span class="badge badge-unregistered" title="Not in registry yet">Not registered</span></td>
        <td><span class="palace-status"><span class="status-dot ${dotClass}" title="${esc(title)}" aria-hidden="true"></span><span class="badge badge-${esc(p.status)}">${esc(p.status)}</span></span></td>
        <td>${p.tcpPort || '—'}</td>
        <td>${p.httpPort || '—'}</td>
        <td>
          <div class="actions">
            <button type="button" class="primary" onclick='openRegisterPalaceModal(${nm})'>Register…</button>
            <button type="button" onclick='palaceAction(${nm},"start")'>Start</button>
            <button type="button" onclick='palaceAction(${nm},"stop")'>Stop</button>
            <button type="button" onclick='palaceAction(${nm},"restart")'>Restart</button>
            ${orphanSettingsBtn}
            ${removeBtn}
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
      const backupBtn = `<button type="button" onclick='downloadPalaceHomeBackup(${nm})' title="Download tar.gz of this palace user's home directory (gzip -9)">Media Backup</button>`;
      const summaryClass = isAdmin ? 'palace-row-summary' : '';
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
            <div class="palace-detail-block" style="margin-left:auto;">
              <span class="palace-detail-label">Actions</span>
              <div class="palace-detail-actions">
                ${mediaBtn}
                ${backupBtn}
                <button type="button" onclick='palaceAction(${nm},"stop")'>Stop</button>
                <button type="button" onclick='palaceAction(${nm},"start")'>Start</button>
                <button type="button" onclick='palaceAction(${nm},"restart")'>Restart</button>
                ${settingsBtn}
                ${logsBtn}
                ${removeBtn}
              </div>
            </div>
          </div>
        </td> 
      </tr>`;
    }).join('');
  } catch(e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">Error: ${esc(e.message)}</td></tr>`;
    if (unregPanel) unregPanel.style.display = 'none';
  }
}

async function palaceAction(name, action) {
  await fetch(`/api/palaces/${encodeURIComponent(name)}/${action}`, { method: 'POST', headers: headers() });
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
    let fname = `${name}-home-backup.tar.gz`;
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
  try {
    const q = purge ? '?purge=true' : '';
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}${q}`, {
      method: 'DELETE',
      headers: headers(),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
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
      `<span style="color:var(--muted);">→ <a href="#" onclick="gotoUpdateTab();return false" style="color:var(--accent);">Go to Update Binary</a> and click <strong>Update Binary</strong> to download the pserver template, then come back here.</span>`;
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
  const btn = Array.from(document.querySelectorAll('nav button')).find(b => b.textContent.trim() === 'Update Binary');
  if (btn) showTab('update', btn);
}

// Log modal (polls while open — near–real-time tail of pserver.log)
let logLiveTimer = null;
let logLiveName = null;

function _applyLogText(text) {
  const el = $('logContent');
  const fromBottom = el.scrollHeight - el.scrollTop;
  const stickBottom = fromBottom <= el.clientHeight + 80;
  el.textContent = text;
  if (stickBottom) {
    el.scrollTop = el.scrollHeight;
  } else {
    el.scrollTop = Math.max(0, el.scrollHeight - fromBottom);
  }
}

async function fetchPalaceLogs(name) {
  if (!$('logModal').classList.contains('open') || name !== logLiveName) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/logs?lines=200`, { headers: headers() });
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

async function viewLogs(name) {
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  logLiveName = name;
  $('logModalTitle').textContent = `Logs — ${name} · auto-refresh`;
  $('logContent').textContent = 'Loading...';
  $('logModal').classList.add('open');

  await fetchPalaceLogs(name);
  logLiveTimer = setInterval(() => fetchPalaceLogs(name), 2000);
}

function closeLogModal() {
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  logLiveName = null;
  $('logModal').classList.remove('open');
}
