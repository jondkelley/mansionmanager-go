// ===== pserver Binary Update =====

let _pserverStatusPollTimer = null;

async function loadPserverUpdateStatus() {
  const banner     = $('pserverUpdateBanner');
  const verEl      = $('pserverCurrentVersion');
  const lastRunEl  = $('pserverLastRun');
  const pullBtn    = $('pserverPullBtn');
  if (!banner) return;

  try {
    const res = await fetch('/api/pserver/update-status', { headers: headers() });
    if (!res.ok) return;
    const d = await res.json();

    // Version badge
    if (verEl) verEl.textContent = d.lastVersion ? d.lastVersion : '—';

    // Last-run label
    if (lastRunEl) {
      if (d.lastRun) {
        try {
          const dt = new Date(d.lastRun);
          lastRunEl.textContent = 'Last updated ' + dt.toLocaleString();
        } catch (_) {
          lastRunEl.textContent = 'Last updated ' + d.lastRun;
        }
      } else {
        lastRunEl.textContent = 'Not yet updated this session';
      }
    }

    // Banner
    if (d.running) {
      banner.className = 'pserver-update-banner pserver-update-banner-running';
      banner.innerHTML =
        '<span class="pserver-update-banner-icon">↻</span>' +
        '<span><strong>Pulling latest version…</strong> Downloading and installing the newest pserver binary.</span>';
      if (pullBtn) { pullBtn.disabled = true; pullBtn.textContent = 'Pulling…'; }
    } else if (d.lastErr) {
      banner.className = 'pserver-update-banner pserver-update-banner-error';
      banner.innerHTML =
        '<span class="pserver-update-banner-icon">✗</span>' +
        '<span><strong>Last update failed:</strong> ' + esc(d.lastErr) + '</span>';
      if (pullBtn) { pullBtn.disabled = false; pullBtn.textContent = 'Try Again'; }
    } else if (d.lastVersion) {
      banner.className = 'pserver-update-banner pserver-update-banner-ok';
      banner.innerHTML =
        '<span class="pserver-update-banner-icon">✓</span>' +
        '<span>You are on the latest pserver — auto-updates every ' + (d.intervalHours || 2) + '\u00a0hours.</span>';
      if (pullBtn) { pullBtn.disabled = false; pullBtn.textContent = 'Pull Latest Now'; }
    } else {
      banner.className = 'pserver-update-banner pserver-update-banner-neutral';
      banner.innerHTML =
        '<span class="pserver-update-banner-icon">↻</span>' +
        '<span>Auto-updates run every ' + (d.intervalHours || 2) + '\u00a0hours. ' +
        'Click <strong>Pull Latest Now</strong> to update immediately.</span>';
      if (pullBtn) { pullBtn.disabled = false; pullBtn.textContent = 'Pull Latest Now'; }
    }
  } catch (_) {}
}

function _startPserverStatusPoll() {
  _stopPserverStatusPoll();
  // Poll every 3 s while update is running, every 30 s otherwise.
  _pserverStatusPollTimer = setInterval(async () => {
    await loadPserverUpdateStatus();
  }, 3000);
}

function _stopPserverStatusPoll() {
  if (_pserverStatusPollTimer) {
    clearInterval(_pserverStatusPollTimer);
    _pserverStatusPollTimer = null;
  }
}

async function runUpdate(restartAll) {
  const stream  = $('updateStream');
  const pullBtn = $('pserverPullBtn');
  stream.textContent = '';
  stream.style.display = '';

  if (pullBtn) { pullBtn.disabled = true; pullBtn.textContent = 'Pulling…'; }

  // Immediately show "pulling" state in banner without waiting for next poll.
  const banner = $('pserverUpdateBanner');
  if (banner) {
    banner.className = 'pserver-update-banner pserver-update-banner-running';
    banner.innerHTML =
      '<span class="pserver-update-banner-icon">↻</span>' +
      '<span><strong>Pulling latest version…</strong> Downloading and installing the newest pserver binary.</span>';
  }

  _startPserverStatusPoll();

  const res = await fetch(`/api/update?restartAll=${restartAll}`, { method: 'POST', headers: headers() });
  await streamSSE(res, stream);

  _stopPserverStatusPoll();
  await loadPserverUpdateStatus();
  await loadRolloutPanel();
}

