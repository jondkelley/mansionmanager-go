let MEDIA_MODAL_NAME = '';
let MEDIA_PREVIEW_OBJECT_URL = '';
const MEDIA_PREVIEW_MAX_BYTES = 20 << 20;
const MEDIA_IMAGE_EXTENSIONS = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'avif'];
const MEDIA_VIDEO_EXTENSIONS = ['mp4', 'webm', 'ogv', 'mov', 'm4v'];
const MEDIA_AUDIO_EXTENSIONS = ['mp3', 'wav', 'ogg', 'opus', 'm4a', 'aac', 'flac'];
const MEDIA_TEXT_EXTENSIONS = ['txt', 'log', 'json', 'xml', 'csv', 'md', 'html', 'htm', 'css', 'js', 'ts', 'mjs', 'c', 'cpp', 'h', 'hpp', 'plist', 'yaml', 'yml', 'sh', 'bat', 'ini', 'conf', 'cfg', 'pat', 'prefs'];
/** @type {File[] | null} */
let MEDIA_PENDING_UPLOAD_FILES = null;
let MEDIA_RENAME_FROM = '';
let MEDIA_DELETE_PATH = '';
let mediaSearchDebounce = null;
let MEDIA_SORT_KEY = 'name';
let MEDIA_SORT_DIR = 'asc';

function getMediaUploadFiles() {
  if (MEDIA_PENDING_UPLOAD_FILES && MEDIA_PENDING_UPLOAD_FILES.length) return MEDIA_PENDING_UPLOAD_FILES;
  const inp = $('mediaUploadInput');
  if (!inp || !inp.files || inp.files.length === 0) return [];
  return Array.from(inp.files);
}

function updateMediaUploadHint() {
  const line = $('mediaUploadHintLine');
  if (!line) return;
  const n = getMediaUploadFiles().length;
  if (n === 0) {
    line.innerHTML = 'Drag and drop files here (multiple ok) or <label class="choose-files" for="mediaUploadInput">choose files</label>.';
  } else {
    line.textContent = `${n} file${n === 1 ? '' : 's'} ready — drag more or choose files, then Upload.`;
  }
}

function clearMediaPendingUpload() {
  MEDIA_PENDING_UPLOAD_FILES = null;
  const inp = $('mediaUploadInput');
  if (inp) inp.value = '';
  updateMediaUploadHint();
}

function openMediaMessageModal(title, body, isError) {
  $('mediaMessageTitle').textContent = title || 'Notice';
  const b = $('mediaMessageBody');
  b.textContent = body || '';
  b.classList.toggle('error', !!isError);
  $('mediaMessageModal').classList.add('open');
}

function closeMediaMessageModal() {
  $('mediaMessageModal').classList.remove('open');
}

function openMediaRenameModal(fromPath) {
  MEDIA_RENAME_FROM = fromPath;
  $('mediaRenameFromDisplay').textContent = fromPath;
  $('mediaRenameError').textContent = '';
  const base = fromPath.includes('/') ? fromPath.slice(fromPath.lastIndexOf('/') + 1) : fromPath;
  const inp = $('mediaRenameInput');
  inp.value = base;
  $('mediaRenameModal').classList.add('open');
  setTimeout(() => { inp.focus(); inp.select(); }, 0);
}

function closeMediaRenameModal() {
  $('mediaRenameModal').classList.remove('open');
  MEDIA_RENAME_FROM = '';
}

async function submitMediaRename() {
  const name = MEDIA_MODAL_NAME;
  const fromPath = MEDIA_RENAME_FROM;
  const to = ($('mediaRenameInput').value || '').trim();
  $('mediaRenameError').textContent = '';
  if (!name || !fromPath) return;
  if (!to) {
    $('mediaRenameError').textContent = 'Enter a new name or path.';
    return;
  }
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/media/rename`, {
      method: 'POST',
      headers: headers(),
      body: JSON.stringify({ from: fromPath, to }),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('mediaRenameError').textContent = data.error || ('HTTP ' + res.status);
      return;
    }
    closeMediaRenameModal();
    refreshMediaModal();
  } catch (e) {
    $('mediaRenameError').textContent = e.message;
  }
}

function openMediaDeleteModal(relPath) {
  MEDIA_DELETE_PATH = relPath;
  $('mediaDeletePathDisplay').textContent = relPath;
  $('mediaDeleteError').textContent = '';
  $('mediaDeleteModal').classList.add('open');
}

function closeMediaDeleteModal() {
  $('mediaDeleteModal').classList.remove('open');
  MEDIA_DELETE_PATH = '';
}

async function submitMediaDelete() {
  const name = MEDIA_MODAL_NAME;
  const relPath = MEDIA_DELETE_PATH;
  $('mediaDeleteError').textContent = '';
  if (!name || !relPath) return;
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/media/file?name=` + encodeURIComponent(relPath), {
      method: 'DELETE',
      headers: headers(),
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      $('mediaDeleteError').textContent = data.error || ('HTTP ' + res.status);
      return;
    }
    closeMediaDeleteModal();
    refreshMediaModal();
  } catch (e) {
    $('mediaDeleteError').textContent = e.message;
  }
}

