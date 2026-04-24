// ===== Nginx =====
async function loadNginxSettingsForm() {
  const note = $('ngxSettingsNote');
  try {
    const res = await fetch('/api/nginx/settings', { headers: headers() });
    if (!res.ok) {
      if (note) note.textContent = 'Could not load settings (HTTP ' + res.status + ').';
      return;
    }
    const s = await res.json();
    if ($('ngxMediaHost')) $('ngxMediaHost').value = s.mediaHost || '';
    if ($('ngxEdgeScheme')) $('ngxEdgeScheme').value = s.edgeScheme || 'https';
    if ($('ngxMatchScheme')) $('ngxMatchScheme').value = s.matchScheme || 'both';
    if ($('ngxCertDir')) $('ngxCertDir').value = s.certDir || '';
    const rp = $('ngxComputedRp');
    if (rp) rp.innerHTML = 'Effective <code>--reverseproxymedia</code>: <strong>' + esc(s.reverseProxyMedia || '') + '</strong>';
    if (note) note.textContent = '';
  } catch (e) {
    if (note) note.textContent = 'Error: ' + e.message;
  }
}

async function checkMediaHostDNS(silent) {
  const hint = $('ngxDnsHint');
  const host = $('ngxMediaHost') ? $('ngxMediaHost').value.trim() : '';
  if (!host) { if (hint) hint.textContent = ''; return null; }
  if (hint) { hint.style.color = 'var(--muted)'; hint.textContent = 'Checking DNS…'; }
  try {
    const res = await fetch('/api/nginx/dns-check?host=' + encodeURIComponent(host), { headers: headers() });
    if (!res.ok) { if (hint) hint.textContent = ''; return null; }
    const d = await res.json();
    if (hint) {
      if (d.lookupError) {
        hint.style.color = 'var(--danger, #c0392b)';
        hint.textContent = '⚠ DNS lookup failed: ' + d.lookupError;
      } else if (!d.match) {
        hint.style.color = 'var(--danger, #c0392b)';
        const resolved = (d.resolvedTo || []).join(', ') || '?';
        const server = d.serverIP ? ' (this machine: ' + d.serverIP + ')' : '';
        hint.textContent = '⚠ ' + host + ' resolves to ' + resolved + server + ' — no match found on this machine.';
      } else {
        hint.style.color = 'var(--success, #27ae60)';
        const resolved = (d.resolvedTo || []).join(', ');
        hint.textContent = '✓ ' + host + ' → ' + resolved;
      }
    }
    return d;
  } catch (_) {
    if (hint) hint.textContent = '';
    return null;
  }
}

