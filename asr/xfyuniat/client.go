package xfyuniat

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
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

const defaultHost = "iat-api.xfyun.cn"

func init() {
	asr.RegisterWithMeta("xfyun-iat", New, asr.ProviderMeta{
		DisplayName: "讯飞语音听写",
		Fields: []asr.FieldDef{
			{Key: "app_id", Label: "App ID", Help: "讯飞 App ID", Type: asr.FieldText},
			{Key: "api_key", Label: "API Key", Help: "讯飞 API Key", Type: asr.FieldSecret, Secret: true},
			{Key: "api_secret", Label: "API Secret", Help: "讯飞 API Secret", Type: asr.FieldSecret, Secret: true},
			{Key: "domain", Label: "领域", Help: "应用领域", Type: asr.FieldSelect,
				Options: []string{"iat", "medical", "gov-seat-assistant", "seat-assistant", "gov-ansys", "gov-nav", "fin-nav", "fin-ansys"},
				Labels:  []string{"日常用语", "医疗", "政务坐席", "金融坐席", "政务分析", "政务导航", "金融导航", "金融分析"}},
			{Key: "accent", Label: "方言", Help: "mandarin=普通话, xfime-mianqie=方言免切", Type: asr.FieldText, Default: "mandarin"},
			{Key: "dwa", Label: "动态修正", Help: "wpgs 开启语音纠偏", Type: asr.FieldSelect, Options: []string{"", "wpgs"}, Labels: []string{"关闭", "wpgs 开启"}},
		},
	})
}

type client struct {
	common    asr.Common
	appID     string
	apiKey    string
	apiSec    string
	host      string
	domain    string // iat, medical, gov-seat-assistant, etc.
	accent    string // mandarin, etc.
	dwa       string
	logger    *slog.Logger
	conn      *websocket.Conn
	connMu    sync.Mutex
	resultCh  chan asr.Result
	done      chan struct{}
	final     chan struct{}
	finalOnce sync.Once
	textMu    sync.RWMutex
	lastText  string
	seq       int
	segments  map[int]string
}

