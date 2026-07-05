package doubao

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	hdrVersion       = 0x10
	hdrHeaderSize    = 0x01
	msgFullClientReq = 0x10
	msgAudioOnly     = 0x20
	hdrJSON          = 0x10
	hdrRawAudio      = 0x00
)

func init() {
	asr.Register("doubao", New)
}

type client struct {
	common     asr.Common
	appKey     string
	accessKey  string
	resourceID string
	logger     *slog.Logger
	conn       *websocket.Conn
	connMu     sync.Mutex
	resultCh   chan asr.Result
	done       chan struct{}
	final      chan struct{}
	finalOnce  sync.Once
	textMu     sync.RWMutex
	lastText   string
}

// New creates a Doubao/ByteDance SAUC ASR client.
func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appKey, _ := providerCfg["app_key"].(string)
	accessKey, _ := providerCfg["access_key"].(string)
	resourceID, _ := providerCfg["resource_id"].(string)
	if resourceID == "" {
		resourceID = "volc.bigasr.sauc.duration"
	}
	if appKey == "" || accessKey == "" {
		return nil, fmt.Errorf("doubao: app_key and access_key are required")
	}
	return &client{
		common: common, appKey: appKey, accessKey: accessKey,
		resourceID: resourceID, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	url := "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async"
	c.logger.Info("connecting to ASR", "url", url, "provider", "doubao")
	conn, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"X-Api-App-Key":     {c.appKey},
			"X-Api-Access-Key":  {c.accessKey},
			"X-Api-Resource-Id": {c.resourceID},
			"X-Api-Request-Id":  {uuid.New().String()},
			"X-Api-Connect-Id":  {uuid.New().String()},
			"X-Api-Sequence":    {"-1"},
		},
	})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial: HTTP %d %s: %w", resp.StatusCode, resp.Status, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.conn = conn
	if err := c.sendFullClientRequest(ctx); err != nil {
		conn.Close(websocket.StatusInternalError, "init failed")
		return fmt.Errorf("send init: %w", err)
	}
	c.logger.Info("ASR connected", "provider", "doubao")
	return nil
}

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	flags := byte(0x00)
	if isLast {
		flags = 0x02
	}
	header := []byte{hdrVersion | hdrHeaderSize, msgAudioOnly | flags, hdrRawAudio, 0x00}
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(pcm)))
	return c.conn.Write(ctx, websocket.MessageBinary, append(append(header, size...), pcm...))
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
		c.parseResponse(data)
	}
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "done")
	}
	return nil
}

func (c *client) sendFullClientRequest(ctx context.Context) error {
	request := map[string]interface{}{
		"model_name": "bigmodel", "enable_itn": true, "enable_punc": true,
		"enable_ddc": false, "enable_word": false,
		"enable_nonstream": true, "result_type": "full", "show_utterances": true,
	}
	if len(c.common.Hotwords) > 0 {
		if contextJSON, err := hotwordsContext(c.common.Hotwords); err == nil && contextJSON != "" {
			request["corpus"] = map[string]interface{}{"context": contextJSON}
		}
	}
	payload := map[string]interface{}{
		"user":    map[string]string{"uid": "audio-talk-ai"},
		"audio":   map[string]interface{}{"format": "pcm", "rate": 16000, "bits": 16, "channel": 1, "codec": "raw"},
		"request": request,
	}
	jsonBytes, _ := json.Marshal(payload)
	header := []byte{hdrVersion | hdrHeaderSize, msgFullClientReq, hdrJSON, 0x00}
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(jsonBytes)))
	return c.conn.Write(ctx, websocket.MessageBinary, append(append(header, size...), jsonBytes...))
}

func hotwordsContext(words []string) (string, error) {
	hotwords := make([]map[string]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		hotwords = append(hotwords, map[string]string{"word": word})
	}
	if len(hotwords) == 0 {
		return "", nil
	}
	b, err := json.Marshal(map[string]interface{}{"hotwords": hotwords})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *client) parseResponse(data []byte) {
	if len(data) < 12 {
		return
	}
	flags := data[1] & 0x0F
	msgType := (data[1] >> 4) & 0x0F
	if msgType != 0x09 {
		return
	}
	payloadSize := binary.BigEndian.Uint32(data[8:12])
	if int(payloadSize)+12 > len(data) {
		return
	}
	payload := data[12 : 12+payloadSize]
	var resp struct {
		Result struct {
			Text       string `json:"text"`
			Utterances []struct {
				Text     string `json:"text"`
				Definite bool   `json:"definite"`
			} `json:"utterances"`
		} `json:"result"`
	}
	if json.Unmarshal(payload, &resp) != nil {
		return
	}
	text := resp.Result.Text
	isFinal := flags == 0x02 || flags == 0x03
	for _, u := range resp.Result.Utterances {
		if u.Definite {
			isFinal = true
		}
	}
	if text != "" {
		c.textMu.Lock()
		c.lastText = text
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: text, IsFinal: isFinal}
		if isFinal {
			c.finalOnce.Do(func() { close(c.final) })
		}
	}
}