async function saveNginxSettings() {
  // DNS sanity check before the main confirm.
  const dnsResult = await checkMediaHostDNS();
  if (dnsResult && !dnsResult.match) {
    const reason = dnsResult.lookupError
      ? 'DNS lookup failed: ' + dnsResult.lookupError
      : 'No DNS record was found pointing to this machine. Media may not work correctly.';
    if (!confirm(reason + '\n\nContinue saving anyway?')) return;
  }

  const restartAll = $('ngxRestartAll') && $('ngxRestartAll').checked;
  const q = restartAll
    ? 'Save settings, rewrite all palman systemd units with the new --reverseproxymedia URL, regenerate nginx, and restart every palace service?'
    : 'Save settings and rewrite systemd units + nginx config? Running pservers keep the old advertised URL until you restart each palace (or enable restart below and save again).';
  if (!confirm(q)) return;
  const note = $('ngxSettingsNote');
  try {
    const body = {
      mediaHost: $('ngxMediaHost').value.trim(),
      edgeScheme: $('ngxEdgeScheme').value,
      matchScheme: $('ngxMatchScheme').value,
      certDir: $('ngxCertDir').value.trim(),
      restartAll,
    };
    const res = await fetch('/api/nginx/settings', {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify(body),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    let msg = 'Saved.';
    if (data.unitsRewritten) msg += ' Systemd units updated.';
    if (data.restartWarning) msg += ' Restart warning: ' + data.restartWarning;
    else if (restartAll) msg += ' All palace services restarted.';
    if (note) note.textContent = msg;
    loadNginxSettingsForm();
    loadNginxStatus();
  } catch (e) {
    alert(e.message);
  }
}

async function loadNginxStatus() {
  const info = $('nginxInfo');
  try {
    const res = await fetch('/api/nginx/status', { headers: headers() });
    const s = await res.json();
    const lastRun = s.lastRun ? new Date(s.lastRun).toLocaleString() : 'never';
    const nextRun = s.nextRun ? new Date(s.nextRun).toLocaleString() : '—';
    const exitBadge = s.exitCode === 0 ? '<span class="badge badge-ok">ok</span>' : `<span class="badge badge-failed">exit ${s.exitCode}</span>`;
    info.innerHTML = `
      <div class="nginx-stat"><span class="label">Last Run</span><span class="value">${lastRun}</span></div>
      <div class="nginx-stat"><span class="label">Exit Code</span><span class="value">${exitBadge}</span></div>
      <div class="nginx-stat"><span class="label">Next Run</span><span class="value">${nextRun}</span></div>
    `;
    if (s.output) {
      $('nginxStream').textContent = s.output;
    }
  } catch(e) {
    info.textContent = 'Error: ' + e.message;
  }
}

async function triggerNginxRegen() {
  const stream = $('nginxStream');
  stream.textContent = '';
  const res = await fetch('/api/nginx/regen', { method: 'POST', headers: headers() });
  await streamSSE(res, stream);
  loadNginxStatus();
}

// ===== Bootstrap / Setup =====
const STEP_LABELS = {
  deps: 'System Dependencies',
  dns: 'DNS Check',
  cert: "Let's Encrypt Certificate",
  dhparam: 'DH Parameters',
  hook: 'Certbot Renewal Hook',
  nginx: 'Nginx Config Generation',
  config: 'Manager Config',
};

async function loadBootstrapStatus() {
  const list = $('stepList');
  try {
    const res = await fetch('/api/bootstrap/status', { headers: headers() });
    const steps = await res.json();
    list.innerHTML = steps.map(s => `
      <div class="step-row">
        <span class="step-id">${esc(s.id)}</span>
        <span style="flex:0 0 auto">${badgeForState(s.state)}</span>
        <span class="step-msg">${esc(STEP_LABELS[s.id] || s.id)}${s.message ? ' — ' + esc(s.message) : ''}</span>
      </div>`).join('');
  } catch(e) {
    list.innerHTML = `<span style="color:var(--red)">Error: ${esc(e.message)}</span>`;
  }
}

function badgeForState(state) {
  const map = { ok: 'badge-ok', failed: 'badge-failed', skipped: 'badge-inactive', unknown: 'badge-unknown' };
  return `<span class="badge ${map[state] || 'badge-unknown'}">${esc(state)}</span>`;
}

async function enableLogrotateAll() {
  const stream = $('setupStream');
  stream.textContent = '';
  const res = await fetch('/api/host/logrotate-enable-all', {
    method: 'POST',
    headers: headers(),
  });
  await streamSSE(res, stream);
}

async function runBootstrap() {
  const stream = $('setupStream');
  stream.textContent = '';
  const body = {
    mediaHost: $('ngxMediaHost') ? $('ngxMediaHost').value.trim() || undefined : undefined,
    email: $('setupEmail').value.trim() || undefined,
    staging: $('setupStaging').value === 'true',
    edgeScheme: $('ngxEdgeScheme') ? $('ngxEdgeScheme').value : undefined,
  };
  const res = await fetch('/api/bootstrap/run', {
    method: 'POST',
    headers: headers(),
    body: JSON.stringify(body),
  });
  await streamSSE(res, stream, () => loadBootstrapStatus());
}
