// ===== Palaces =====
function syncPalaceSettingsMode() {
  const raw = $('palaceSettingsModeRaw').checked;
  $('palaceSettingsStructured').style.display = raw ? 'none' : '';
  $('palaceSettingsRawWrap').style.display = raw ? '' : 'none';
}

function syncPalaceServerprefsEditMode() {
  const adv = $('palaceServerprefsModeAdvanced') && $('palaceServerprefsModeAdvanced').checked;
  const g = $('palaceServerprefsGuided');
  const a = $('palaceServerprefsAdvanced');
  if (g) g.style.display = adv ? 'none' : '';
  if (a) a.style.display = adv ? '' : 'none';
}

function parseCommaIntList(s) {
  if (!s || !String(s).trim()) return [];
  return String(s)
    .split(/[\s,]+/)
    .map(x => parseInt(x.trim(), 10))
    .filter(n => Number.isFinite(n));
}

function populateServerPrefsGuidedFromForm(f) {
  if (!f) return;
  const setv = (id, v) => {
    const el = $(id);
    if (el) el.value = v != null && v !== '' ? String(v) : '';
  };
  setv('spfWebsite', f.website);
  setv('spfYpLanguage', f.ypLanguage);
  setv('spfYpCategory', f.ypCategory);
  setv('spfYpDescription', f.ypDescription);
  setv('spfTimeoutRoomId', f.timeoutRoomId != null ? f.timeoutRoomId : '');
  setv('spfAutopurgeDays', f.autopurgeBanlistDays != null ? f.autopurgeBanlistDays : '');
  if ($('spfUnicodeNames')) $('spfUnicodeNames').checked = !!f.unicodeNames;
  if ($('spfUnicodeFull')) $('spfUnicodeFull').checked = f.unicodeFull !== false;
  if ($('spfAltNames')) $('spfAltNames').checked = f.altNames !== false;
  if ($('spfNoLooseProps')) $('spfNoLooseProps').checked = !!f.noLoosePropsNonOps;
  if ($('spfEspEnabled')) $('spfEspEnabled').checked = f.espEnabled !== false;
  if ($('spfRoomAnnotations')) {
    const ra = (f.roomAnnotations || 'everyone').toLowerCase();
    $('spfRoomAnnotations').value = ['everyone', 'wizards', 'off'].includes(ra) ? ra : 'everyone';
  }
  if ($('spfWizAuthoring')) {
    const wz = (f.wizAuthoring || 'on').toLowerCase();
    $('spfWizAuthoring').value = ['on', 'off', 'bless', 'godonly'].includes(wz) ? wz : 'on';
  }
  if ($('spfWizAuthoringAnnotation')) $('spfWizAuthoringAnnotation').checked = f.wizAuthoringAnnotation !== false;
  if ($('spfNotifyLogon')) $('spfNotifyLogon').checked = !!f.notifyLogon;
  if ($('spfNotifyLogoff')) $('spfNotifyLogoff').checked = !!f.notifyLogoff;
  if ($('spfPublicMedia')) $('spfPublicMedia').checked = f.publicMedia !== false;
  if ($('spfSecureProps')) $('spfSecureProps').checked = !!f.secureProps;
  if ($('spfMediaManagerEnabled')) $('spfMediaManagerEnabled').checked = f.mediaManagerEnabled !== false;
  if ($('spfMediaManagerRank')) {
    const r = (f.mediaManagerRank || 'owners').toLowerCase();
    $('spfMediaManagerRank').value = ['wizards', 'gods'].includes(r) ? r : 'owners';
  }
  if ($('spfMediaUploadRank')) {
    const u = (f.mediaUploadConfigRank || 'owners').toLowerCase();
    $('spfMediaUploadRank').value = ['wizards', 'gods', 'off'].includes(u) ? u : 'owners';
  }
  if ($('spfLegacyBlock')) $('spfLegacyBlock').checked = !!f.legacyClientsBlock;
  if ($('spfOverflowRooms')) $('spfOverflowRooms').value = Array.isArray(f.overflowRoomIds) ? f.overflowRoomIds.join(', ') : '';
  if ($('spfPropFreezeRooms')) $('spfPropFreezeRooms').value = Array.isArray(f.propFreezeRoomIds) ? f.propFreezeRoomIds.join(', ') : '';
  if ($('spfRatbotsRooms')) $('spfRatbotsRooms').value = Array.isArray(f.ratbotsAllowedRoomIds) ? f.ratbotsAllowedRoomIds.join(', ') : '';
  const fk = f.floodKill || {};
  if ($('spfFkEnabled')) $('spfFkEnabled').checked = !!fk.enabled;
  setv('spfFkTime', fk.time);
  setv('spfFkMove', fk.move);
  setv('spfFkChat', fk.chat);
  setv('spfFkWhisper', fk.whisper);
  setv('spfFkEsp', fk.esp);
  setv('spfFkPage', fk.page);
  setv('spfFkProp', fk.prop);
  setv('spfFkPropdrop', fk.propdrop);
  setv('spfFkDraw', fk.draw);
  setv('spfFkUsername', fk.username);
  const sl = f.soundLimit || {};
  if ($('spfSlEnabled')) $('spfSlEnabled').checked = sl.enabled !== false;
  setv('spfSlTimes', sl.times != null ? sl.times : 10);
  setv('spfSlTimeframe', sl.timeframe != null ? sl.timeframe : 60);
  const pw = f.passwordSecurity || {};
  setv('spfPwMinLen', pw.minLength != null ? pw.minLength : 8);
  if ($('spfPwRequireNumber')) $('spfPwRequireNumber').checked = pw.requireNumber !== false;
  if ($('spfPwRequireSymbol')) $('spfPwRequireSymbol').checked = !!pw.requireSymbol;
  if ($('spfPwRequireUpper')) $('spfPwRequireUpper').checked = !!pw.requireUpper;
  if ($('spfPwRequireLower')) $('spfPwRequireLower').checked = !!pw.requireLower;
}

function collectServerPrefsFormDTO() {
  const num = (id, def) => {
    const v = parseInt($(id).value, 10);
    return Number.isFinite(v) ? v : def;
  };
  const fk = {
    enabled: $('spfFkEnabled').checked,
    time: num('spfFkTime', 0),
    move: num('spfFkMove', 0),
    chat: num('spfFkChat', 0),
    whisper: num('spfFkWhisper', 0),
    esp: num('spfFkEsp', 0),
    page: num('spfFkPage', 0),
    prop: num('spfFkProp', 0),
    propdrop: num('spfFkPropdrop', 0),
    draw: num('spfFkDraw', 0),
    username: num('spfFkUsername', 0),
  };
  return {
    website: ($('spfWebsite') && $('spfWebsite').value) || '',
    ypLanguage: ($('spfYpLanguage') && $('spfYpLanguage').value) || '',
    ypCategory: ($('spfYpCategory') && $('spfYpCategory').value) || '',
    ypDescription: ($('spfYpDescription') && $('spfYpDescription').value) || '',
    timeoutRoomId: num('spfTimeoutRoomId', 0),
    autopurgeBanlistDays: (() => {
      const v = parseInt($('spfAutopurgeDays').value, 10);
      return Number.isFinite(v) ? v : 0;
    })(),
    unicodeNames: $('spfUnicodeNames').checked,
    unicodeFull: $('spfUnicodeFull').checked,
    altNames: $('spfAltNames').checked,
    noLoosePropsNonOps: $('spfNoLooseProps').checked,
    espEnabled: $('spfEspEnabled').checked,
    roomAnnotations: ($('spfRoomAnnotations') && $('spfRoomAnnotations').value) || 'everyone',
    wizAuthoring: ($('spfWizAuthoring') && $('spfWizAuthoring').value) || 'on',
    wizAuthoringAnnotation: !$('spfWizAuthoringAnnotation') || $('spfWizAuthoringAnnotation').checked,
    notifyLogon: $('spfNotifyLogon').checked,
    notifyLogoff: $('spfNotifyLogoff').checked,
    publicMedia: $('spfPublicMedia').checked,
    secureProps: $('spfSecureProps').checked,
    mediaManagerEnabled: $('spfMediaManagerEnabled').checked,
    mediaManagerRank: ($('spfMediaManagerRank') && $('spfMediaManagerRank').value) || 'owners',
    mediaUploadConfigRank: ($('spfMediaUploadRank') && $('spfMediaUploadRank').value) || 'owners',
    legacyClientsBlock: $('spfLegacyBlock').checked,
    overflowRoomIds: parseCommaIntList($('spfOverflowRooms') && $('spfOverflowRooms').value),
    propFreezeRoomIds: parseCommaIntList($('spfPropFreezeRooms') && $('spfPropFreezeRooms').value),
    ratbotsAllowedRoomIds: parseCommaIntList($('spfRatbotsRooms') && $('spfRatbotsRooms').value),
    floodKill: fk,
    soundLimit: {
      enabled: $('spfSlEnabled').checked,
      times: num('spfSlTimes', 10),
      timeframe: num('spfSlTimeframe', 60),
    },
    passwordSecurity: {
      minLength: Math.max(1, num('spfPwMinLen', 8)),
      requireNumber: $('spfPwRequireNumber').checked,
      requireSymbol: $('spfPwRequireSymbol').checked,
      requireUpper: $('spfPwRequireUpper').checked,
      requireLower: $('spfPwRequireLower').checked,
    },
  };
}

function palaceSettingsSwitchTab(tab) {
  SETTINGS_PREFS_TAB = tab || 'pserver';
  const tabs = [
    { id: 'pserver', btn: 'palacePrefsTabPserver', panel: 'palacePrefsPanelPserver' },
    { id: 'serverprefs', btn: 'palacePrefsTabServerprefs', panel: 'palacePrefsPanelServerprefs' },
    { id: 'ranks', btn: 'palacePrefsTabRanks', panel: 'palacePrefsPanelRanks' },
    { id: 'ratbot', btn: 'palacePrefsTabRatbot', panel: 'palacePrefsPanelRatbot' },
    { id: 'misc', btn: 'palacePrefsTabMisc', panel: 'palacePrefsPanelMisc' },
  ];
  tabs.forEach(t => {
    const active = t.id === SETTINGS_PREFS_TAB;
    const btn = $(t.btn);
    const panel = $(t.panel);
    if (btn) btn.classList.toggle('active', active);
    if (panel) panel.classList.toggle('active', active);
  });
}

function ratbotRowsToDOM() {
  const tbody = $('ratbotQuestionsBody');
  if (!tbody) return;
  if (!Array.isArray(SETTINGS_RATBOT_ROWS) || SETTINGS_RATBOT_ROWS.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty">No questions yet. Click + Add question.</td></tr>';
    return;
  }
  tbody.innerHTML = SETTINGS_RATBOT_ROWS.map((row, idx) => `
    <tr>
      <td><input type="text" data-rb="${idx}" data-k="question" value="${attrEsc(row.question || '')}" /></td>
      <td><input type="text" data-rb="${idx}" data-k="a" value="${attrEsc((row.options && row.options[0]) || '')}" /></td>
      <td><input type="text" data-rb="${idx}" data-k="b" value="${attrEsc((row.options && row.options[1]) || '')}" /></td>
      <td><input type="text" data-rb="${idx}" data-k="c" value="${attrEsc((row.options && row.options[2]) || '')}" /></td>
      <td><input type="text" data-rb="${idx}" data-k="d" value="${attrEsc((row.options && row.options[3]) || '')}" /></td>
      <td class="narrow">
        <select data-rb="${idx}" data-k="correct">
          <option value="A" ${row.correct === 'A' ? 'selected' : ''}>A</option>
          <option value="B" ${row.correct === 'B' ? 'selected' : ''}>B</option>
          <option value="C" ${row.correct === 'C' ? 'selected' : ''}>C</option>
          <option value="D" ${row.correct === 'D' ? 'selected' : ''}>D</option>
        </select>
      </td>
      <td><button type="button" onclick="removeRatbotQuestionRow(${idx})">Remove</button></td>
    </tr>
  `).join('');
}

function addRatbotQuestionRow() {
  SETTINGS_RATBOT_ROWS.push({ question: '', options: ['', '', '', ''], correct: 'A' });
  ratbotRowsToDOM();
}

