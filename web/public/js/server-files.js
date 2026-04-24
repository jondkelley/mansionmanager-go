async function loadServerFilesPalaces() {
  const sel = $('sfPalace');
  try {
    const res = await fetch('/api/palaces', { headers: headers() });
    if (!res.ok) return;
    const rows = await res.json();
    const prev = sel.value || SF_PALACE;
    if (!Array.isArray(rows) || rows.length === 0) {
      sel.innerHTML = '<option value="">— no palaces —</option>';
      sel.disabled = true;
      return;
    }
    sel.disabled = false;
    sel.innerHTML = rows.map(p =>
      `<option value="${attrEsc(p.name)}">${esc(p.name)}</option>`
    ).join('');
    const pick = prev && rows.some(r => r.name === prev) ? prev : rows[0].name;
    sel.value = pick;
    SF_PALACE = pick;
  } catch (_) {}
}

async function loadServerFileList() {
  const sel = $('sfPalace');
  const name = sel.value;
  const tbody = $('sfFilesBody');
  $('sfViewerWrap').style.display = 'none';
  $('sfContent').value = '';
  $('sfBinaryNote').style.display = 'none';
  $('sfSaveBtn').style.display = 'none';
  SF_FILE = '';
  SF_FILE_ENCODING = 'utf8';
  SF_ALLOW_SAVE = false;
  SF_PALACE = name || '';
  if (!name) {
    tbody.innerHTML = '<tr><td colspan="3" class="empty">No palace selected.</td></tr>';
    return;
  }
  tbody.innerHTML = '<tr><td colspan="3" class="empty">Loading…</td></tr>';
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/server-files`, { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="3" class="empty">${esc(data.error || ('HTTP ' + res.status))}</td></tr>`;
      return;
    }
    const files = data.files || [];
    if (files.length === 0) {
      tbody.innerHTML = '<tr><td colspan="3" class="empty">No matching files (pserver.pat, pserver.prefs, pserver.log / rotated logs, or *.json in palace root).</td></tr>';
      return;
    }
    tbody.innerHTML = files.map(f => {
      const fn = JSON.stringify(f.name);
      const nm = JSON.stringify(name);
      const sz = typeof f.size === 'number' ? formatBytes(f.size) : '—';
      const logFam = isPalaceServerLogFamily(f.name);
      const editorBtn = logFam
        ? ''
        : `<button type="button" onclick='viewServerFile(${nm}, ${fn})'>Editor</button>`;
      return `<tr>
        <td><code>${esc(f.name)}</code></td>
        <td>${esc(sz)}</td>
        <td class="actions">
          <button type="button" onclick='openServerFileInBrowser(${nm}, ${fn})'>View</button>
          <button type="button" onclick='downloadServerFileDirect(${nm}, ${fn})'>Download</button>
          ${editorBtn}
        </td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="3" class="empty">${esc(e.message)}</td></tr>`;
  }
}

function openPatUploadModal() {
  const name = $('sfPalace').value;
  if (!name) {
    alert('Select a palace first.');
    return;
  }
  PAT_UPLOAD_NAME = name;
  $('patUploadPalaceLabel').innerHTML = 'Palace: <strong>' + esc(name) + '</strong>';
  $('patUploadError').textContent = '';
  $('patUploadFile').value = '';
  $('patUploadStepConfirm').style.display = '';
  $('patUploadStepUpload').style.display = 'none';
  const btn = $('patUploadSubmitBtn');
  if (btn) btn.disabled = false;
  $('patUploadModal').classList.add('open');
}

function patUploadShowPicker() {
  $('patUploadStepConfirm').style.display = 'none';
  $('patUploadStepUpload').style.display = 'block';
}

function closePatUploadModal() {
  $('patUploadModal').classList.remove('open');
  PAT_UPLOAD_NAME = '';
  $('patUploadStepConfirm').style.display = '';
  $('patUploadStepUpload').style.display = 'none';
}

