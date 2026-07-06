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
  setHotkeyValue(cfg.Voice.push_to_talk || 'F9');
  document.getElementById('cfg-mode').value = cfg.Voice.mode || 'toggle';
  document.getElementById('cfg-auto-submit').value = cfg.Voice.auto_submit ? 'true' : 'false';
  document.getElementById('cfg-stop-delay').value = cfg.Voice.stop_delay_ms || 0;
  document.getElementById('cfg-hotwords').value = (cfg.Voice.hotwords || []).join(', ');
}

// Save voice config
async function saveVoiceConfig() {
  const vc = {
    push_to_talk: getHotkeyValue(),
    mode: document.getElementById('cfg-mode').value,
    auto_submit: document.getElementById('cfg-auto-submit').value === 'true',
    stop_delay_ms: parseInt(document.getElementById('cfg-stop-delay').value) || 0,
    hotwords: document.getElementById('cfg-hotwords').value.split(/[,，]/).map(s => s.trim()).filter(Boolean),
  };
  await api('PUT', '/config/voice', vc);
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
    container.innerHTML = '<p style="color:#90a4ae;font-size:13px">暂无服务商，请添加。</p>';
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
      input = `<input type="${isSecret ? 'password' : 'text'}" id="${id}" placeholder="${f.placeholder || ''}">`;
    }
    return `<label>${f.label}</label>${input}`;
  }).join('') + '</div>';
}

// History
async function loadHistory() {
  const data = await api('GET', '/history?offset=' + historyOffset + '&limit=' + PAGE_SIZE);
  document.getElementById('h-sessions').textContent = data.sessions;
  document.getElementById('h-chars').textContent = data.chars;
  document.getElementById('h-duration').textContent = Math.round(data.duration);

  const tbody = document.getElementById('history-body');
  if (!data.items || data.items.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" style="color:#90a4ae;text-align:center">暂无记录</td></tr>';
  } else {
    tbody.innerHTML = data.items.map(item => `
      <tr>
        <td>${formatTime(item.created_at)}</td>
        <td>${PROVIDER_NAMES[item.provider] || item.provider || '-'}</td>
        <td class="text-cell">${escapeHtml(item.text)}</td>
        <td>${item.duration_sec.toFixed(1)}s</td>
        <td>${item.chars}</td>
      </tr>
    `).join('');
  }

  const totalPages = Math.ceil(data.total / PAGE_SIZE);
  const currentPage = Math.floor(historyOffset / PAGE_SIZE) + 1;
  document.getElementById('page-info').textContent = currentPage + ' / ' + totalPages;
  document.getElementById('prev-page').disabled = historyOffset <= 0;
  document.getElementById('next-page').disabled = historyOffset + PAGE_SIZE >= data.total;
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
setInterval(pollStatus, 2000);
pollStatus();