function removeRatbotQuestionRow(idx) {
  SETTINGS_RATBOT_ROWS.splice(idx, 1);
  ratbotRowsToDOM();
}

function collectRatbotRowsFromDOM() {
  const rows = SETTINGS_RATBOT_ROWS.map((row, idx) => {
    const get = key => {
      const el = document.querySelector(`[data-rb="${idx}"][data-k="${key}"]`);
      return el ? el.value : '';
    };
    return {
      question: get('question').trim(),
      options: [get('a').trim(), get('b').trim(), get('c').trim(), get('d').trim()],
      correct: get('correct').trim().toUpperCase(),
    };
  });
  return rows;
}

async function loadPalaceMisc() {
  if (!SETTINGS_PALACE) return;
  $('palaceMiscError').textContent = '';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/misc`, { headers: headers() });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceMiscError').textContent = out.error || ('HTTP ' + res.status);
      return;
    }
    $('palaceMiscVerbosity').value = String(out.verbosity || 1);
  } catch (e) {
    $('palaceMiscError').textContent = e.message || String(e);
  }
}

async function savePalaceMisc() {
  if (!SETTINGS_PALACE) return;
  $('palaceMiscError').textContent = '';
  const btn = $('palaceMiscSaveBtn');
  const v = parseInt($('palaceMiscVerbosity').value, 10);
  if (!Number.isFinite(v) || v < 1 || v > 5) {
    $('palaceMiscError').textContent = 'Verbosity must be between 1 and 5.';
    return;
  }
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/misc`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ verbosity: v }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceMiscError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    closePalaceSettingsModal();
    loadPalaces();
  } catch (e) {
    $('palaceMiscError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

async function refreshRatbotFileList() {
  if (!SETTINGS_PALACE) return;
  $('ratbotEditorError').textContent = '';
  $('ratbotEditorInfo').textContent = '';
  const sel = $('ratbotFileSelect');
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/ratbot/files`, { headers: headers() });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('ratbotEditorError').textContent = out.error || ('HTTP ' + res.status);
      return;
    }
    const files = Array.isArray(out.files) ? out.files : [];
    sel.innerHTML = files.map(f => `<option value="${attrEsc(f)}">${esc(f)}</option>`).join('');
    if (files.length === 0) {
      sel.innerHTML = '<option value="">(none yet)</option>';
      SETTINGS_RATBOT_CURRENT_FILE = '';
      SETTINGS_RATBOT_ROWS = [];
      ratbotRowsToDOM();
      $('ratbotEditorInfo').textContent = 'No files found yet. Enter a new file name and add questions.';
      return;
    }
    if (SETTINGS_RATBOT_CURRENT_FILE && files.includes(SETTINGS_RATBOT_CURRENT_FILE)) {
      sel.value = SETTINGS_RATBOT_CURRENT_FILE;
    }
    SETTINGS_RATBOT_CURRENT_FILE = sel.value;
    await loadSelectedRatbotFile();
  } catch (e) {
    $('ratbotEditorError').textContent = e.message || String(e);
  }
}

async function loadSelectedRatbotFile() {
  if (!SETTINGS_PALACE) return;
  const sel = $('ratbotFileSelect');
  const name = (sel && sel.value) || '';
  SETTINGS_RATBOT_CURRENT_FILE = name;
  if (!name) {
    SETTINGS_RATBOT_ROWS = [];
    ratbotRowsToDOM();
    return;
  }
  $('ratbotEditorError').textContent = '';
  $('ratbotEditorInfo').textContent = '';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/ratbot/file?name=${encodeURIComponent(name)}`, { headers: headers() });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('ratbotEditorError').textContent = out.error || ('HTTP ' + res.status);
      return;
    }
    SETTINGS_RATBOT_ROWS = Array.isArray(out.questions) ? out.questions.map(q => ({
      question: q.question || '',
      options: Array.isArray(q.options) ? [q.options[0] || '', q.options[1] || '', q.options[2] || '', q.options[3] || ''] : ['', '', '', ''],
      correct: (q.correct || 'A').toUpperCase(),
    })) : [];
    ratbotRowsToDOM();
    const invalid = Number(out.invalidLineCount || 0);
    $('ratbotEditorInfo').textContent = invalid > 0
      ? `Loaded ${SETTINGS_RATBOT_ROWS.length} questions. ${invalid} non-empty line(s) were ignored because they do not match ratbot format.`
      : `Loaded ${SETTINGS_RATBOT_ROWS.length} questions.`;
  } catch (e) {
    $('ratbotEditorError').textContent = e.message || String(e);
  }
}

async function saveRatbotTrivia() {
  if (!SETTINGS_PALACE) return;
  $('ratbotEditorError').textContent = '';
  $('ratbotEditorInfo').textContent = '';
  const btn = $('ratbotSaveBtn');
  const newName = ($('ratbotNewFileName').value || '').trim();
  const chosen = (($('ratbotFileSelect') && $('ratbotFileSelect').value) || '').trim();
  const name = newName || chosen;
  if (!name) {
    $('ratbotEditorError').textContent = 'Choose an existing file or enter a new file name.';
    return;
  }
  const questions = collectRatbotRowsFromDOM();
  if (!questions.length) {
    $('ratbotEditorError').textContent = 'Add at least one question before saving.';
    return;
  }
  for (let i = 0; i < questions.length; i++) {
    const q = questions[i];
    if (!q.question || q.options.some(o => !o) || !['A', 'B', 'C', 'D'].includes(q.correct)) {
      $('ratbotEditorError').textContent = `Question ${i + 1} is incomplete.`;
      return;
    }
  }
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/ratbot/file`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ name, questions }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('ratbotEditorError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    SETTINGS_RATBOT_CURRENT_FILE = out.name || name;
    $('ratbotNewFileName').value = '';
    $('ratbotEditorInfo').textContent = `Saved ${out.questionCount || questions.length} questions to ${SETTINGS_RATBOT_CURRENT_FILE}.`;
    await refreshRatbotFileList();
    btn.disabled = false;
  } catch (e) {
    $('ratbotEditorError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

async function createBlankRatbotFileFromInput(ev) {
  if (ev && ev.key !== 'Enter') return;
  if (ev) ev.preventDefault();
  if (!SETTINGS_PALACE) return;
  $('ratbotEditorError').textContent = '';
  $('ratbotEditorInfo').textContent = '';
  const name = ($('ratbotNewFileName').value || '').trim();
  if (!name) {
    $('ratbotEditorError').textContent = 'Enter a file name first.';
    return;
  }
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/ratbot/file`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ name, questions: [] }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('ratbotEditorError').textContent = out.error || ('HTTP ' + res.status);
      return;
    }
    SETTINGS_RATBOT_CURRENT_FILE = out.name || name;
    $('ratbotNewFileName').value = '';
    SETTINGS_RATBOT_ROWS = [];
    ratbotRowsToDOM();
    await refreshRatbotFileList();
    $('ratbotEditorInfo').textContent = `Created blank trivia file ${SETTINGS_RATBOT_CURRENT_FILE}.`;
  } catch (e) {
    $('ratbotEditorError').textContent = e.message || String(e);
  }
}