// ===== Rollout panel =====

function semverOptionsHTML(snap, selected) {
  const opts = [];
  opts.push(`<option value="latest"${selected === 'latest' ? ' selected' : ''}>latest — always tracks newest</option>`);
  for (const v of snap.versions || []) {
    const lab = `${esc(v.semver)} — ${esc(v.target || '?')} · archived ${esc((v.archivedAt || '').slice(0,10))}`;
    opts.push(`<option value="${attrEsc(v.semver)}"${selected === v.semver ? ' selected' : ''}>${lab}</option>`);
  }
  return opts.join('');
}

async function loadRolloutPanel() {
  const meta   = $('rolloutMeta');
  const tbody  = $('rolloutBody');
  const allSel = $('rolloutAllSemver');
  try {
    const [vRes, pRes] = await Promise.all([
      fetch('/api/binary-versions', { headers: headers() }),
      fetch('/api/palaces',         { headers: headers() }),
    ]);
    if (!vRes.ok) {
      meta.textContent = 'Could not load binary versions.';
      tbody.innerHTML = `<tr><td colspan="4" class="empty">HTTP ${vRes.status}</td></tr>`;
      return;
    }
    const snap    = await vRes.json();
    const palaces = pRes.ok ? await pRes.json() : [];
    const t = snap.template || {};
    meta.innerHTML =
      `<strong>Template:</strong> <code>${esc(t.semver || t.tag || '—')}</code>` +
      (t.target ? ` &nbsp;·&nbsp; <code>${esc(t.target)}</code>` : '') +
      `<br><strong>Shared binary:</strong> <code>${esc(snap.installPath)}</code>` +
      ` &nbsp;·&nbsp; <strong>Archive:</strong> <code>${esc(snap.versionsDir)}</code>`;

    const commonSel = allSel ? allSel.value : 'latest';
    if (allSel) allSel.innerHTML = semverOptionsHTML(snap, commonSel);

    if (!Array.isArray(palaces) || palaces.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="empty">No palaces yet. Provision one on the Palaces tab.</td></tr>';
      return;
    }

    tbody.innerHTML = palaces.map(p => {
      const pin  = !p.pserverVersion || p.pserverVersion === 'latest' ? 'latest' : p.pserverVersion;
      const isLatest = pin === 'latest';
      const enc  = encodeURIComponent(p.name);
      const selHtml = semverOptionsHTML(snap, pin);

      const pinBadge = isLatest
        ? `<span class="badge badge-tracking-latest" title="Will use the newest pserver binary on next restart">tracking latest</span>`
        : `<span class="badge badge-pinned" title="Pinned to a specific pserver build">pinned <code style="font-size:11px;">${esc(pin)}</code></span>`;

      return `<tr data-pn="${enc}">
        <td><strong>${esc(p.name)}</strong></td>
        <td>${pinBadge}</td>
        <td>
          <select class="rollout-sel" style="min-width:260px;">${selHtml}</select>
          <label style="margin-left:10px;font-size:12px;color:var(--muted);white-space:nowrap;"><input type="checkbox" class="rollout-restart"/> Restart now</label>
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
  const name   = decodeURIComponent(tr.dataset.pn);
  const sel    = tr.querySelector('.rollout-sel');
  const semver = sel ? sel.value : 'latest';
  const restartEl = tr.querySelector('.rollout-restart');
  const restart   = restartEl ? restartEl.checked : false;
  if (restart && !confirm(`Restart service for palace "${name}" after switching to "${semver}"?`)) return;

  const res  = await fetch(`/api/palaces/${encodeURIComponent(name)}/pserver-version`, {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ semver, restart }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) { alert(data.error || `HTTP ${res.status}`); return; }
  loadPalaces();
  loadRolloutPanel();
}

async function rolloutApplyAll() {
  const semver  = $('rolloutAllSemver').value;
  const restart = $('rolloutAllRestart').checked;
  if (!confirm(`Set ALL palaces to "${semver}"${restart ? ' and restart every service' : ''}?`)) return;
  const res  = await fetch('/api/rollout', {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify({ semver, restart }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) { alert(data.error || `HTTP ${res.status}`); return; }
  loadPalaces();
  loadRolloutPanel();
}

// ===== Manager version badge + self-update =====

function semverLt(a, b) {
  const parse = s => (s || 'dev').replace(/^v/, '').split(/[-+]/)[0]
    .split('.').map(n => parseInt(n, 10) || 0);
  const [aM, am, ap] = parse(a);
  const [bM, bm, bp] = parse(b);
  if (aM !== bM) return aM < bM;
  if (am !== bm) return am < bm;
  return ap < bp;
}

(function initVersionBadge() {
  const ver   = window.__PM_VERSION__ || 'dev';
  const hash  = window.__PM_GIT_HASH__ || '';
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

  const curEl          = $('mgrCurrentVersion');
  const latestEl       = $('mgrLatestVersion');
  const hashEl         = $('mgrGitHashBadge');
  const releaseLink    = $('mgrReleaseLink');
  const upToDate       = $('mgrUpToDate');
  const updateAvail    = $('mgrUpdateAvailable');
  const updateAvailTxt = $('mgrUpdateAvailableText');
  const releaseSummary = $('mgrReleaseSummary');
  const dot            = $('updateDot');

  if (curEl) curEl.textContent = window.__PM_VERSION__ || 'dev';
  if (hashEl) {
    const h = window.__PM_GIT_HASH__ || '';
    hashEl.textContent = h ? '(' + h + ')' : '';
  }
  if (latestEl) latestEl.textContent = 'checking…';
  if (upToDate)    upToDate.style.display    = 'none';
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
        if (upToDate)    upToDate.style.display    = 'none';
        if (updateAvailTxt) {
          let txt = latest.tag;
          if (latest.publishedAt) {
            try { txt += ' — published ' + new Date(latest.publishedAt).toLocaleDateString(); } catch (_) {}
          }
          updateAvailTxt.textContent = txt;
        }
        if (releaseSummary && latest.summary) releaseSummary.textContent = latest.summary;
        if (dot) dot.style.display = '';
      } else {
        if (upToDate)    upToDate.style.display    = '';
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

// Called after login for admin users — silently checks for manager updates.
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
  const stream    = $('mgrUpdateStream');
  const btn       = $('mgrUpdateBtn');
  const reconnect = $('mgrReconnectBox');

  stream.textContent  = '';
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
    startManagerReconnect();
  } finally {
    if (btn) btn.disabled = false;
  }
}

function startManagerReconnect() {
  const reconnect = $('mgrReconnectBox');
  const dots      = $('mgrReconnectDots');
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
        window.__PM_VERSION__     = cfg.version     || window.__PM_VERSION__;
        window.__PM_GIT_HASH__    = cfg.gitHash     || '';
        window.__PM_GITHUB_REPO__ = cfg.githubRepo  || window.__PM_GITHUB_REPO__;

        const badge = $('managerVersionBadge');
        if (badge) {
          const ver  = window.__PM_VERSION__ || 'dev';
          const hash = window.__PM_GIT_HASH__ || '';
          badge.textContent = ver;
          if (hash) badge.innerHTML = ver + ' <span class="git-hash">(' + esc(hash) + ')</span>';
        }

        if (reconnect) {
          reconnect.style.background   = 'rgba(95,212,160,.08)';
          reconnect.style.borderColor  = 'rgba(95,212,160,.3)';
          reconnect.style.color        = 'var(--green)';
          reconnect.innerHTML = '<strong>Palace Manager updated successfully!</strong> Refreshing page…';
        }
        setTimeout(() => location.reload(), 2000);
      }
    } catch (_) {}
  }, 2000);
}
