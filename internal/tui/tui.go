package tui

import (
	"fmt"
	"strings"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/config"
	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
	"gitee.com/AY77-OP/audio-talk-ai/plugins/voice"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	lStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	vStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	aStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	wStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	eStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	dStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	hStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1)
)

type backendMsg hotkey.ProviderInfo
type devMsg struct {
	Devices []string
	Error   error
}
type fieldType int

const (
	fString fieldType = iota
	fToggle
	fSelect
)

type field struct {
	label   string
	key     string
	help    string
	fType   fieldType
	input   textinput.Model
	boolVal bool
	opts    []string
	optIdx  int
}

// BackgroundMsg is sent when the user wants to switch to background mode.
type BackgroundMsg struct{}

type Model struct {
	w, h          int
	ready         bool
	info          hotkey.ProviderInfo
	logs          []string
	devices       []string
	cfg           *config.Config
	debug         bool
	showLogs      bool
	OnSave        func(*config.Config) error
	fields        []field
	cursor        int
	editing       bool
	helpVisible   bool
	logExpanded   bool
	providerNames   []string
	providerIdx     int
	providerField   int    // index of the asr_provider field in fields, -1 if absent
	background      bool   // user requested background mode
	lastAddedType   string // last added provider type, used as default in add_provider dropdown
	previewType     string // provider type being previewed in "添加服务商" (empty = no preview)
}

func New(cfg *config.Config) *Model {
	providers := cfg.ResolveASRProviders()
	names := make([]string, len(providers))
	pIdx := 0
	for i, p := range providers {
		names[i] = p.Name
		if p.Default {
			pIdx = i
		}
	}
	m := &Model{
		cfg:           cfg,
		logs:          make([]string, 0, 100),
		cursor:        -1,
		showLogs:      true,
		providerNames: names,
		providerIdx:   pIdx,
		providerField: -1,
	}
	m.rebuildFields()
	return m
}

func (m *Model) rebuildFields() {
	vc := m.cfg.Voice
	ti := func(v string) textinput.Model { t := textinput.New(); t.SetValue(v); t.Cursor.Blink = false; return t }

	fs := []field{
		{label: "语音输入", key: "enabled", help: "关闭后不注册热键", fType: fToggle, boolVal: vc.Enabled},
		{label: "热键", key: "push_to_talk", help: "例: Alt+Super / F9 / Ctrl+Alt+Tab；不支持字母、数字、标点、空格等普通字符键", fType: fString, input: ti(vc.PushToTalk)},
		{label: "模式", key: "mode", help: "toggle 切换 / hold 按住", fType: fSelect, opts: []string{"toggle", "hold"}, optIdx: idxOf([]string{"toggle", "hold"}, vc.Mode)},
	}

	// Provider selector (when providers exist in cfg.ASRs)
	m.providerField = -1
	if len(m.cfg.ASRs) > 0 {
		// Sync providerNames from cfg.ASRs
		m.providerNames = make([]string, len(m.cfg.ASRs))
		for i, p := range m.cfg.ASRs {
			m.providerNames[i] = p.Name
		}
		if m.providerIdx >= len(m.providerNames) {
			m.providerIdx = 0
		}
		m.providerField = len(fs)
		fs = append(fs, field{label: "ASR 提供商", key: "asr_provider", help: "选择语音识别服务", fType: fSelect, opts: m.providerNames, optIdx: m.providerIdx})
	}

	// Provider-specific credential fields
	fs = append(fs, m.credentialFields()...)

	// "选择服务商" selector
	providerTypes := []string{"doubao", "openai-realtime", "openai-whisper", "mimo-asr", "xfyun-spark"}
	addIdx := idxOf(providerTypes, m.lastAddedType)
	if m.previewType != "" {
		addIdx = idxOf(providerTypes, m.previewType)
	}
	fs = append(fs, field{label: "选择服务商", key: "add_provider", help: "选择类型后配置凭据", fType: fSelect, opts: providerTypes, optIdx: addIdx})

	// Preview credential fields for the type being added
	if m.previewType != "" {
		fs = append(fs, m.previewCredentialFields(m.previewType)...)
		// Confirm button
		fs = append(fs, field{label: "添加当前配置的服务商", key: "confirm_add", help: "按 Enter 确认添加", fType: fSelect, opts: []string{"否", "是"}, optIdx: 0})
	}

	// "删除服务商" action (only when there are providers)
	if len(m.cfg.ASRs) > 0 {
		fs = append(fs, field{label: "删除当前服务商", key: "del_provider", help: "按 Enter 删除当前选中的服务商", fType: fSelect, opts: []string{"否", "是"}, optIdx: 0})
	}

	fs = append(fs,
		field{label: "自动上屏", key: "auto_submit", help: "识别后自动粘贴", fType: fToggle, boolVal: vc.AutoSubmit},
		field{label: "停止延迟(ms)", key: "stop_delay_ms", help: "松手后补录毫秒", fType: fString, input: ti(fmt.Sprintf("%d", vc.StopDelayMs))},
		field{label: "热词", key: "hotwords", help: "逗号分隔术语", fType: fString, input: ti(strings.Join(vc.Hotwords, ", "))},
	)
	m.fields = fs
}

