package xfyunrtasr

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/gorilla/websocket"
)

const defaultHost = "office-api-ast-dx.iflyaisol.com"

func init() {
	asr.RegisterWithMeta("xfyun-rtasr", New, asr.ProviderMeta{
		DisplayName: "讯飞实时转写",
		Fields: []asr.FieldDef{
			{Key: "app_id", Label: "App ID", Help: "讯飞 App ID", Type: asr.FieldText},
			{Key: "api_key", Label: "AccessKeyId", Help: "讯飞 AccessKeyId", Type: asr.FieldSecret, Secret: true},
			{Key: "api_secret", Label: "AccessKeySecret", Help: "讯飞 AccessKeySecret", Type: asr.FieldSecret, Secret: true},
			{Key: "lang", Label: "语种", Help: "autodialect=中英+方言, autominor=多语种", Type: asr.FieldSelect,
				Options: []string{"autodialect", "autominor"}, Labels: []string{"中英+方言 (autodialect)", "多语种 (autominor)"}},
			{Key: "pd", Label: "领域", Help: "可选: court/edu/finance/medical/tech/sport/gov", Type: asr.FieldText},
		},
	})
}

type client struct {
	common    asr.Common
	appID     string
	accessKey string // accessKeyId
	secretKey string // accessKeySecret
	host      string
	lang      string // autodialect or autominor
	pd        string // domain parameter
	logger    *slog.Logger
	conn      *websocket.Conn
	connMu    sync.Mutex
	resultCh  chan asr.Result
	done      chan struct{}
	final     chan struct{}
	finalOnce sync.Once
	textMu    sync.RWMutex
	lastText  string
	sessionID string
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appID, _ := providerCfg["app_id"].(string)
	accessKey, _ := providerCfg["api_key"].(string)
	secretKey, _ := providerCfg["api_secret"].(string)
	if appID == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("xfyun-rtasr: app_id, api_key (accessKeyId), api_secret (accessKeySecret) are required")
	}
	host, _ := providerCfg["host"].(string)
	if host == "" {
		host = defaultHost
	}
	lang, _ := providerCfg["lang"].(string)
	if lang == "" {
		lang = "autodialect"
	}
	pd, _ := providerCfg["pd"].(string)

	return &client{
		common: common, appID: appID, accessKey: accessKey, secretKey: secretKey,
		host: host, lang: lang, pd: pd, logger: logger,
		resultCh:  make(chan asr.Result, 64),
		done:      make(chan struct{}),
		final:     make(chan struct{}),
		sessionID: fmt.Sprintf("%d", time.Now().UnixNano()),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	wsURL := c.buildAuthURL()
	c.logger.Info("connecting to iFlytek RTASR", "host", c.host, "app_id", c.appID, "lang", c.lang)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, resp, err := dialer.Dial(wsURL, http.Header{})
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket dial: HTTP %d %s: %w", resp.StatusCode, resp.Status, err)
		}
		return fmt.Errorf("websocket dial: %w", err)
	}
	c.conn = conn
	c.logger.Info("iFlytek RTASR connected")
	return nil
}

func (c *client) buildAuthURL() string {
	// Build timestamp in format: 2006-01-02T15:04:05+0800
	utc := time.Now().Format("2006-01-02T15:04:05+0800")

	// Build params map (excluding signature)
	params := map[string]string{
		"appId":         c.appID,
		"accessKeyId":   c.accessKey,
		"utc":           utc,
		"lang":          c.lang,
		"audio_encode":  "pcm_s16le",
		"samplerate":    "16000",
	}
	if c.pd != "" {
		params["pd"] = c.pd
	}

	// Generate signature
	signature := c.generateSignature(params)
	params["signature"] = signature

	// Build URL
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	return fmt.Sprintf("wss://%s/ast/communicate/v1?%s", c.host, vals.Encode())
}

