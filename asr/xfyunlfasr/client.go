package xfyunlfasr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/asr"
)

func init() {
	lfasrFields := []asr.FieldDef{
		{Key: "app_id", Label: "App ID", Help: "讯飞 App ID", Type: asr.FieldText},
		{Key: "api_key", Label: "API Key", Help: "讯飞 API Key / AccessKeyId", Type: asr.FieldSecret, Secret: true},
		{Key: "api_secret", Label: "API Secret", Help: "讯飞 API Secret / SecretKey", Type: asr.FieldSecret, Secret: true},
		{Key: "pd", Label: "领域", Help: "可选: court/edu/finance/medical/tech/sport", Type: asr.FieldText},
	}
	asr.RegisterWithMeta("xfyun-lfasr", NewStandard, asr.ProviderMeta{
		DisplayName: "讯飞录音转写",
		Fields:      lfasrFields,
	})
	asr.RegisterWithMeta("xfyun-lfasr-llm", NewLargeModel, asr.ProviderMeta{
		DisplayName: "讯飞转写大模型",
		Fields: []asr.FieldDef{
			{Key: "app_id", Label: "App ID", Help: "讯飞 App ID", Type: asr.FieldText},
			{Key: "api_key", Label: "AccessKeyId", Help: "讯飞 AccessKeyId", Type: asr.FieldSecret, Secret: true},
			{Key: "api_secret", Label: "AccessKeySecret", Help: "讯飞 AccessKeySecret", Type: asr.FieldSecret, Secret: true},
			{Key: "pd", Label: "领域", Help: "可选: court/edu/finance/medical/tech/sport", Type: asr.FieldText},
		},
	})
	asr.RegisterWithMeta("xfyun-lfasr-fast", NewFast, asr.ProviderMeta{
		DisplayName: "讯飞极速转写",
		Fields:      lfasrFields,
	})
}

// variant determines which API path to use.
type variant int

const (
	variantStandard   variant = iota // raasr.xfyun.cn, HMAC-SHA1(MD5(appId+ts), secretKey)
	variantLargeModel                // office-api-ist-dx.iflyaisol.com, HMAC-SHA1 signature
	variantFast                      // upload-ost-api.xfyun.cn + ost-api.xfyun.cn, HMAC-SHA256
)

type client struct {
	common    asr.Common
	variant   variant
	appID     string
	apiKey    string // APIKey or accessKeyId
	apiSecret string // APISecret or accessKeySecret or secretKey
	language  string
	pd        string // domain parameter
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

func newClient(v variant, common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	appID, _ := providerCfg["app_id"].(string)
	apiKey, _ := providerCfg["api_key"].(string)
	apiSecret, _ := providerCfg["api_secret"].(string)
	if appID == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("xfyun-lfasr: app_id, api_key, api_secret are required")
	}
	language, _ := providerCfg["language"].(string)
	if language == "" {
		language = "cn"
	}
	pd, _ := providerCfg["pd"].(string)
	return &client{
		common: common, variant: v,
		appID: appID, apiKey: apiKey, apiSecret: apiSecret,
		language: language, pd: pd, logger: logger,
		resultCh: make(chan asr.Result, 64),
		done:     make(chan struct{}),
		final:    make(chan struct{}),
	}, nil
}

func NewStandard(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	return newClient(variantStandard, common, providerCfg, logger)
}

func NewLargeModel(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	return newClient(variantLargeModel, common, providerCfg, logger)
}

func NewFast(common asr.Common, providerCfg map[string]interface{}, logger *slog.Logger) (asr.Client, error) {
	return newClient(variantFast, common, providerCfg, logger)
}