// credentialFields returns provider-specific input fields based on the selected provider type.
func (m *Model) credentialFields() []field {
	ti := func(v string) textinput.Model { t := textinput.New(); t.SetValue(v); t.Cursor.Blink = false; return t }

	// No providers configured: no credential fields, just the add dropdown
	if len(m.cfg.ASRs) == 0 {
		return nil
	}

	// Multi-provider mode: show fields for the selected provider
	var p config.ASRProviderConfig
	if m.providerIdx < len(m.cfg.ASRs) {
		p = m.cfg.ASRs[m.providerIdx]
	}

	switch p.Type {
	case "doubao":
		return []field{
			{label: "App Key", key: "p_app_key", help: "火山 App ID", fType: fString, input: ti(p.AppKey)},
			{label: "Access Key", key: "p_access_key", help: "火山 Access Token", fType: fString, input: ti(p.AccessKey)},
		}
	case "openai-realtime", "openai-whisper":
		fields := []field{
			{label: "API Key", key: "p_api_key", help: "OpenAI API Key", fType: fString, input: ti(p.ApiKey)},
			{label: "Model", key: "p_model", help: "模型名", fType: fString, input: ti(p.Model)},
		}
		if p.Type == "openai-whisper" {
			fields = append(fields, field{label: "Endpoint", key: "p_base_url", help: "API 端点（留空用默认）", fType: fString, input: ti(p.BaseURL)})
		}
		return fields
	case "mimo-asr":
		return []field{
			{label: "API Key", key: "p_api_key", help: "MiMo API Key", fType: fString, input: ti(p.ApiKey)},
			{label: "Model", key: "p_model", help: "模型名（默认 mimo-v2.5-asr）", fType: fString, input: ti(p.Model)},
			{label: "Endpoint", key: "p_base_url", help: "API 端点（留空用默认）", fType: fString, input: ti(p.BaseURL)},
		}
	case "xfyun-spark":
		return []field{
			{label: "App ID", key: "p_app_id", help: "讯飞 App ID", fType: fString, input: ti(p.AppID)},
			{label: "API Key", key: "p_api_key", help: "讯飞 API Key", fType: fString, input: ti(p.ApiKey)},
			{label: "API Secret", key: "p_api_secret", help: "讯飞 API Secret", fType: fString, input: ti(p.ApiSecret)},
			{label: "动态修正", key: "p_dwa", help: "留空关闭，wpgs 开启", fType: fString, input: ti(p.DWA)},
		}
	default:
		return nil
	}
}

