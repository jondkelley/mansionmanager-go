// Basic Auth credentials kept in memory for the session.
// sessionStorage survives page refreshes but not tab close.
let AUTH_HEADER = sessionStorage.getItem('pm_auth') || '';
let SESSION = null;
let EDIT_USER = null;
let REMOVE_PALACE_NAME = null;
let DELETE_USER_NAME = null;
let REGISTER_PALACE_NAME = null;
let SETTINGS_PALACE = null;
let SETTINGS_RAW_SNAPSHOT = '';
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