func (c *client) Connect(_ context.Context) error {
	c.logger.Info("iFlytek LFASR ready", "variant", c.variant, "app_id", c.appID)
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

func (c *client) Close() error { return nil }

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

	// Convert PCM to WAV for upload
	wav := pcmToWAV(pcm, 16000, 1)
	durationMs := len(pcm) / 32 // 16kHz * 2bytes = 32000 bytes/sec

	c.logger.Info("uploading audio to iFlytek LFASR", "bytes", len(wav), "duration_ms", durationMs)

	var text string
	var err error
	switch c.variant {
	case variantStandard:
		text, err = c.transcribeStandard(wav, durationMs)
	case variantLargeModel:
		text, err = c.transcribeLargeModel(wav, durationMs)
	case variantFast:
		text, err = c.transcribeFast(wav, durationMs)
	}

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

// ─── Standard (raasr.xfyun.cn) ───────────────────────────────────────────

func (c *client) transcribeStandard(wav []byte, durationMs int) (string, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	signa := c.standardSigna(ts)

	fileName := "audio.wav"
	uploadURL := fmt.Sprintf("https://raasr.xfyun.cn/v2/api/upload?duration=%d&signa=%s&fileName=%s&fileSize=%d&appId=%s&ts=%s&language=%s&standardWav=1",
		durationMs/1000, url.QueryEscape(signa), url.QueryEscape(fileName), len(wav), c.appID, ts, c.language)
	if c.pd != "" {
		uploadURL += "&pd=" + url.QueryEscape(c.pd)
	}
	if len(c.common.Hotwords) > 0 {
		uploadURL += "&hotWord=" + url.QueryEscape(strings.Join(c.common.Hotwords, "|"))
	}

	// Upload
	body := bytes.NewReader(wav)
	req, _ := http.NewRequest("POST", uploadURL, body)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("lfasr upload: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var uploadResp struct {
		Code    string `json:"code"`
		DescInfo string `json:"descInfo"`
		Content struct {
			OrderID string `json:"orderId"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("lfasr upload response: %w", err)
	}
	if uploadResp.Code != "000000" {
		return "", fmt.Errorf("lfasr upload error %s: %s", uploadResp.Code, uploadResp.DescInfo)
	}
	orderID := uploadResp.Content.OrderID
	c.logger.Info("lfasr standard upload success", "orderId", orderID)

	// Poll for result
	return c.pollStandard(orderID)
}

func (c *client) pollStandard(orderID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("lfasr poll timeout")
		case <-ticker.C:
			ts := fmt.Sprintf("%d", time.Now().Unix())
			signa := c.standardSigna(ts)
			queryURL := fmt.Sprintf("https://raasr.xfyun.cn/v2/api/getResult?signa=%s&orderId=%s&appId=%s&ts=%s",
				url.QueryEscape(signa), url.QueryEscape(orderID), c.appID, ts)

			req, _ := http.NewRequest("POST", queryURL, nil)
			req.Header.Set("Content-Type", "multipart/form-data")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				c.logger.Warn("lfasr poll error", "error", err)
				continue
			}
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var queryResp struct {
				Code    string `json:"code"`
				Content struct {
					OrderInfo struct {
						Status int `json:"status"`
					} `json:"orderInfo"`
					OrderResult string `json:"orderResult"`
				} `json:"content"`
			}
			if err := json.Unmarshal(respBody, &queryResp); err != nil {
				c.logger.Warn("lfasr poll parse error", "error", err)
				continue
			}
			if queryResp.Code != "000000" {
				continue
			}
			// status: 0=created, 3=processing, 4=completed, -1=failed
			switch queryResp.Content.OrderInfo.Status {
			case 4:
				return c.parseLatticeResult(queryResp.Content.OrderResult)
			case -1:
				return "", fmt.Errorf("lfasr transcribe failed")
			}
			c.logger.Info("lfasr polling...", "status", queryResp.Content.OrderInfo.Status)
		}
	}
}

func (c *client) standardSigna(ts string) string {
	baseString := c.appID + ts
	md5Hash := fmt.Sprintf("%x", md5.Sum([]byte(baseString)))
	mac := hmac.New(sha1.New, []byte(c.apiSecret))
	mac.Write([]byte(md5Hash))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ─── Large Model (office-api-ist-dx.iflyaisol.com) ───────────────────────

func (c *client) transcribeLargeModel(wav []byte, durationMs int) (string, error) {
	ts := time.Now().Format("2006-01-02T15:04:05+0800")
	randomStr := randomString(16)

	params := map[string]string{
		"accessKeyId":   c.apiKey,
		"dateTime":      ts,
		"duration":      fmt.Sprintf("%d", durationMs),
		"fileName":      "audio.wav",
		"fileSize":      fmt.Sprintf("%d", len(wav)),
		"language":      c.language,
		"signatureRandom": randomStr,
		"appId":         c.appID,
	}
	if c.pd != "" {
		params["pd"] = c.pd
	}

	signature := c.largeModelSignature(params)
	params["signature"] = signature

	// Build URL
	uploadURL := "https://office-api-ist-dx.iflyaisol.com/v2/upload?" + encodeParams(params)

	body := bytes.NewReader(wav)
	req, _ := http.NewRequest("POST", uploadURL, body)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("lfasr-llm upload: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var uploadResp struct {
		Code    string `json:"code"`
		DescInfo string `json:"descInfo"`
		Content struct {
			OrderID string `json:"orderId"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("lfasr-llm upload response: %w", err)
	}
	if uploadResp.Code != "000000" {
		return "", fmt.Errorf("lfasr-llm upload error %s: %s", uploadResp.Code, uploadResp.DescInfo)
	}
	orderID := uploadResp.Content.OrderID
	c.logger.Info("lfasr-llm upload success", "orderId", orderID)

	return c.pollLargeModel(orderID)
}

func (c *client) pollLargeModel(orderID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("lfasr-llm poll timeout")
		case <-ticker.C:
			ts := time.Now().Format("2006-01-02T15:04:05+0800")
			randomStr := randomString(16)

			params := map[string]string{
				"accessKeyId":   c.apiKey,
				"dateTime":      ts,
				"orderId":       orderID,
				"resultType":    "transfer",
				"signatureRandom": randomStr,
			}
			signature := c.largeModelSignature(params)
			params["signature"] = signature

			queryURL := "https://office-api-ist-dx.iflyaisol.com/v2/getResult?" + encodeParams(params)

			req, _ := http.NewRequest("POST", queryURL, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("signature", signature)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				c.logger.Warn("lfasr-llm poll error", "error", err)
				continue
			}
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var queryResp struct {
				Code    string `json:"code"`
				Content struct {
					OrderInfo struct {
						Status int `json:"status"`
					} `json:"orderInfo"`
					OrderResult string `json:"orderResult"`
				} `json:"content"`
			}
			if err := json.Unmarshal(respBody, &queryResp); err != nil {
				c.logger.Warn("lfasr-llm poll parse error", "error", err)
				continue
			}
			if queryResp.Code != "000000" {
				continue
			}
			switch queryResp.Content.OrderInfo.Status {
			case 4:
				return c.parseLatticeResult(queryResp.Content.OrderResult)
			case -1:
				return "", fmt.Errorf("lfasr-llm transcribe failed")
			}
			c.logger.Info("lfasr-llm polling...", "status", queryResp.Content.OrderInfo.Status)
		}
	}
}