// previewCredentialFields returns empty credential fields for a provider type being previewed.
func (m *Model) previewCredentialFields(providerType string) []field {
	ti := func(v string) textinput.Model { t := textinput.New(); t.SetValue(v); t.Cursor.Blink = false; return t }

	switch providerType {
	case "doubao":
		return []field{
			{label: "App Key", key: "new_app_key", help: "火山 App ID", fType: fString, input: ti("")},
			{label: "Access Key", key: "new_access_key", help: "火山 Access Token", fType: fString, input: ti("")},
		}
	case "openai-realtime", "openai-whisper":
		fields := []field{
			{label: "API Key", key: "new_api_key", help: "OpenAI API Key", fType: fString, input: ti("")},
			{label: "Model", key: "new_model", help: "模型名", fType: fString, input: ti("")},
		}
		if providerType == "openai-whisper" {
			fields = append(fields, field{label: "Endpoint", key: "new_base_url", help: "API 端点（留空用默认）", fType: fString, input: ti("")})
		}
		return fields
	case "mimo-asr":
		return []field{
			{label: "API Key", key: "new_api_key", help: "MiMo API Key", fType: fString, input: ti("")},
			{label: "Model", key: "new_model", help: "模型名（默认 mimo-v2.5-asr）", fType: fString, input: ti("")},
			{label: "Endpoint", key: "new_base_url", help: "API 端点（留空用默认）", fType: fString, input: ti("")},
		}
	case "xfyun-spark":
		return []field{
			{label: "App ID", key: "new_app_id", help: "讯飞 App ID", fType: fString, input: ti("")},
			{label: "API Key", key: "new_api_key", help: "讯飞 API Key", fType: fString, input: ti("")},
			{label: "API Secret", key: "new_api_secret", help: "讯飞 API Secret", fType: fString, input: ti("")},
			{label: "动态修正", key: "new_dwa", help: "留空关闭，wpgs 开启", fType: fString, input: ti("")},
		}
	default:
		return nil
	}
}

// switchProvider changes the selected provider and rebuilds credential fields.
func (m *Model) switchProvider(newIdx int) {
	if newIdx == m.providerIdx || newIdx < 0 || newIdx >= len(m.providerNames) {
		return
	}
	// Save current credential field values to the old provider
	m.saveProviderFields(m.providerIdx)
	m.providerIdx = newIdx
	// Rebuild fields, preserving cursor position where possible
	oldCursor := m.cursor
	m.rebuildFields()
	// Adjust cursor if it was in the credential area
	if oldCursor >= len(m.fields) {
		m.cursor = len(m.fields) - 1
	}
}

// addProvider creates a new provider entry from preview fields and switches to it.
func (m *Model) addProvider(providerType string) {
	// Save current provider fields first
	m.saveProviderFields(m.providerIdx)

	name := providerType
	seen := make(map[string]bool)
	for _, n := range m.providerNames {
		seen[n] = true
	}
	if seen[name] {
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s-%d", providerType, i)
			if !seen[candidate] {
				name = candidate
				break
			}
		}
	}

	// Build new provider with values from preview fields
	newProvider := config.ASRProviderConfig{
		Name:      name,
		Type:      providerType,
		Default:   len(m.cfg.ASRs) == 0,
		AppKey:    m.fieldValue("new_app_key"),
		AccessKey: m.fieldValue("new_access_key"),
		ApiKey:    m.fieldValue("new_api_key"),
		Model:     m.fieldValue("new_model"),
		BaseURL:   m.fieldValue("new_base_url"),
		AppID:     m.fieldValue("new_app_id"),
		ApiSecret: m.fieldValue("new_api_secret"),
		DWA:       m.fieldValue("new_dwa"),
	}
	m.cfg.ASRs = append(m.cfg.ASRs, newProvider)

	m.providerNames = append(m.providerNames, name)
	m.providerIdx = len(m.cfg.ASRs) - 1
	m.lastAddedType = providerType
	m.previewType = "" // clear preview

	m.rebuildFields()
	m.logf("✅ 已添加服务商: %s (%s)", name, providerType)
}