function populatePrefsFormFromDTO(f) {
  if (!f) return;
  $('psfServerName').value = f.serverName || '';
  $('psfSysop').value = f.sysop || '';
  $('psfURL').value = f.url || '';
  $('psfWebsite').value = f.website || '';
  $('psfMOTD').value = f.motd || '';
  $('psfBlurb').value = f.blurb || '';
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

function rankTierLabel(n) {
  const m = { 0: 'Guest', 1: 'Member', 2: 'Wizard', 3: 'God', 4: 'Owner' };
  return m[n] != null ? m[n] : String(n);
}

function filterPalaceRanksRows() {
  const inp = $('palaceRanksFilter');
  const q = inp ? inp.value.trim().toLowerCase() : '';
  document.querySelectorAll('#palaceRanksBody tr[data-rank-row="1"]').forEach(tr => {
    const hay = (tr.getAttribute('data-search') || '').toLowerCase();
    tr.style.display = !q || hay.includes(q) ? '' : 'none';
  });
}

function renderPalaceRanksTable(commands) {
  const tb = $('palaceRanksBody');
  if (!tb) return;
  if (!Array.isArray(commands) || !commands.length) {
    tb.innerHTML = '<tr><td colspan="2" class="empty">No rank commands to configure.</td></tr>';
    return;
  }
  tb.innerHTML = commands.map(c => {
    const def = c.defaultRank;
    const useDef = c.override == null;
    const ovr = c.override;
    const searchBlob = (c.name + ' ' + (c.label || '')).toLowerCase().replace(/`/g, '');
    return `<tr data-rank-row="1" data-search="${attrEsc(searchBlob)}">
      <td><code>${esc(c.name)}</code>${c.extraInPrefs ? ' <span style="font-size:11px;color:var(--muted);">(prefs only)</span>' : ''}<div style="font-size:12px;color:var(--muted);margin-top:4px;line-height:1.4;">${esc(c.label)}</div></td>
      <td>
        <select data-rank-cmd="${attrEsc(c.name)}" class="palace-rank-select" style="min-width:220px;">
          <option value="def" ${useDef ? 'selected' : ''}>Built-in default (${esc(rankTierLabel(def))})</option>
          <option value="0" ${!useDef && ovr === 0 ? 'selected' : ''}>Guest</option>
          <option value="1" ${!useDef && ovr === 1 ? 'selected' : ''}>Member</option>
          <option value="2" ${!useDef && ovr === 2 ? 'selected' : ''}>Wizard</option>
          <option value="3" ${!useDef && ovr === 3 ? 'selected' : ''}>God</option>
          <option value="4" ${!useDef && ovr === 4 ? 'selected' : ''}>Owner</option>
        </select>
      </td>
    </tr>`;
  }).join('');
  filterPalaceRanksRows();
}

function collectCommandRanksPayload() {
  const ranks = {};
  document.querySelectorAll('select[data-rank-cmd]').forEach(sel => {
    const cmd = sel.getAttribute('data-rank-cmd');
    if (!cmd) return;
    if (sel.value === 'def') {
      ranks[cmd] = null;
    } else {
      const n = parseInt(sel.value, 10);
      if (Number.isFinite(n)) ranks[cmd] = n;
    }
  });
  return ranks;
}

async function savePalaceCommandRanks() {
  if (!SETTINGS_PALACE) return;
  $('palaceRanksError').textContent = '';
  $('palaceRanksInfo').textContent = '';
  const btn = $('palaceRanksSaveBtn');
  const ranks = collectCommandRanksPayload();
  if (!Object.keys(ranks).length) {
    $('palaceRanksError').textContent = 'Nothing to save (no command rows).';
    return;
  }
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/command-ranks`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ ranks }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceRanksError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    $('palaceRanksInfo').textContent = out.note || 'Saved serverprefs.json. Use Reload server config to apply to the running pserver.';
    const r2 = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/command-ranks`, { headers: headers() });
    const again = await r2.json().catch(() => ({}));
    if (r2.ok && Array.isArray(again.commands)) {
      renderPalaceRanksTable(again.commands);
    }
    btn.disabled = false;
  } catch (e) {
    $('palaceRanksError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

async function reloadPalaceServerConfig() {
  if (!SETTINGS_PALACE) return;
  $('palaceRanksError').textContent = '';
  $('palaceRanksInfo').textContent = '';
  const btn = $('palaceRanksReloadBtn');
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/reload-config`, {
      method: 'POST',
      headers: headers(),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceRanksError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    $('palaceRanksInfo').textContent = (out.note || 'Reload signal sent.') + ' You can confirm in the palace log that pat/prefs reloaded.';
    btn.disabled = false;
  } catch (e) {
    $('palaceRanksError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

function formatPalaceServerprefsJSON() {
  const el = $('palaceServerprefsEditor');
  if (!el) return;
  $('palaceServerprefsError').textContent = '';
  $('palaceServerprefsInfo').textContent = '';
  try {
    const o = JSON.parse((el.value || '').trim() || '{}');
    el.value = JSON.stringify(o, null, 2) + '\n';
    $('palaceServerprefsInfo').textContent = 'Formatted.';
  } catch (e) {
    $('palaceServerprefsError').textContent = e.message || String(e);
  }
}

function validatePalaceServerprefsJSON() {
  const el = $('palaceServerprefsEditor');
  if (!el) return;
  $('palaceServerprefsError').textContent = '';
  $('palaceServerprefsInfo').textContent = '';
  try {
    JSON.parse((el.value || '').trim() || '{}');
    $('palaceServerprefsInfo').textContent = 'JSON is valid.';
  } catch (e) {
    $('palaceServerprefsError').textContent = e.message || String(e);
  }
}

async function savePalaceServerprefsJSON() {
  if (!SETTINGS_PALACE) return;
  const el = $('palaceServerprefsEditor');
  if (!el) return;
  $('palaceServerprefsError').textContent = '';
  $('palaceServerprefsInfo').textContent = '';
  let text = el.value || '';
  try {
    const o = JSON.parse(text.trim() || '{}');
    text = JSON.stringify(o, null, 2) + '\n';
  } catch (e) {
    $('palaceServerprefsError').textContent = 'Fix JSON before save: ' + (e.message || String(e));
    return;
  }
  const btn = $('palaceServerprefsSaveBtn');
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/server-files/serverprefs.json`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ content: text }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceServerprefsError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    el.value = text;
    $('palaceServerprefsInfo').textContent = (out.restarted ? 'Saved and restarted.' : 'Saved.') + ' Reopen or use Reload server config if you changed files elsewhere.';
    btn.disabled = false;
    loadPalaces();
  } catch (e) {
    $('palaceServerprefsError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

async function savePalaceServerprefsGuided() {
  if (!SETTINGS_PALACE) return;
  $('palaceServerprefsError').textContent = '';
  $('palaceServerprefsInfo').textContent = '';
  const btn = $('palaceServerprefsSaveBtn');
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/serverprefs-form`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ form: collectServerPrefsFormDTO() }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceServerprefsError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    $('palaceServerprefsInfo').textContent = (out.restarted ? 'Saved guided fields and restarted.' : 'Saved.') + ' Sensitive keys on disk were left unchanged.';
    btn.disabled = false;
    loadPalaces();
  } catch (e) {
    $('palaceServerprefsError').textContent = e.message || String(e);
    btn.disabled = false;
  }
}

async function savePalaceServerprefs() {
  const adv = $('palaceServerprefsModeAdvanced') && $('palaceServerprefsModeAdvanced').checked;
  if (adv) await savePalaceServerprefsJSON();
  else await savePalaceServerprefsGuided();
}

async function reloadPalaceServerConfigFromServerprefsTab() {
  if (!SETTINGS_PALACE) return;
  $('palaceServerprefsError').textContent = '';
  $('palaceServerprefsInfo').textContent = '';
  const btn = $('palaceServerprefsReloadBtn');
  btn.disabled = true;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SETTINGS_PALACE)}/reload-config`, {
      method: 'POST',
      headers: headers(),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('palaceServerprefsError').textContent = out.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    $('palaceServerprefsInfo').textContent = (out.note || 'SIGHUP sent.') + ' Running pserver should reread pat, prefs, and serverprefs.json.';
    btn.disabled = false;
  } catch (e) {
    $('palaceServerprefsError').textContent = e.message || String(e);
    btn.disabled = false;
  }
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
  SETTINGS_RATBOT_ROWS = [];
  SETTINGS_RATBOT_CURRENT_FILE = '';
  $('palaceSettingsError').textContent = '';
  $('palaceSettingsSaveBtn').disabled = false;
  $('palaceMiscError').textContent = '';
  $('palaceMiscSaveBtn').disabled = false;
  $('ratbotEditorError').textContent = '';
  $('ratbotEditorInfo').textContent = '';
  $('ratbotSaveBtn').disabled = false;
  if ($('ratbotNewFileName')) $('ratbotNewFileName').value = '';
  if ($('palaceRanksError')) $('palaceRanksError').textContent = '';
  if ($('palaceRanksInfo')) $('palaceRanksInfo').textContent = '';
  if ($('palaceRanksSaveBtn')) $('palaceRanksSaveBtn').disabled = false;
  if ($('palaceRanksReloadBtn')) $('palaceRanksReloadBtn').disabled = false;
  if ($('palaceServerprefsSaveBtn')) $('palaceServerprefsSaveBtn').disabled = false;
  if ($('palaceServerprefsReloadBtn')) $('palaceServerprefsReloadBtn').disabled = false;
  if ($('palaceServerprefsEditor')) $('palaceServerprefsEditor').value = '';
  if ($('palaceServerprefsError')) $('palaceServerprefsError').textContent = '';
  if ($('palaceServerprefsInfo')) $('palaceServerprefsInfo').textContent = '';
  if ($('palaceServerprefsPreservedNote')) {
    $('palaceServerprefsPreservedNote').style.display = 'none';
    $('palaceServerprefsPreservedNote').textContent = '';
  }
  if ($('palaceServerprefsModeGuided')) $('palaceServerprefsModeGuided').checked = true;
  syncPalaceServerprefsEditMode();
  if ($('palaceRanksBody')) $('palaceRanksBody').innerHTML = '<tr><td colspan="2" class="empty">Loading…</td></tr>';
  if ($('palaceRanksFilter')) $('palaceRanksFilter').value = '';
  $('palaceSettingsTitle').textContent = 'Palace Preferences — ' + name;
  $('palaceSettingsLead').textContent = '';
  $('palaceSettingsContent').value = '';
  $('palaceUnknownTail').value = '';
  $('palaceSettingsWarnings').style.display = 'none';
  $('palaceSettingsWarnings').textContent = '';
  $('palaceSettingsModeForm').checked = true;
  palaceSettingsSwitchTab('pserver');
  syncPalaceSettingsMode();

  $('palaceSettingsModal').classList.add('open');

  try {
    const [pres, pform, fres, spFormRes, spj, crr] = await Promise.all([
      fetch(`/api/palaces/${encodeURIComponent(name)}`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/prefs-form`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/server-files/pserver.prefs`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/serverprefs-form`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/server-files/serverprefs.json`, { headers: headers() }),
      fetch(`/api/palaces/${encodeURIComponent(name)}/command-ranks`, { headers: headers() }),
    ]);
    const pd = await pres.json().catch(() => ({}));
    const formData = await pform.json().catch(() => ({}));
    const rawFile = await fres.json().catch(() => ({}));
    const spFormData = await spFormRes.json().catch(() => ({}));
    const spData = await spj.json().catch(() => ({}));
    const rankData = await crr.json().catch(() => ({}));

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
    if (crr.ok && Array.isArray(rankData.commands)) {
      renderPalaceRanksTable(rankData.commands);
    } else if (!crr.ok) {
      if ($('palaceRanksBody')) {
        $('palaceRanksBody').innerHTML = '<tr><td colspan="2" class="empty">Could not load command ranks.</td></tr>';
      }
      if ($('palaceRanksError')) {
        $('palaceRanksError').textContent = rankData.error || ('command-ranks HTTP ' + crr.status);
      }
    }

    if (spFormRes.ok && spFormData.form) {
      populateServerPrefsGuidedFromForm(spFormData.form);
      const pk = Array.isArray(spFormData.preservedKeys) ? spFormData.preservedKeys : [];
      if (pk.length && $('palaceServerprefsPreservedNote')) {
        $('palaceServerprefsPreservedNote').style.display = '';
        $('palaceServerprefsPreservedNote').textContent =
          'This palace also has on-disk data we never touch from this tab: ' + pk.map(esc).join(', ') + '.';
      }
    } else if ($('palaceServerprefsError')) {
      $('palaceServerprefsError').textContent =
        spFormData.error || ('serverprefs-form HTTP ' + spFormRes.status);
    }

    const ed = $('palaceServerprefsEditor');
    if (ed) {
      if (spj.ok && typeof spData.content === 'string') {
        const raw = spData.content.trim();
        if (!raw) {
          ed.value = '{\n  "version": 1\n}\n';
        } else {
          try {
            ed.value = JSON.stringify(JSON.parse(raw), null, 2) + '\n';
          } catch (e) {
            ed.value = spData.content;
            $('palaceServerprefsInfo').textContent =
              'Advanced JSON: could not pretty-print — ' + (e.message || e);
          }
        }
      } else if (spj.status === 404) {
        ed.value = '{\n  "version": 1\n}\n';
      } else if (!spj.ok) {
        ed.value = '{\n  "version": 1\n}\n';
        const extra = spData.error || ('serverprefs.json HTTP ' + spj.status);
        if (!spFormRes.ok) {
          $('palaceServerprefsError').textContent =
            ($('palaceServerprefsError').textContent ? $('palaceServerprefsError').textContent + ' · ' : '') + extra;
        } else if ($('palaceServerprefsInfo')) {
          $('palaceServerprefsInfo').textContent = 'Guided form loaded; raw file fetch: ' + extra;
        }
      }
    }

    void loadPalaceMisc();
    void refreshRatbotFileList();
  } catch (e) {
    $('palaceSettingsError').textContent = e.message || String(e);
  }
}

function closePalaceSettingsModal() {
  $('palaceSettingsModal').classList.remove('open');
  SETTINGS_PALACE = null;
  SETTINGS_RAW_SNAPSHOT = '';
  SETTINGS_PREFS_TAB = 'pserver';
  SETTINGS_RATBOT_ROWS = [];
  SETTINGS_RATBOT_CURRENT_FILE = '';
  if ($('palaceServerprefsModeGuided')) $('palaceServerprefsModeGuided').checked = true;
  syncPalaceServerprefsEditMode();
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

/** Inline SVGs for service controls (stroke icons in the Feather / Heroicons tradition; `currentColor` matches button text). */
const PALACE_CTL_SVG = {
  stop:
    '<svg class="palace-ctl-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="5" y="5" width="14" height="14" rx="1.5"/></svg>',
  start:
    '<svg class="palace-ctl-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7L8 5z"/></svg>',
  restart:
    '<svg class="palace-ctl-icon" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M23 4v6h-6"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>',
};

/** Stop / Start / Restart — runs immediately (no confirmation). */
function palaceServiceControlButtonsHTML(nameJson) {
  return (
    `<button type="button" class="palace-ctl-btn" title="Stop" aria-label="Stop" onclick='void palaceAction(${nameJson},"stop")'>${PALACE_CTL_SVG.stop}Stop</button>` +
    `<button type="button" class="palace-ctl-btn" title="Start" aria-label="Start" onclick='void palaceAction(${nameJson},"start")'>${PALACE_CTL_SVG.start}Start</button>` +
    `<button type="button" class="palace-ctl-btn" title="Restart" aria-label="Restart" onclick='void palaceAction(${nameJson},"restart")'>${PALACE_CTL_SVG.restart}Restart</button>`
  );
}

const PROVISION_TCP_RANGE = [9990, 10990];
const PROVISION_HTTP_RANGE = [6000, 7000];
const PALACE_EXPANDED = new Set();

// ----- Join notify (sound when online count increases; localStorage + smart polling) -----
const JOIN_NOTIFY_STORAGE_KEY = 'palaceJoinNotify';
const JOIN_DING_URL = '/assets/dingdong.mp3';
let JOIN_NOTIFY_ENABLED = false;
try {
  JOIN_NOTIFY_ENABLED = localStorage.getItem(JOIN_NOTIFY_STORAGE_KEY) === '1';
} catch (_) {}

let LAST_PALACE_MAIN_LIST = [];
let LAST_PALACE_LIST_META = { isAdmin: false, isTenant: false };
const JOIN_NOTIFY_BASELINE = new Map();
let collapsedJoinMasterTimer = null;
let collapsedJoinTimeoutIds = [];
let joinNotifyBarWired = false;

function isPalacesTabActive() {
  const el = $('tab-palaces');
  return !!(el && el.classList.contains('active'));
}

function playJoinNotifyDing() {
  try {
    const a = new Audio(JOIN_DING_URL);
    a.volume = 0.65;
    a.play().catch(() => {});
  } catch (_) {}
}

function maybeNotifyPalaceUserJoin(name, d) {
  if (!JOIN_NOTIFY_ENABLED || !d) return;
  const cur = Number(d.user_count);
  if (!Number.isFinite(cur)) return;
  const prev = JOIN_NOTIFY_BASELINE.get(name);
  if (prev !== undefined && cur > prev) {
    playJoinNotifyDing();
  }
  JOIN_NOTIFY_BASELINE.set(name, cur);
}

async function fetchPalaceStatsForJoinNotify(name) {
  if (!JOIN_NOTIFY_ENABLED || !name) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/stats`, { headers: headers() });
    if (!res.ok) return;
    const d = await res.json();
    maybeNotifyPalaceUserJoin(name, d);
  } catch (_) {}
}

