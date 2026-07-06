package xfyunspark

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/coder/websocket"
)

const defaultHost = "iat.xf-yun.com"

func init() {
	asr.Register("xfyun-spark", New)
}

type client struct {
	common   asr.Common
	appID    string
	apiKey   string
	apiSec   string
	host     string
	dwa      string
	logger   *slog.Logger
	conn     *websocket.Conn
	connMu   sync.Mutex
	resultCh chan asr.Result
	done     chan struct{}
	final    chan struct{}
	finalOnce sync.Once
	textMu   sync.RWMutex
	lastText string
	seq      int
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appID, _ := providerCfg["app_id"].(string)
	apiKey, _ := providerCfg["api_key"].(string)
	apiSec, _ := providerCfg["api_secret"].(string)
	if appID == "" || apiKey == "" || apiSec == "" {
		return nil, fmt.Errorf("xfyun-spark: app_id, api_key, api_secret are required")
	}
	host, _ := providerCfg["host"].(string)
	if host == "" {
		host = defaultHost
	}
	dwa, _ := providerCfg["dwa"].(string)
	return &client{
		common: common, appID: appID, apiKey: apiKey, apiSec: apiSec,
		host: host, dwa: dwa, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	wsURL := c.buildAuthURL()
	c.logger.Info("connecting to iFlytek Spark ASR", "host", c.host, "app_id", c.appID)
	conn, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial: HTTP %d %s: %w", resp.StatusCode, resp.Status, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.conn = conn
	c.logger.Info("iFlytek Spark ASR connected")
	return nil
}

func (c *client) buildAuthURL() string {
	now := time.Now().UTC()
	date := now.Format("Mon, 02 Jan 2006 15:04:05 GMT")
	signOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET /v1 HTTP/1.1", c.host, date)
	mac := hmac.New(sha256.New, []byte(c.apiSec))
	mac.Write([]byte(signOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s",algorithm="hmac-sha256",headers="host date request-line",signature="%s"`, c.apiKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authOrigin))
	return fmt.Sprintf("wss://%s/v1?authorization=%s&date=%s&host=%s",
		c.host,
		url.QueryEscape(authorization),
		url.QueryEscape(date),
		c.host)
}

const frameSize = 1280 // 40ms at 16kHz 16-bit mono

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	offset := 0
	for offset < len(pcm) {
		end := offset + frameSize
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := pcm[offset:end]
		offset = end

		c.seq++
		status := 1 // intermediate
		if c.seq == 1 {
			status = 0 // first
		}

		frame := c.buildFrame(chunk, status)
		data, _ := json.Marshal(frame)
		if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
			return err
		}
	}

	// Send last frame marker (empty audio, status=2)
	if isLast {
		c.seq++
		frame := c.buildFrame(nil, 2)
		data, _ := json.Marshal(frame)
		return c.conn.Write(ctx, websocket.MessageText, data)
	}
	return nil
}

func (c *client) buildFrame(pcm []byte, status int) map[string]interface{} {
	header := map[string]interface{}{
		"app_id": c.appID,
		"status": status,
	}

	payload := map[string]interface{}{
		"audio": map[string]interface{}{
			"encoding":    "raw",
			"sample_rate": 16000,
			"channels":    1,
			"bit_depth":   16,
			"seq":         c.seq,
			"status":      status,
			"audio":       base64.StdEncoding.EncodeToString(pcm),
		},
	}

	frame := map[string]interface{}{
		"header":  header,
		"payload": payload,
	}

	if c.seq == 1 {
		lang := c.common.Language
		if lang == "" {
			lang = "zh_cn"
		} else {
			lang = normalizeLang(lang)
		}
		parameter := map[string]interface{}{
			"iat": map[string]interface{}{
				"domain":   "slm",
				"language": lang,
				"accent":   "mandarin",
				"eos":      6000,
				"result": map[string]interface{}{
					"encoding": "utf8",
					"compress": "raw",
					"format":   "json",
				},
			},
		}
		if c.dwa != "" {
			parameter["iat"].(map[string]interface{})["dwa"] = c.dwa
		}
		if len(c.common.Hotwords) > 0 {
			parameter["iat"].(map[string]interface{})["dhw"] = "dhw=utf-8;" + strings.Join(c.common.Hotwords, "|")
		}
		frame["parameter"] = parameter
	}

	return frame
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
		Header struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  int    `json:"status"`
		} `json:"header"`
		Payload *struct {
			Result *struct {
				Text   string `json:"text"`
				Status int    `json:"status"`
				Pgs    string `json:"pgs"`
				Rg     []int  `json:"rg"`
			} `json:"result"`
		} `json:"payload"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return
	}
	if msg.Header.Code != 0 {
		c.resultCh <- asr.Result{Error: fmt.Errorf("xfyun error %d: %s", msg.Header.Code, msg.Header.Message)}
		return
	}
	if msg.Payload == nil || msg.Payload.Result == nil {
		return
	}
	text := c.decodeResultText(msg.Payload.Result.Text)
	if text == "" {
		return
	}
	isFinal := msg.Header.Status == 2

	if msg.Payload.Result.Pgs == "rpl" && len(msg.Payload.Result.Rg) >= 2 {
		// Dynamic correction: replace previous results
		c.textMu.Lock()
		c.lastText = text
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: text, IsFinal: isFinal}
	} else {
		c.textMu.Lock()
		c.lastText += text
		full := c.lastText
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}
	}

	if isFinal {
		c.finalOnce.Do(func() { close(c.final) })
	}
}

func (c *client) decodeResultText(encoded string) string {
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	var result struct {
		Ws []struct {
			Cw []struct {
				W string `json:"w"`
			} `json:"cw"`
		} `json:"ws"`
	}
	if json.Unmarshal(decoded, &result) != nil {
		return ""
	}
	var sb strings.Builder
	for _, ws := range result.Ws {
		for _, cw := range ws.Cw {
			sb.WriteString(cw.W)
		}
	}
	return sb.String()
}

func normalizeLang(lang string) string {
	lower := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(lower, "zh") {
		return "zh_cn"
	}
	if strings.HasPrefix(lower, "en") {
		return "en_us"
	}
	if len(lower) >= 2 {
		return lower[:2] + "_" + lower[:2]
	}
	return "zh_cn"
}