func (c *client) generateSignature(params map[string]string) string {
	// 1. Sort all params by key (excluding signature)
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 2. Build baseString: URL-encoded key=value pairs joined by "&"
	var parts []string
	for _, k := range keys {
		v := params[k]
		if v == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
	}
	baseString := strings.Join(parts, "&")

	// 3. HMAC-SHA1 with accessKeySecret
	mac := hmac.New(sha1.New, []byte(c.secretKey))
	mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

const frameSize = 1280 // 40ms at 16kHz 16-bit mono

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// iFlytek is sensitive to clipping. Reduce volume to prevent saturation.
	if len(pcm) > 1 {
		adjusted := make([]byte, len(pcm))
		for i := 0; i+1 < len(pcm); i += 2 {
			s := int32(int16(pcm[i]) | int16(pcm[i+1])<<8)
			v := s / 3
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			adjusted[i] = byte(v)
			adjusted[i+1] = byte(v >> 8)
		}
		pcm = adjusted
	}

	// Send audio as Binary WebSocket messages
	offset := 0
	for offset < len(pcm) {
		end := offset + frameSize
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := pcm[offset:end]
		offset = end

		if err := c.conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
			return fmt.Errorf("rtasr send audio: %w", err)
		}
	}

	// If isLast, send end signal as JSON text message
	if isLast {
		endMsg := map[string]interface{}{
			"end":       true,
			"sessionId": c.sessionID,
		}
		data, _ := json.Marshal(endMsg)
		c.logger.Info("rtasr sending end signal", "sessionId", c.sessionID)
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return fmt.Errorf("rtasr send end: %w", err)
		}
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
	c.logger.Info("rtasr ReceiveLoop started")
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.logger.Info("rtasr ReceiveLoop exited", "error", err)
			return
		}
		c.handleMessage(data)
	}
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *client) handleMessage(data []byte) {
	// Response format: {"action":"result","code":"...","data":{...},"desc":"...","sid":"..."}
	var msg struct {
		Action string          `json:"action"`
		Code   string          `json:"code"`
		Data   json.RawMessage `json:"data"`
		Desc   string          `json:"desc"`
		Sid    string          `json:"sid"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Warn("rtasr parse error", "error", err, "data", string(data[:min(len(data), 200)]))
		return
	}

	// Check for errors
	if msg.Action == "error" || (msg.Code != "" && msg.Code != "0") {
		errMsg := fmt.Errorf("讯飞RTASR错误 %s: %s", msg.Code, msg.Desc)
		c.logger.Error("rtasr error", "code", msg.Code, "desc", msg.Desc)
		c.resultCh <- asr.Result{Error: errMsg}
		return
	}

	// Skip "started" action
	if msg.Action == "started" {
		c.logger.Info("rtasr session started", "sid", msg.Sid)
		return
	}

	// Parse result data
	var resultData struct {
		SegID int `json:"seg_id"`
		Cn    struct {
			St struct {
				Bg   int    `json:"bg"`
				Ed   int    `json:"ed"`
				Type string `json:"type"` // "0" = final, "1" = intermediate
				Rt   []struct {
					Ws []struct {
						Cw []struct {
							W  string `json:"w"`
							Wp string `json:"wp"` // n=normal, s=smooth, p=punctuation, g=segment
						} `json:"cw"`
					} `json:"ws"`
				} `json:"rt"`
			} `json:"st"`
		} `json:"cn"`
		Ls bool `json:"ls"` // true = last frame
	}
	if err := json.Unmarshal(msg.Data, &resultData); err != nil {
		c.logger.Warn("rtasr data parse error", "error", err)
		return
	}

	// Extract text from ws[].cw[].w
	var sb strings.Builder
	for _, rt := range resultData.Cn.St.Rt {
		for _, ws := range rt.Ws {
			for _, cw := range ws.Cw {
				sb.WriteString(cw.W)
			}
		}
	}
	text := sb.String()
	isFinal := resultData.Ls
	resultType := resultData.Cn.St.Type // "0" = final segment, "1" = intermediate

	c.logger.Info("rtasr result", "text", text, "isFinal", isFinal, "type", resultType, "seg_id", resultData.SegID)

	// Only emit non-empty results
	if text == "" {
		if isFinal {
			c.finalOnce.Do(func() { close(c.final) })
		}
		return
	}

	// Update last text (accumulate all segments)
	c.textMu.Lock()
	c.lastText += text
	full := c.lastText
	c.textMu.Unlock()

	// Emit result (type "0" = confirmed result, type "1" = intermediate)
	c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}

	if isFinal {
		c.finalOnce.Do(func() { close(c.final) })
	}
}