function pauseCollapsedJoinPolling() {
  if (collapsedJoinMasterTimer) {
    clearInterval(collapsedJoinMasterTimer);
    collapsedJoinMasterTimer = null;
  }
  for (const id of collapsedJoinTimeoutIds) {
    clearTimeout(id);
  }
  collapsedJoinTimeoutIds = [];
}

function syncCollapsedJoinPolling() {
  pauseCollapsedJoinPolling();
  if (!JOIN_NOTIFY_ENABLED || !isPalacesTabActive()) return;
  const { isAdmin, isTenant } = LAST_PALACE_LIST_META;
  if (!isAdmin || isTenant) return;

  const collapsed = LAST_PALACE_MAIN_LIST.filter(
    p => p.httpPort && !PALACE_EXPANDED.has(p.name)
  );
  if (collapsed.length === 0) return;

  const period = 20000;
  const n = collapsed.length;
  const gap = Math.min(1500, Math.max(200, Math.floor(16000 / n)));

  const runRound = () => {
    for (const id of collapsedJoinTimeoutIds) {
      clearTimeout(id);
    }
    collapsedJoinTimeoutIds = [];
    collapsed.forEach((p, i) => {
      const id = setTimeout(() => fetchPalaceStatsForJoinNotify(p.name), i * gap);
      collapsedJoinTimeoutIds.push(id);
    });
  };

  runRound();
  collapsedJoinMasterTimer = setInterval(runRound, period);
}

function updatePalaceJoinNotifyBar() {
  const bar = $('palaceJoinNotifyBar');
  const toggle = $('palaceJoinNotifyToggle');
  if (!bar || !toggle) return;
  const show = !!(SESSION && (SESSION.role === 'admin' || SESSION.role === 'tenant'));
  bar.style.display = show ? '' : 'none';
  if (!show) return;
  toggle.checked = JOIN_NOTIFY_ENABLED;
  if (!joinNotifyBarWired) {
    joinNotifyBarWired = true;
    toggle.addEventListener('change', () => {
      JOIN_NOTIFY_ENABLED = !!toggle.checked;
      try {
        localStorage.setItem(JOIN_NOTIFY_STORAGE_KEY, JOIN_NOTIFY_ENABLED ? '1' : '0');
      } catch (_) {}
      JOIN_NOTIFY_BASELINE.clear();
      syncCollapsedJoinPolling();
    });
  }
}

/** Preset + custom slider for new-palace home quota (binary MB / GB to match server usage). */
const PROVISION_QUOTA_LEVELS = (function () {
  const MB = 1024 * 1024;
  const GB = 1024 * 1024 * 1024;
  const a = [
    { label: 'Unlimited', bytes: 0 },
    { label: '1 MB', bytes: MB },
    { label: '500 MB', bytes: 500 * MB },
  ];
  for (let g = 1; g <= 10; g++) {
    a.push({ label: `${g} GB`, bytes: g * GB });
  }
  a.push({ label: 'Custom…', bytes: -1 });
  return a;
})();

function formatPalaceQuotaShort(bytes) {
  if (!Number.isFinite(bytes) || bytes < 0) return '—';
  const MB = 1024 * 1024;
  const GB = 1024 * 1024 * 1024;
  if (bytes >= GB) {
    const g = bytes / GB;
    const s = Number.isInteger(g) ? String(g) : String(Math.round(g * 10) / 10).replace(/\.0$/, '');
    return `${s} GB`;
  }
  const m = bytes / MB;
  const s = Number.isInteger(m) ? String(m) : String(Math.round(m * 10) / 10).replace(/\.0$/, '');
  return `${s} MB`;
}

function syncProvisionQuotaLabel() {
  const slider = $('pQuotaSlider');
  const label = $('pQuotaLabel');
  const wrap = $('pQuotaCustomWrap');
  if (!slider || !label) return;
  const i = parseInt(slider.value, 10);
  const row = PROVISION_QUOTA_LEVELS[Math.min(Math.max(i, 0), PROVISION_QUOTA_LEVELS.length - 1)];
  if (row.bytes === -1) {
    if (wrap) wrap.style.display = '';
    label.textContent = 'Custom maximum (enter MiB below).';
  } else {
    if (wrap) wrap.style.display = 'none';
    label.textContent = 'Max home storage: ' + row.label;
  }
}

function provisionQuotaSelectedBytes() {
  const slider = $('pQuotaSlider');
  if (!slider) return 0;
  const i = parseInt(slider.value, 10);
  const row = PROVISION_QUOTA_LEVELS[Math.min(Math.max(i, 0), PROVISION_QUOTA_LEVELS.length - 1)];
  if (row.bytes === -1) {
    const mib = parseFloat(($('pQuotaCustomMiB') && $('pQuotaCustomMiB').value) || '');
    if (!Number.isFinite(mib) || mib <= 0) return null;
    return Math.round(mib * 1024 * 1024);
  }
  return row.bytes;
}

function palaceQuotaChartId(name) {
  return 'pquota-' + encodeURIComponent(name).replace(/[^a-zA-Z0-9_-]/g, '_');
}

const PALACE_QUOTA_SVG_NS = 'http://www.w3.org/2000/svg';

/** Rounded track + solid fill + diagonal hash overlay (same idea as the D3 snippet), driven by homeUsedBytes / quotaBytesMax. */
function renderPalaceQuotaCharts() {
  document.querySelectorAll('.palace-quota-chart[data-quota-max]').forEach((holder) => {
    const max = Number(holder.getAttribute('data-quota-max'));
    const used = Number(holder.getAttribute('data-quota-used') || '0');
    const over = holder.getAttribute('data-quota-over') === '1';
    if (!Number.isFinite(max) || max <= 0) return;

    const w = 320;
    const h = 20;
    const usableW = w - 2;
    let frac = used / max;
    if (!Number.isFinite(frac) || frac < 0) frac = 0;
    const barW = Math.min(usableW, frac * usableW);
    const innerH = h - 2;

    holder.textContent = '';
    const svg = document.createElementNS(PALACE_QUOTA_SVG_NS, 'svg');
    svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
    svg.setAttribute('preserveAspectRatio', 'none');
    svg.setAttribute('class', 'palace-quota-svg');
    svg.setAttribute('width', String(w));
    svg.setAttribute('height', String(h));

    const patId = `${holder.id || 'pquota'}-hash`;
    const defs = document.createElementNS(PALACE_QUOTA_SVG_NS, 'defs');
    const pattern = document.createElementNS(PALACE_QUOTA_SVG_NS, 'pattern');
    pattern.setAttribute('id', patId);
    pattern.setAttribute('width', '8');
    pattern.setAttribute('height', '8');
    pattern.setAttribute('patternUnits', 'userSpaceOnUse');
    pattern.setAttribute('patternTransform', 'rotate(10)');
    const pline = document.createElementNS(PALACE_QUOTA_SVG_NS, 'line');
    pline.setAttribute('x1', '0');
    pline.setAttribute('y1', '0');
    pline.setAttribute('x2', '8');
    pline.setAttribute('y2', '8');
    pline.setAttribute('stroke', over ? 'rgba(255,200,200,0.55)' : 'rgba(255,255,255,0.35)');
    pline.setAttribute('stroke-width', '2');
    pattern.appendChild(pline);
    defs.appendChild(pattern);
    svg.appendChild(defs);

    const bg = document.createElementNS(PALACE_QUOTA_SVG_NS, 'rect');
    bg.setAttribute('x', '0');
    bg.setAttribute('y', '0');
    bg.setAttribute('rx', '6');
    bg.setAttribute('ry', '6');
    bg.setAttribute('width', String(w));
    bg.setAttribute('height', String(h));
    bg.setAttribute('fill', 'rgba(0,0,0,0.6)');
    svg.appendChild(bg);

    const fillColor = over ? 'rgba(255,150,150,0.75)' : 'rgba(255,255,255,0.6)';
    if (barW > 0.5) {
      const r1 = document.createElementNS(PALACE_QUOTA_SVG_NS, 'rect');
      r1.setAttribute('x', '1');
      r1.setAttribute('y', '1');
      r1.setAttribute('rx', '1');
      r1.setAttribute('ry', '1');
      r1.setAttribute('width', String(barW));
      r1.setAttribute('height', String(innerH));
      r1.setAttribute('fill', fillColor);
      svg.appendChild(r1);

      const r2 = document.createElementNS(PALACE_QUOTA_SVG_NS, 'rect');
      r2.setAttribute('x', '1');
      r2.setAttribute('y', '1');
      r2.setAttribute('rx', '1');
      r2.setAttribute('ry', '1');
      r2.setAttribute('width', String(barW));
      r2.setAttribute('height', String(innerH));
      r2.setAttribute('fill', `url(#${patId})`);
      svg.appendChild(r2);
    }

    const tip = document.createElementNS(PALACE_QUOTA_SVG_NS, 'title');
    tip.textContent = `${formatPalaceQuotaShort(used)} / ${formatPalaceQuotaShort(max)}`;
    svg.appendChild(tip);

    holder.appendChild(svg);
  });
}

function palaceQuotaDetailBlockHTML(p) {
  const max = p.quotaBytesMax;
  if (!max) {
    return `<div class="palace-detail-block palace-quota-slot">
      <span class="palace-detail-label">Quota</span>
      <span class="palace-detail-value">Unlimited</span>
    </div>`;
  }
  const over = !!p.quotaExceeded;
  const cls = over ? 'palace-detail-value palace-quota-over' : 'palace-detail-value';
  const u = p.homeUsedBytes != null ? p.homeUsedBytes : 0;
  const qid = palaceQuotaChartId(p.name);
  const overData = over ? ' data-quota-over="1"' : '';
  return `<div class="palace-detail-block palace-quota-detail palace-quota-slot">
    <span class="palace-detail-label">Quota</span>
    <div class="palace-quota-chart" id="${esc(qid)}" data-quota-used="${esc(String(u))}" data-quota-max="${esc(String(max))}"${overData} role="img" aria-label="Home storage ${esc(formatPalaceQuotaShort(u))} of ${esc(formatPalaceQuotaShort(max))}"></div>
    <span class="${cls}">${esc(formatPalaceQuotaShort(u))} / ${esc(formatPalaceQuotaShort(max))}</span>
  </div>`;
}

function quotaSliderIndexForMaxBytes(max) {
  if (!max) return 0;
  for (let i = 0; i < PROVISION_QUOTA_LEVELS.length; i++) {
    const b = PROVISION_QUOTA_LEVELS[i].bytes;
    if (b > 0 && b === max) return i;
  }
  return PROVISION_QUOTA_LEVELS.length - 1;
}

function syncEditPalaceQuotaLabel() {
  const slider = $('eQuotaSlider');
  const label = $('eQuotaLabel');
  const wrap = $('eQuotaCustomWrap');
  if (!slider || !label) return;
  const i = parseInt(slider.value, 10);
  const row = PROVISION_QUOTA_LEVELS[Math.min(Math.max(i, 0), PROVISION_QUOTA_LEVELS.length - 1)];
  if (row.bytes === -1) {
    if (wrap) wrap.style.display = '';
    label.textContent = 'Custom maximum (enter MiB below).';
  } else {
    if (wrap) wrap.style.display = 'none';
    label.textContent = 'Max home storage: ' + row.label;
  }
}

function editPalaceQuotaSelectedBytes() {
  const slider = $('eQuotaSlider');
  if (!slider) return 0;
  const i = parseInt(slider.value, 10);
  const row = PROVISION_QUOTA_LEVELS[Math.min(Math.max(i, 0), PROVISION_QUOTA_LEVELS.length - 1)];
  if (row.bytes === -1) {
    const mib = parseFloat(($('eQuotaCustomMiB') && $('eQuotaCustomMiB').value) || '');
    if (!Number.isFinite(mib) || mib <= 0) return null;
    return Math.round(mib * 1024 * 1024);
  }
  return row.bytes;
}

