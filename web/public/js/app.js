// Tab switching
function showTab(name, btn) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.querySelectorAll('nav button').forEach(b => b.classList.remove('active'));
  $('tab-' + name).classList.add('active');
  btn.classList.add('active');
  if (name === 'palaces') loadPalaces();
  if (name === 'server-files') loadServerFilesPalaces().then(() => loadServerFileList());
  if (name === 'users') loadUsers();
  if (name === 'update') { loadRolloutPanel(); loadManagerVersionInfo(); }
  if (name === 'nginx') { loadNginxSettingsForm(); loadNginxStatus(); loadBootstrapStatus(); }
}

document.addEventListener('keydown', (ev) => {
  if (ev.key !== 'Escape') return;
  if ($('mediaPreviewModal').classList.contains('open')) {
    closeMediaPreviewModal();
    ev.preventDefault();
  } else if ($('mediaMessageModal').classList.contains('open')) {
    closeMediaMessageModal();
    ev.preventDefault();
  } else if ($('mediaRenameModal').classList.contains('open')) {
    closeMediaRenameModal();
    ev.preventDefault();
  } else if ($('mediaDeleteModal').classList.contains('open')) {
    closeMediaDeleteModal();
    ev.preventDefault();
  } else if ($('mediaModal').classList.contains('open')) {
    closeMediaModal();
    ev.preventDefault();
  } else if ($('patUploadModal').classList.contains('open')) {
    closePatUploadModal();
    ev.preventDefault();
  } else if ($('deleteUserModal').classList.contains('open')) {
    closeDeleteUserModal();
    ev.preventDefault();
  } else if ($('palaceSettingsModal').classList.contains('open')) {
    closePalaceSettingsModal();
    ev.preventDefault();
  } else if ($('registerPalaceModal').classList.contains('open')) {
    closeRegisterPalaceModal();
    ev.preventDefault();
  } else if ($('removePalaceModal').classList.contains('open')) {
    closeRemovePalaceModal();
    ev.preventDefault();
  }
});

// Initial load — check auth before showing the UI.
(async () => {
  if (!AUTH_HEADER) { showLogin(); return; }
  const res = await fetch('/api/session', { headers: headers() }).catch(() => null);
  if (!res || res.status === 401) { showLogin(); return; }
  const data = await res.json();
  hideLogin();
  await afterSessionData(data);
})();
setInterval(async () => {
  if ($('loginScreen').classList.contains('visible')) return;
  if ($('passwordGate').classList.contains('visible')) return;
  loadPalaces();
}, 15000);