(function initMediaUploadDropzone() {
  const zone = $('mediaDropZone');
  const inp = $('mediaUploadInput');
  if (!zone || !inp) return;
  zone.addEventListener('dragenter', ev => {
    ev.preventDefault();
    ev.stopPropagation();
    zone.classList.add('media-drop-active');
  });
  zone.addEventListener('dragleave', ev => {
    ev.preventDefault();
    ev.stopPropagation();
    if (ev.relatedTarget != null && zone.contains(/** @type {Node} */ (ev.relatedTarget))) return;
    zone.classList.remove('media-drop-active');
  });
  zone.addEventListener('dragover', ev => {
    ev.preventDefault();
    ev.stopPropagation();
    ev.dataTransfer.dropEffect = 'copy';
  });
  zone.addEventListener('drop', ev => {
    ev.preventDefault();
    ev.stopPropagation();
    zone.classList.remove('media-drop-active');
    const dt = ev.dataTransfer;
    if (!dt || !dt.files || dt.files.length === 0) return;
    const incoming = Array.from(dt.files);
    const cur = MEDIA_PENDING_UPLOAD_FILES ? MEDIA_PENDING_UPLOAD_FILES.slice() : (inp.files && inp.files.length ? Array.from(inp.files) : []);
    const next = cur.concat(incoming);
    MEDIA_PENDING_UPLOAD_FILES = next;
    inp.value = '';
    updateMediaUploadHint();
  });
  inp.addEventListener('change', () => {
    MEDIA_PENDING_UPLOAD_FILES = null;
    updateMediaUploadHint();
  });
})();

(function initMediaRenameEnter() {
  const inp = $('mediaRenameInput');
  if (!inp) return;
  inp.addEventListener('keydown', ev => {
    if (ev.key === 'Enter') {
      ev.preventDefault();
      submitMediaRename();
    }
  });
})();

function scheduleMediaSearch() {
  clearTimeout(mediaSearchDebounce);
  mediaSearchDebounce = setTimeout(() => refreshMediaModal(), 280);
}

function setMediaSort(key) {
  if (MEDIA_SORT_KEY === key) {
    MEDIA_SORT_DIR = MEDIA_SORT_DIR === 'asc' ? 'desc' : 'asc';
  } else {
    MEDIA_SORT_KEY = key;
    MEDIA_SORT_DIR = (key === 'size') ? 'desc' : 'asc';
  }
  refreshMediaModal();
}

function updateMediaSortHeaders() {
  document.querySelectorAll('[data-media-sort]').forEach(el => {
    const key = el.getAttribute('data-media-sort') || '';
    const active = key === MEDIA_SORT_KEY;
    const arrow = active ? (MEDIA_SORT_DIR === 'asc' ? ' ▲' : ' ▼') : '';
    const label = key ? (key.charAt(0).toUpperCase() + key.slice(1)) : '';
    el.classList.toggle('active', active);
    el.textContent = label + arrow;
  });
}

function openPalaceMediaModal(palaceName) {
  MEDIA_MODAL_NAME = palaceName;
  $('mediaModalTitle').textContent = 'Media — ' + palaceName;
  $('mediaModalPath').textContent = '';
  $('mediaRefsNote').style.display = 'none';
  $('mediaRefsNote').textContent = '';
  $('mediaSearch').value = '';
  clearMediaPendingUpload();
  $('mediaTableBody').innerHTML = '<tr><td colspan="7" class="empty">Loading…</td></tr>';
  $('mediaModal').classList.add('open');
  refreshMediaModal();
}

function closeMediaModal() {
  closeMediaPreviewModal();
  $('mediaModal').classList.remove('open');
  MEDIA_MODAL_NAME = '';
}