async function submitPatUpload() {
  const name = PAT_UPLOAD_NAME;
  const inp = $('patUploadFile');
  if (!name || !inp.files || inp.files.length === 0) {
    $('patUploadError').textContent = 'Choose a file.';
    return;
  }
  $('patUploadError').textContent = '';
  const btn = $('patUploadSubmitBtn');
  btn.disabled = true;
  try {
    const fd = new FormData();
    fd.append('file', inp.files[0]);
    const h = { ...headers() };
    delete h['Content-Type'];
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/pat-upload`, { method: 'POST', headers: h, body: fd });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('patUploadError').textContent = data.error || ('HTTP ' + res.status);
      btn.disabled = false;
      return;
    }
    closePatUploadModal();
    loadPalaces();
    if ($('tab-server-files').classList.contains('active')) {
      loadServerFileList();
    }
  } catch (e) {
    $('patUploadError').textContent = e.message;
    btn.disabled = false;
  }
}

async function openServerFileInBrowser(palaceName, fileName) {
  try {
    const url = `/api/palaces/${encodeURIComponent(palaceName)}/server-files/${encodeURIComponent(fileName)}?inline=1`;
    const res = await fetch(url, { headers: authHeaders() });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    const blob = await res.blob();
    const objUrl = URL.createObjectURL(blob);
    window.open(objUrl, '_blank', 'noopener,noreferrer');
    setTimeout(() => URL.revokeObjectURL(objUrl), 120000);
  } catch (e) {
    alert(e.message);
  }
}

async function downloadServerFileDirect(palaceName, fileName) {
  try {
    const url = `/api/palaces/${encodeURIComponent(palaceName)}/server-files/${encodeURIComponent(fileName)}?download=1`;
    const res = await fetch(url, { headers: authHeaders() });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      alert(data.error || ('HTTP ' + res.status));
      return;
    }
    const blob = await res.blob();
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = href;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(href);
  } catch (e) {
    alert(e.message);
  }
}

async function viewServerFile(palaceName, fileName) {
  SF_PALACE = palaceName;
  SF_FILE = fileName;
  $('sfViewerWrap').style.display = 'block';
  $('sfViewerTitle').textContent = fileName;
  $('sfContent').value = 'Loading…';
  $('sfBinaryNote').style.display = 'none';
  $('sfDownloadBtn').disabled = false;
  $('sfSaveBtn').style.display = 'none';
  SF_FILE_ENCODING = 'utf8';
  SF_ALLOW_SAVE = false;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(palaceName)}/server-files/${encodeURIComponent(fileName)}`, { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      SF_ALLOW_SAVE = false;
      $('sfSaveBtn').style.display = 'none';
      $('sfContent').readOnly = true;
      $('sfContent').value = data.error || ('HTTP ' + res.status);
      return;
    }
    if (data.encoding === 'base64') {
      SF_FILE_ENCODING = 'base64';
      SF_ALLOW_SAVE = false;
      $('sfContent').readOnly = true;
      $('sfSaveBtn').style.display = 'none';
      $('sfContent').value = data.content || '';
      $('sfBinaryNote').style.display = 'block';
    } else {
      SF_FILE_ENCODING = 'utf8';
      const logFam = isPalaceServerLogFamily(fileName);
      SF_ALLOW_SAVE = !logFam;
      $('sfContent').readOnly = logFam;
      $('sfSaveBtn').style.display = logFam ? 'none' : '';
      let text = typeof data.content === 'string' ? data.content : JSON.stringify(data, null, 2);
      if (fileName.toLowerCase().endsWith('.json')) {
        try {
          text = JSON.stringify(JSON.parse(text), null, 2);
        } catch (_) { /* leave raw */ }
      }
      $('sfContent').value = text;
    }
  } catch (e) {
    SF_ALLOW_SAVE = false;
    $('sfSaveBtn').style.display = 'none';
    $('sfContent').readOnly = true;
    $('sfContent').value = 'Error: ' + e.message;
  }
}

async function downloadServerFileBlob() {
  if (!SF_PALACE || !SF_FILE) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SF_PALACE)}/server-files/${encodeURIComponent(SF_FILE)}?download=1`, { headers: authHeaders() });
    if (!res.ok) {
      alert('Download failed: HTTP ' + res.status);
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = SF_FILE;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  } catch (e) {
    alert(e.message);
  }
}

function formatLocalBackupStamp(d) {
  const p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}_${p(d.getHours())}-${p(d.getMinutes())}-${p(d.getSeconds())}`;
}

function padTar512(len) {
  const r = len % 512;
  return r === 0 ? len : len + (512 - r);
}

