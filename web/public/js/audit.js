function syncAuditFilterUI() {
  const admin = SESSION && SESSION.role === 'admin';
  const tenant = SESSION && SESSION.role === 'tenant';
  const tw = $('auditFilterTenantWrap');
  if (tw) tw.style.display = admin ? '' : 'none';
  const hint = $('auditFilterHint');
  const label = $('auditFilterActorLabel');
  if (!hint || !label) return;
  if (tenant) {
    hint.style.display = '';
    hint.textContent =
      'Optional: enter a subaccount username to show only actions performed under that login.';
    label.textContent = 'Subaccount';
  } else if (admin) {
    hint.style.display = '';
    hint.textContent =
      'Optional: filter by manager username (any role). Tenant filter limits rows to that tenant’s palaces and account-scoped actions.';
    label.textContent = 'Actor';
  } else {
    hint.style.display = 'none';
    label.textContent = 'Actor';
  }
}

function resetAuditFilters() {
  const p = $('auditFilterPalace');
  if (p) p.value = '';
  const t = $('auditFilterTenant');
  if (t) t.value = '';
  const a = $('auditFilterActor');
  if (a) a.value = '';
  const lim = $('auditFilterLimit');
  if (lim) lim.value = '200';
}

function formatAuditDetail(d) {
  if (!d || typeof d !== 'object') return '';
  const keys = Object.keys(d);
  if (!keys.length) return '';
  return keys.sort().map(k => `${k}=${d[k]}`).join(', ');
}

async function loadAuditLog() {
  syncAuditFilterUI();
  const tbody = $('auditLogBody');
  const errEl = $('auditLogError');
  if (!tbody) return;
  if (errEl) errEl.textContent = '';

  tbody.replaceChildren();
  const loading = document.createElement('tr');
  const loadingTd = document.createElement('td');
  loadingTd.colSpan = 7;
  loadingTd.className = 'empty';
  loadingTd.textContent = 'Loading…';
  loading.appendChild(loadingTd);
  tbody.appendChild(loading);

  const params = new URLSearchParams();
  const palace = ($('auditFilterPalace') && $('auditFilterPalace').value.trim()) || '';
  const tenantF = ($('auditFilterTenant') && $('auditFilterTenant').value.trim()) || '';
  const actor = ($('auditFilterActor') && $('auditFilterActor').value.trim()) || '';
  let limVal = 200;
  const limInput = $('auditFilterLimit');
  if (limInput && limInput.value.trim() !== '') {
    const n = parseInt(limInput.value.trim(), 10);
    if (!isNaN(n) && n > 0) limVal = Math.min(2000, n);
  }
  if (palace) params.set('palace', palace);
  if (SESSION && SESSION.role === 'admin' && tenantF) params.set('tenant', tenantF);
  if (actor) params.set('actor', actor);
  params.set('limit', String(limVal));

  try {
    const res = await fetch('/api/audit-log?' + params.toString(), { headers: authHeaders() });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      tbody.replaceChildren();
      const tr = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = 7;
      td.className = 'empty';
      td.textContent = j.error || ('HTTP ' + res.status);
      tr.appendChild(td);
      tbody.appendChild(tr);
      if (errEl) errEl.textContent = j.error || ('HTTP ' + res.status);
      return;
    }
    const rows = await res.json();
    tbody.replaceChildren();
    if (!Array.isArray(rows) || rows.length === 0) {
      const tr = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = 7;
      td.className = 'empty';
      td.textContent = 'No entries match your filters.';
      tr.appendChild(td);
      tbody.appendChild(tr);
      return;
    }
    for (const row of rows) {
      const tr = document.createElement('tr');
      for (const key of ['ts', 'actor', 'actorRole', 'scopeTenant', 'palace', 'action']) {
        const td = document.createElement('td');
        const v = row[key];
        td.textContent = v == null ? '' : String(v);
        tr.appendChild(td);
      }
      const tdD = document.createElement('td');
      tdD.style.maxWidth = '320px';
      tdD.style.wordBreak = 'break-word';
      tdD.style.color = 'var(--muted)';
      tdD.textContent = formatAuditDetail(row.detail);
      tr.appendChild(tdD);
      tbody.appendChild(tr);
    }
  } catch (e) {
    tbody.replaceChildren();
    const tr = document.createElement('tr');
    const td = document.createElement('td');
    td.colSpan = 7;
    td.className = 'empty';
    td.textContent = 'Connection error: ' + e.message;
    tr.appendChild(td);
    tbody.appendChild(tr);
    if (errEl) errEl.textContent = 'Connection error: ' + e.message;
  }
}
