// State
let providers = [];
let editingProvider = null;
let historyOffset = 0;
const PAGE_SIZE = 30;
let providerMeta = {}; // fetched from /api/provider-types

// Navigation
document.querySelectorAll('.nav-link').forEach(link => {
  link.addEventListener('click', e => {
    e.preventDefault();
    const page = link.dataset.page;
    document.querySelectorAll('.nav-link').forEach(l => l.classList.remove('active'));
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
    link.classList.add('active');
    document.getElementById('page-' + page).classList.add('active');
    if (page === 'history') loadHistory();
    if (page === 'config') loadProviders();
  });
});

// Toast
function toast(msg, type = 'success') {
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

// API helpers
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch('/api' + path, opts);
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
}

// Hotkey preset
function onHotkeyPresetChange() {
  const sel = document.getElementById('cfg-hotkey-preset');
  const custom = document.getElementById('cfg-push-to-talk');
  if (sel.value === 'custom') {
    custom.style.display = 'block';
    custom.focus();
  } else {
    custom.style.display = 'none';
    custom.value = sel.value;
  }
}

function setHotkeyValue(val) {
  const sel = document.getElementById('cfg-hotkey-preset');
  const custom = document.getElementById('cfg-push-to-talk');
  const opts = Array.from(sel.options).map(o => o.value);
  if (opts.includes(val)) {
    sel.value = val;
    custom.style.display = 'none';
  } else {
    sel.value = 'custom';
    custom.style.display = 'block';
    custom.value = val;
  }
}

function getHotkeyValue() {
  const sel = document.getElementById('cfg-hotkey-preset');
  if (sel.value === 'custom') {
    return document.getElementById('cfg-push-to-talk').value;
  }
  return sel.value;
}

// ---- Hotkey capture (press-the-combo) ----
// Browsers can capture most combos via keydown. Plain modifier-only combos
// (e.g. Ctrl+Alt) are unreliable in the browser (Alt steals focus, lone
// modifier keydown is inconsistent), so we accept modifier-only on a 600ms
// settle timer and fall back to a hint if nothing fires.
let hotkeyCapturing = false;
let hotkeySettleTimer = null;
let hotkeyTimeoutTimer = null;
let hotkeyMods = []; // ordered modifier display names currently held

function captureKeyCode(code) {
  // Map event.code → project key name (or null = modifier, or '' = unsupported text key)
  if (code.startsWith('Control')) return { mod: 'Ctrl' };
  if (code.startsWith('Alt')) return { mod: 'Alt' };
  if (code.startsWith('Shift')) return { mod: 'Shift' };
  if (code.startsWith('Meta')) return { mod: 'Super' };
  const m = code.match(/^F([1-9]|1[0-9]|2[0-4])$/);
  if (m) return { key: 'F' + m[1] };
  if (code === 'Enter') return { key: 'Enter' };
  if (code === 'Space') return { key: 'Space' };
  if (code === 'Tab') return { key: 'Tab' };
  if (code === 'Escape') return { key: 'Escape' };
  if (code.startsWith('Arrow')) return { key: { ArrowUp: 'Up', ArrowDown: 'Down', ArrowLeft: 'Left', ArrowRight: 'Right' }[code] };
  if (code === 'Home' || code === 'End' || code === 'PageUp' || code === 'PageDown' || code === 'Insert' || code === 'Delete')
    return { key: code };
  // Everything else (letters, digits, punctuation) is a "text key" → rejected.
  return { text: true };
}

function startHotkeyCapture() {
  const hint = document.getElementById('hotkey-capture-hint');
  const btn = document.getElementById('btn-hotkey-capture');
  if (hotkeyCapturing) return;
  hotkeyCapturing = true;
  btn.disabled = true;
  hotkeyMods = [];
  hint.style.display = 'block';
  hint.textContent = '请按下你想要的组合键…（Esc 取消）';
  document.addEventListener('keydown', onCaptureKey, true);
  document.addEventListener('keyup', onCaptureKeyUp, true);
  clearTimeout(hotkeyTimeoutTimer);
  hotkeyTimeoutTimer = setTimeout(endHotkeyCaptureTimeout, 8000);
}

function endHotkeyCapture() {
  hotkeyCapturing = false;
  const btn = document.getElementById('btn-hotkey-capture');
  const hint = document.getElementById('hotkey-capture-hint');
  if (btn) btn.disabled = false;
  document.removeEventListener('keydown', onCaptureKey, true);
  document.removeEventListener('keyup', onCaptureKeyUp, true);
  clearTimeout(hotkeySettleTimer);
  clearTimeout(hotkeyTimeoutTimer);
  return hint;
}