func New(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appID, _ := providerCfg["app_id"].(string)
	apiKey, _ := providerCfg["api_key"].(string)
	apiSec, _ := providerCfg["api_secret"].(string)
	if appID == "" || apiKey == "" || apiSec == "" {
		return nil, fmt.Errorf("xfyun-iat: app_id, api_key, api_secret are required")
	}
	host, _ := providerCfg["host"].(string)
	if host == "" {
		host = defaultHost
	}
	domain, _ := providerCfg["domain"].(string)
	if domain == "" {
		domain = "iat"
	}
	accent, _ := providerCfg["accent"].(string)
	if accent == "" {
		accent = "mandarin"
	}
	dwa, _ := providerCfg["dwa"].(string)
	return &client{
		common: common, appID: appID, apiKey: apiKey, apiSec: apiSec,
		host: host, domain: domain, accent: accent, dwa: dwa, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
		segments: make(map[int]string),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	wsURL := c.buildAuthURL()
	c.logger.Info("connecting to iFlytek IAT ASR", "host", c.host, "domain", c.domain, "app_id", c.appID)
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
	c.logger.Info("iFlytek IAT ASR connected")
	return nil
}

func (c *client) buildAuthURL() string {
	now := time.Now().UTC()
	date := now.Format("Mon, 02 Jan 2006 15:04:05 GMT")
	signOrigin := fmt.Sprintf("host: %s\ndate: %s\nGET /v2/iat HTTP/1.1", c.host, date)
	mac := hmac.New(sha256.New, []byte(c.apiSec))
	mac.Write([]byte(signOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s",algorithm="hmac-sha256",headers="host date request-line",signature="%s"`, c.apiKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authOrigin))
	return fmt.Sprintf("wss://%s/v2/iat?authorization=%s&date=%s&host=%s",
		c.host,
		url.QueryEscape(authorization),
		url.QueryEscape(date),
		c.host)
}

const frameSize = 1280 // 40ms at 16kHz 16-bit mono

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// Reduce volume to prevent clipping (same as xfyun-spark).
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

	offset := 0
	for offset < len(pcm) {
		end := offset + frameSize
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := pcm[offset:end]
		offset = end

		c.seq++
		status := 1
		if c.seq == 1 {
			status = 0
		}
		if isLast && offset >= len(pcm) {
			status = 2
		}

		frame := c.buildFrame(chunk, status)
		data, _ := json.Marshal(frame)
		if c.seq == 1 {
			c.logger.Info("xfyun-iat first frame", "json", string(data[:min(len(data), 500)]))
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
	}

	if isLast && len(pcm) == 0 {
		c.seq++
		frame := map[string]interface{}{
			"data": map[string]interface{}{
				"status": 2,
			},
		}
		data, _ := json.Marshal(frame)
		c.logger.Info("xfyun-iat last frame (empty)")
		return c.conn.WriteMessage(websocket.TextMessage, data)
	}
	return nil
}

func (c *client) buildFrame(pcm []byte, status int) map[string]interface{} {
	data := map[string]interface{}{
		"status":   status,
		"format":   "audio/L16;rate=16000",
		"encoding": "raw",
		"audio":    base64.StdEncoding.EncodeToString(pcm),
	}

	frame := map[string]interface{}{
		"data": data,
	}

	if c.seq == 1 {
		frame["common"] = map[string]interface{}{
			"app_id": c.appID,
		}
		business := map[string]interface{}{
			"language": "zh_cn",
			"domain":   c.domain,
			"accent":   c.accent,
			"eos":      6000,
		}
		if c.dwa != "" {
			business["dwa"] = c.dwa
		}
		if len(c.common.Hotwords) > 0 {
			business["dhw"] = strings.Join(c.common.Hotwords, ",")
		}
		frame["business"] = business
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
	c.logger.Info("xfyun-iat ReceiveLoop started")
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.logger.Info("xfyun-iat ReceiveLoop exited", "error", err)
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
	var msg struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Sid     string `json:"sid"`
		Data    *struct {
			Status int `json:"status"`
			Result *struct {
				Sn  int    `json:"sn"`
				Ls  bool   `json:"ls"`
				Pgs string `json:"pgs"`
				Rg  []int  `json:"rg"`
				Ws  []struct {
					Cw []struct {
						W string `json:"w"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"result"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return
	}
	if msg.Code != 0 {
		errMsg := fmt.Errorf("讯飞IAT错误 %d: %s", msg.Code, msg.Message)
		c.logger.Error("xfyun-iat ASR error", "code", msg.Code, "message", msg.Message)
		c.resultCh <- asr.Result{Error: errMsg}
		return
	}
	if msg.Data == nil || msg.Data.Result == nil {
		return
	}

	isFinal := msg.Data.Status == 2

	// Build text from ws[].cw[].w
	var sb strings.Builder
	for _, ws := range msg.Data.Result.Ws {
		for _, cw := range ws.Cw {
			sb.WriteString(cw.W)
		}
	}
	text := sb.String()
	c.logger.Info("xfyun-iat result", "text", text, "isFinal", isFinal, "pgs", msg.Data.Result.Pgs, "sn", msg.Data.Result.Sn)

	if isFinal && text == "" {
		c.finalOnce.Do(func() { close(c.final) })
		return
	}
	if text == "" {
		return
	}

	// Handle dynamic correction (dwa=wpgs) using segment map
	if msg.Data.Result.Pgs == "rpl" && len(msg.Data.Result.Rg) >= 2 {
		c.textMu.Lock()
		for i := msg.Data.Result.Rg[0]; i <= msg.Data.Result.Rg[1]; i++ {
			delete(c.segments, i)
		}
		c.segments[msg.Data.Result.Sn] = text
		c.lastText = c.buildSegmentText()
		full := c.lastText
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}
	} else if msg.Data.Result.Pgs == "apd" {
		c.textMu.Lock()
		c.segments[msg.Data.Result.Sn] = text
		c.lastText = c.buildSegmentText()
		full := c.lastText
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}
	} else {
		// No pgs field: append mode
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

func (c *client) buildSegmentText() string {
	if len(c.segments) == 0 {
		return ""
	}
	maxSn := 0
	for sn := range c.segments {
		if sn > maxSn {
			maxSn = sn
		}
	}
	var sb strings.Builder
	for i := 1; i <= maxSn; i++ {
		if text, ok := c.segments[i]; ok {
			sb.WriteString(text)
		}
	}
	return sb.String()
}
