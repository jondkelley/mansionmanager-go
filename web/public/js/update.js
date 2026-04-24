// ===== Update Binary =====
async function runUpdate(restartAll) {
  const stream = $('updateStream');
  stream.textContent = '';
  const res = await fetch(`/api/update?restartAll=${restartAll}`, { method: 'POST', headers: headers() });
  await streamSSE(res, stream);
  loadRolloutPanel();
}

function semverOptionsHTML(snap, selected) {
  const opts = [];
  opts.push(`<option value="latest"${selected === 'latest' ? ' selected' : ''}>latest (${esc(snap.installPath || '/usr/local/bin/pserver')})</option>`);
  for (const v of snap.versions || []) {
    const lab = `${esc(v.semver)} — ${esc(v.target || '?')} · ${esc(v.archivedAt || '')}`;
    opts.push(`<option value="${attrEsc(v.semver)}"${selected === v.semver ? ' selected' : ''}>${lab}</option>`);
  }
  return opts.join('');
}

async function loadRolloutPanel() {
  const meta = $('rolloutMeta');
  const tbody = $('rolloutBody');
  const allSel = $('rolloutAllSemver');
  try {
    const [vRes, pRes] = await Promise.all([
      fetch('/api/binary-versions', { headers: headers() }),
      fetch('/api/palaces', { headers: headers() }),
    ]);
    if (!vRes.ok) {
      meta.textContent = 'Could not load binary versions.';
      tbody.innerHTML = `<tr><td colspan="4" class="empty">HTTP ${vRes.status}</td></tr>`;
      return;
    }
    const snap = await vRes.json();
    const palaces = pRes.ok ? await pRes.json() : [];
    const t = snap.template || {};
    meta.innerHTML =
      `<strong>Template</strong> (live): semver <code>${esc(t.semver || '—')}</code>, tag <code>${esc(t.tag || '—')}</code>, target <code>${esc(t.target || '—')}</code><br>` +
      `<strong>Archive</strong>: <code>${esc(snap.versionsDir)}</code> · <code>versions.json</code> · shared binary <code>${esc(snap.installPath)}</code>`;

    const commonSel = allSel ? allSel.value : 'latest';
    if (allSel) {
      allSel.innerHTML = semverOptionsHTML(snap, commonSel);
    }

    if (!Array.isArray(palaces) || palaces.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty">No palaces yet. Provision one on the Palaces tab.</td></tr>';
      return;
    }

    tbody.innerHTML = palaces.map(p => {
      const pin = !p.pserverVersion || p.pserverVersion === 'latest' ? 'latest' : p.pserverVersion;
      const cur = esc(p.pserverVersion || 'latest');
      const enc = encodeURIComponent(p.name);
      const selHtml = semverOptionsHTML(snap, pin);
      return `<tr data-pn="${enc}">
        <td><strong>${esc(p.name)}</strong></td>
        <td><code>${cur}</code></td>
        <td>
          <select class="rollout-sel" style="min-width:260px;">${selHtml}</select>
          <label style="margin-left:10px;font-size:12px;color:var(--muted);white-space:nowrap;"><input type="checkbox" class="rollout-restart"/> Restart</label>
        </td>
        <td><button type="button" onclick="rolloutApplyRow(this)">Apply</button></td>
      </tr>`;
    }).join('');
  } catch (e) {
    meta.textContent = '';
    tbody.innerHTML = `<tr><td colspan="4" class="empty">Error: ${esc(e.message)}</td></tr>`;
  }
}