function writeTarOctalField(header, offset, fieldLen, value) {
  const te = new TextEncoder();
  const str = value.toString(8) + '\0';
  const enc = te.encode(str);
  header.fill(0, offset, offset + fieldLen);
  for (let i = 0; i < Math.min(fieldLen, enc.length); i++) header[offset + i] = enc[i];
}

/** Single-file POSIX ustar tarball (raw bytes). */
function createTarArchiveSingle(fileName, data) {
  if (fileName.length > 100) throw new Error('File name is too long for tar (max 100 characters).');
  const te = new TextEncoder();
  const header = new Uint8Array(512);
  header.fill(0);
  header.set(te.encode(fileName), 0);
  writeTarOctalField(header, 100, 8, 0o644);
  writeTarOctalField(header, 108, 8, 0);
  writeTarOctalField(header, 116, 8, 0);
  writeTarOctalField(header, 124, 12, data.length);
  writeTarOctalField(header, 136, 12, Math.floor(Date.now() / 1000));
  header[156] = 48; // '0' regular file
  header.set(te.encode('ustar'), 257);
  header.set(te.encode('00'), 263);
  for (let i = 148; i < 156; i++) header[i] = 0x20;
  let sum = 0;
  for (let i = 0; i < 512; i++) sum += header[i];
  const chkStr = sum.toString(8).padStart(6, '0') + '\0 ';
  const chkEnc = te.encode(chkStr);
  const chkFinal = new Uint8Array(8);
  chkFinal.fill(32);
  for (let i = 0; i < Math.min(8, chkEnc.length); i++) chkFinal[i] = chkEnc[i];
  header.set(chkFinal, 148);

  const padded = padTar512(data.length);
  const out = new Uint8Array(512 + padded + 1024);
  out.set(header, 0);
  out.set(data, 512);
  return out;
}

async function gzipUint8(raw) {
  if (typeof CompressionStream === 'undefined') {
    throw new Error('This browser cannot gzip (no CompressionStream). Use Download for a raw copy.');
  }
  const blob = new Blob([raw]);
  const stream = blob.stream().pipeThrough(new CompressionStream('gzip'));
  return new Uint8Array(await new Response(stream).arrayBuffer());
}

function triggerDownloadBlob(blob, filename) {
  const href = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(href);
}

async function sfDownloadBackupTarGz() {
  const palaceName = SF_PALACE;
  const fileName = SF_FILE;
  if (!palaceName || !fileName) return;
  const url = `/api/palaces/${encodeURIComponent(palaceName)}/server-files/${encodeURIComponent(fileName)}?download=1`;
  const res = await fetch(url, { headers: authHeaders() });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || ('HTTP ' + res.status));
  }
  const raw = new Uint8Array(await res.arrayBuffer());
  const tar = createTarArchiveSingle(fileName, raw);
  const gz = await gzipUint8(tar);
  const stamp = formatLocalBackupStamp(new Date());
  triggerDownloadBlob(new Blob([gz], { type: 'application/gzip' }), `${fileName}.${stamp}.tar.gz`);
}

function sfSaveBackupClick(ev) {
  ev.preventDefault();
  $('sfSaveError').textContent = '';
  sfDownloadBackupTarGz().catch(e => {
    $('sfSaveError').textContent = e.message || String(e);
  });
}

function openSfSaveModal() {
  if (!SF_PALACE || !SF_FILE || !SF_ALLOW_SAVE) return;
  $('sfSaveError').textContent = '';
  $('sfSaveModal').classList.add('open');
}

function closeSfSaveModal() {
  $('sfSaveModal').classList.remove('open');
}

async function confirmSfSave() {
  $('sfSaveError').textContent = '';
  if (!SF_PALACE || !SF_FILE || !SF_ALLOW_SAVE) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(SF_PALACE)}/server-files/${encodeURIComponent(SF_FILE)}`, {
      method: 'PUT',
      headers: headers(),
      body: JSON.stringify({ content: $('sfContent').value }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('sfSaveError').textContent = data.error || ('HTTP ' + res.status);
      return;
    }
    closeSfSaveModal();
    await viewServerFile(SF_PALACE, SF_FILE);
  } catch (e) {
    $('sfSaveError').textContent = e.message || String(e);
  }
}