function onCaptureKey(e) {
  if (!hotkeyCapturing) return;
  e.preventDefault();
  e.stopPropagation();
  const hint = document.getElementById('hotkey-capture-hint');
  if (e.code === 'Escape') {
    endHotkeyCapture();
    hint.textContent = '已取消捕获';
    setTimeout(() => hint.style.display = 'none', 1000);
    return;
  }
  const mapped = captureKeyCode(e.code);
  if (mapped.mod) {
    if (!hotkeyMods.includes(mapped.mod)) hotkeyMods.push(mapped.mod);
    hint.textContent = '已按下：' + hotkeyMods.join(' + ') + '（继续按修饰键，或按一个功能键确定）';
    // Settle timer: if user holds only modifiers and stops, treat as modifier-only combo.
    clearTimeout(hotkeySettleTimer);
    hotkeySettleTimer = setTimeout(finishHotkeyCapture, 600, hotkeyMods.slice());
    return;
  }
  if (mapped.text) {
    endHotkeyCapture();
    hint.textContent = '✗ 不支持普通字符键，请用功能键（F1-F24）或修饰键组合';
    toast('不支持普通字符键（字母/数字/标点）');
    return;
  }
  // A real key (F-key, Enter, arrows, etc.) — finalize immediately.
  const parts = hotkeyMods.slice();
  parts.push(mapped.key);
  finishHotkeyCapture(parts);
}

function onCaptureKeyUp(e) {
  if (!hotkeyCapturing) return;
  const mapped = captureKeyCode(e.code);
  if (mapped.mod) {
    hotkeyMods = hotkeyMods.filter(m => m !== mapped.mod);
    // Restart the settle timer so release resets the wait window.
    clearTimeout(hotkeySettleTimer);
    if (hotkeyMods.length > 0) {
      hotkeySettleTimer = setTimeout(finishHotkeyCapture, 600, hotkeyMods.slice());
    }
  }
}

async function finishHotkeyCapture(parts) {
  const hint = endHotkeyCapture();
  if (!parts || parts.length === 0) {
    hint.textContent = '未捕获到有效组合';
    return;
  }
  const comboStr = parts.join('+');
  hint.textContent = '已捕获：' + parts.join(' + ') + '，正在校验…';
  try {
    const v = await api('GET', '/hotkey/validate?s=' + encodeURIComponent(comboStr));
    if (v.ok) {
      setHotkeyValue(v.combo);
      hint.textContent = '✓ 已设置：' + v.combo;
      toast('热键已设置为 ' + v.combo + '，记得点「保存语音设置」生效');
    } else {
      hint.textContent = '✗ 无效：' + (v.reason || '未知原因');
      toast('热键无效：' + (v.reason || ''));
    }
  } catch (err) {
    hint.textContent = '✗ 校验失败：' + err.message;
  }
}

function endHotkeyCaptureTimeout() {
  if (!hotkeyCapturing) return;
  endHotkeyCapture();
  const hint = document.getElementById('hotkey-capture-hint');
  hint.textContent = '未捕获到按键。纯修饰键组合（如 Ctrl+Alt）在浏览器中可能无法捕获，请从下拉选择预设或手动输入。';
}


// ---- Audio input device picker ----
// Dropdown convention: "" = system default; "__custom__" = manual entry;
// any other value = a concrete device name returned by the backend.
const DEVICE_CUSTOM = '__custom__';

async function loadDevices() {
  const sel = document.getElementById('cfg-device');
  const custom = document.getElementById('cfg-device-custom');
  // Remember the previously selected value so a refresh doesn't reset it.
  const prevValue = sel.dataset.value || '';
  let devices = [];
  try {
    const resp = await api('GET', '/devices/audio');
    devices = resp.devices || [];
    if (resp.error) {
      toast('获取设备列表失败：' + resp.error);
    }
  } catch (e) {
    toast('获取设备列表失败：' + e.message);
  }
  sel.innerHTML = '';
  const addOpt = (value, label) => {
    const opt = document.createElement('option');
    opt.value = value;
    opt.textContent = label;
    sel.appendChild(opt);
  };
  addOpt('', '系统默认');
  for (const name of devices) {
    addOpt(name, name);
  }
  addOpt(DEVICE_CUSTOM, '手动输入...');
  setDeviceValue(prevValue);
}

