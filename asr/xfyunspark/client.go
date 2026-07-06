package xfyunspark

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
	"os"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
	"github.com/gorilla/websocket"
)

const defaultHost = "iat.xf-yun.com"

func init() {
	asr.Register("xfyun-spark", New)
}

type client struct {
	common    asr.Common
	appID     string
	apiKey    string
	apiSec    string
	host      string
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
	// DC offset removal
	dcOffset int32
	dcCalibrated bool
	// Dynamic correction segment map (sn → text)
	segments map[int]string
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
		segments: make(map[int]string),
	}, nil
}

func (c *client) Connect(ctx context.Context) error {
	wsURL := c.buildAuthURL()
	c.logger.Info("connecting to iFlytek Spark ASR", "host", c.host, "app_id", c.appID)
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

var pcmDumpFile *os.File

func (c *client) SendAudio(ctx context.Context, pcm []byte, isLast bool) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	// iFlytek is sensitive to clipping. Reduce volume to prevent saturation.
	if len(pcm) > 1 {
		adjusted := make([]byte, len(pcm))
		for i := 0; i+1 < len(pcm); i += 2 {
			s := int32(int16(pcm[i]) | int16(pcm[i+1])<<8)
			// Divide by 3 to bring clipped audio into normal range
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
		// If this is the last chunk and isLast, use status=2 (per SDK behavior)
		status := 1
		if c.seq == 1 {
			status = 0
		}
		if isLast && offset >= len(pcm) {
			status = 2 // last frame carries the final audio data
		}

		frame := c.buildFrame(chunk, status)
		data, _ := json.Marshal(frame)
		if c.seq == 1 {
			c.logger.Info("xfyun first frame", "json", string(data[:min(len(data), 500)]))
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return err
		}
		// No delay needed: recorder already produces audio at real-time rate
	}

	// If isLast but no PCM data, send empty last frame
	if isLast && len(pcm) == 0 {
		c.seq++
		frame := c.buildFrame(nil, 2)
		data, _ := json.Marshal(frame)
		c.logger.Info("xfyun last frame (empty)")
		return c.conn.WriteMessage(websocket.TextMessage, data)
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
		// Doc: language is fixed to zh_cn for this model (supports zh + en + 202 dialects)
		parameter := map[string]interface{}{
			"iat": map[string]interface{}{
				"domain":   "slm",
				"language": "zh_cn",
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
	c.logger.Info("xfyun ReceiveLoop started")
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.logger.Info("xfyun ReceiveLoop exited", "error", err)
			return
		}
		c.logger.Info("xfyun received message", "bytes", len(data))
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
		Header struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  int    `json:"status"`
		} `json:"header"`
		Payload *struct {
			Result *struct {
				Text   string `json:"text"`
				Status int    `json:"status"`
			} `json:"result"`
		} `json:"payload"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return
	}
	if msg.Header.Code != 0 {
		errMsg := fmt.Errorf("讯飞错误 %d: %s", msg.Header.Code, msg.Header.Message)
		c.logger.Error("xfyun ASR error", "code", msg.Header.Code, "message", msg.Header.Message)
		c.resultCh <- asr.Result{Error: errMsg}
		return
	}
	if msg.Payload == nil || msg.Payload.Result == nil {
		return
	}

	// Decode the base64 text field to get the actual JSON content
	decoded, err := base64.StdEncoding.DecodeString(msg.Payload.Result.Text)
	if err != nil || len(decoded) == 0 {
		isFinal := msg.Header.Status == 2
		if isFinal {
			c.finalOnce.Do(func() { close(c.final) })
		}
		return
	}

	// Parse the decoded JSON to extract text, pgs, rg, sn
	var textData struct {
		Sn  int `json:"sn"`
		Ws  []struct {
			Cw []struct {
				W string `json:"w"`
			} `json:"cw"`
		} `json:"ws"`
		Pgs string `json:"pgs"`
		Rg  []int `json:"rg"`
	}
	if json.Unmarshal(decoded, &textData) != nil {
		return
	}

	// Build text from ws[].cw[].w
	var sb strings.Builder
	for _, ws := range textData.Ws {
		for _, cw := range ws.Cw {
			sb.WriteString(cw.W)
		}
	}
	text := sb.String()
	isFinal := msg.Header.Status == 2
	c.logger.Info("xfyun result", "text", text, "isFinal", isFinal, "pgs", textData.Pgs, "sn", textData.Sn)

	if isFinal && text == "" {
		c.finalOnce.Do(func() { close(c.final) })
		return
	}
	if text == "" {
		return
	}

	// Handle dynamic correction (dwa=wpgs) using segment map
	// "apd": append new segment; "rpl": replace segments in rg range
	if textData.Pgs == "rpl" && len(textData.Rg) >= 2 {
		c.textMu.Lock()
		// Remove replaced segments
		for i := textData.Rg[0]; i <= textData.Rg[1]; i++ {
			delete(c.segments, i)
		}
		c.segments[textData.Sn] = text
		c.lastText = c.buildSegmentText()
		full := c.lastText
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}
	} else if textData.Pgs == "apd" {
		c.textMu.Lock()
		c.segments[textData.Sn] = text
		c.lastText = c.buildSegmentText()
		full := c.lastText
		c.textMu.Unlock()
		c.resultCh <- asr.Result{Text: full, IsFinal: isFinal}
	} else {
		// No pgs field: append mode (each result is incremental)
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
	// Find max SN to know how many segments to concatenate
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
