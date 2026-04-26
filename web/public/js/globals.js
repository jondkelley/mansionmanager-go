// Basic Auth credentials kept in memory for the session.
// sessionStorage survives page refreshes but not tab close.
let AUTH_HEADER = sessionStorage.getItem('pm_auth') || '';
let SESSION = null;

/** Subaccounts: delegated permission on a palace. Admin and tenant have full access. */
function palaceRBAC(palaceName, perm) {
  if (!SESSION) return false;
  const r = SESSION.role;
  if (r === 'admin' || r === 'tenant') return true;
  if (r !== 'subaccount') return false;
  const m = SESSION.palacePerms || {};
  const arr = m[palaceName];
  return Array.isArray(arr) && arr.indexOf(perm) >= 0;
}
let EDIT_USER = null;
let REMOVE_PALACE_NAME = null;
/** @type {string|null} Palace registry name when opening the admin Edit modal */
let EDIT_PALACE_ORIG = null;
let DELETE_USER_NAME = null;
let REGISTER_PALACE_NAME = null;
let SETTINGS_PALACE = null;
let SETTINGS_RAW_SNAPSHOT = '';
let SETTINGS_PREFS_TAB = 'pserver';
let SETTINGS_RATBOT_ROWS = [];
let SETTINGS_RATBOT_CURRENT_FILE = '';
/** After unregister-only removal, scroll the unregistered panel into view once. */
let SCROLL_UNREGISTER_PANEL = false;
/** @type {string} */
let SF_PALACE = '';
/** @type {string} */
let SF_FILE = '';
/** @type {'utf8'|'base64'} */
let SF_FILE_ENCODING = 'utf8';
/** Logs / logrotate files are view-only (no Save / backup modal). */
let SF_ALLOW_SAVE = false;
let PAT_UPLOAD_NAME = '';
let PROPS_PALACE = '';