function onDevicePresetChange() {
  const sel = document.getElementById('cfg-device');
  const custom = document.getElementById('cfg-device-custom');
  if (sel.value === DEVICE_CUSTOM) {
    custom.style.display = 'block';
  } else {
    custom.style.display = 'none';
  }
}

function setDeviceValue(value) {
  const sel = document.getElementById('cfg-device');
  const custom = document.getElementById('cfg-device-custom');
  const known = Array.from(sel.options).some(o => o.value === value);
  if (known) {
    sel.value = value;
    custom.style.display = 'none';
  } else {
    sel.value = DEVICE_CUSTOM;
    custom.style.display = 'block';
    custom.value = value || '';
  }
  sel.dataset.value = value || '';
}

function getDeviceValue() {
  const sel = document.getElementById('cfg-device');
  if (sel.value === DEVICE_CUSTOM) {
    return document.getElementById('cfg-device-custom').value;
  }
  return sel.value;
}

// Load config
async function loadConfig() {
  const cfg = await api('GET', '/config');
  setHotkeyValue(cfg.voice.push_to_talk || 'F9');
  document.getElementById('cfg-mode').value = cfg.voice.mode || 'toggle';
  document.getElementById('cfg-auto-submit').value = cfg.voice.auto_submit ? 'true' : 'false';
  document.getElementById('cfg-stop-delay').value = cfg.voice.stop_delay_ms || 0;
  document.getElementById('cfg-hotwords').value = (cfg.voice.hotwords || []).join(', ');
  await loadDevices();
  setDeviceValue(cfg.voice.device || '');
}

// Save voice config
async function saveVoiceConfig() {
  // Load full config first, then only update the fields we manage
  const cfg = await api('GET', '/config');
  cfg.voice.push_to_talk = getHotkeyValue();
  cfg.voice.mode = document.getElementById('cfg-mode').value;
  cfg.voice.device = getDeviceValue();
  cfg.voice.auto_submit = document.getElementById('cfg-auto-submit').value === 'true';
  cfg.voice.stop_delay_ms = parseInt(document.getElementById('cfg-stop-delay').value) || 0;
  cfg.voice.hotwords = document.getElementById('cfg-hotwords').value.split(/[,，]/).map(s => s.trim()).filter(Boolean);
  await api('PUT', '/config/voice', cfg.voice);
  toast('语音设置已保存');
}

// Load providers
async function loadProviders() {
  providers = await api('GET', '/providers');
  renderProviders();
}

function renderProviders() {
  const container = document.getElementById('provider-list');
  if (providers.length === 0) {
    container.innerHTML = '<div class="empty-state">暂无服务商，点击下方按钮添加。</div>';
    return;
  }
  container.innerHTML = providers.map((p, i) => `
    <div class="provider-item ${p.default ? 'default' : ''}">
      <div class="provider-info">
        <span class="provider-name">${p.name}</span>
        <span class="provider-type">${(providerMeta[p.type] || {}).display_name || p.type}</span>
        ${p.default ? '<span class="default-badge">默认</span>' : ''}
      </div>
      <div class="provider-actions">
        ${p.default ? '' : `<button class="btn btn-secondary btn-small" onclick="setDefault('${p.name}')"> 设为默认</button>`}
        <button class="btn btn-secondary btn-small" onclick="editProvider(${i})">编辑</button>
        <button class="btn btn-danger btn-small" onclick="deleteProvider('${p.name}')">删除</button>
      </div>
    </div>
  `).join('');
}

// Add provider
function showAddProvider() {
  document.getElementById('add-provider-card').style.display = 'block';
  updateAddFields();
}

function hideAddProvider() {
  document.getElementById('add-provider-card').style.display = 'none';
}

function updateAddFields() {
  const type = document.getElementById('add-type').value;
  const meta = providerMeta[type];
  const fields = meta ? meta.fields : [];
  document.getElementById('add-fields').innerHTML = buildFieldsHTML(fields, 'add');
}

async function addProvider() {
  const type = document.getElementById('add-type').value;
  const name = type + '-' + Date.now().toString(36);
  const p = { name, type };
  const meta = providerMeta[type];
  const fields = meta ? meta.fields : [];
  for (const f of fields) {
    p[f.key] = document.getElementById('add-' + f.key)?.value || '';
  }
  try {
    await api('POST', '/providers', p);
    toast('已添加服务商: ' + name);
    hideAddProvider();
    await loadProviders();
  } catch (e) { toast(e.message, 'error'); }
}