async function rolloutApplyRow(btn) {
  const tr = btn.closest('tr');
  if (!tr || !tr.dataset.pn) return;
  const name = decodeURIComponent(tr.dataset.pn);
  const sel = tr.querySelector('.rollout-sel');
  const semver = sel ? sel.value : 'latest';
  const restartEl = tr.querySelector('.rollout-restart');
  const restart = restartEl ? restartEl.checked : false;
  if (restart && !confirm(`Restart service for palace "${name}" after switching to "${semver}"?`)) {
    return;
  }
  const body = JSON.stringify({ semver, restart });
  const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/pserver-version`, {
    method: 'POST',
    headers: headers(),
    body,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    alert(data.error || `HTTP ${res.status}`);
    return;
  }
  loadPalaces();
  loadRolloutPanel();
}

async function rolloutApplyAll() {
  const semver = $('rolloutAllSemver').value;
  const restart = $('rolloutAllRestart').checked;
  if (!confirm(`Set ALL palaces to "${semver}"${restart ? ' and restart every service' : ''}?`)) return;
  const res = await fetch('/api/rollout', {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ semver, restart }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    alert(data.error || `HTTP ${res.status}`);
    return;
  }
  loadPalaces();
  loadRolloutPanel();
}

// ===== Manager version badge + self-update =====

function semverLt(a, b) {
  // Returns true when a is strictly older than b (semantic version comparison).
  const parse = s => (s || 'dev').replace(/^v/, '').split(/[-+]/)[0]
    .split('.').map(n => parseInt(n, 10) || 0);
  const [aM, am, ap] = parse(a);
  const [bM, bm, bp] = parse(b);
  if (aM !== bM) return aM < bM;
  if (am !== bm) return am < bm;
  return ap < bp;
}

(function initVersionBadge() {
  const ver = window.__PM_VERSION__ || 'dev';
  const hash = window.__PM_GIT_HASH__ || '';
  const badge = $('managerVersionBadge');
  if (!badge) return;
  badge.textContent = ver;
  if (hash) {
    badge.title = 'git ' + hash;
    badge.innerHTML = ver + ' <span class="git-hash">(' + esc(hash) + ')</span>';
  }
})();

async function loadManagerVersionInfo() {
  const repo = window.__PM_GITHUB_REPO__ || '';
  const card = $('managerUpdateCard');
  if (!card) return;
  if (!repo) { card.style.display = 'none'; return; }
  card.style.display = '';

  const curEl = $('mgrCurrentVersion');
  const latestEl = $('mgrLatestVersion');
  const hashEl = $('mgrGitHashBadge');
  const releaseLink = $('mgrReleaseLink');
  const upToDate = $('mgrUpToDate');
  const updateAvail = $('mgrUpdateAvailable');
  const updateAvailText = $('mgrUpdateAvailableText');
  const releaseSummary = $('mgrReleaseSummary');
  const dot = $('updateDot');

  if (curEl) curEl.textContent = window.__PM_VERSION__ || 'dev';
  if (hashEl) {
    const h = window.__PM_GIT_HASH__ || '';
    hashEl.textContent = h ? '(' + h + ')' : '';
  }
  if (latestEl) latestEl.textContent = 'checking…';
  if (upToDate) upToDate.style.display = 'none';
  if (updateAvail) updateAvail.style.display = 'none';

  try {
    const res = await fetch('/api/manager/version', { headers: headers() });
    if (!res.ok) { if (latestEl) latestEl.textContent = 'unavailable'; return; }
    const data = await res.json();

    if (curEl) curEl.textContent = data.current || window.__PM_VERSION__ || 'dev';

    const latest = data.latest;
    if (latest && latest.tag) {
      if (latestEl) {
        latestEl.textContent = latest.tag;
        latestEl.style.color = data.updateAvailable ? 'var(--green)' : 'var(--muted)';
      }
      if (releaseLink && latest.releaseUrl) {
        releaseLink.href = latest.releaseUrl;
        releaseLink.style.display = '';
      }

      if (data.updateAvailable) {
        if (updateAvail) updateAvail.style.display = '';
        if (upToDate) upToDate.style.display = 'none';
        if (updateAvailText) {
          let txt = latest.tag;
          if (latest.publishedAt) {
            try {
              const d = new Date(latest.publishedAt);
              txt += ' — published ' + d.toLocaleDateString();
            } catch (_) {}
          }
          updateAvailText.textContent = txt;
        }
        if (releaseSummary && latest.summary) {
          releaseSummary.textContent = latest.summary;
        }
        // Pulse the nav badge.
        if (dot) dot.style.display = '';
      } else {
        if (upToDate) upToDate.style.display = '';
        if (updateAvail) updateAvail.style.display = 'none';
        if (dot) dot.style.display = 'none';
        if (latestEl) latestEl.style.color = 'var(--muted)';
      }
    } else {
      if (latestEl) latestEl.textContent = '(unavailable)';
    }
  } catch (e) {
    if (latestEl) latestEl.textContent = 'unavailable';
  }
}

// Called after login for admin users — silently checks for updates and lights
// the nav dot if a newer release is available.
async function silentUpdateCheck() {
  const repo = window.__PM_GITHUB_REPO__ || '';
  if (!repo) return;
  try {
    const res = await fetch('/api/manager/version', { headers: headers() });
    if (!res.ok) return;
    const data = await res.json();
    const dot = $('updateDot');
    if (dot) dot.style.display = data.updateAvailable ? '' : 'none';
  } catch (_) {}
}

let mgrReconnectTimer = null;

async function runManagerUpdate() {
  const stream = $('mgrUpdateStream');
  const btn = $('mgrUpdateBtn');
  const reconnect = $('mgrReconnectBox');

  stream.textContent = '';
  stream.style.display = '';
  if (reconnect) reconnect.style.display = 'none';
  if (btn) btn.disabled = true;

  try {
    const res = await fetch('/api/manager/update', { method: 'POST', headers: headers() });
    let restarting = false;
    await streamSSE(res, stream, okObj => {
      if (okObj && okObj.restarting) restarting = true;
    });
    if (restarting) startManagerReconnect();
  } catch (e) {
    const p = document.createElement('p');
    p.style.color = 'var(--red)';
    p.textContent = 'Connection lost — ' + e.message;
    stream.appendChild(p);
    // Dropped connection almost certainly means the service is restarting.
    startManagerReconnect();
  } finally {
    if (btn) btn.disabled = false;
  }
}

function startManagerReconnect() {
  const reconnect = $('mgrReconnectBox');
  const dots = $('mgrReconnectDots');
  if (reconnect) reconnect.style.display = '';
  if (mgrReconnectTimer) clearInterval(mgrReconnectTimer);

  let attempt = 0;
  mgrReconnectTimer = setInterval(async () => {
    attempt++;
    if (dots) dots.textContent = '.'.repeat((attempt % 4) + 1);
    try {
      const res = await fetch('/api/ui/config').catch(() => null);
      if (res && res.ok) {
        clearInterval(mgrReconnectTimer);
        mgrReconnectTimer = null;
        const cfg = await res.json().catch(() => ({}));
        window.__PM_VERSION__ = cfg.version || window.__PM_VERSION__;
        window.__PM_GIT_HASH__ = cfg.gitHash || '';
        window.__PM_GITHUB_REPO__ = cfg.githubRepo || window.__PM_GITHUB_REPO__;

        // Update badge with new version + hash.
        const badge = $('managerVersionBadge');
        if (badge) {
          const ver = window.__PM_VERSION__ || 'dev';
          const hash = window.__PM_GIT_HASH__ || '';
          badge.textContent = ver;
          if (hash) badge.innerHTML = ver + ' <span class="git-hash">(' + esc(hash) + ')</span>';
        }

        if (reconnect) {
          reconnect.style.background = 'rgba(95,212,160,.08)';
          reconnect.style.borderColor = 'rgba(95,212,160,.3)';
          reconnect.style.color = 'var(--green)';
          reconnect.innerHTML = '<strong>Palace Manager updated successfully!</strong> Refreshing page…';
        }
        setTimeout(() => location.reload(), 2000);
      }
    } catch (_) {}
  }, 2000);
}