func (c *client) largeModelSignature(params map[string]string) string {
	// Sort params, exclude "signature"
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	// Natural sort (same as Java TreeMap)
	sortStrings(keys)

	var sb strings.Builder
	for i, k := range keys {
		v := params[k]
		if v == "" {
			continue
		}
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(url.QueryEscape(v))
	}
	baseString := sb.String()

	mac := hmac.New(sha1.New, []byte(c.apiSecret))
	mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ─── Fast (upload-ost-api.xfyun.cn + ost-api.xfyun.cn) ───────────────────

func (c *client) transcribeFast(wav []byte, durationMs int) (string, error) {
	// Step 1: Upload small file (< 30MB)
	fileURL, err := c.fastUpload(wav)
	if err != nil {
		return "", err
	}

	// Step 2: Create transcription task
	taskID, err := c.fastCreateTask(fileURL, len(wav), durationMs/1000)
	if err != nil {
		return "", err
	}

	// Step 3: Poll for result
	return c.fastPoll(taskID)
}

func (c *client) fastUpload(wav []byte) (string, error) {
	host := "upload-ost-api.xfyun.cn"
	date := time.Now().UTC().Format(time.RFC1123)
	requestLine := "POST /file/upload HTTP/1.1"

	// Build multipart body
	var bodyBuf bytes.Buffer
	w := multipart.NewWriter(&bodyBuf)
	boundary := w.Boundary()

	// Write file field
	fw, _ := w.CreateFormFile("data", "audio.wav")
	fw.Write(wav)
	// Write app_id
	w.WriteField("app_id", c.appID)
	// Write request_id
	w.WriteField("request_id", fmt.Sprintf("%d", time.Now().UnixNano()))
	w.Close()

	bodyBytes := bodyBuf.Bytes()

	// Compute digest
	digestHash := sha256.Sum256(bodyBytes)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestHash[:])

	// Compute signature
	signOrigin := fmt.Sprintf("host: %s\ndate: %s\n%s\ndigest: %s", host, date, requestLine, digest)
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(signOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	authOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line digest", signature="%s"`, c.apiKey, signature)

	uploadURL := fmt.Sprintf("https://%s/file/upload", host)
	req, _ := http.NewRequest("POST", uploadURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Host", host)
	req.Header.Set("Date", date)
	req.Header.Set("Digest", digest)
	req.Header.Set("Authorization", authOrigin)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("lfasr-fast upload: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var uploadResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &uploadResp); err != nil {
		return "", fmt.Errorf("lfasr-fast upload response: %w", err)
	}
	if uploadResp.Code != 0 {
		return "", fmt.Errorf("lfasr-fast upload error %d: %s", uploadResp.Code, uploadResp.Message)
	}
	c.logger.Info("lfasr-fast upload success")
	return uploadResp.Data.URL, nil
}

func (c *client) fastCreateTask(audioURL string, fileSize, durationSec int) (string, error) {
	host := "ost-api.xfyun.cn"
	date := time.Now().UTC().Format(time.RFC1123)
	requestLine := "POST /v2/ost/pro_create HTTP/1.1"

	taskBody := map[string]interface{}{
		"common": map[string]interface{}{
			"app_id": c.appID,
		},
		"business": map[string]interface{}{
			"request_id": fmt.Sprintf("%d", time.Now().UnixNano()),
			"language":   "zh_cn",
			"domain":     "pro_ost_ed",
			"accent":     "mandarin",
			"duration":   durationSec,
		},
		"data": map[string]interface{}{
			"audio_url":  audioURL,
			"audio_src":  "http",
			"audio_size": fileSize,
			"format":     "audio/L16;rate=16000",
			"encoding":   "raw",
		},
	}
	if c.pd != "" {
		taskBody["business"].(map[string]interface{})["pd"] = c.pd
	}
	if len(c.common.Hotwords) > 0 {
		taskBody["business"].(map[string]interface{})["dhw"] = strings.Join(c.common.Hotwords, ",")
	}

	bodyJSON, _ := json.Marshal(taskBody)
	digestHash := sha256.Sum256(bodyJSON)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestHash[:])

	signOrigin := fmt.Sprintf("host: %s\ndate: %s\n%s\ndigest: %s", host, date, requestLine, digest)
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(signOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line digest", signature="%s"`, c.apiKey, signature)

	createURL := fmt.Sprintf("https://%s/v2/ost/pro_create", host)
	req, _ := http.NewRequest("POST", createURL, bytes.NewReader(bodyJSON))
	req.Header.Set("Host", host)
	req.Header.Set("Date", date)
	req.Header.Set("Digest", digest)
	req.Header.Set("Authorization", authOrigin)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("lfasr-fast create task: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var createResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return "", fmt.Errorf("lfasr-fast create response: %w", err)
	}
	if createResp.Code != 0 {
		return "", fmt.Errorf("lfasr-fast create error %d: %s", createResp.Code, createResp.Message)
	}
	c.logger.Info("lfasr-fast task created", "task_id", createResp.Data.TaskID)
	return createResp.Data.TaskID, nil
}

func (c *client) fastPoll(taskID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("lfasr-fast poll timeout")
		case <-ticker.C:
			text, done, err := c.fastQuery(taskID)
			if err != nil {
				c.logger.Warn("lfasr-fast poll error", "error", err)
				continue
			}
			if done {
				return text, nil
			}
			c.logger.Info("lfasr-fast polling...")
		}
	}
}