function editPalaceOpen(name) {
  if (!name || !SESSION || SESSION.role !== 'admin') return;
  PALACE_EXPANDED.add(name);
  void loadPalaces();
  void openEditPalaceModal(name);
}

async function openEditPalaceModal(origName) {
  EDIT_PALACE_ORIG = origName;
  $('editPalaceError').textContent = '';
  $('editPalaceModal').classList.add('open');
  const btn = $('editPalaceSubmit');
  if (btn) btn.disabled = false;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(origName)}`, { headers: headers() });
    const p = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('editPalaceError').textContent = p.error || ('HTTP ' + res.status);
      return;
    }
    $('ePalaceName').value = (p.name || '').toLowerCase();
    $('eTcpPort').value = p.tcpPort != null && p.tcpPort !== '' ? String(p.tcpPort) : '';
    $('eHttpPort').value = p.httpPort != null && p.httpPort !== '' ? String(p.httpPort) : '';
    const max = p.quotaBytesMax || 0;
    const idx = quotaSliderIndexForMaxBytes(max);
    $('eQuotaSlider').value = String(idx);
    const row = PROVISION_QUOTA_LEVELS[idx];
    if (row && row.bytes === -1 && max > 0) {
      $('eQuotaCustomMiB').value = String(Math.round(max / (1024 * 1024)));
    } else {
      $('eQuotaCustomMiB').value = '';
    }
    syncEditPalaceQuotaLabel();
  } catch (e) {
    $('editPalaceError').textContent = e.message || String(e);
  }
}

function closeEditPalaceModal() {
  $('editPalaceModal').classList.remove('open');
  EDIT_PALACE_ORIG = null;
}

async function submitEditPalaceAdmin() {
  const orig = EDIT_PALACE_ORIG;
  if (!orig) return;
  $('editPalaceError').textContent = '';
  const btn = $('editPalaceSubmit');
  if (btn) btn.disabled = true;
  const name = ($('ePalaceName').value || '').trim().toLowerCase();
  const tcp = parseInt($('eTcpPort').value, 10);
  const http = parseInt($('eHttpPort').value, 10);
  const qb = editPalaceQuotaSelectedBytes();
  if (qb === null) {
    $('editPalaceError').textContent = 'Enter a valid custom quota in MiB.';
    if (btn) btn.disabled = false;
    return;
  }
  if (!name || !Number.isFinite(tcp) || !Number.isFinite(http) || tcp <= 0 || http <= 0) {
    $('editPalaceError').textContent = 'Name, TCP port, and HTTP port are required.';
    if (btn) btn.disabled = false;
    return;
  }
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(orig)}`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ name, tcpPort: tcp, httpPort: http, quotaBytesMax: qb }),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('editPalaceError').textContent = out.error || ('HTTP ' + res.status);
      if (btn) btn.disabled = false;
      return;
    }
    const newN = out.name || name;
    if (orig !== newN) {
      PALACE_EXPANDED.delete(orig);
      PALACE_EXPANDED.add(newN);
    }
    closeEditPalaceModal();
    loadPalaces();
    loadNginxStatus();
    setTimeout(loadNginxStatus, 2500);
  } catch (e) {
    $('editPalaceError').textContent = e.message || String(e);
    if (btn) btn.disabled = false;
  }
}

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
        pauseCollapsedJoinPolling();
        return;
      }
    }
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty">Could not load palaces (HTTP ${res.status})</td></tr>`;
      if (unregPanel) unregPanel.style.display = 'none';
      pauseCollapsedJoinPolling();
      return;
    }
    const data = await res.json();
    const isAdmin = !!(SESSION && SESSION.role === 'admin');
    const isTenant = !!(SESSION && SESSION.role === 'tenant');
    const canAdmin = isAdmin;
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
      LAST_PALACE_MAIN_LIST = [];
      LAST_PALACE_LIST_META = { isAdmin, isTenant };
      pauseCollapsedJoinPolling();
      updatePalaceJoinNotifyBar();
      return;
    }
    if (mainList.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty">No registered palaces in the manager. Unregistered instances are listed below.</td></tr>';
      LAST_PALACE_MAIN_LIST = [];
      LAST_PALACE_LIST_META = { isAdmin, isTenant };
      pauseCollapsedJoinPolling();
      updatePalaceJoinNotifyBar();
      return;
    }
    const expandedList = mainList.filter(p => p.httpPort && (isTenant || PALACE_EXPANDED.has(p.name)));
    tbody.innerHTML = mainList.map(p => {
      const { dotClass, title } = palaceStatusDot(p.status);
      const nm = JSON.stringify(p.name);
      const expanded = isTenant || PALACE_EXPANDED.has(p.name);
      const expandGlyph = expanded ? '&#9662;' : '&#9656;';
      const pserv = `<code>${esc(p.pserverVersion || 'latest')}</code>`;
      const editBtn = isAdmin
        ? `<button type="button" onclick='event.stopPropagation();editPalaceOpen(${nm})' title="Edit name, ports, quota (admin)">Edit</button>`
        : '';
      const removeBtn = isAdmin
        ? `<button type="button" class="danger" onclick='event.stopPropagation();openRemovePalaceModal(${nm})'>Remove</button>`
        : '';
      const logsBtn = `<button type="button" onclick='viewLogs(${nm})'>Logs</button>`;
      const settingsBtn = `<button type="button" onclick='openPalaceSettingsModal(${nm})'>Settings</button>`;
      const mediaBtn = `<button type="button" onclick='openPalaceMediaModal(${nm})' title="Media folder on disk (systemd -m)">Media</button>`;
      const backupsBtn = `<button type="button" onclick='openPalaceBackupsModal(${nm})' title="Config snapshots and full-home download">Backups</button>`;
      const filesBtn = `<button type="button" onclick='openServerFilesModal(${nm})'>Files</button>`;
      const usersBtn = p.httpPort
        ? `<button type="button" onclick='openPalaceUsersModal(${nm})'>Users</button>`
        : '';
      const bansBtn = (p.httpPort && (isAdmin || isTenant))
        ? `<button type="button" onclick='openPalaceBansModal(${nm})'>Bans</button>`
        : '';
      const propsBtn = (p.httpPort && (isAdmin || isTenant))
        ? `<button type="button" onclick='openPalacePropsModal(${nm})'>Props</button>`
        : '';
      const pagesBtn = p.httpPort
        ? `<button type="button" onclick='openPalacePagesModal(${nm})'>Pages</button>`
        : '';
      const summaryClass = isAdmin ? 'palace-row-summary' : '';
      const sid = palaceStatId(p.name);
      const controlBtns = palaceServiceControlButtonsHTML(nm);
      const overQuotaBadge = p.quotaExceeded
        ? '<span class="badge badge-over-quota">OVER QUOTA</span>'
        : '';
      return `
      <tr class="${summaryClass}${expanded ? ' palace-row-open' : ''}"${isAdmin ? ` onclick='togglePalaceAccordion(${nm})'` : ''}>
        <td>
          <span class="palace-name-cell">
            <span class="palace-expander">${isAdmin ? expandGlyph : '&nbsp;'}</span>
            <strong>${esc(p.name)}</strong>
          </span>
        </td>
        <td><span class="palace-status"><span class="status-dot ${dotClass}" title="${esc(title)}" aria-hidden="true"></span><span class="badge badge-${esc(p.status)}">${esc(p.status)}</span>${overQuotaBadge}</span></td>
        <td>${p.tcpPort || '—'}</td>
        <td>${p.httpPort || '—'}</td>
        <td style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">${pserv}${editBtn}${removeBtn}</td>
      </tr>
      <tr class="palace-details-row" style="display:${expanded ? '' : 'none'};">
        <td colspan="5">
          <div class="palace-details-top">
            <div class="palace-details-user-ctl">
              <div class="palace-detail-block palace-detail-block--user">
                <span class="palace-detail-label">Service user</span>
                <span class="palace-detail-value"><code>${esc(p.user || p.name)}</code></span>
              </div>
              <div class="palace-details-ctl-manage">
                <div class="palace-detail-block">
                  <span class="palace-detail-label">Control</span>
                  <div class="palace-detail-actions">${controlBtns}</div>
                </div>
                <div class="palace-detail-block">
                  <span class="palace-detail-label">Manage</span>
                  <div class="palace-detail-actions">
                    ${logsBtn}
                    ${usersBtn}
                    ${bansBtn}
                    ${mediaBtn}
                    ${filesBtn}
                    ${settingsBtn}
                    ${propsBtn}
                    ${pagesBtn}
                    ${backupsBtn}
                  </div>
                </div>
              </div>
            </div>
            ${palaceQuotaDetailBlockHTML(p)}
          </div>
          ${p.httpPort ? `
          <div class="palace-stats-strip" id="${sid}">
            <div class="palace-stats-layout">
              <section class="palace-stat-group palace-stat-group--live" aria-label="Live server stats">
                <h4 class="palace-stat-group-title">Now</h4>
                <div class="palace-stat-group--live-inner">
                  <div class="palace-gauge-wrap" id="${sid}-gauge">
                    <svg class="palace-gauge-svg" viewBox="0 0 100 100" role="img" aria-label="Load vs max users (concentric rings)">
                      <g transform="translate(50,50)">
                        <circle class="palace-gauge-track" r="34" cx="0" cy="0" fill="none" stroke-width="5" />
                        <circle class="palace-gauge-track" r="26" cx="0" cy="0" fill="none" stroke-width="5" />
                        <circle class="palace-gauge-track" r="18" cx="0" cy="0" fill="none" stroke-width="5" />
                        <circle class="palace-gauge-fill palace-gauge-fill--online" data-gauge="online" r="34" cx="0" cy="0" fill="none" stroke-width="5" stroke-dasharray="0 9999" transform="rotate(-90)">
                          <title>Online vs max (outer ring)</title>
                        </circle>
                        <circle class="palace-gauge-fill palace-gauge-fill--peak" data-gauge="peak" r="26" cx="0" cy="0" fill="none" stroke-width="5" stroke-dasharray="0 9999" transform="rotate(-90)">
                          <title>Record peak vs max (middle ring)</title>
                        </circle>
                        <circle class="palace-gauge-fill palace-gauge-fill--avg" data-gauge="avg" r="18" cx="0" cy="0" fill="none" stroke-width="5" stroke-dasharray="0 9999" transform="rotate(-90)">
                          <title>Average population vs max (inner ring)</title>
                        </circle>
                      </g>
                    </svg>
                    <ul class="palace-gauge-key">
                      <li><span class="palace-gauge-key-swatch palace-gauge-key-swatch--online" aria-hidden="true"></span> Outer — online ÷ max</li>
                      <li><span class="palace-gauge-key-swatch palace-gauge-key-swatch--peak" aria-hidden="true"></span> Middle — peak ÷ max</li>
                      <li><span class="palace-gauge-key-swatch palace-gauge-key-swatch--avg" aria-hidden="true"></span> Inner — avg pop. ÷ max</li>
                    </ul>
                  </div>
                  <div class="palace-stat-group-cells">
                    <div class="palace-stat-item palace-stat-item--hero" id="${sid}-online"><span class="palace-stat-value">—</span><span class="palace-stat-label">Online</span></div>
                    <div class="palace-stat-item palace-stat-item--tight" id="${sid}-max"><span class="palace-stat-value">—</span><span class="palace-stat-label">Max users</span></div>
                    <div class="palace-stat-item palace-stat-item--tight" id="${sid}-uptime"><span class="palace-stat-value">—</span><span class="palace-stat-label">Uptime</span></div>
                    <div class="palace-stat-item palace-stat-item--tight" id="${sid}-rooms"><span class="palace-stat-value">—</span><span class="palace-stat-label">Rooms</span></div>
                  </div>
                </div>
              </section>
              <section class="palace-stat-group" aria-label="Staff">
                <h4 class="palace-stat-group-title">Staff<span class="palace-stat-group-sub">Stack = share of connected staff · not vs max users</span></h4>
                <div class="palace-staff-stack" id="${sid}-staff-stack">
                  <div class="palace-staff-stack-bar" role="presentation">
                    <div class="palace-staff-stack-seg palace-staff-stack-wiz" data-staff="wiz" style="width:0%"></div>
                    <div class="palace-staff-stack-seg palace-staff-stack-god" data-staff="god" style="width:0%"></div>
                    <div class="palace-staff-stack-seg palace-staff-stack-owner" data-staff="owner" style="width:0%"></div>
                  </div>
                </div>
                <div class="palace-stat-group-cells">
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-ops"><span class="palace-stat-value">—</span><span class="palace-stat-label">Wizzes</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-gods"><span class="palace-stat-value">—</span><span class="palace-stat-label">Gods</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-owners"><span class="palace-stat-value">—</span><span class="palace-stat-label">Owners</span></div>
                </div>
              </section>
              <section class="palace-stat-group" aria-label="Visitors">
                <h4 class="palace-stat-group-title">Visitors</h4>
                <div class="palace-stat-group-cells">
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-today"><span class="palace-stat-value">—</span><span class="palace-stat-label">Today</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-week"><span class="palace-stat-value">—</span><span class="palace-stat-label">This week</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-uniquevis"><span class="palace-stat-value">—</span><span class="palace-stat-label">Unique</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-visitsper"><span class="palace-stat-value">—</span><span class="palace-stat-label">Visits / user</span></div>
                </div>
              </section>
              <section class="palace-stat-group" aria-label="Sessions and peak">
                <h4 class="palace-stat-group-title">Sessions &amp; peak</h4>
                <div class="palace-stat-group-cells">
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-avgtime"><span class="palace-stat-value">—</span><span class="palace-stat-label">Avg visit</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-avgpop"><span class="palace-stat-value">—</span><span class="palace-stat-label">Avg pop.</span></div>
                  <div class="palace-stat-item palace-stat-item--tight" id="${sid}-peakpop"><span class="palace-stat-value">—</span><span class="palace-stat-label">Peak pop.</span></div>
                  <div class="palace-stat-item palace-stat-item--tight palace-stat-item--wrap" id="${sid}-peakat"><span class="palace-stat-value">—</span><span class="palace-stat-label">Peak at</span></div>
                </div>
              </section>
            </div>
          </div>` : ''}
        </td> 
      </tr>`;
    }).join('');
    LAST_PALACE_MAIN_LIST = mainList;
    LAST_PALACE_LIST_META = { isAdmin, isTenant };
    syncPalaceStatsPolling(expandedList);
    syncCollapsedJoinPolling();
    updatePalaceJoinNotifyBar();
    requestAnimationFrame(() => renderPalaceQuotaCharts());
  } catch(e) {
    tbody.innerHTML = `<tr><td colspan="5" class="empty">Error: ${esc(e.message)}</td></tr>`;
    if (unregPanel) unregPanel.style.display = 'none';
    pauseCollapsedJoinPolling();
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
  if ($('pQuotaSlider')) $('pQuotaSlider').value = '6';
  if ($('pQuotaCustomMiB')) $('pQuotaCustomMiB').value = '';
  syncProvisionQuotaLabel();
  syncProvisionPortMode();
}
function closeProvisionModal() {
  $('provisionModal').classList.remove('open');
  loadPalaces();
}

