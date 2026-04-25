/* PAT / Iptscrae syntax highlighting for the server file editor */
(function () {
  'use strict';

  // ─── Iptscrae language constants ───

  var EVENT_NAMES = new Set([
    'ALARM','COLORCHANGE','ENTER','FACECHANGE','FRAMECHANGE',
    'HTTPERROR','HTTPRECEIVED','HTTPRECEIVEPROGRESS','HTTPSENDPROGRESS',
    'IDLE','INCHAT','KEYDOWN','KEYUP',
    'LEAVE','LOOSEPROPADDED','LOOSEPROPDELETED','LOOSEPROPMOVED',
    'MOUSEDOWN','MOUSEDRAG','MOUSEMOVE','MOUSEUP',
    'NAMECHANGE','OUTCHAT',
    'ROOMLOAD','ROOMREADY','ROLLOUT','ROLLOVER','SELECT',
    'SERVERMSG','SIGNON','SIGNOFF','STATECHANGE',
    'USERENTER','USERLEAVE','USERMOVE',
    'WEBDOCBEGIN','WEBDOCDONE','WEBSTATUS','WEBTITLE'
  ]);

  var CONTROL_FLOW = new Set([
    'IF','IFELSE','WHILE','FOREACH','EXEC','BREAK','RETURN','EXIT',
    'ON','AND','OR','NOT','GLOBAL','DEF'
  ]);

  var EXTERNAL_VARS = new Set([
    'CHATSTR','WHOCHANGE','LASTNAME','WHOMOVE','WHOENTER','WHOLEAVE',
    'WHATPROP','WHATINDEX','LASTSTATE','CONTENTS','HEADERS','TYPE',
    'FILENAME','ERRORMSG','DOCURL','NEWSTATUS','NEWTITLE'
  ]);

  // Token types
  var TT = {
    WS: 0, COMMENT: 1, STRING: 2, NUMBER: 3, COMMAND: 4,
    CONTROL: 5, EVENT: 6, VARIABLE: 7, EXTVAR: 8, OPERATOR: 9,
    BRACKET: 10, UNKNOWN: 11,
    ROOM_NAME: 12, SECTION_NAME: 13
  };

  var TOKEN_CLASS = {};
  TOKEN_CLASS[TT.WS]           = '';
  TOKEN_CLASS[TT.COMMENT]      = 'ipt-comment';
  TOKEN_CLASS[TT.STRING]       = 'ipt-string';
  TOKEN_CLASS[TT.NUMBER]       = 'ipt-number';
  TOKEN_CLASS[TT.COMMAND]      = 'ipt-command';
  TOKEN_CLASS[TT.CONTROL]      = 'ipt-control';
  TOKEN_CLASS[TT.EVENT]        = 'ipt-event';
  TOKEN_CLASS[TT.VARIABLE]     = 'ipt-variable';
  TOKEN_CLASS[TT.EXTVAR]       = 'ipt-extvar';
  TOKEN_CLASS[TT.OPERATOR]     = 'ipt-operator';
  TOKEN_CLASS[TT.BRACKET]      = 'ipt-bracket';
  TOKEN_CLASS[TT.UNKNOWN]      = '';
  TOKEN_CLASS[TT.ROOM_NAME]    = 'ipt-room';
  TOKEN_CLASS[TT.SECTION_NAME] = 'ipt-section';

  // ─── Tokenizer ───

  function escHtml(t) {
    if (t.indexOf('&') === -1 && t.indexOf('<') === -1 && t.indexOf('>') === -1) return t;
    return t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function tokenize(script) {
    var tokens = [];
    var i = 0;
    var len = script.length;
    while (i < len) {
      var cc = script.charCodeAt(i);
      // Whitespace
      if (cc === 32 || cc === 9 || cc === 13 || cc === 10) {
        var start = i;
        while (i < len) { var c = script.charCodeAt(i); if (c !== 32 && c !== 9 && c !== 13 && c !== 10) break; i++; }
        tokens.push({ type: TT.WS, text: script.substring(start, i) });
      // Comment: # or ;
      } else if (cc === 35 || cc === 59) {
        var start = i; i++;
        while (i < len) { var c = script.charCodeAt(i); if (c === 13 || c === 10) break; i++; }
        tokens.push({ type: TT.COMMENT, text: script.substring(start, i) });
      // String: "..."
      } else if (cc === 34) {
        var start = i; i++;
        while (i < len) { var c = script.charCodeAt(i); if (c === 92) { i += 2; continue; } if (c === 34) { i++; break; } i++; }
        tokens.push({ type: TT.STRING, text: script.substring(start, i) });
      // Brackets: { } [ ]
      } else if (cc === 123 || cc === 125 || cc === 91 || cc === 93) {
        tokens.push({ type: TT.BRACKET, text: script.charAt(i) }); i++;
      // Number (including negative)
      } else if ((cc >= 48 && cc <= 57) || (cc === 45 && i + 1 < len && script.charCodeAt(i + 1) >= 48 && script.charCodeAt(i + 1) <= 57)) {
        var start = i; if (cc === 45) i++;
        while (i < len && script.charCodeAt(i) >= 48 && script.charCodeAt(i) <= 57) i++;
        tokens.push({ type: TT.NUMBER, text: script.substring(start, i) });
      // Identifier: letters, digits, underscore
      } else if ((cc >= 65 && cc <= 90) || (cc >= 97 && cc <= 122) || cc === 95) {
        var start = i;
        while (i < len) { var c = script.charCodeAt(i); if (!((c >= 65 && c <= 90) || (c >= 97 && c <= 122) || (c >= 48 && c <= 57) || c === 95)) break; i++; }
        var word = script.substring(start, i);
        var upper = word.toUpperCase();
        var type;
        if (CONTROL_FLOW.has(upper)) {
          type = (upper === 'ON' && word !== 'ON') ? TT.VARIABLE : TT.CONTROL;
        } else if (EVENT_NAMES.has(upper)) {
          type = (word === upper) ? TT.EVENT : TT.VARIABLE;
        } else if (EXTERNAL_VARS.has(upper)) {
          type = TT.EXTVAR;
        } else {
          type = TT.VARIABLE;
        }
        tokens.push({ type: type, text: word });
      // Operator / punctuation
      } else {
        var start = i; i++;
        while (i < len) {
          var c = script.charCodeAt(i);
          if (c === 32 || c === 9 || c === 13 || c === 10 || c === 34 ||
              c === 123 || c === 125 || c === 91 || c === 93 || c === 35 || c === 59 ||
              (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122) || c === 95) break;
          i++;
        }
        tokens.push({ type: TT.OPERATOR, text: script.substring(start, i) });
      }
    }
    return tokens;
  }

  // ─── PAT-level annotation ───
  // At depth 0: bare identifiers/strings before { are room names
  // At depth 1: bare identifiers/strings before { are hotspot/section names
  // Inside blocks: normal iptscrae (ON EVENT coloring, extvar scoping)

  function annotate(tokens) {
    // Pass 1: color ON keywords that precede an event name
    for (var t = 0; t < tokens.length; t++) {
      if (tokens[t].type === TT.CONTROL && tokens[t].text.toUpperCase() === 'ON') {
        var next = t + 1;
        while (next < tokens.length && tokens[next].type === TT.WS) next++;
        if (next < tokens.length && tokens[next].type === TT.EVENT) tokens[t].type = TT.EVENT;
      }
    }

    // Pass 2: depth tracking, room/section naming, extvar scoping
    var depth = 0;
    var eventStack = []; // { event: string, depth: number }
    var pendingNamed = []; // non-ws/comment tokens seen at current depth before next {

    for (var t = 0; t < tokens.length; t++) {
      var tok = tokens[t];
      if (tok.type === TT.WS || tok.type === TT.COMMENT) continue;

      if (tok.type === TT.BRACKET) {
        if (tok.text === '{') {
          // Name the preceding identifiers/strings as room or section
          if (pendingNamed.length > 0) {
            var isIptHandler = pendingNamed.some(function(w) {
              return w.type === TT.EVENT || w.type === TT.CONTROL;
            });
            if (!isIptHandler) {
              var nameType = depth === 0 ? TT.ROOM_NAME : (depth === 1 ? TT.SECTION_NAME : null);
              if (nameType !== null) {
                for (var p = 0; p < pendingNamed.length; p++) {
                  var pw = pendingNamed[p];
                  if (pw.type === TT.VARIABLE || pw.type === TT.STRING || pw.type === TT.NUMBER) {
                    pw.type = nameType;
                  }
                }
              }
            }
            // Check if this is an ON EVENT { block
            var isOnEvent = false;
            var foundOn = false;
            for (var p = 0; p < pendingNamed.length; p++) {
              var upper = pendingNamed[p].text ? pendingNamed[p].text.toUpperCase() : '';
              if (!foundOn && upper === 'ON') { foundOn = true; continue; }
              if (foundOn && EVENT_NAMES.has(upper)) { isOnEvent = true; break; }
            }
            if (isOnEvent) {
              var evName = '';
              var seenOn = false;
              for (var p = 0; p < pendingNamed.length; p++) {
                var upper = pendingNamed[p].text ? pendingNamed[p].text.toUpperCase() : '';
                if (!seenOn && upper === 'ON') { seenOn = true; continue; }
                if (seenOn && EVENT_NAMES.has(upper)) { evName = upper; break; }
              }
              if (evName) eventStack.push({ event: evName, depth: depth });
            }
          }
          depth++;
          pendingNamed = [];
        } else if (tok.text === '}') {
          depth = Math.max(0, depth - 1);
          pendingNamed = [];
          if (eventStack.length > 0 && eventStack[eventStack.length - 1].depth === depth) {
            eventStack.pop();
          }
        }
        continue;
      }

      // Scope external vars: only valid in their associated events
      if (tok.type === TT.EXTVAR) {
        // leave as-is for simplicity — glow color indicates "special"
      }

      // Accumulate tokens before the next {
      if (depth === 0 || depth === 1) {
        pendingNamed.push(tok);
      }
    }
  }

  function renderHtml(tokens) {
    var html = '';
    for (var i = 0; i < tokens.length; i++) {
      var tok = tokens[i];
      var cls = TOKEN_CLASS[tok.type] || '';
      var esc = escHtml(tok.text);
      html += cls ? '<span class="' + cls + '">' + esc + '</span>' : esc;
    }
    return html;
  }

  function highlightPat(text) {
    var tokens = tokenize(text);
    annotate(tokens);
    return renderHtml(tokens);
  }

  // ─── Editor overlay wiring ───

  var _highlightActive = false;
  var _rafId = 0;

  function scheduleUpdate() {
    if (_rafId) return;
    _rafId = requestAnimationFrame(function () {
      _rafId = 0;
      doUpdate();
    });
  }

  function doUpdate() {
    if (!_highlightActive) return;
    var ta = document.getElementById('sfContent');
    var hl = document.getElementById('sfHighlight');
    if (!ta || !hl) return;
    hl.innerHTML = highlightPat(ta.value) + '\n';
    syncScroll();
  }

  function syncScroll() {
    var ta = document.getElementById('sfContent');
    var hl = document.getElementById('sfHighlight');
    if (!ta || !hl) return;
    hl.scrollTop = ta.scrollTop;
    hl.scrollLeft = ta.scrollLeft;
  }

  var _listenersAttached = false;

  function initSfHighlighter() {
    if (_listenersAttached) return;
    _listenersAttached = true;
    var ta = document.getElementById('sfContent');
    if (!ta) return;
    ta.addEventListener('input', scheduleUpdate);
    ta.addEventListener('scroll', syncScroll);
  }

  function setSfHighlightMode(enabled) {
    _highlightActive = !!enabled;
    var ta = document.getElementById('sfContent');
    var hl = document.getElementById('sfHighlight');
    if (!ta || !hl) return;
    if (enabled) {
      ta.classList.add('sf-highlight-active');
      hl.style.display = 'block';
      doUpdate();
    } else {
      ta.classList.remove('sf-highlight-active');
      hl.style.display = 'none';
      hl.innerHTML = '';
    }
  }

  // ─── Expand / collapse ───

  var _expanded = false;

  function toggleSfExpand() {
    _expanded = !_expanded;
    var wrap = document.getElementById('sfEditorWrap');
    var btn = document.getElementById('sfExpandBtn');
    var fileListWrap = document.getElementById('sfFileListWrap');
    var topControls = document.getElementById('sfTopControls');
    if (!wrap) return;
    if (_expanded) {
      wrap.classList.add('sf-expanded');
      if (fileListWrap) fileListWrap.style.display = 'none';
      if (topControls) topControls.style.display = 'none';
      if (btn) btn.textContent = 'Collapse \u2191';
    } else {
      wrap.classList.remove('sf-expanded');
      if (fileListWrap) fileListWrap.style.display = '';
      if (topControls) topControls.style.display = '';
      if (btn) btn.textContent = 'Expand \u2193';
    }
    syncScroll();
  }

  function resetSfExpand() {
    if (!_expanded) return;
    _expanded = false;
    var wrap = document.getElementById('sfEditorWrap');
    var btn = document.getElementById('sfExpandBtn');
    var fileListWrap = document.getElementById('sfFileListWrap');
    var topControls = document.getElementById('sfTopControls');
    if (wrap) wrap.classList.remove('sf-expanded');
    if (btn) btn.textContent = 'Expand \u2193';
    if (fileListWrap) fileListWrap.style.display = '';
    if (topControls) topControls.style.display = '';
  }

  // ─── Public API ───

  window.sfPatHighlight = {
    init: initSfHighlighter,
    setMode: setSfHighlightMode,
    update: doUpdate,
    toggleExpand: toggleSfExpand,
    resetExpand: resetSfExpand
  };
})();
