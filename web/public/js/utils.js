const $ = id => document.getElementById(id);

/** pserver.log and rotated variants (including *.gz). */
function isPalaceServerLogFamily(fileName) {
  const low = String(fileName || '').toLowerCase();
  return low === 'pserver.log' || low.startsWith('pserver.log.');
}

function formatBytes(n) {
  if (n < 1024) return n + ' B';
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
  return (n / (1024 * 1024)).toFixed(1) + ' MiB';
}

function esc(s) {
  return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function attrEsc(s) {
  return String(s ?? '').replace(/&/g,'&amp;').replace(/"/g,'&quot;');
}

function appendStreamLine(el, line) {
  const span = document.createElement('span');
  const lo = line.toLowerCase();
  if (/^error[: ]|^\s*fatal[: ]/.test(lo)) {
    span.style.color = 'var(--red)';
    span.style.fontWeight = '600';
  } else if (/^warn(ing)?[: ]/.test(lo)) {
    span.style.color = 'var(--yellow)';
  }
  span.textContent = line;
  el.appendChild(span);
  el.appendChild(document.createTextNode('\n'));
}

// onDone(okObj) — okObj is the first parsed object with ok:true, or null if none was seen.
async function streamSSE(res, el, onDone) {
  if (!res.ok) {
    el.textContent = `HTTP ${res.status}: ${await res.text()}`;
    if (onDone) onDone(null);
    return;
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  let okObj = null;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const parts = buf.split('\n\n');
    buf = parts.pop();
    for (const part of parts) {
      const line = part.replace(/^data:\s?/, '');
      if (!line) continue;
      try {
        const obj = JSON.parse(line);
        if (obj.ok && !okObj) okObj = obj;
        if (obj.log) {
          appendStreamLine(el, obj.log);
        } else if (obj.id && obj.state) {
          appendStreamLine(el, `[${obj.id}] ${obj.state}${obj.message ? ': ' + obj.message : ''}`);
        } else {
          appendStreamLine(el, JSON.stringify(obj));
        }
      } catch {
        appendStreamLine(el, line);
      }
      el.scrollTop = el.scrollHeight;
    }
  }
  if (onDone) onDone(okObj);
}
