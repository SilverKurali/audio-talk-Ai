// State
let providers = [];
let editingProvider = null;
let historyOffset = 0;
const PAGE_SIZE = 30;

// Provider type definitions
const PROVIDER_FIELDS = {
  'doubao': [
    { key: 'app_key', label: 'App Key', placeholder: '火山 App ID' },
    { key: 'access_key', label: 'Access Key', placeholder: '火山 Access Token' },
  ],
  'openai-realtime': [
    { key: 'api_key', label: 'API Key', placeholder: 'OpenAI API Key' },
    { key: 'model', label: 'Model', placeholder: 'gpt-4o-mini-transcribe' },
  ],
  'openai-whisper': [
    { key: 'api_key', label: 'API Key', placeholder: 'OpenAI API Key' },
    { key: 'model', label: 'Model', placeholder: 'whisper-1' },
    { key: 'base_url', label: 'Endpoint', placeholder: '留空用默认' },
  ],
  'xiaomi-mimo-asr': [
    { key: 'api_key', label: 'API Key', placeholder: 'MiMo API Key' },
    { key: 'model', label: 'Model', placeholder: 'mimo-v2.5-asr' },
  ],
  'xiaomi-mimo-asr-TokenPlan': [
    { key: 'api_key', label: 'API Key', placeholder: 'MiMo API Key' },
    { key: 'model', label: 'Model', placeholder: 'mimo-v2.5-asr' },
  ],
  'xfyun-spark': [
    { key: 'app_id', label: 'App ID', placeholder: '讯飞 App ID' },
    { key: 'api_key', label: 'API Key', placeholder: '讯飞 API Key' },
    { key: 'api_secret', label: 'API Secret', placeholder: '讯飞 API Secret' },
    { key: 'dwa', label: '动态修正', type: 'select', options: ['', 'wpgs'], labels: ['关闭', 'wpgs 开启'] },
  ],
};

const PROVIDER_NAMES = {
  'doubao': '豆包',
  'openai-realtime': 'OpenAI Realtime',
  'openai-whisper': 'OpenAI Whisper',
  'xiaomi-mimo-asr': '小米 MiMo',
  'xiaomi-mimo-asr-TokenPlan': '小米 MiMo Token Plan',
  'xfyun-spark': '讯飞星火',
};

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

// Load config
async function loadConfig() {
  const cfg = await api('GET', '/config');
  setHotkeyValue(cfg.voice.push_to_talk || 'F9');
  document.getElementById('cfg-mode').value = cfg.voice.mode || 'toggle';
  document.getElementById('cfg-auto-submit').value = cfg.voice.auto_submit ? 'true' : 'false';
  document.getElementById('cfg-stop-delay').value = cfg.voice.stop_delay_ms || 0;
  document.getElementById('cfg-hotwords').value = (cfg.voice.hotwords || []).join(', ');
}

// Save voice config
async function saveVoiceConfig() {
  // Load full config first, then only update the fields we manage
  const cfg = await api('GET', '/config');
  cfg.voice.push_to_talk = getHotkeyValue();
  cfg.voice.mode = document.getElementById('cfg-mode').value;
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
        <span class="provider-type">${PROVIDER_NAMES[p.type] || p.type}</span>
        ${p.default ? '<span class="default-badge">默认</span>' : ''}
      </div>
      <div class="provider-actions">
        ${p.default ? '' : `<button class="btn btn-secondary btn-small" onclick="setDefault('${p.name}')">设为默认</button>`}
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
  const fields = PROVIDER_FIELDS[type] || [];
  document.getElementById('add-fields').innerHTML = buildFieldsHTML(fields, 'add');
}

async function addProvider() {
  const type = document.getElementById('add-type').value;
  const name = type + '-' + Date.now().toString(36);
  const p = { name, type };
  const fields = PROVIDER_FIELDS[type] || [];
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
  const fields = PROVIDER_FIELDS[editingProvider.type] || [];
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
  const fields = PROVIDER_FIELDS[editingProvider.type] || [];
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

// Field builder
function buildFieldsHTML(fields, prefix) {
  return '<div class="form-grid">' + fields.map(f => {
    const id = prefix + '-' + f.key;
    let input;
    if (f.type === 'select') {
      input = `<select id="${id}">${f.options.map((o, i) =>
        `<option value="${o}">${f.labels[i]}</option>`
      ).join('')}</select>`;
    } else {
      const isSecret = f.key.includes('secret') || f.key.includes('key') || f.key.includes('access');
      if (isSecret) {
        input = `<div class="input-wrap">`
          + `<input type="password" id="${id}" placeholder="${f.placeholder || ''}">`
          + `<button type="button" class="input-eye" title="显示/隐藏" onclick="toggleSecret('${id}', this)">&#128065;</button>`
          + `</div>`;
      } else {
        input = `<input type="text" id="${id}" placeholder="${f.placeholder || ''}">`;
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
        <td>${PROVIDER_NAMES[item.provider] || item.provider || '-'}</td>
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
loadConfig();
loadProviders();
initDatePicker();
setInterval(pollStatus, 2000);
pollStatus();