// Edit provider
function editProvider(idx) {
  editingProvider = providers[idx];
  document.getElementById('edit-provider-name').textContent = editingProvider.name;
  const meta = providerMeta[editingProvider.type];
  const fields = meta ? meta.fields : [];
  document.getElementById('edit-fields').innerHTML = buildFieldsHTML(fields, 'edit');
  for (const f of fields) {
    const el = document.getElementById('edit-' + f.key);
    if (el) el.value = editingProvider[f.key] || '';
  }
  document.getElementById('edit-provider-modal').style.display = 'flex';
}

function hideEditProvider() {
  document.getElementById('edit-provider-modal').style.display = 'none';
  editingProvider = null;
}

async function saveProvider() {
  if (!editingProvider) return;
  const meta = providerMeta[editingProvider.type];
  const fields = meta ? meta.fields : [];
  const p = { ...editingProvider };
  for (const f of fields) {
    p[f.key] = document.getElementById('edit-' + f.key)?.value || '';
  }
  try {
    await api('PUT', '/providers/' + editingProvider.name, p);
    toast('已保存');
    hideEditProvider();
    await loadProviders();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteProvider(name) {
  if (!confirm('确定删除服务商 ' + name + '？')) return;
  try {
    await api('DELETE', '/providers/' + name);
    toast('已删除');
    await loadProviders();
  } catch (e) { toast(e.message, 'error'); }
}

async function setDefault(name) {
  try {
    await api('PUT', '/providers-default/' + name);
    toast('已设为默认: ' + name);
    await loadProviders();
  } catch (e) { toast(e.message, 'error'); }
}

// Field builder — works with metadata FieldDef format
function buildFieldsHTML(fields, prefix) {
  return '<div class="form-grid">' + fields.map(f => {
    const id = prefix + '-' + f.key;
    let input;
    if (f.type === 'select') {
      const labels = f.labels || f.options || [];
      input = `<select id="${id}">${(f.options || []).map((o, i) =>
        `<option value="${o}">${labels[i] || o}</option>`
      ).join('')}</select>`;
    } else {
      const isSecret = f.type === 'secret' || f.secret ||
        f.key.includes('secret') || f.key.includes('key') || f.key.includes('access');
      if (isSecret) {
        input = `<div class="input-wrap">`
          + `<input type="password" id="${id}" placeholder="${f.help || f.default || ''}">`
          + `<button type="button" class="input-eye" title="显示/隐藏" onclick="toggleSecret('${id}', this)">&#128065;</button>`
          + `</div>`;
      } else {
        input = `<input type="text" id="${id}" placeholder="${f.help || f.default || ''}">`;
      }
    }
    return `<label>${f.label}</label>${input}`;
  }).join('') + '</div>';
}

// Toggle secret field visibility
function toggleSecret(id, btn) {
  const el = document.getElementById(id);
  if (!el) return;
  const show = el.type === 'password';
  el.type = show ? 'text' : 'password';
  btn.style.color = show ? 'var(--accent)' : '';
}

// History
async function loadHistory() {
  const data = await api('GET', '/history?offset=' + historyOffset + '&limit=' + PAGE_SIZE);
  document.getElementById('h-sessions').textContent = data.sessions;
  document.getElementById('h-chars').textContent = data.chars;
  const el = document.getElementById('h-duration');
  el.textContent = formatDuration(data.duration, el._fmt);
  if (!el._bound) {
    el._fmt = 's';
    el._sec = data.duration;
    el._bound = true;
    el.style.cursor = 'pointer';
    el.addEventListener('click', function() {
      this._fmt = this._fmt === 's' ? 'hms' : 's';
      this.textContent = formatDuration(this._sec, this._fmt);
      document.getElementById('h-duration-label').textContent = this._fmt === 's' ? '总时长 (s)' : '总时长 (H/M/S)';
    });
  } else {
    el._sec = data.duration;
  }

  const tbody = document.getElementById('history-body');
  if (!data.items || data.items.length === 0) {
    tbody.innerHTML = '<tr><td colspan="6"><div class="empty-state">暂无记录</div></td></tr>';
  } else {
    tbody.innerHTML = data.items.map(item => `
      <tr>
        <td><input type="checkbox" class="history-checkbox" value="${item.id}"></td>
        <td>${formatTime(item.created_at)}</td>
        <td>${(providerMeta[item.provider] || {}).display_name || item.provider || '-'}</td>
        <td class="text-cell">${escapeHtml(item.text)}</td>
        <td>${item.duration_sec.toFixed(1)}s</td>
        <td>${item.chars}</td>
      </tr>
    `).join('');
  }

  document.getElementById('select-all-head').checked = false;
  syncSelectAll(false);
  const totalPages = Math.ceil(data.total / PAGE_SIZE);
  const currentPage = Math.floor(historyOffset / PAGE_SIZE) + 1;
  document.getElementById('page-info').textContent = currentPage + ' / ' + totalPages;
  document.getElementById('prev-page').disabled = historyOffset <= 0;
  document.getElementById('next-page').disabled = historyOffset + PAGE_SIZE >= data.total;
}

function formatDuration(sec, fmt) {
  sec = Math.round(sec) || 0;
  if (fmt === 'hms') {
    const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
    let t = '';
    if (h > 0) t += h + 'H';
    if (m > 0 || h > 0) t += m + 'M';
    t += s + 'S';
    return t;
  }
  return String(sec);
}

function toggleSelectAll() {
  const header = document.getElementById('select-all-head');
  header.checked = !header.checked;
  document.querySelectorAll('.history-checkbox').forEach(cb => cb.checked = header.checked);
}

function syncSelectAll(checked) {
  document.querySelectorAll('.history-checkbox').forEach(cb => cb.checked = checked);
}

function getSelectedIDs() {
  return Array.from(document.querySelectorAll('.history-checkbox:checked')).map(cb => parseInt(cb.value));
}

async function deleteSelected() {
  const ids = getSelectedIDs();
  if (ids.length === 0) { toast('请先选择记录', 'error'); return; }
  if (!confirm(`确定删除 ${ids.length} 条记录？`)) return;
  try {
    const res = await api('DELETE', '/history', { ids });
    toast(`已删除 ${res.deleted} 条`);
    loadHistory();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteByDate() {
  const from = document.getElementById('date-from-display').dataset.date;
  const to = document.getElementById('date-to-display').dataset.date;
  if (!from || !to) { toast('请选择开始和结束日期', 'error'); return; }
  if (!confirm(`确定删除 ${from} 至 ${to} 的所有记录？`)) return;
  try {
    const res = await api('DELETE', '/history', { from, to });
    toast(`已删除 ${res.deleted} 条`);
    document.getElementById('date-from-display').textContent = '选择开始日期 ▼';
    document.getElementById('date-to-display').textContent = '选择结束日期 ▼';
    document.getElementById('date-from-display').dataset.date = '';
    document.getElementById('date-to-display').dataset.date = '';
    dpDateStats = null; // refresh date stats
    loadHistory();
  } catch (e) { toast(e.message, 'error'); }
}

function exportHistory(format) {
  window.open('/api/history/export?format=' + format, '_blank');
}

function prevPage() {
  historyOffset = Math.max(0, historyOffset - PAGE_SIZE);
  loadHistory();
}

function nextPage() {
  historyOffset += PAGE_SIZE;
  loadHistory();
}

function formatTime(t) {
  const d = new Date(t);
  return d.toLocaleDateString('zh-CN') + ' ' + d.toLocaleTimeString('zh-CN');
}

function escapeHtml(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// Status polling
async function pollStatus() {
  try {
    const data = await api('GET', '/status');
    const el = document.getElementById('recording-status');
    el.textContent = { idle: '待机', connecting: '连接中', recording: '录音中', stopping: '停止中', error: '错误' }[data.state] || data.state;
    el.className = data.state;
    document.getElementById('status-detail').textContent = data.detail || '';
  } catch (e) {}
}

// Init
async function loadProviderMeta() {
  try {
    providerMeta = await api('GET', '/provider-types');
    // Populate the add-type select
    const sel = document.getElementById('add-type');
    sel.innerHTML = '';
    for (const [type, meta] of Object.entries(providerMeta).sort((a, b) => a[0].localeCompare(b[0]))) {
      const opt = document.createElement('option');
      opt.value = type;
      opt.textContent = meta.display_name || type;
      sel.appendChild(opt);
    }
  } catch (e) {
    console.error('Failed to load provider metadata:', e);
  }
}

loadProviderMeta().then(() => {
  loadConfig();
  loadProviders();
});
initDatePicker();
setInterval(pollStatus, 2000);
pollStatus();
