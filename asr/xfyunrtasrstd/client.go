package xfyunrtasrstd

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/gorilla/websocket"
)

const defaultHost = "rtasr.xfyun.cn"

func init() {
	asr.RegisterWithMeta("xfyun-rtasr-std", New, asr.ProviderMeta{
		DisplayName: "讯飞实时转写标准版",
		Fields: []asr.FieldDef{
			{Key: "app_id", Label: "App ID", Help: "讯飞 App ID", Type: asr.FieldText},
			{Key: "api_key", Label: "API Key", Help: "讯飞 API Key", Type: asr.FieldSecret, Secret: true},
			{Key: "lang", Label: "语种", Help: "cn=中文, en=英文", Type: asr.FieldSelect,
				Options: []string{"cn", "en"}, Labels: []string{"中文 (cn)", "英文 (en)"}},
			{Key: "pd", Label: "领域", Help: "可选: court/edu/finance/medical/tech/sport/gov", Type: asr.FieldText},
		},
	})
}

type client struct {
	common   asr.Common
	appID    string
	apiKey   string // apiKey for HMAC-SHA1(MD5(appid+ts), apiKey)
	host     string
	lang     string // cn or en
	pd       string // domain parameter
	logger   *slog.Logger
	conn     *websocket.Conn
	connMu   sync.Mutex
	resultCh chan asr.Result
	done     chan struct{}
	final    chan struct{}
	finalOnce sync.Once
	textMu   sync.RWMutex
	lastText string
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appID, _ := providerCfg["app_id"].(string)
	apiKey, _ := providerCfg["api_key"].(string)
	if appID == "" || apiKey == "" {
		return nil, fmt.Errorf("xfyun-rtasr-std: app_id and api_key are required")
	}
	host, _ := providerCfg["host"].(string)
	if host == "" {
		host = defaultHost
	}
	lang, _ := providerCfg["lang"].(string)
	if lang == "" {
		lang = "cn"
	}
	pd, _ := providerCfg["pd"].(string)

	return &client{
		common: common, appID: appID, apiKey: apiKey,
		host: host, lang: lang, pd: pd, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	wsURL := c.buildAuthURL()
	c.logger.Info("connecting to iFlytek RTASR Standard", "host", c.host, "app_id", c.appID, "lang", c.lang)
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
	c.logger.Info("iFlytek RTASR Standard connected")
	return nil
}

func (c *client) buildAuthURL() string {
	ts := fmt.Sprintf("%d", time.Now().Unix())

	// signa = Base64(HMAC-SHA1(MD5(appid+ts), apiKey))
	baseString := c.appID + ts
	md5Hash := fmt.Sprintf("%x", md5.Sum([]byte(baseString)))
	mac := hmac.New(sha1.New, []byte(c.apiKey))
	mac.Write([]byte(md5Hash))
	signa := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	vals := url.Values{}
	vals.Set("appid", c.appID)
	vals.Set("ts", ts)
	vals.Set("signa", signa)
	if c.lang != "" {
		vals.Set("lang", c.lang)
	}
	if c.pd != "" {
		vals.Set("pd", c.pd)
	}
	return fmt.Sprintf("wss://%s/v1/ws?%s", c.host, vals.Encode())
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
			return fmt.Errorf("rtasr-std send audio: %w", err)
		}
	}

	// If isLast, send end signal as JSON text message
	if isLast {
		// Standard version uses {"end": "true"} (string)
		endMsg := `{"end": "true"}`
		c.logger.Info("rtasr-std sending end signal")
		if err := c.conn.WriteMessage(websocket.TextMessage, []byte(endMsg)); err != nil {
			return fmt.Errorf("rtasr-std send end: %w", err)
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
	c.logger.Info("rtasr-std ReceiveLoop started")
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.logger.Info("rtasr-std ReceiveLoop exited", "error", err)
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
	// Response format: {"action":"result","code":"...","data":"...","desc":"...","sid":"..."}
	// Note: In standard version, "data" is a STRING (not raw JSON object)
	var msg struct {
		Action string `json:"action"`
		Code   string `json:"code"`
		Data   string `json:"data"`
		Desc   string `json:"desc"`
		Sid    string `json:"sid"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Warn("rtasr-std parse error", "error", err, "data", string(data[:min(len(data), 200)]))
		return
	}

	// Check for errors
	if msg.Action == "error" || (msg.Code != "" && msg.Code != "0") {
		errMsg := fmt.Errorf("讯飞RTASR标准版错误 %s: %s", msg.Code, msg.Desc)
		c.logger.Error("rtasr-std error", "code", msg.Code, "desc", msg.Desc)
		c.resultCh <- asr.Result{Error: errMsg}
		return
	}

	// Skip "started" action
	if msg.Action == "started" {
		c.logger.Info("rtasr-std session started", "sid", msg.Sid)
		return
	}

	// data is a JSON string, parse it
	if msg.Data == "" {
		return
	}

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
							Wp string `json:"wp"`
						} `json:"cw"`
					} `json:"ws"`
				} `json:"rt"`
			} `json:"st"`
		} `json:"cn"`
		Ls bool `json:"ls"` // true = last frame
	}
	if err := json.Unmarshal([]byte(msg.Data), &resultData); err != nil {
		c.logger.Warn("rtasr-std data parse error", "error", err, "data", msg.Data[:min(len(msg.Data), 200)])
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

	c.logger.Info("rtasr-std result", "text", text, "isFinal", isFinal, "type", resultData.Cn.St.Type, "seg_id", resultData.SegID)

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

	c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}

	if isFinal {
		c.finalOnce.Do(func() { close(c.final) })
	}
}