function closeMediaPreviewModal() {
  const body = $('mediaPreviewBody');
  if (body) {
    body.querySelectorAll('video, audio').forEach(el => {
      try { el.pause(); } catch (_) {}
    });
    body.innerHTML = '';
  }
  if (MEDIA_PREVIEW_OBJECT_URL) {
    URL.revokeObjectURL(MEDIA_PREVIEW_OBJECT_URL);
    MEDIA_PREVIEW_OBJECT_URL = '';
  }
  const wrap = $('mediaPreviewModal');
  if (wrap) wrap.classList.remove('open');
}

function mediaPreviewKind(relPath, mime) {
  const ext = (relPath.includes('.') ? relPath.substring(relPath.lastIndexOf('.') + 1) : '').toLowerCase();
  const m = (mime || '').split(';')[0].trim().toLowerCase();
  if (m.startsWith('image/')) return 'image';
  if (m.startsWith('video/')) return 'video';
  if (m.startsWith('audio/')) return 'audio';
  if (m === 'application/pdf') return 'pdf';
  if (m.startsWith('text/')) return 'text';
  if (MEDIA_IMAGE_EXTENSIONS.includes(ext)) return 'image';
  if (MEDIA_VIDEO_EXTENSIONS.includes(ext)) return 'video';
  if (MEDIA_AUDIO_EXTENSIONS.includes(ext)) return 'audio';
  if (ext === 'pdf') return 'pdf';
  if (MEDIA_TEXT_EXTENSIONS.includes(ext)) return 'text';
  return '';
}

function mediaFileExtension(path) {
  const base = path.includes('/') ? path.slice(path.lastIndexOf('/') + 1) : path;
  const idx = base.lastIndexOf('.');
  if (idx <= 0 || idx >= base.length - 1) return '';
  return base.slice(idx + 1).toLowerCase();
}

function mediaExtensionGroup(ext) {
  if (!ext) return 'Other';
  if (MEDIA_IMAGE_EXTENSIONS.includes(ext)) return 'Images';
  if (MEDIA_VIDEO_EXTENSIONS.includes(ext)) return 'Video';
  if (MEDIA_AUDIO_EXTENSIONS.includes(ext)) return 'Audio';
  if (ext === 'pdf') return 'Documents';
  if (MEDIA_TEXT_EXTENSIONS.includes(ext)) return 'Text / Data';
  return 'Other';
}

function mediaThumbCell(name, isDir) {
  if (isDir) return '<span class="media-preview-thumb-empty">dir</span>';
  if (mediaPreviewKind(name, '') !== 'image') return '<span class="media-preview-thumb-empty">-</span>';
  const nm = JSON.stringify(name);
  const palace = encodeURIComponent(MEDIA_MODAL_NAME);
  const rel = encodeURIComponent(name);
  const src = `/api/palaces/${palace}/media/download?name=${rel}`;
  return `<button type="button" title="Preview image" onclick='openMediaPreview(${nm})' style="padding:0;border:none;background:transparent;box-shadow:none;">
    <img class="media-preview-thumb" src="${src}" alt="" loading="lazy" decoding="async" />
  </button>`;
}

function rebuildMediaTypeFilter(files) {
  const sel = $('mediaTypeFilter');
  if (!sel) return;
  const previous = sel.value || '';
  const extMap = new Map();
  for (const f of (files || [])) {
    if (f && !f.is_dir) {
      const ext = mediaFileExtension(f.name || '');
      if (!ext) continue;
      extMap.set(ext, (extMap.get(ext) || 0) + 1);
    }
  }
  const groupOrder = ['Images', 'Video', 'Audio', 'Documents', 'Text / Data', 'Other'];
  const grouped = {};
  for (const [ext, count] of extMap.entries()) {
    const g = mediaExtensionGroup(ext);
    if (!grouped[g]) grouped[g] = [];
    grouped[g].push({ ext, count });
  }
  let html = '<option value="">All file extensions</option>';
  for (const group of groupOrder) {
    const items = grouped[group];
    if (!items || items.length === 0) continue;
    items.sort((a, b) => a.ext.localeCompare(b.ext));
    html += `<optgroup label="${esc(group)}">`;
    for (const item of items) {
      html += `<option value="${esc(item.ext)}">.${esc(item.ext)} (${item.count})</option>`;
    }
    html += '</optgroup>';
  }
  sel.innerHTML = html;
  if (previous && extMap.has(previous)) {
    sel.value = previous;
  } else {
    sel.value = '';
  }
}