func (c *client) fastQuery(taskID string) (string, bool, error) {
	host := "ost-api.xfyun.cn"
	date := time.Now().UTC().Format(time.RFC1123)
	requestLine := "POST /v2/ost/query HTTP/1.1"

	queryBody := map[string]interface{}{
		"common": map[string]interface{}{
			"app_id": c.appID,
		},
		"business": map[string]interface{}{
			"task_id": taskID,
		},
	}
	bodyJSON, _ := json.Marshal(queryBody)
	digestHash := sha256.Sum256(bodyJSON)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestHash[:])

	signOrigin := fmt.Sprintf("host: %s\ndate: %s\n%s\ndigest: %s", host, date, requestLine, digest)
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(signOrigin))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authOrigin := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line digest", signature="%s"`, c.apiKey, signature)

	queryURL := fmt.Sprintf("https://%s/v2/ost/query", host)
	req, _ := http.NewRequest("POST", queryURL, bytes.NewReader(bodyJSON))
	req.Header.Set("Host", host)
	req.Header.Set("Date", date)
	req.Header.Set("Digest", digest)
	req.Header.Set("Authorization", authOrigin)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("lfasr-fast query: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var queryResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status    int    `json:"status"`
			TaskID    string `json:"task_id"`
			ResultStr string `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &queryResp); err != nil {
		return "", false, fmt.Errorf("lfasr-fast query response: %w", err)
	}
	if queryResp.Code != 0 {
		return "", false, fmt.Errorf("lfasr-fast query error %d: %s", queryResp.Code, queryResp.Message)
	}

	// status: 0=created, 1=processing, 2=completed, -1=failed
	switch queryResp.Data.Status {
	case 2:
		text, err := c.parseFastResult(queryResp.Data.ResultStr)
		return text, true, err
	case -1:
		return "", true, fmt.Errorf("lfasr-fast transcribe failed")
	}
	return "", false, nil
}

func (c *client) parseFastResult(resultStr string) (string, error) {
	// Fast transcription result format: JSON with segments
	var result struct {
		Lattice []struct {
			JSON1Best string `json:"json_1best"`
		} `json:"lattice"`
	}
	if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
		// Try plain text
		return resultStr, nil
	}
	return c.extractLatticeText(result.Lattice)
}

// ─── Shared utilities ─────────────────────────────────────────────────────

func (c *client) parseLatticeResult(orderResult string) (string, error) {
	if orderResult == "" {
		return "", nil
	}
	var result struct {
		Lattice []struct {
			JSON1Best string `json:"json_1best"`
		} `json:"lattice"`
	}
	if err := json.Unmarshal([]byte(orderResult), &result); err != nil {
		return "", fmt.Errorf("lfasr parse lattice: %w", err)
	}
	return c.extractLatticeText(result.Lattice)
}

func (c *client) extractLatticeText(lattice []struct {
	JSON1Best string `json:"json_1best"`
}) (string, error) {
	var sb strings.Builder
	for _, l := range lattice {
		var best struct {
			St struct {
				Rt []struct {
					Ws []struct {
						Cw []struct {
							W string `json:"w"`
						} `json:"cw"`
					} `json:"ws"`
				} `json:"rt"`
			} `json:"st"`
		}
		if err := json.Unmarshal([]byte(l.JSON1Best), &best); err != nil {
			continue
		}
		for _, rt := range best.St.Rt {
			for _, ws := range rt.Ws {
				for _, cw := range ws.Cw {
					sb.WriteString(cw.W)
				}
			}
		}
	}
	return sb.String(), nil
}

func encodeParams(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	return vals.Encode()
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
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
