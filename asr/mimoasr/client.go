package mimoasr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
)

const defaultEndpoint = "https://api.xiaomimimo.com/v1/chat/completions"

func init() {
	asr.Register("mimo-asr", New)
}

type client struct {
	common   asr.Common
	apiKey   string
	model    string
	endpoint string
	logger   *slog.Logger
	mu       sync.Mutex
	buf      []byte
	resultCh chan asr.Result
	done     chan struct{}
	final    chan struct{}
	finalOnce sync.Once
	textMu   sync.RWMutex
	lastText string
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	apiKey, _ := providerCfg["api_key"].(string)
	if apiKey == "" {
		return nil, fmt.Errorf("mimo-asr: api_key is required")
	}
	model, _ := providerCfg["model"].(string)
	if model == "" {
		model = "mimo-v2.5-asr"
	}
	endpoint, _ := providerCfg["endpoint"].(string)
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
	c.logger.Info("MiMo ASR ready", "model", c.model)
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

	c.logger.Info("sending audio to MiMo ASR", "bytes", len(pcm))
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
	wav := pcmToWAV(pcm, 16000, 1)
	b64 := base64.StdEncoding.EncodeToString(wav)

	lang := c.common.Language
	if lang == "" {
		lang = "auto"
	} else {
		lang = normalizeLang(lang)
	}

	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{{
				"type": "input_audio",
				"input_audio": map[string]interface{}{
					"data":   fmt.Sprintf("data:audio/wav;base64,%s", b64),
					"format": "wav",
				},
			}},
		}},
		"asr_options": map[string]interface{}{
			"language": lang,
		},
	}

	if len(c.common.Hotwords) > 0 {
		reqBody["asr_options"].(map[string]interface{})["hotwords"] = c.common.Hotwords
	}

	body, _ := json.Marshal(reqBody)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mimo-asr request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mimo-asr API: HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("mimo-asr response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("mimo-asr: empty response")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func normalizeLang(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lower, "zh") {
		return "zh"
	}
	if strings.HasPrefix(lower, "en") {
		return "en"
	}
	return "auto"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