function sortMediaFiles(files) {
  const dirMul = MEDIA_SORT_DIR === 'asc' ? 1 : -1;
  return (files || []).slice().sort((a, b) => {
    const aIsDir = !!a.is_dir;
    const bIsDir = !!b.is_dir;
    if (aIsDir !== bIsDir) return aIsDir ? -1 : 1;
    let av = '';
    let bv = '';
    if (MEDIA_SORT_KEY === 'size') {
      av = Number(a.size || 0);
      bv = Number(b.size || 0);
      if (av !== bv) return (av < bv ? -1 : 1) * dirMul;
    } else if (MEDIA_SORT_KEY === 'type') {
      av = String(a.file_type || '').toLowerCase();
      bv = String(b.file_type || '').toLowerCase();
      if (av !== bv) return (av < bv ? -1 : 1) * dirMul;
    } else if (MEDIA_SORT_KEY === 'room') {
      av = String(a.used_in_room || '').toLowerCase();
      bv = String(b.used_in_room || '').toLowerCase();
      if (av !== bv) return (av < bv ? -1 : 1) * dirMul;
    } else if (MEDIA_SORT_KEY === 'door') {
      av = String(a.used_in_door || '').toLowerCase();
      bv = String(b.used_in_door || '').toLowerCase();
      if (av !== bv) return (av < bv ? -1 : 1) * dirMul;
    } else {
      av = String(a.name || '').toLowerCase();
      bv = String(b.name || '').toLowerCase();
      if (av !== bv) return (av < bv ? -1 : 1) * dirMul;
    }
    const an = String(a.name || '').toLowerCase();
    const bn = String(b.name || '').toLowerCase();
    if (an === bn) return 0;
    return (an < bn ? -1 : 1) * dirMul;
  });
}

async function openMediaPreview(relPath) {
  const palaceName = MEDIA_MODAL_NAME;
  if (!palaceName) return;
  closeMediaPreviewModal();
  $('mediaPreviewTitle').textContent = relPath;
  const bodyEl = $('mediaPreviewBody');
  bodyEl.innerHTML = '<p class="empty">Loading…</p>';
  $('mediaPreviewModal').classList.add('open');

  try {
    const url = `/api/palaces/${encodeURIComponent(palaceName)}/media/download?name=` + encodeURIComponent(relPath);
    const res = await fetch(url, { headers: headers() });
    const mimeHdr = (res.headers.get('Content-Type') || '').split(';')[0].trim().toLowerCase();
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      bodyEl.innerHTML = `<p class="empty" style="color:var(--red);">${esc(data.error || ('HTTP ' + res.status))}</p>`;
      return;
    }
    const cl = res.headers.get('Content-Length');
    if (cl && !Number.isNaN(parseInt(cl, 10)) && parseInt(cl, 10) > MEDIA_PREVIEW_MAX_BYTES) {
      bodyEl.innerHTML = `<p class="empty">This file is ${formatBytes(parseInt(cl, 10))}. Preview is limited to ${formatBytes(MEDIA_PREVIEW_MAX_BYTES)} — use Download.</p>`;
      return;
    }
    const blob = await res.blob();
    if (blob.size > MEDIA_PREVIEW_MAX_BYTES) {
      bodyEl.innerHTML = `<p class="empty">This file is ${formatBytes(blob.size)}. Preview is limited to ${formatBytes(MEDIA_PREVIEW_MAX_BYTES)} — use Download.</p>`;
      return;
    }
    const mime = (blob.type && blob.type !== 'application/octet-stream') ? blob.type.toLowerCase() : mimeHdr;
    const kind = mediaPreviewKind(relPath, mime);
    MEDIA_PREVIEW_OBJECT_URL = URL.createObjectURL(blob);

    if (kind === 'image') {
      bodyEl.innerHTML = `<img alt="" src="${MEDIA_PREVIEW_OBJECT_URL}" style="max-width:100%;max-height:min(72vh,680px);object-fit:contain;border-radius:8px;" />`;
      return;
    }
    if (kind === 'video') {
      bodyEl.innerHTML = `<video class="media-preview-frame" src="${MEDIA_PREVIEW_OBJECT_URL}" controls playsinline></video>`;
      return;
    }
    if (kind === 'audio') {
      bodyEl.innerHTML = `<audio src="${MEDIA_PREVIEW_OBJECT_URL}" controls style="width:100%;max-width:520px;"></audio>`;
      return;
    }
    if (kind === 'pdf') {
      bodyEl.innerHTML = `<iframe class="media-preview-frame" title="Preview" src="${MEDIA_PREVIEW_OBJECT_URL}" style="min-height:min(72vh,680px);"></iframe>`;
      return;
    }
    if (kind === 'text') {
      const text = await blob.text();
      bodyEl.innerHTML = `<pre class="media-preview-text">${esc(text)}</pre>`;
      URL.revokeObjectURL(MEDIA_PREVIEW_OBJECT_URL);
      MEDIA_PREVIEW_OBJECT_URL = '';
      return;
    }
    bodyEl.innerHTML = `<p class="empty">No preview for this type. Use <strong>Download</strong> to open it locally.</p>`;
    URL.revokeObjectURL(MEDIA_PREVIEW_OBJECT_URL);
    MEDIA_PREVIEW_OBJECT_URL = '';
  } catch (e) {
    bodyEl.innerHTML = `<p class="empty" style="color:var(--red);">${esc(e.message)}</p>`;
  }
}

