package openaiwhisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
)

const defaultEndpoint = "https://api.openai.com/v1/audio/transcriptions"

func init() {
	asr.RegisterWithMeta("openai-whisper", New, asr.ProviderMeta{
		DisplayName: "OpenAI Whisper",
		Fields: []asr.FieldDef{
			{Key: "api_key", Label: "API Key", Help: "OpenAI API Key", Type: asr.FieldSecret, Secret: true},
			{Key: "model", Label: "Model", Help: "模型名", Type: asr.FieldText, Default: "whisper-1"},
			{Key: "base_url", Label: "Endpoint", Help: "API 端点（留空用默认）", Type: asr.FieldText},
		},
	})
}

type client struct {
	common    asr.Common
	apiKey    string
	model     string
	endpoint  string
	logger    *slog.Logger
	mu        sync.Mutex
	buf       []byte
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
		return nil, fmt.Errorf("openai-whisper: api_key is required")
	}
	model, _ := providerCfg["model"].(string)
	if model == "" {
		model = "whisper-1"
	}
	endpoint, _ := providerCfg["endpoint"].(string)
	if endpoint == "" {
		// TUI/WebUI forms write the typed config field "base_url".
		endpoint, _ = providerCfg["base_url"].(string)
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &client{
		common: common, apiKey: apiKey, model: model,
		endpoint: endpoint, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func (c *client) Connect(_ context.Context) error {
	c.logger.Info("OpenAI Whisper ASR ready", "model", c.model)
	return nil
}

func (c *client) SendAudio(_ context.Context, pcm []byte, isLast bool) error {
	c.mu.Lock()
	c.buf = append(c.buf, pcm...)
	c.mu.Unlock()
	if isLast {
		go c.transcribeAndDeliver()
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
	<-ctx.Done()
}

func (c *client) Close() error {
	return nil
}

func (c *client) transcribeAndDeliver() {
	defer close(c.resultCh)
	defer close(c.done)

	c.mu.Lock()
	pcm := c.buf
	c.buf = nil
	c.mu.Unlock()

	if len(pcm) == 0 {
		return
	}

	c.logger.Info("sending audio to OpenAI Whisper", "bytes", len(pcm))
	text, err := c.transcribe(pcm)
	if err != nil {
		c.resultCh <- asr.Result{Error: err}
		return
	}
	if text != "" {
		c.textMu.Lock()
		c.lastText = text
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: text, IsFinal: true}
		c.finalOnce.Do(func() { close(c.final) })
	}
}

func (c *client) transcribe(pcm []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// Write the audio file as WAV
	if err := w.WriteField("model", c.model); err != nil {
		return "", err
	}
	lang := c.common.Language
	if lang == "" {
		lang = "zh"
	}
	if err := w.WriteField("language", normalizeLang(lang)); err != nil {
		return "", err
	}
	if len(c.common.Hotwords) > 0 {
		if err := w.WriteField("prompt", strings.Join(c.common.Hotwords, ", ")); err != nil {
			return "", err
		}
	}

	part, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	wav := pcmToWAV(pcm, 16000, 1)
	if _, err := part.Write(wav); err != nil {
		return "", err
	}
	w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("whisper API: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("whisper response: %w", err)
	}
	return result.Text, nil
}

func normalizeLang(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lower, "zh") {
		return "zh"
	}
	if len(lower) >= 2 {
		return lower[:2]
	}
	return lower
}

// pcmToWAV wraps raw PCM16 data in a WAV header.
func pcmToWAV(pcm []byte, sampleRate, channels int) []byte {
	dataSize := len(pcm)
	byteRate := sampleRate * channels * 2
	blockAlign := channels * 2
	header := make([]byte, 44)
	copy(header, "RIFF")
	putLE32(header[4:], uint32(36+dataSize))
	copy(header[8:], "WAVE")
	copy(header[12:], "fmt ")
	putLE32(header[16:], 16)
	putLE16(header[20:], 1) // PCM
	putLE16(header[22:], uint16(channels))
	putLE32(header[24:], uint32(sampleRate))
	putLE32(header[28:], uint32(byteRate))
	putLE16(header[32:], uint16(blockAlign))
	putLE16(header[34:], 16) // bits per sample
	copy(header[36:], "data")
	putLE32(header[40:], uint32(dataSize))
	return append(header, pcm...)
}

func putLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
