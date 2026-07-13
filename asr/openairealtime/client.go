package openairealtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/coder/websocket"
)

const defaultURL = "wss://api.openai.com/v1/realtime"

func init() {
	asr.RegisterWithMeta("openai-realtime", New, asr.ProviderMeta{
		DisplayName: "OpenAI Realtime",
		Fields: []asr.FieldDef{
			{Key: "api_key", Label: "API Key", Help: "OpenAI API Key", Type: asr.FieldSecret, Secret: true},
			{Key: "model", Label: "Model", Help: "模型名", Type: asr.FieldText, Default: "gpt-4o-mini-transcribe"},
		},
	})
}

type client struct {
	common    asr.Common
	apiKey    string
	model     string
	baseURL   string
	logger    *slog.Logger
	conn      *websocket.Conn
	connMu    sync.Mutex
	resultCh  chan asr.Result
	done      chan struct{}
	final     chan struct{}
	finalOnce sync.Once
	textMu    sync.RWMutex
	lastText  string
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	apiKey, _ := providerCfg["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("openai-realtime: api_key is required")
	}
	model, _ := providerCfg["model"].(string)
	if model == "" {
		model = "gpt-4o-mini-transcribe"
	}
	baseURL, _ := providerCfg["base_url"].(string)
	if baseURL == "" {
		baseURL = defaultURL
	}
	return &client{
		common: common, apiKey: apiKey, model: model,
		baseURL: baseURL, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	c.logger.Info("connecting to OpenAI Realtime ASR", "url", c.baseURL, "model", c.model)
	conn, resp, err := websocket.Dial(ctx, c.baseURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"Authorization": {"Bearer " + c.apiKey},
			"OpenAI-Beta":   {"realtime=v1"},
		},
	})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial: HTTP %d %s: %w", resp.StatusCode, resp.Status, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.conn = conn

	lang := c.common.Language
	if lang == "" {
		lang = "zh"
	} else {
		lang = normalizeLang(lang)
	}
	if err := c.sendSessionUpdate(ctx, lang); err != nil {
		conn.Close(websocket.StatusInternalError, "init failed")
		return fmt.Errorf("session update: %w", err)
	}
	c.logger.Info("OpenAI Realtime ASR connected", "model", c.model)
	return nil
}

func (c *client) sendSessionUpdate(ctx context.Context, lang string) error {
	transcription := map[string]interface{}{
		"model":    c.model,
		"language": lang,
	}
	if len(c.common.Hotwords) > 0 {
		transcription["prompt"] = strings.Join(c.common.Hotwords, ", ")
	}
	update := map[string]interface{}{
		"type": "session.update",
		"session": map[string]interface{}{
			"input_audio_format": "pcm16",
			"input_audio_transcription": transcription,
			"turn_detection": nil,
		},
	}
	data, _ := json.Marshal(update)
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// Resample 16kHz -> 24kHz if needed (linear interpolation)
	resampled := resample16to24(pcm)

	appendMsg := map[string]interface{}{
		"type":  "input_audio_buffer.append",
		"audio": base64.StdEncoding.EncodeToString(resampled),
	}
	data, _ := json.Marshal(appendMsg)
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return err
	}

	if isLast {
		commitMsg := map[string]interface{}{"type": "input_audio_buffer.commit"}
		commitData, _ := json.Marshal(commitMsg)
		return c.conn.Write(ctx, websocket.MessageText, commitData)
	}
	return nil
}

func (c *client) Results() <-chan asr.Result { return c.resultCh }
func (c *client) Done() <-chan struct{}      { return c.done }
func (c *client) Final() <-chan struct{}     { return c.final }

func (c *client) LastText() string {
	c.textMu.RLock()
	defer c.textMu.RUnlock()
	return c.lastText
}

func (c *client) ReceiveLoop(ctx context.Context) {
	defer close(c.resultCh)
	defer close(c.done)
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}
		c.handleMessage(data)
	}
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "done")
	}
	return nil
}

func (c *client) handleMessage(data []byte) {
	var msg struct {
		Type    string `json:"type"`
		Delta   string `json:"delta"`
		Transcript string `json:"transcript"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return
	}
	switch msg.Type {
	case "conversation.item.input_audio_transcription.delta":
		if msg.Delta != "" {
			c.textMu.Lock()
			c.lastText += msg.Delta
			text := c.lastText
			c.textMu.Unlock()
			c.resultCh <- asr.Result{Text: text, IsFinal: false}
		}
	case "conversation.item.input_audio_transcription.completed":
		text := msg.Transcript
		if text == "" {
			c.textMu.RLock()
			text = c.lastText
			c.textMu.RUnlock()
		}
		if text != "" {
			c.textMu.Lock()
			c.lastText = text
			c.textMu.Unlock()
			c.resultCh <- asr.Result{Text: text, IsFinal: true}
			c.finalOnce.Do(func() { close(c.final) })
		}
	case "error":
		if msg.Error != nil {
			c.resultCh <- asr.Result{Error: fmt.Errorf("openai realtime: %s", msg.Error.Message)}
		}
	}
}

// resample16to24 upsamples 16kHz PCM16 to 24kHz using linear interpolation.
func resample16to24(pcm16 []byte) []byte {
	samples := len(pcm16) / 2
	if samples == 0 {
		return pcm16
	}
	// 16kHz -> 24kHz: ratio 3/2, output = samples * 3 / 2
	outSamples := samples * 3 / 2
	out := make([]byte, outSamples*2)
	for i := 0; i < outSamples; i++ {
		srcPos := float64(i) * 2.0 / 3.0
		idx := int(srcPos)
		frac := srcPos - float64(idx)
		var s0, s1 int16
		if idx < samples {
			s0 = int16(pcm16[idx*2]) | int16(pcm16[idx*2+1])<<8
		}
		if idx+1 < samples {
			s1 = int16(pcm16[(idx+1)*2]) | int16(pcm16[(idx+1)*2+1])<<8
		}
		interpolated := float64(s0)*(1-frac) + float64(s1)*frac
		clamped := int16(math.Max(-32768, math.Min(32767, interpolated)))
		out[i*2] = byte(clamped)
		out[i*2+1] = byte(clamped >> 8)
	}
	return out
}

func normalizeLang(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lower, "zh") {
		return "zh"
	}
	if strings.HasPrefix(lower, "en") {
		return "en"
	}
	if strings.HasPrefix(lower, "ja") {
		return "ja"
	}
	if len(lower) >= 2 {
		return lower[:2]
	}
	return lower
}