// deleteProvider removes a provider by index and rebuilds fields.
func (m *Model) deleteProvider(idx int) {
	if idx < 0 || idx >= len(m.cfg.ASRs) {
		return
	}
	name := m.cfg.ASRs[idx].Name
	m.cfg.ASRs = append(m.cfg.ASRs[:idx], m.cfg.ASRs[idx+1:]...)

	// If we deleted the default, make the first one default
	if len(m.cfg.ASRs) > 0 {
		hasDefault := false
		for _, p := range m.cfg.ASRs {
			if p.Default {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			m.cfg.ASRs[0].Default = true
		}
	}

	// Adjust provider index
	if m.providerIdx >= len(m.cfg.ASRs) {
		m.providerIdx = max(0, len(m.cfg.ASRs)-1)
	}

	m.rebuildFields()
	m.logf("✅ 已删除服务商: %s", name)
}

// fieldValue returns the current value of a field by key.
func (m *Model) fieldValue(key string) string {
	for _, f := range m.fields {
		if f.key == key {
			return f.input.Value()
		}
	}
	return ""
}

func (m *Model) SetDebug(debug bool) {
	m.debug = debug
}

// WantsBackground returns true if the user requested to switch to background mode.
func (m *Model) WantsBackground() bool {
	return m.background
}

func idxOf(opts []string, v string) int {
	for i, o := range opts {
		if o == v {
			return i
		}
	}
	return 0
}

func (m *Model) Init() tea.Cmd { return tea.Batch(fetchDevices(), tea.EnterAltScreen, tickRefresh()) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h, m.ready = msg.Width, msg.Height, true
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	case BackgroundMsg:
		m.background = true
		return m, tea.Quit
	case backendMsg:
		m.info = hotkey.ProviderInfo(msg)
	case refreshMsg:
		return m, tickRefresh()
	case devMsg:
		if msg.Error != nil {
			m.logf("设备: %s", msg.Error)
		} else {
			m.devices = msg.Devices
		}
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()

	// Editing a field
	if m.editing && m.cursor >= 0 && m.cursor < len(m.fields) {
		f := &m.fields[m.cursor]
		switch k {
		case "esc":
			m.editing = false
			f.input.Blur()
			if f.key == "add_provider" {
				m.previewType = ""
				m.rebuildFields()
			}
			return nil
		case "enter":
			m.editing = false
			f.input.Blur()
			if f.key == "add_provider" {
				// Show preview fields for selected type
				m.previewType = f.opts[f.optIdx]
				m.rebuildFields()
				return nil
			}
			if f.key == "confirm_add" && f.optIdx == 1 {
				m.addProvider(m.previewType)
				return nil
			}
			if f.key == "del_provider" && f.optIdx == 1 {
				m.deleteProvider(m.providerIdx)
				return nil
			}
			m.save()
			return nil
		}
		switch f.fType {
		case fSelect:
			oldIdx := f.optIdx
			switch k {
			case "j", "down":
				f.optIdx++
				if f.optIdx >= len(f.opts) {
					f.optIdx = 0
				}
			case "k", "up":
				f.optIdx--
				if f.optIdx < 0 {
					f.optIdx = len(f.opts) - 1
				}
			}
			// If this is the provider selector and the index changed, rebuild fields
			if f.key == "asr_provider" && f.optIdx != oldIdx {
				m.switchProvider(f.optIdx)
			}
			// If this is the add_provider selector, show preview fields
			if f.key == "add_provider" && f.optIdx != oldIdx {
				m.previewType = f.opts[f.optIdx]
				m.rebuildFields()
				// Keep cursor on add_provider field
				for i, ff := range m.fields {
					if ff.key == "add_provider" {
						m.cursor = i
						m.editing = true
						break
					}
				}
			}
		case fToggle:
			switch k {
			case " ":
				f.boolVal = !f.boolVal
			case "j", "down", "k", "up":
				m.editing = false
				m.cursor++
				if m.cursor >= len(m.fields) {
					m.cursor = 0
				}
			}
		case fString:
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return cmd
		}
		return nil
	}

	// Navigation mode
	switch k {
	case "q", "ctrl+c":
		return tea.Quit
	case "b":
		m.save()
		return func() tea.Msg { return BackgroundMsg{} }
	case "s":
		m.save()
		return nil
	case "e", "i", "enter":
		m.editing = true
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.fields[m.cursor].fType == fString {
			m.fields[m.cursor].input.Focus()
		}
	case "j", "down":
		if m.cursor < 0 {
			m.cursor = 0
		} else {
			m.cursor++
			if m.cursor >= len(m.fields) {
				m.cursor = len(m.fields) - 1
			}
		}
	case "k", "up":
		if m.cursor < 0 {
			m.cursor = 0
		} else {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
	case "l":
		if m.showLogs {
			m.logExpanded = !m.logExpanded
		}
	case "h":
		m.helpVisible = !m.helpVisible
	}
	return nil
}

func (m *Model) save() {
	next := *m.cfg
	vc := &next.Voice
	for _, f := range m.fields {
		switch f.key {
		case "enabled":
			vc.Enabled = f.boolVal
		case "push_to_talk":
			vc.PushToTalk = f.input.Value()
		case "mode":
			vc.Mode = f.opts[f.optIdx]
		case "asr_provider":
			next.UpdateASRDefault(f.opts[f.optIdx])
		case "auto_submit":
			vc.AutoSubmit = f.boolVal
		case "stop_delay_ms":
			fmt.Sscanf(f.input.Value(), "%d", &vc.StopDelayMs)
		case "hotwords":
			vc.Hotwords = splitList(f.input.Value())
		}
	}
	// Save provider-specific credential fields
	m.saveProviderFieldsTo(&next, m.providerIdx)
	combo, err := config.ParseHotkey(vc.PushToTalk)
	if err != nil {
		m.logf("❌ 热键格式错误: %s", err)
		m.restorePushToTalkField()
		return
	}
	if combo.Key.IsTextKey() {
		m.logf("❌ 热键不支持普通字符键: %s", combo)
		m.logf("   请使用 Alt+Super、F9、Alt+F8、Ctrl+Alt+Tab 等全局快捷键")
		m.restorePushToTalkField()
		return
	}
	m.cfg = &next
	if err := config.Save(m.cfg); err != nil {
		m.logf("保存失败: %s", err)
	} else {
		m.logf("✅ 配置已保存到 %s", config.FindConfig())
	}
	m.logf("  push_to_talk=%s", vc.PushToTalk)
	if m.OnSave != nil {
		if err := m.OnSave(m.cfg); err != nil {
			m.logf("❌ 热键注册失败: %s", err)
		}
	}
}

// saveProviderFields writes current credential field values back to cfg.ASRs[pIdx].
func (m *Model) saveProviderFields(pIdx int) {
	m.saveProviderFieldsTo(m.cfg, pIdx)
}

func (m *Model) saveProviderFieldsTo(cfg *config.Config, pIdx int) {
	if pIdx < 0 || pIdx >= len(cfg.ASRs) {
		return
	}
	p := &cfg.ASRs[pIdx]
	for _, f := range m.fields {
		switch f.key {
		case "p_app_key":
			p.AppKey = f.input.Value()
		case "p_access_key":
			p.AccessKey = f.input.Value()
		case "p_api_key":
			p.ApiKey = f.input.Value()
		case "p_api_secret":
			p.ApiSecret = f.input.Value()
		case "p_model":
			p.Model = f.input.Value()
		case "p_base_url":
			p.BaseURL = f.input.Value()
		case "p_app_id":
			p.AppID = f.input.Value()
		case "p_dwa":
			p.DWA = f.input.Value()
		}
	}
}

func (m *Model) restorePushToTalkField() {
	value := m.cfg.Voice.PushToTalk
	if !validVoiceHotkeyString(value) {
		value = config.Default().Voice.PushToTalk
	}
	for i := range m.fields {
		if m.fields[i].key == "push_to_talk" {
			m.fields[i].input.SetValue(value)
			break
		}
	}
	m.logf("↩️  热键已恢复为 %s", value)
}

func validVoiceHotkeyString(value string) bool {
	combo, err := config.ParseHotkey(value)
	return err == nil && !combo.Key.IsTextKey()
}

func splitList(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
func (m *Model) View() string {
	if !m.ready {
		return "loading..."
	}
	var b strings.Builder
	b.WriteString(tStyle.Render("🎙️ 🗣️ Audio Talk AI") + "\n")
	b.WriteString(vStyle.Render("减少用键盘的次数，改用口喷吧。") + "\n")
	b.WriteString(m.renderVoiceStats() + "\n\n")
	b.WriteString(lStyle.Render("── 配置 (e 编辑, s 保存, h 帮助, j/k 导航) ──") + "\n")
	for i, f := range m.fields {
		marker := "  "
		if i == m.cursor {
			if m.editing {
				marker = aStyle.Render("▶ ")
			} else {
				marker = aStyle.Render("▸ ")
			}
		}
		line := marker + lStyle.Render(f.label+": ") + m.renderField(i, f)
		if m.helpVisible && f.help != "" {
			line += " " + dStyle.Render("("+f.help+")")
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + lStyle.Render("── 录音状态 ──") + "\n")
	b.WriteString("  " + m.renderVoiceStatus() + "\n")
	if m.showLogs {
		b.WriteString("\n" + lStyle.Render("── 日志 (l 展开) ──") + "\n")
		maxLogs := 5
		if m.logExpanded {
			maxLogs = 100
		}
		// TUI internal logs
		s := 0
		if len(m.logs) > maxLogs {
			s = len(m.logs) - maxLogs
		}
		for _, l := range m.logs[s:] {
			b.WriteString("  " + dStyle.Render(l) + "\n")
		}
		// Voice plugin logs
		sv := 0
		if len(voice.TUILogBuf) > maxLogs {
			sv = len(voice.TUILogBuf) - maxLogs
		}
		for _, l := range voice.TUILogBuf[sv:] {
			b.WriteString("  " + dStyle.Render(l) + "\n")
		}
	}
	b.WriteString(hStyle.Render("  j/k 导航 | e 编辑 | h 帮助 | esc 退出编辑 | s 保存 | b 后台运行 | q 退出"))
	return b.String()
}

func (m *Model) renderVoiceStats() string {
	stats := voice.TUIStats()
	cpm := 0.0
	if stats.AudioDuration > 0 {
		cpm = float64(stats.Chars) / stats.AudioDuration.Minutes()
	}
	lastCPM := 0.0
	if stats.LastAudioDuration > 0 {
		lastCPM = float64(stats.LastTextChars) / stats.LastAudioDuration.Minutes()
	}
	last := dStyle.Render("最近速度 ") + wStyle.Render("0") + dStyle.Render(" 字/分钟")
	if stats.LastAudioDuration > 0 {
		last = dStyle.Render("最近速度 ") + wStyle.Render(fmt.Sprintf("%.0f", lastCPM)) + dStyle.Render(" 字/分钟")
	}
	return strings.Join([]string{
		dStyle.Render("历史 ") + aStyle.Render(fmt.Sprintf("%d", stats.Sessions)) + dStyle.Render(" 次"),
		dStyle.Render("总字数 ") + aStyle.Render(fmt.Sprintf("%d", stats.Chars)),
		dStyle.Render("平均速度 ") + wStyle.Render(fmt.Sprintf("%.0f", cpm)) + dStyle.Render(" 字/分钟"),
		last,
	}, "  |  ")
}

func (m *Model) renderVoiceStatus() string {
	status := voice.TUIStatus()
	label := "待机"
	style := dStyle
	switch status.State {
	case "connecting":
		label, style = "连接中", wStyle
	case "recording":
		label, style = "录音中", aStyle
	case "stopping_delayed":
		label, style = "延迟停止", wStyle
	case "stopping":
		label, style = "停止中", wStyle
	case "error":
		label, style = "错误", eStyle
	}
	parts := []string{style.Render(label)}
	if status.Detail != "" {
		parts = append(parts, vStyle.Render(status.Detail))
	}
	if !status.StopAt.IsZero() {
		remaining := time.Until(status.StopAt)
		if remaining < 0 {
			remaining = 0
		}
		parts = append(parts, fmt.Sprintf("剩余 %dms", remaining.Milliseconds()))
	}
	if status.State == "error" && !status.ErrorUntil.IsZero() {
		remaining := time.Until(status.ErrorUntil)
		if remaining < 0 {
			remaining = 0
		}
		parts = append(parts, fmt.Sprintf("保留 %ds", int(remaining.Seconds())))
	}
	if status.PendingFinishes > 0 {
		parts = append(parts, fmt.Sprintf("后台收尾 %d", status.PendingFinishes))
	}
	if status.SessionID > 0 {
		parts = append(parts, dStyle.Render(fmt.Sprintf("#%d", status.SessionID)))
	}
	if !m.debug {
		return strings.Join(parts, "  ")
	}
	if !status.UpdatedAt.IsZero() {
		age := time.Since(status.UpdatedAt)
		if age < 0 {
			age = 0
		}
		parts = append(parts, dStyle.Render(fmt.Sprintf("更新 %dms 前", age.Milliseconds())))
	}
	if !status.LastHotkeyAt.IsZero() {
		age := time.Since(status.LastHotkeyAt)
		if age < 0 {
			age = 0
		}
		parts = append(parts, dStyle.Render(fmt.Sprintf("热键 %s %dms 前", status.LastHotkeyType, age.Milliseconds())))
	}
	if !status.LastHandledAt.IsZero() {
		age := time.Since(status.LastHandledAt)
		if age < 0 {
			age = 0
		}
		parts = append(parts, dStyle.Render(fmt.Sprintf("处理 %s %dms 前", status.LastHandledType, age.Milliseconds())))
	}
	if status.QueuedHotkeys > 0 || status.HandledHotkeys > 0 {
		parts = append(parts, dStyle.Render(fmt.Sprintf("入队/处理 %d/%d q=%d", status.QueuedHotkeys, status.HandledHotkeys, status.EventQueueLen)))
	}
	return strings.Join(parts, "  ")
}

func (m *Model) renderField(i int, f field) string {
	editing := m.editing && m.cursor == i
	switch f.fType {
	case fString:
		v := f.input.Value()
		// Mask sensitive fields
		if !editing && isSecretField(f.key) && len(v) > 8 {
			v = v[:8] + "***"
		}
		if editing {
			return f.input.View()
		}
		return vStyle.Render(v)
	case fToggle:
		if f.boolVal {
			return aStyle.Render("● 开") + "  " + dStyle.Render("(空格)")
		}
		return dStyle.Render("○ 关  (空格)")
	case fSelect:
		v := f.opts[f.optIdx]
		if editing {
			return aStyle.Render("[" + v + " ▲▼]")
		}
		return vStyle.Render(v)
	}
	return ""
}

func isSecretField(key string) bool {
	switch key {
	case "access_key", "p_access_key", "p_api_key", "p_api_secret":
		return true
	}
	return false
}

func (m *Model) logf(format string, args ...interface{}) {
	if !m.showLogs {
		return
	}
	m.logs = append(m.logs, fmt.Sprintf(format, args...))
}

func SetProviderInfo(info hotkey.ProviderInfo) tea.Cmd {
	return func() tea.Msg { return backendMsg(info) }
}
func fetchDevices() tea.Cmd {
	return func() tea.Msg {
		devices, err := voice.ListDevices()
		if err != nil {
			return devMsg{Error: err}
		}
		return devMsg{Devices: devices}
	}
}

type refreshMsg struct{}

func tickRefresh() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return refreshMsg{} })
}

// Handle refreshMsg in Update