async function refreshMediaModal() {
  const name = MEDIA_MODAL_NAME;
  const tbody = $('mediaTableBody');
  if (!name) return;
  updateMediaSortHeaders();
  const q = $('mediaSearch').value.trim();
  const extFilter = ($('mediaTypeFilter')?.value || '').trim().toLowerCase();
  try {
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/media/files?q=` + encodeURIComponent(q), { headers: headers() });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      tbody.innerHTML = `<tr><td colspan="7" class="empty">${esc(data.error || ('HTTP ' + res.status))}</td></tr>`;
      $('mediaModalPath').textContent = '';
      return;
    }
    $('mediaModalPath').textContent = data.media_dir ? ('Folder: ' + data.media_dir) : '';
    if (data.refs_note) {
      $('mediaRefsNote').textContent = data.refs_note;
      $('mediaRefsNote').style.display = '';
    }
    const files = data.files || [];
    rebuildMediaTypeFilter(files);
    const selectedExt = ($('mediaTypeFilter')?.value || '').trim().toLowerCase();
    const displayFiles = selectedExt
      ? files.filter(f => !f.is_dir && mediaFileExtension(f.name || '') === selectedExt)
      : files;
    const sortedFiles = sortMediaFiles(displayFiles);
    if (sortedFiles.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="empty">No files match.</td></tr>';
      return;
    }
    tbody.innerHTML = sortedFiles.map(f => {
      const nm = JSON.stringify(f.name);
      const sz = f.is_dir ? '—' : formatBytes(f.size);
      const actions = f.is_dir ? ''
        : `<button type="button" onclick='downloadMediaRel(${nm})'>Download</button> ` +
          `<button type="button" onclick='openMediaRenameModal(${nm})'>Rename</button> ` +
          `<button type="button" class="danger" onclick='openMediaDeleteModal(${nm})'>Delete</button>`;
      return `<tr>
        <td class="media-preview-thumb-cell">${mediaThumbCell(f.name, f.is_dir)}</td>
        <td style="max-width:280px;word-break:break-all;"><code>${esc(f.name)}</code></td>
        <td>${esc(f.file_type || '')}</td>
        <td>${esc(sz)}</td>
        <td style="font-size:11px;color:var(--muted);">${esc(f.used_in_room || '—')}</td>
        <td style="font-size:11px;color:var(--muted);">${esc(f.used_in_door || '—')}</td>
        <td><div class="actions">${actions}</div></td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = `<tr><td colspan="7" class="empty">${esc(e.message)}</td></tr>`;
  }
}

async function uploadMediaFiles() {
  const name = MEDIA_MODAL_NAME;
  const files = getMediaUploadFiles();
  if (!name) return;
  if (!files.length) {
    openMediaMessageModal('No files selected', 'Drag files into the upload area, or use choose files, then click Upload.', false);
    return;
  }
  const fd = new FormData();
  for (let i = 0; i < files.length; i++) fd.append('file', files[i]);
  try {
    const h = { ...headers() };
    delete h['Content-Type'];
    const res = await fetch(`/api/palaces/${encodeURIComponent(name)}/media/upload`, { method: 'POST', headers: h, body: fd });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      openMediaMessageModal('Upload failed', data.error || ('HTTP ' + res.status), true);
      return;
    }
    clearMediaPendingUpload();
    refreshMediaModal();
  } catch (e) {
    openMediaMessageModal('Upload failed', e.message, true);
  }
}

async function downloadMediaRel(relPath) {
  const name = MEDIA_MODAL_NAME;
  if (!name) return;
  try {
    const url = `/api/palaces/${encodeURIComponent(name)}/media/download?name=` + encodeURIComponent(relPath);
    const res = await fetch(url, { headers: headers() });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      openMediaMessageModal('Download failed', data.error || ('HTTP ' + res.status), true);
      return;
    }
    const blob = await res.blob();
    const href = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = href;
    a.download = relPath.split('/').pop() || 'file';
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(href);
  } catch (e) {
    openMediaMessageModal('Download failed', e.message, true);
  }
}