function _setProvisionRunning(running) {
  ['pName','pYPHost','pYPPort','pTCPAuto','pHTTPAuto','pQuotaSlider','pQuotaCustomMiB'].forEach(id => {
    const el = $(id);
    if (el) el.disabled = running;
  });
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

  const quotaBytesMax = provisionQuotaSelectedBytes();
  if (quotaBytesMax === null) {
    alert('Enter a valid custom quota in MiB (e.g. 8192 for 8 GiB).');
    return;
  }

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
    body: JSON.stringify({ name, tcpPort, httpPort, ypHost, ypPort, quotaBytesMax }),
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

// Log modal (polls while open — near–real-time tail of pserver.log / chat.log)
let logLiveTimer = null;
let logLiveName = null;
let logActiveFile = 'pserver.log';
let logAllLines = [];
let logViewMode = 'server'; // 'server' | 'chat'
let logChatRawLines = [];
let logChatFormatHint = 'json';
let logChatFetchError = null;

function syncLogPanels() {
  const server = logViewMode === 'server';
  const sp = $('logServerPanel');
  const cp = $('logChatPanel');
  if (sp) sp.style.display = server ? '' : 'none';
  if (cp) cp.style.display = server ? 'none' : '';
}

function _parseCSVLine(line) {
  const out = [];
  let i = 0;
  const s = String(line || '');
  while (i < s.length) {
    if (s[i] === '"') {
      i++;
      let cell = '';
      while (i < s.length) {
        if (s[i] === '"' && s[i + 1] === '"') {
          cell += '"';
          i += 2;
          continue;
        }
        if (s[i] === '"') {
          i++;
          break;
        }
        cell += s[i++];
      }
      out.push(cell);
      if (s[i] === ',') i++;
      continue;
    }
    const comma = s.indexOf(',', i);
    if (comma < 0) {
      out.push(s.slice(i));
      break;
    }
    out.push(s.slice(i, comma));
    i = comma + 1;
  }
  return out;
}

const _CHAT_CSV_HEADER = [
  'ts', 'kind', 'room_id', 'room_name', 'target_room_id', 'target_room_name',
  'from_user_id', 'from_name', 'from_prefs_key', 'from_puid_ctr', 'from_crc', 'from_counter', 'from_guest',
  'to_user_id', 'to_name', 'to_prefs_key', 'to_puid_ctr', 'to_crc', 'to_counter', 'to_guest',
  'text', 'xtalk', 'undecrypted',
];

function _parseChatJSONLines(lines) {
  const out = [];
  for (const line of lines) {
    const t = String(line || '').trim();
    if (!t || !t.startsWith('{')) continue;
    try {
      out.push(JSON.parse(t));
    } catch (_) { /* skip */ }
  }
  return out;
}

function _parseChatCSV(lines) {
  if (!lines.length) return [];
  let start = 0;
  let header = _parseCSVLine(lines[0]);
  if (
    header.length >= 2 &&
    header[0].toLowerCase() === 'ts' &&
    header[1].toLowerCase() === 'kind'
  ) {
    start = 1;
  } else {
    header = _CHAT_CSV_HEADER.slice();
  }
  const out = [];
  for (let i = start; i < lines.length; i++) {
    const row = _parseCSVLine(lines[i].replace(/\r$/, ''));
    if (!row.length) continue;
    const o = {};
    header.forEach((h, idx) => {
      o[h.trim()] = row[idx];
    });
    out.push(o);
  }
  return out;
}

function parseChatLogLines(rawLines, formatHint) {
  const lines = (rawLines || []).map(l => String(l || '').replace(/\r$/, ''));
  const hint = String(formatHint || '').toLowerCase();
  let fmt = hint === 'csv' || hint === 'json' ? hint : '';
  if (!fmt) {
    const first = lines.find(l => l.trim());
    if (first && first.trim().startsWith('{')) fmt = 'json';
    else if (first && /^ts\s*,\s*kind\s*,/i.test(first.trim())) fmt = 'csv';
    else fmt = 'json';
  }
  return fmt === 'csv' ? _parseChatCSV(lines) : _parseChatJSONLines(lines);
}

function _chatEntryKindFilters() {
  return {
    basic: $('logChatKindBasic') && $('logChatKindBasic').checked,
    whisper: $('logChatKindWhisper') && $('logChatKindWhisper').checked,
    esp: $('logChatKindEsp') && $('logChatKindEsp').checked,
  };
}

function _chatEntryMatches(e, kinds, term) {
  const kind = String(e.kind || '').toLowerCase();
  if (kind === 'basic' && !kinds.basic) return false;
  if (kind === 'whisper' && !kinds.whisper) return false;
  if (kind === 'esp' && !kinds.esp) return false;
  if (kind && kind !== 'basic' && kind !== 'whisper' && kind !== 'esp') {
    if (!kinds.basic && !kinds.whisper && !kinds.esp) return false;
  }
  if (!term) return true;
  const fields = [
    e.ts, e.kind, e.room_name, e.room_id, e.target_room_name, e.target_room_id,
    e.from_name, e.to_name, e.text, e.from_user_id, e.to_user_id,
    e.from_prefs_key, e.to_prefs_key,
  ].map(x => String(x ?? '').toLowerCase());
  return fields.some(f => f.includes(term));
}

function _fmtChatTs(ts) {
  if (ts == null || ts === '') return '—';
  const d = new Date(ts);
  if (Number.isFinite(d.getTime())) return d.toLocaleString();
  return esc(String(ts));
}

function _fmtRoomLine(roomId, roomName) {
  const idPart = roomId !== undefined && roomId !== '' && String(roomId) !== '0'
    ? `#${esc(String(roomId))}`
    : (roomName ? '' : '—');
  const namePart = roomName ? esc(String(roomName)) : '';
  if (idPart && namePart) return `${idPart} ${namePart}`;
  if (namePart) return namePart;
  if (idPart) return idPart;
  return '—';
}

function _renderOneChatEntry(e) {
  const kind = String(e.kind || '').toLowerCase();
  const kindLabel = kind === 'basic' ? 'Message' : kind === 'whisper' ? 'Whisper' : kind === 'esp' ? 'ESP' : esc(kind || '?');
  let badgeClass = 'log-chat-kind-other';
  if (kind === 'basic') badgeClass = 'log-chat-kind-basic';
  else if (kind === 'whisper') badgeClass = 'log-chat-kind-whisper';
  else if (kind === 'esp') badgeClass = 'log-chat-kind-esp';

  let meta = '';
  if (kind === 'esp') {
    meta =
      `<div class="log-chat-meta">` +
      `<strong>From</strong> ${_fmtRoomLine(e.room_id, e.room_name)} · ` +
      `<strong>To</strong> ${_fmtRoomLine(e.target_room_id, e.target_room_name)}` +
      `</div>`;
  } else {
    meta = `<div class="log-chat-meta"><strong>Room</strong> ${_fmtRoomLine(e.room_id, e.room_name)}</div>`;
  }

  let users = '';
  if (kind === 'basic') {
    users =
      `<div class="log-chat-users">` +
      `<span class="log-chat-user">${esc(e.from_name)}</span> ` +
      `<span class="log-chat-id">(${esc(String(e.from_user_id ?? ''))})</span>` +
      `</div>`;
  } else {
    users =
      `<div class="log-chat-users">` +
      `<span class="log-chat-user">${esc(e.from_name)}</span> ` +
      `<span class="log-chat-id">(${esc(String(e.from_user_id ?? ''))})</span>` +
      ` → ` +
      `<span class="log-chat-user">${esc(e.to_name)}</span> ` +
      `<span class="log-chat-id">(${esc(String(e.to_user_id ?? ''))})</span>` +
      `</div>`;
  }

  const flags = [];
  if (e.xtalk === true || e.xtalk === 'true') flags.push('xtalk');
  if (e.undecrypted === true || e.undecrypted === 'true') flags.push('undecrypted');
  const flagHtml = flags.length
    ? `<span class="log-chat-flags">${flags.map(f => esc(f)).join(', ')}</span>`
    : '';

  return (
    `<div class="log-chat-entry">` +
    `<div class="log-chat-entry-head">` +
    `<span class="log-chat-ts">${_fmtChatTs(e.ts)}</span>` +
    `<span class="log-chat-kind-badge ${badgeClass}">${kindLabel}</span>${flagHtml}` +
    `</div>${meta}${users}` +
    `<div class="log-chat-text">${esc(e.text ?? '')}</div>` +
    `</div>`
  );
}

function renderChatLogView() {
  const content = $('logChatContent');
  const emptyEl = $('logChatEmpty');
  if (!content) return;

  const fromBottom = content.scrollHeight - content.scrollTop;
  const stickBottom = fromBottom <= content.clientHeight + 80;

  if (logChatFetchError) {
    content.innerHTML = `<div class="log-line"><span class="log-evt-error">${esc(logChatFetchError)}</span></div>`;
    if (emptyEl) emptyEl.style.display = 'none';
    return;
  }

  const entries = parseChatLogLines(logChatRawLines, logChatFormatHint);
  const term = ($('logChatSearch') && $('logChatSearch').value.toLowerCase().trim()) || '';
  const kinds = _chatEntryKindFilters();
  const filtered = entries.filter(e => _chatEntryMatches(e, kinds, term));

  if (entries.length === 0) {
    content.innerHTML = '';
    if (emptyEl) {
      emptyEl.textContent = 'No chat logs to view.';
      emptyEl.style.display = '';
    }
    return;
  }

  if (filtered.length === 0) {
    content.innerHTML = '<div class="log-chat-nomatch">No entries match your filters or search.</div>';
    if (emptyEl) emptyEl.style.display = 'none';
  } else {
    content.innerHTML = filtered.map(_renderOneChatEntry).join('');
    if (emptyEl) emptyEl.style.display = 'none';
  }

  if (stickBottom) {
    content.scrollTop = content.scrollHeight;
  } else {
    content.scrollTop = Math.max(0, content.scrollHeight - fromBottom);
  }
}

function applyChatLogFilter() {
  renderChatLogView();
}

async function fetchPalaceChatLogs(name) {
  if (!$('logModal').classList.contains('open') || name !== logLiveName) return;
  if (logViewMode !== 'chat') return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/chat-logs?lines=500`, { headers: headers() });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      logChatFetchError = data.error || `HTTP ${res.status}`;
      logChatRawLines = [];
      renderChatLogView();
      return;
    }
    const data = await res.json();
    logChatFetchError = null;
    logChatRawLines = data.lines || [];
    logChatFormatHint = data.format || 'json';
    renderChatLogView();
  } catch (e) {
    logChatFetchError = e.message || String(e);
    logChatRawLines = [];
    renderChatLogView();
  }
}

async function onLogViewKindChange() {
  const sel = $('logViewKind');
  logViewMode = sel && sel.value === 'chat' ? 'chat' : 'server';
  syncLogPanels();
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  if (!logLiveName) return;
  if (logViewMode === 'chat') {
    const cs = $('logChatSearch');
    if (cs) cs.value = '';
    logChatFetchError = null;
    await fetchPalaceChatLogs(logLiveName);
    if ($('logAutoUpdate') && $('logAutoUpdate').checked) {
      logLiveTimer = setInterval(() => fetchPalaceChatLogs(logLiveName), 2000);
    }
  } else {
    await _loadLogFileSelector(logLiveName);
    await selectLogFile(logLiveName, logActiveFile);
    if ($('logAutoUpdate') && $('logAutoUpdate').checked && logActiveFile === 'pserver.log') {
      logLiveTimer = setInterval(() => fetchPalaceLogs(logLiveName), 2000);
    }
  }
}

function _statusClass(code) {
  const n = Number(code);
  if (!Number.isFinite(n)) return 'log-http-status-unknown';
  if (n >= 200 && n < 300) return 'log-http-status-ok';
  if (n >= 300 && n < 400) return 'log-http-status-redirect';
  if (n >= 400 && n < 500) return 'log-http-status-client';
  if (n >= 500 && n < 600) return 'log-http-status-server';
  return 'log-http-status-unknown';
}

function _decorateLogIds(html) {
  return html
    .replace(/\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/g, '<span class="log-id">$&</span>')
    .replace(/\b(?:uuid|puid|crc|cnt|trackId|id|spec)=[^\s"]+/g, '<span class="log-id">$&</span>')
    .replace(/\{[A-Z0-9]{6,}\}/g, '<span class="log-id">$&</span>');
}

function _lineClass(raw) {
  const s = raw.toLowerCase();
  if (s.includes('error') || s.includes('failed') || s.includes('panic')) return 'log-evt-error';
  if (s.includes('signal terminated') || s.includes('shutting down') || s.includes('exiting') || s.includes('sighup')) return 'log-evt-shutdown';
  if (s.includes('starting') || s.includes('server ready') || s.includes('listening on') || s.includes('loaded ') || s.includes('watching ')) return 'log-evt-startup';
  if (s.includes('audit ') || s.includes('made owner') || s.includes('unbanned by') || s.includes('page from system')) return 'log-evt-audit';
  if (s.includes('tracked user') || s.includes('trackip')) return 'log-evt-track';
  if (s.includes('new connection') || s.includes('logged on') || s.includes('disconnecting') || s.includes('changed name')) return 'log-evt-user';
  return '';
}

function _renderLogLine(rawLine) {
  let line = String(rawLine || '');
  let tsHtml = '';
  const ts = line.match(/^((?:\d{4}-\d{2}-\d{2}\s+)?\d{2}:\d{2}:\d{2})(\s+)(.*)$/);
  if (ts) {
    tsHtml = `<span class="log-ts">${esc(ts[1])}</span>${esc(ts[2])}`;
    line = ts[3];
  }

  const http = line.match(/^HTTP\s+([A-Z]+)\s+(\S+)\s+->\s+(\d{3})$/);
  if (http) {
    const statusClass = _statusClass(http[3]);
    return `<div class="log-line">${tsHtml}<span class="log-http-method">HTTP ${esc(http[1])}</span> <span class="log-http-path">${esc(http[2])}</span> -> <span class="${statusClass}">${esc(http[3])}</span></div>`;
  }

  const cls = _lineClass(line);
  const body = _decorateLogIds(esc(line));
  if (cls) return `<div class="log-line"><span class="${cls}">${tsHtml}${body}</span></div>`;
  return `<div class="log-line">${tsHtml}${body}</div>`;
}

function _renderLogContent() {
  const el = $('logContent');
  const searchEl = $('logSearch');
  const term = searchEl ? searchEl.value.toLowerCase().trim() : '';
  const fromBottom = el.scrollHeight - el.scrollTop;
  const stickBottom = fromBottom <= el.clientHeight + 80;
  const lines = term ? logAllLines.filter(l => l.toLowerCase().includes(term)) : logAllLines;
  el.innerHTML = lines.map(_renderLogLine).join('');
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
  if (logViewMode !== 'server' || logActiveFile !== 'pserver.log') return;
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
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  if (!checked || !logLiveName) return;
  if (logViewMode === 'server' && logActiveFile === 'pserver.log') {
    logLiveTimer = setInterval(() => fetchPalaceLogs(logLiveName), 2000);
  } else if (logViewMode === 'chat') {
    logLiveTimer = setInterval(() => fetchPalaceChatLogs(logLiveName), 2000);
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
    if (autoUpdate && autoUpdate.checked && !logLiveTimer && logViewMode === 'server') {
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
  logViewMode = 'server';
  logChatRawLines = [];
  logChatFormatHint = 'json';
  logChatFetchError = null;
  const vk = $('logViewKind');
  if (vk) vk.value = 'server';
  syncLogPanels();

  $('logModalTitle').textContent = `Logs — ${name}`;
  $('logContent').textContent = 'Loading…';
  $('logFileSelector').innerHTML = '';
  const searchEl = $('logSearch');
  if (searchEl) searchEl.value = '';
  const chatSearch = $('logChatSearch');
  if (chatSearch) chatSearch.value = '';
  const autoUpdate = $('logAutoUpdate');
  if (autoUpdate) autoUpdate.checked = true;

  $('logModal').classList.add('open');

  await Promise.all([
    _loadLogFileSelector(name),
    fetchPalaceLogs(name),
  ]);

  if ($('logAutoUpdate') && $('logAutoUpdate').checked) {
    logLiveTimer = setInterval(() => fetchPalaceLogs(name), 2000);
  }
}

function closeLogModal() {
  if (logLiveTimer) {
    clearInterval(logLiveTimer);
    logLiveTimer = null;
  }
  logLiveName = null;
  logActiveFile = 'pserver.log';
  logAllLines = [];
  logViewMode = 'server';
  logChatRawLines = [];
  logChatFormatHint = 'json';
  logChatFetchError = null;
  const vk = $('logViewKind');
  if (vk) vk.value = 'server';
  syncLogPanels();
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

function formatStatFloat(v, digits = 2) {
  const n = Number(v);
  if (!Number.isFinite(n)) return '—';
  return n.toFixed(digits);
}

function formatPeakAt(iso) {
  if (!iso) return 'Unknown';
  const d = new Date(iso);
  if (!Number.isFinite(d.getTime())) return String(iso);
  return d.toLocaleString();
}

function setStatEl(sid, key, value) {
  const el = document.getElementById(`${sid}-${key}`);
  if (!el) return;
  const val = el.querySelector('.palace-stat-value');
  if (val) val.textContent = value;
}

function _palaceGaugeRingFraction(circle, fraction) {
  if (!circle) return;
  const r = parseFloat(circle.getAttribute('r'));
  if (!Number.isFinite(r) || r <= 0) return;
  const C = 2 * Math.PI * r;
  const f = Math.max(0, Math.min(1, Number(fraction) || 0));
  circle.setAttribute('stroke-dasharray', `${f * C} ${C}`);
}

function _palaceGaugeSetTitle(circle, text) {
  if (!circle) return;
  let t = circle.querySelector('title');
  if (!t) {
    t = document.createElementNS('http://www.w3.org/2000/svg', 'title');
    circle.appendChild(t);
  }
  t.textContent = text;
}

/** Concentric rings: online/max (outer), peak/max (middle), avg pop/max (inner). Tooltips: SVG title elements. */
function applyPalaceGauge(sid, d) {
  const wrap = document.getElementById(`${sid}-gauge`);
  if (!wrap) return;
  const onlineRing = wrap.querySelector('[data-gauge="online"]');
  const peakRing = wrap.querySelector('[data-gauge="peak"]');
  const avgRing = wrap.querySelector('[data-gauge="avg"]');
  if (!d) {
    _palaceGaugeRingFraction(onlineRing, 0);
    _palaceGaugeRingFraction(peakRing, 0);
    _palaceGaugeRingFraction(avgRing, 0);
    _palaceGaugeSetTitle(onlineRing, 'Online vs max — no data');
    _palaceGaugeSetTitle(peakRing, 'Peak vs max — no data');
    _palaceGaugeSetTitle(avgRing, 'Avg population vs max — no data');
    return;
  }
  const maxU = Number(d.max_users);
  if (!Number.isFinite(maxU) || maxU <= 0) {
    _palaceGaugeRingFraction(onlineRing, 0);
    _palaceGaugeRingFraction(peakRing, 0);
    _palaceGaugeRingFraction(avgRing, 0);
    _palaceGaugeSetTitle(onlineRing, 'Online vs max — max users not available');
    _palaceGaugeSetTitle(peakRing, 'Peak vs max — max users not available');
    _palaceGaugeSetTitle(avgRing, 'Avg population vs max — max users not available');
    return;
  }
  const online = Number(d.user_count) || 0;
  const peak = Number(d.peak_population) || 0;
  const avgPop = Number(d.average_population) || 0;
  _palaceGaugeRingFraction(onlineRing, online / maxU);
  _palaceGaugeRingFraction(peakRing, peak / maxU);
  _palaceGaugeRingFraction(avgRing, avgPop / maxU);
  const pct = n => `${Math.round((Math.min(n, maxU) / maxU) * 1000) / 10}%`;
  _palaceGaugeSetTitle(
    onlineRing,
    `Now online: ${online} / ${maxU} users (${pct(online)}) — outer ring; full circle = 100% of max`
  );
  _palaceGaugeSetTitle(
    peakRing,
    `Record peak: ${peak} / ${maxU} users (${pct(peak)}) — middle ring`
  );
  _palaceGaugeSetTitle(
    avgRing,
    `Average population: ${formatStatFloat(avgPop, 2)} / ${maxU} (${pct(avgPop)}) — inner ring`
  );
}

/** Horizontal stack: wizzes / gods / owners as share of total connected staff (not vs max users). */
function applyPalaceStaffStack(sid, d) {
  const root = document.getElementById(`${sid}-staff-stack`);
  if (!root) return;
  const wizEl = root.querySelector('[data-staff="wiz"]');
  const godEl = root.querySelector('[data-staff="god"]');
  const ownEl = root.querySelector('[data-staff="owner"]');
  if (!wizEl || !godEl || !ownEl) return;

  const zeroBar = () => {
    wizEl.style.width = '0%';
    godEl.style.width = '0%';
    ownEl.style.width = '0%';
  };

  if (!d) {
    zeroBar();
    wizEl.removeAttribute('title');
    godEl.removeAttribute('title');
    ownEl.removeAttribute('title');
    return;
  }

  const wiz = Math.max(0, Math.floor(Number(d.operators) || 0));
  const god = Math.max(0, Math.floor((Number(d.gods) || 0) + (Number(d.hosts) || 0)));
  let own = Number(d.owners);
  if (!Number.isFinite(own)) own = 0;
  own = Math.max(0, Math.floor(own));
  const total = wiz + god + own;

  if (total <= 0) {
    zeroBar();
    wizEl.title = 'Wizzes: 0';
    godEl.title = 'Gods: 0';
    ownEl.title = 'Owners: 0';
    return;
  }

  let pw = (100 * wiz) / total;
  let pg = (100 * god) / total;
  let po = (100 * own) / total;
  const drift = 100 - (pw + pg + po);
  if (Math.abs(drift) > 0.001) {
    po += drift;
  }

  wizEl.style.width = `${pw}%`;
  godEl.style.width = `${pg}%`;
  ownEl.style.width = `${po}%`;

  const share = n => `${Math.round((n / total) * 1000) / 10}%`;
  wizEl.title = `Wizzes: ${wiz} (${share(wiz)} of connected staff)`;
  godEl.title = `Gods: ${god} (${share(god)} of connected staff)`;
  ownEl.title = `Owners: ${own} (${share(own)} of connected staff)`;
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
  setStatEl(sid, 'uniquevis', d.total_unique_visitors ?? '—');
  setStatEl(sid, 'visitsper', formatStatFloat(d.visits_per_user, 2));
  setStatEl(sid, 'avgtime', `${Math.round(Number(d.average_visit_seconds) || 0)}s`);
  setStatEl(sid, 'avgpop', formatStatFloat(d.average_population, 2));
  setStatEl(sid, 'peakpop', d.peak_population ?? '—');
  setStatEl(sid, 'peakat', formatPeakAt(d.peak_population_at));
  setStatEl(sid, 'uptime', formatUptime(d.start_time));
  applyPalaceGauge(sid, d);
  applyPalaceStaffStack(sid, d);
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
      applyPalaceGauge(sid, null);
      applyPalaceStaffStack(sid, null);
      return;
    }
    const d = await res.json();
    const entry = PALACE_STAT_TIMERS.get(name) || {};
    entry.startTime = d.start_time;
    entry.lastData  = d;
    PALACE_STAT_TIMERS.set(name, entry);

    applyPalaceStats(sid, d);
    maybeNotifyPalaceUserJoin(name, d);
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

const PALACE_USERS_IPINFO_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false"><path d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7c-2.76 0-5 2.24-5 5s2.24 5 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1zM8 13h8v-2H8v2zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4c2.76 0 5-2.24 5-5s-2.24-5-5-5z"/></svg>';

function palaceUsersIpInfoHref(ip) {
  const s = String(ip || '').trim();
  if (!s) return '#';
  return 'https://ipinfo.io/' + encodeURIComponent(s);
}

function palaceUsersIpCell(ip) {
  const s = String(ip || '').trim();
  if (!s) return '<span class="muted">—</span>';
  const href = palaceUsersIpInfoHref(s);
  const label = `View ${s} on ipinfo.io (opens in new tab)`;
  return `<a class="palace-users-ipinfo" href="${esc(href)}" target="_blank" rel="noopener noreferrer" title="${esc(label)}" aria-label="${esc(label)}">${esc(s)}${PALACE_USERS_IPINFO_SVG}</a>`;
}

function palaceUsersOsCell(os) {
  const raw = String(os || '').trim() || '?';
  let src = '';
  switch (raw) {
    case 'Mac':
      src = '/img/os/mac.png';
      break;
    case 'Windows':
      src = '/img/os/win.png';
      break;
    case 'Linux':
      src = '/img/os/lnx.png';
      break;
    default:
      break;
  }
  if (src) {
    const t = esc(raw);
    return `<span class="palace-users-os-wrap" title="${t}"><img class="palace-users-os-icon" src="${src}" width="20" height="20" alt="${t}" /></span>`;
  }
  return `<span class="palace-users-os-text" title="${esc(raw)}">${esc(raw)}</span>`;
}

let palaceUsersLiveName = null;
let palaceUsersTimer = null;
let palaceUsersRows = [];
let palaceUsersSelectedUser = null;
let palaceUsersSelectedAction = 'ban';

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
      palaceUsersRows = [];
      $('palaceUsersBody').innerHTML = `<div class="empty">Error: HTTP ${res.status}</div>`;
      return;
    }
    const users = await res.json();
    palaceUsersRows = Array.isArray(users) ? users : [];
    if (!Array.isArray(users) || users.length === 0) {
      $('palaceUsersCount').textContent = '0 users';
      $('palaceUsersBody').innerHTML = '<div class="empty">No users connected</div>';
      return;
    }
    $('palaceUsersCount').textContent = `${users.length} user${users.length === 1 ? '' : 's'}`;
    $('palaceUsersBody').innerHTML = users.map(u => `
      <article class="palace-user-card">
        <div class="palace-user-card-head">
          <div class="palace-user-main">
            <strong>${esc(u.name || '?')}</strong>
            <span class="badge">${esc(u.role || '?')}</span>
            <span class="palace-user-meta">#${u.id} · ${formatSignonTime(u.signon_seconds)}</span>
          </div>
          <button type="button" class="palace-user-moderate-btn" data-user-id="${u.id}">Actions…</button>
        </div>
        <div class="palace-user-grid">
          <div><span class="k">Client</span><code>${esc(u.client_version || '?')}</code></div>
          <div class="palace-user-os-col"><span class="k">OS</span>${palaceUsersOsCell(u.os)}</div>
          <div><span class="k">Room</span>${esc(u.room_name || '?')}</div>
          <div><span class="k">IP</span><span class="palace-users-ip-cell">${palaceUsersIpCell(u.ip)}</span></div>
          <div class="wide"><span class="k">UUID</span><code>${esc(u.uuid || '')}</code></div>
          <div><span class="k">PUID</span><code>${u.puid_ctr || 0}</code></div>
          <div><span class="k">CRC</span><code>${esc(u.crc || '')}</code></div>
          <div><span class="k">CNT</span><code>${u.cnt || 0}</code></div>
          <div class="wide"><span class="k">Key</span><code>${esc(u.wiz_key || '')}</code></div>
        </div>
      </article>`).join('');
  } catch (e) {
    palaceUsersRows = [];
    $('palaceUsersBody').innerHTML = `<div class="empty">Error: ${esc(e.message)}</div>`;
  }
}

async function openPalaceUsersModal(name) {
  if (palaceUsersTimer) {
    clearInterval(palaceUsersTimer);
    palaceUsersTimer = null;
  }
  palaceUsersLiveName = name;
  palaceUsersRows = [];
  palaceUsersSelectedUser = null;
  $('palaceUsersModalTitle').textContent = `Connected Users — ${name}`;
  $('palaceUsersCount').textContent = '';
  $('palaceUsersBody').innerHTML = '<div class="empty">Loading…</div>';
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
  palaceUsersRows = [];
  closePalaceUserActionModal();
  $('palaceUsersModal').classList.remove('open');
}

function palaceUserActionLabel(action) {
  switch (action) {
    case 'ban': return 'Ban';
    case 'kill': return 'Kill';
    case 'track': return 'Track';
    case 'disconnect': return 'Disconnect';
    default: return action;
  }
}

function setPalaceUserActionAlert(msg, type) {
  const el = $('palaceUserActionAlert');
  if (!el) return;
  el.textContent = msg || '';
  el.className = 'bans-alert' + (type ? ' ' + type : '');
  el.style.display = type ? 'block' : 'none';
}

function setPalaceUserActionMode(action) {
  palaceUsersSelectedAction = action;
  const duration = $('palaceUserActionDuration');
  const label = $('palaceUserActionDurationLabel');
  const hint = $('palaceUserActionHint');
  const buttons = document.querySelectorAll('.palace-user-action-pick');
  buttons.forEach(btn => btn.classList.toggle('primary', btn.dataset.action === action));
  buttons.forEach(btn => { if (btn.dataset.action !== action) btn.classList.remove('primary'); });

  if (action === 'ban') {
    duration.disabled = false;
    duration.value = duration.value || '1d';
    label.textContent = 'Ban Duration';
    hint.textContent = 'Timed deny + kick. Use 0 for permanent.';
  } else if (action === 'kill') {
    duration.disabled = false;
    duration.value = duration.value || '5';
    label.textContent = 'Duration';
    hint.textContent = 'Timed lockout. Use 0 for kick-only.';
  } else if (action === 'track') {
    duration.disabled = false;
    duration.value = duration.value || '60';
    label.textContent = 'Track Duration';
    hint.textContent = 'Creates a track record (no kick).';
  } else {
    duration.disabled = true;
    duration.value = '0';
    label.textContent = 'Duration';
    hint.textContent = 'Immediate disconnect (same as kill 0).';
  }
}

function openPalaceUserActionModal(userId) {
  const targetID = parseInt(userId, 10);
  const u = palaceUsersRows.find(row => parseInt(row.id, 10) === targetID);
  if (!u) return;
  palaceUsersSelectedUser = u;
  setPalaceUserActionAlert('');
  $('palaceUserActionTitle').textContent = `Moderate User — ${u.name || ('#' + u.id)}`;
  $('palaceUserActionDetails').textContent = [
    `ID: ${u.id}`,
    `Online: ${formatSignonTime(u.signon_seconds || 0)}`,
    `Role: ${u.role || '?'}`,
    `Name: ${u.name || '?'}`,
    `Client: ${u.client_version || '?'}`,
    `OS: ${u.os || '?'}`,
    `Room: ${u.room_name || '?'}`,
    `IP: ${u.ip || '?'}`,
    `UUID: ${u.uuid || ''}`,
    `PUID: ${u.puid_ctr || 0}`,
    `CRC: ${u.crc || ''}`,
    `CNT: ${u.cnt || 0}`,
    `Key: ${u.wiz_key || ''}`,
  ].join('\n');
  $('palaceUserActionReason').value = '';
  $('palaceUserActionDuration').value = '';
  $('palaceUserActionSubmit').disabled = false;
  setPalaceUserActionMode('ban');
  $('palaceUserActionModal').classList.add('open');
}

function closePalaceUserActionModal() {
  palaceUsersSelectedUser = null;
  $('palaceUserActionModal').classList.remove('open');
}

async function submitPalaceUserAction() {
  if (!palaceUsersSelectedUser || !palaceUsersLiveName) return;
  const action = palaceUsersSelectedAction;
  const body = {
    action,
    target_user_id: palaceUsersSelectedUser.id,
    reason: $('palaceUserActionReason').value.trim(),
  };
  const duration = ($('palaceUserActionDuration').value || '').trim();
  if (action === 'ban' || action === 'kill' || action === 'track') {
    if (!duration) {
      setPalaceUserActionAlert('Duration is required for this action.', 'error');
      return;
    }
    body.duration = duration;
  } else if (action === 'disconnect') {
    body.duration = '0';
  }

  const btn = $('palaceUserActionSubmit');
  btn.disabled = true;
  setPalaceUserActionAlert('');
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palaceUsersLiveName)}/palace-users/moderate`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify(body),
    });
    const out = await res.json().catch(() => ({}));
    if (!res.ok) {
      setPalaceUserActionAlert(out.error || out.message || ('HTTP ' + res.status), 'error');
      btn.disabled = false;
      return;
    }
    setPalaceUserActionAlert(out.message || `${palaceUserActionLabel(action)} sent.`, 'success');
    await fetchPalaceUsers(palaceUsersLiveName);
    setTimeout(() => {
      if ($('palaceUserActionModal').classList.contains('open')) closePalaceUserActionModal();
    }, 700);
  } catch (e) {
    setPalaceUserActionAlert('Network error: ' + e.message, 'error');
    btn.disabled = false;
  }
}

document.addEventListener('click', function (ev) {
  const actionBtn = ev.target.closest('.palace-user-moderate-btn');
  if (actionBtn) {
    openPalaceUserActionModal(actionBtn.dataset.userId);
    return;
  }
  const pickBtn = ev.target.closest('.palace-user-action-pick');
  if (pickBtn) {
    setPalaceUserActionMode(pickBtn.dataset.action || 'ban');
  }
});
