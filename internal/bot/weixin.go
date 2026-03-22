package bot

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/keepmind9/clibot/internal/proxy"
	"github.com/mdp/qrterminal"
)

// ---------------------------------------------------------------------------
// Protocol Constants
// ---------------------------------------------------------------------------

const (
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

const (
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

const (
	QRStatusWait      = "wait"
	QRStatusScaned    = "scaned"
	QRStatusConfirmed = "confirmed"
	QRStatusExpired   = "expired"
)

// ---------------------------------------------------------------------------
// Protocol Structs
// ---------------------------------------------------------------------------

// BaseInfo is sent with most API requests.
type weixinBaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

// QRCodeResponse is the response from GET /ilink/bot/get_bot_qrcode.
type weixinQRCodeResponse struct {
	QRCode          string `json:"qrcode"`            // UUID for polling
	QRCodeImgContent string `json:"qrcode_img_content"` // base64 PNG or URL/HTML
}

// QRStatusResponse is the response from GET /ilink/bot/get_qrcode_status.
type weixinQRStatusResponse struct {
	Status      string `json:"status"` // wait, scaned, confirmed, expired
	BotToken    string `json:"bot_token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
}

// GetUpdatesRequest is sent to POST /ilink/bot/getupdates.
type weixinGetUpdatesRequest struct {
	ILinkBotID    string          `json:"ilink_bot_id,omitempty"`
	ILinkUserID   string          `json:"ilink_user_id,omitempty"`
	GetUpdatesBuf string          `json:"get_updates_buf,omitempty"`
	SyncBuf       string          `json:"sync_buf,omitempty"`
	BaseInfo      weixinBaseInfo `json:"base_info"`
}

// InboundMessageItem represents a single item in an inbound message item_list.
type inboundMessageItem struct {
	Type       int                 `json:"type"`
	TextItem   *inboundTextItem    `json:"text_item,omitempty"`
	ImageItem  *inboundMediaItem   `json:"image_item,omitempty"`
	VoiceItem  *inboundMediaItem   `json:"voice_item,omitempty"`
	FileItem   *inboundMediaItem   `json:"file_item,omitempty"`
	VideoItem  *inboundMediaItem   `json:"video_item,omitempty"`
}

type inboundTextItem struct {
	Text string `json:"text"`
}

type inboundMediaItem struct {
	ImageURL string `json:"image_url"`
}

// GetUpdatesResponse is the response from POST /ilink/bot/getupdates.
type weixinGetUpdatesResponse struct {
	Ret                  int               `json:"ret"`
	Msgs                 []inboundMessage  `json:"msgs"`
	GetUpdatesBuf         string            `json:"get_updates_buf"`
	SyncBuf              string            `json:"sync_buf"`
	LongpollingTimeoutMs int               `json:"longpolling_timeout_ms"`
	ErrCode              int               `json:"errcode"`
	ErrMsg               string            `json:"errmsg"`
}

type inboundMessage struct {
	MessageID    json.Number        `json:"message_id"`
	FromUserID  string             `json:"from_user_id"`
	ToUserID    string             `json:"to_user_id"`
	ClientID    string             `json:"client_id"`
	CreateTimeMs int64              `json:"create_time_ms"`
	MessageType int                `json:"message_type"`
	MessageState int               `json:"message_state"`
	ContextToken string            `json:"context_token"`
	ItemList    []inboundMessageItem `json:"item_list"`
}

// SendMessageBody is sent to POST /ilink/bot/sendmessage.
type weixinSendMessageBody struct {
	Msg      weixinOutboundMsg `json:"msg"`
	BaseInfo weixinBaseInfo    `json:"base_info"`
}

type weixinOutboundMsg struct {
	FromUserID   string                  `json:"from_user_id"`
	ToUserID     string                  `json:"to_user_id"`
	ClientID     string                  `json:"client_id"`
	MessageType  int                     `json:"message_type"`
	MessageState int                     `json:"message_state"`
	ContextToken string                  `json:"context_token"`
	ItemList     []weixinOutboundItem    `json:"item_list"`
}

type weixinOutboundItem struct {
	Type      int                    `json:"type"`
	TextItem  *outboundTextItem      `json:"text_item,omitempty"`
	ImageItem *outboundImageItem     `json:"image_item,omitempty"`
}

type outboundTextItem struct {
	Text string `json:"text"`
}

type outboundImageItem struct {
	ImageURL string `json:"image_url"`
}

// GetConfigRequest is sent to POST /ilink/bot/getconfig.
type weixinGetConfigRequest struct {
	ILinkUserID  string `json:"ilink_user_id"`
	ContextToken string `json:"context_token"`
	BaseInfo     weixinBaseInfo `json:"base_info"`
}

// GetConfigResponse is the response from POST /ilink/bot/getconfig.
type weixinGetConfigResponse struct {
	TypingTicket string `json:"typing_ticket"`
	Ret          int    `json:"ret"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// SendTypingRequest is sent to POST /ilink/bot/sendtyping.
type weixinSendTypingRequest struct {
	ILinkUserID  string `json:"ilink_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"` // 1=start, 2=stop
	BaseInfo     weixinBaseInfo `json:"base_info"`
}

// ---------------------------------------------------------------------------
// ApiError
// ---------------------------------------------------------------------------

type ApiError struct {
	Status  int
	Code    int
	Message string
}

func (e *ApiError) IsSessionExpired() bool {
	return e.Code == SessionExpiredErrCode
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("weixin api error: status=%d, code=%d, msg=%s", e.Status, e.Code, e.Message)
}

// ---------------------------------------------------------------------------
// Package-level Constants
// ---------------------------------------------------------------------------

const (
	DefaultBaseURL        = "https://ilinkai.weixin.qq.com"
	DefaultBaseVersion    = "1.0.0"
	LongPollTimeout       = 40 * time.Second
	QRCodePollInterval    = 2 * time.Second
	APITimeout            = 15 * time.Second
	MaxMessageLength      = 2000
	MaxChunkLength        = 2000
	SessionExpiredErrCode = -14
)

// ---------------------------------------------------------------------------
// Credentials Management
// ---------------------------------------------------------------------------

type Credentials struct {
	Token     string `json:"token"`
	BaseURL   string `json:"base_url"`
	AccountID string `json:"account_id"`
	UserID    string `json:"user_id"`
}

func DefaultCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".clibot", "weixin", "credentials.json")
}

func loadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse credentials JSON: %w", err)
	}

	creds := &Credentials{}
	for k, v := range raw {
		var val string
		if err := json.Unmarshal(v, &val); err != nil {
			continue
		}
		switch strings.ToLower(k) {
		case "token", "bot_token":
			creds.Token = val
		case "base_url", "baseurl":
			creds.BaseURL = val
		case "account_id", "accountid", "ilink_bot_id":
			creds.AccountID = val
		case "user_id", "userid", "ilink_user_id":
			creds.UserID = val
		}
	}

	if creds.Token == "" {
		return nil, errors.New("credentials file missing token")
	}
	return creds, nil
}

func saveCredentials(path string, creds *Credentials) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create credentials directory: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write credentials file: %w", err)
	}
	return nil
}

func clearCredentials(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials file: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// API Client
// ---------------------------------------------------------------------------

func randomWechatUIN() string {
	var b [4]byte
	rand.Read(b[:])
	return base64.StdEncoding.EncodeToString(b[:])
}

// buildAuthHeaders returns headers for authenticated POST API calls.
func buildAuthHeaders(token string) map[string]string {
	return map[string]string{
		"Content-Type":       "application/json",
		"AuthorizationType":  "ilink_bot_token",
		"Authorization":      "Bearer " + token,
		"X-WECHAT-UIN":       randomWechatUIN(),
	}
}

func buildClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// fetchQRCode requests a new QR code via GET /ilink/bot/get_bot_qrcode?bot_type=3.
func fetchQRCode(client *http.Client, baseURL string) (*weixinQRCodeResponse, error) {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/get_bot_qrcode?bot_type=3"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("iLink-App-ClientVersion", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ApiError{Status: resp.StatusCode, Message: "unexpected status"}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result weixinQRCodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// pollQRStatus polls via GET /ilink/bot/get_qrcode_status?qrcode=<uuid>.
func pollQRStatus(client *http.Client, baseURL, qrUUID string) (*weixinQRStatusResponse, error) {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/get_qrcode_status?" +
		url.Values{"qrcode": {qrUUID}}.Encode()

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("iLink-App-ClientVersion", "1")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result weixinQRStatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// getUpdates long-polls for new messages.
func getUpdates(client *http.Client, baseURL, token, botID, userID, buf, lastSyncBuf string) (*weixinGetUpdatesResponse, error) {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/getupdates"
	reqBody := weixinGetUpdatesRequest{
		ILinkBotID:    botID,
		ILinkUserID:   userID,
		GetUpdatesBuf: buf,
		SyncBuf:       lastSyncBuf,
		BaseInfo:      weixinBaseInfo{ChannelVersion: DefaultBaseVersion},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}
	fmt.Printf("WeChat getUpdates req: %s\n", string(body))

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range buildAuthHeaders(token) {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result weixinGetUpdatesResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrCode != 0 && result.ErrCode != SessionExpiredErrCode {
		return &result, &ApiError{Code: result.ErrCode, Message: result.ErrMsg}
	}
	return &result, nil
}

// ackUpdates sends an acknowledgment to the server to confirm message receipt.
// This prevents the server from re-delivering messages on the next long-poll.
func ackUpdates(client *http.Client, baseURL, token, botID, userID, syncBuf, getUpdatesBuf string) error {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/getupdates"
	reqBody := weixinGetUpdatesRequest{
		ILinkBotID:    botID,
		ILinkUserID:   userID,
		GetUpdatesBuf: getUpdatesBuf,
		SyncBuf:       syncBuf,
		BaseInfo:      weixinBaseInfo{ChannelVersion: DefaultBaseVersion},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	for k, v := range buildAuthHeaders(token) {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // discard body
	return nil
}

// sendMessage sends an outbound message.
func sendMessage(client *http.Client, baseURL, token string, body weixinSendMessageBody) error {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/sendmessage"

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	for k, v := range buildAuthHeaders(token) {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	var result struct {
		Ret     int    `json:"ret"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if result.ErrCode != 0 {
		return &ApiError{Code: result.ErrCode, Message: result.ErrMsg}
	}
	return nil
}

// getConfig retrieves the typing ticket.
func getConfig(client *http.Client, baseURL, token, userID, contextToken string) (*weixinGetConfigResponse, error) {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/getconfig"
	reqBody := weixinGetConfigRequest{
		ILinkUserID:  userID,
		ContextToken: contextToken,
		BaseInfo:     weixinBaseInfo{ChannelVersion: DefaultBaseVersion},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range buildAuthHeaders(token) {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result weixinGetConfigResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// sendTyping sends a typing indicator.
func sendTyping(client *http.Client, baseURL, token, userID, ticket string, status int) error {
	u := strings.TrimSuffix(baseURL, "/") + "/ilink/bot/sendtyping"
	reqBody := weixinSendTypingRequest{
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
		BaseInfo:     weixinBaseInfo{ChannelVersion: DefaultBaseVersion},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	for k, v := range buildAuthHeaders(token) {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// ---------------------------------------------------------------------------
// QR ASCII Renderer
// ---------------------------------------------------------------------------

func printASCIIQR(pngData []byte) {
	r := bytes.NewReader(pngData)

	sig := make([]byte, 8)
	if _, err := io.ReadFull(r, sig); err != nil || !bytes.Equal(sig, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		fmt.Println("(QR code image not renderable, please use a QR scanner app)")
		return
	}

	var width, height int32
	var idatData []byte

	for {
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(r, lenBuf); err != nil {
			break
		}
		length := int(int32(lenBuf[0])<<24 | int32(lenBuf[1])<<16 | int32(lenBuf[2])<<8 | int32(lenBuf[3]))

		typeBuf := make([]byte, 4)
		if _, err := io.ReadFull(r, typeBuf); err != nil {
			break
		}
		chunkType := string(typeBuf)

		data := make([]byte, length)
		if _, err := io.ReadFull(r, data); err != nil {
			break
		}
		// Skip CRC
		if _, err := io.ReadFull(r, make([]byte, 4)); err != nil {
			break
		}

		if chunkType == "IHDR" && length == 13 {
			width = int32(data[0])<<24 | int32(data[1])<<16 | int32(data[2])<<8 | int32(data[3])
			height = int32(data[4])<<24 | int32(data[5])<<16 | int32(data[6])<<8 | int32(data[7])
		} else if chunkType == "IDAT" {
			idatData = append(idatData, data...)
		} else if chunkType == "IEND" {
			break
		}
	}

	if width <= 0 || height <= 0 || len(idatData) == 0 {
		fmt.Println("(QR code image not renderable, please use a QR scanner app)")
		return
	}

	zr, err := zlib.NewReader(bytes.NewReader(idatData))
	if err != nil {
		fmt.Println("(QR code image not renderable, please use a QR scanner app)")
		return
	}
	defer zr.Close()

	raw, err := io.ReadAll(zr)
	if err != nil {
		fmt.Println("(QR code image not renderable, please use a QR scanner app)")
		return
	}

	// PNG rows have a filter byte (0) at the start of each scanline
	bytesPerPixel := 3
	rowStride := int(width)*bytesPerPixel + 1
	if len(raw) < int(height)*rowStride {
		bytesPerPixel = 4
		rowStride = int(width)*bytesPerPixel + 1
		if len(raw) < int(height)*rowStride {
			fmt.Println("(QR code image not renderable, please use a QR scanner app)")
			return
		}
	}

	scaleX := 2
	charsPerRow := int(width) / scaleX

	fmt.Println("┌" + strings.Repeat("─", charsPerRow) + "┐")
	var sb strings.Builder
	sb.Grow(charsPerRow + 2)
	for y := 0; y < int(height); y += 2 {
		row := raw[y*rowStride+1 : y*rowStride+1+int(width)*bytesPerPixel]
		sb.Reset()
		sb.WriteString("│")
		for x := 0; x < int(width); x += scaleX {
			idx := x * bytesPerPixel
			if idx+2 >= len(row) {
				sb.WriteRune('░')
				continue
			}
			r2, g, b := row[idx], row[idx+1], row[idx+2]
			if (int(r2)+int(g)+int(b))/3 < 128 {
				sb.WriteRune('▓')
			} else {
				sb.WriteRune('░')
			}
		}
		sb.WriteString("│")
		fmt.Println(sb.String())
	}
	fmt.Println("└" + strings.Repeat("─", charsPerRow) + "┘")
	fmt.Println("  Please scan the QR code above with WeChat  ")
}

// ---------------------------------------------------------------------------
// WeixinBot
// ---------------------------------------------------------------------------

type WeixinBot struct {
	DefaultTypingIndicator

	// sessionMu protects contextTokens, clientToUser, seenMsgs accessed by handleMessage.
	// cursor and lastSyncBuf are only accessed by longPollLoop (single goroutine).
	// httpClient may be accessed by both longPollLoop and SetProxyManager (called before polling).
	sessionMu sync.RWMutex
	baseURL         string
	credentialsPath string
	credentials     *Credentials
	cursor          string // last GetUpdatesBuf for next request
	lastSyncBuf     string // last SyncBuf for acknowledgment

	contextTokens map[string]string
	clientToUser  map[string]string
	seenMsgs      map[string]bool // deduplication by message_id

	httpClient     *http.Client
	messageHandler func(BotMessage)
	ctx            context.Context
	cancel         context.CancelFunc
	proxyMgr       proxy.Manager
}

func NewWeixinBot(baseURL, credentialsPath string) *WeixinBot {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if credentialsPath == "" {
		credentialsPath = DefaultCredentialsPath()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &WeixinBot{
		baseURL:         baseURL,
		credentialsPath: credentialsPath,
		contextTokens:   make(map[string]string),
		clientToUser:    make(map[string]string),
		seenMsgs:        make(map[string]bool),
		httpClient:      buildClient(APITimeout),
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (b *WeixinBot) SetProxyManager(mgr proxy.Manager) {
	b.proxyMgr = mgr
	if mgr != nil {
		cl, err := mgr.GetHTTPClient("weixin")
		if err == nil && cl != nil {
			b.httpClient = cl
			return
		}
	}
	b.httpClient = buildClient(APITimeout)
}

func (b *WeixinBot) Start(messageHandler func(BotMessage)) error {
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.messageHandler = messageHandler

	creds, err := loadCredentials(b.credentialsPath)
	if err == nil && creds.Token != "" {
		b.credentials = creds
		if creds.BaseURL != "" {
			b.baseURL = creds.BaseURL
		}
	} else {
		if err := b.doQRLogin(); err != nil {
			return fmt.Errorf("QR login failed: %w", err)
		}
	}

	go b.longPollLoop()
	return nil
}

func (b *WeixinBot) doQRLogin() error {
	client := buildClient(APITimeout)

	qrResp, err := fetchQRCode(client, b.baseURL)
	if err != nil {
		return fmt.Errorf("fetch QR code: %w", err)
	}

	fmt.Println("QR code UUID:", qrResp.QRCode)

	if qrResp.QRCodeImgContent != "" {
		// QRCodeImgContent can be either base64 PNG image data or a URL
		if strings.HasPrefix(qrResp.QRCodeImgContent, "http") {
			// It's a URL - use qrterminal to render ASCII QR code
			fmt.Println("Scan the QR code below with WeChat:")
			qrterminal.GenerateHalfBlock(qrResp.QRCodeImgContent, qrterminal.M, os.Stdout)
		} else {
			// It's base64 PNG - decode and render ASCII QR
			imgData, err := base64.StdEncoding.DecodeString(qrResp.QRCodeImgContent)
			if err == nil {
				printASCIIQR(imgData)
			}
		}
	}

	pollClient := buildClient(APITimeout)
	for {
		select {
		case <-b.ctx.Done():
			return errors.New("login cancelled")
		case <-time.After(QRCodePollInterval):
		}

		statusResp, err := pollQRStatus(pollClient, b.baseURL, qrResp.QRCode)
		if err != nil {
			continue
		}

		switch statusResp.Status {
		case QRStatusScaned:
			fmt.Println("QR code scanned. Confirm the login inside WeChat.")
		case QRStatusConfirmed:
			creds := &Credentials{
				Token:     statusResp.BotToken,
				BaseURL:   statusResp.BaseURL,
				AccountID: statusResp.ILinkBotID,
				UserID:    statusResp.ILinkUserID,
			}
			if err := saveCredentials(b.credentialsPath, creds); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			b.sessionMu.Lock()
			b.credentials = creds
			if creds.BaseURL != "" {
				b.baseURL = creds.BaseURL
			}
			b.sessionMu.Unlock()
			fmt.Println("WeChat login successful!")
			return nil
		case QRStatusExpired:
			qrResp, err = fetchQRCode(client, b.baseURL)
			if err != nil {
				return fmt.Errorf("fetch new QR code: %w", err)
			}
			if qrResp.QRCodeImgContent != "" {
				imgData, err := base64.StdEncoding.DecodeString(qrResp.QRCodeImgContent)
				if err == nil {
					printASCIIQR(imgData)
				}
			}
		}
	}
}

func (b *WeixinBot) longPollLoop() {
	backoff := 1 * time.Second
	maxBackoff := 10 * time.Second
	lastHeartbeat := time.Now()

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		// Credentials are set once before this goroutine starts; no mutex needed.
		token := b.credentials.Token
		botID := b.credentials.AccountID
		userID := b.credentials.UserID
		cursor := b.cursor
		lastSyncBuf := b.lastSyncBuf

		if token == "" {
			time.Sleep(backoff)
			continue
		}

		client := &http.Client{Timeout: LongPollTimeout}

		result, err := getUpdates(client, b.baseURL, token, botID, userID, cursor, lastSyncBuf)
		if err != nil {
			var apiErr *ApiError
			if errors.As(err, &apiErr) && apiErr.IsSessionExpired() {
				fmt.Println("WeChat session expired, re-authenticating...")
				b.sessionMu.Lock()
				b.contextTokens = make(map[string]string)
				b.clientToUser = make(map[string]string)
				b.sessionMu.Unlock()
				b.credentials = nil
				b.cursor = ""
				b.lastSyncBuf = ""
				if err := b.doQRLogin(); err != nil {
					fmt.Printf("Re-login failed: %v\n", err)
				}
				backoff = 1 * time.Second
				continue
			}

			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = 1 * time.Second
		if len(result.Msgs) == 0 {
			if time.Since(lastHeartbeat) >= 30*time.Second {
				fmt.Println("WeChat: polling... (idle, token ready)")
				lastHeartbeat = time.Now()
			}
		} else {
			lastHeartbeat = time.Now()
		}

		if len(result.Msgs) > 0 {
			fmt.Printf("WeChat: received %d messages\n", len(result.Msgs))

			// Update cursors BEFORE processing or acking.
			// No mutex needed: only this goroutine writes to cursors.
			if result.GetUpdatesBuf != "" {
				b.cursor = result.GetUpdatesBuf
			}
			if result.SyncBuf != "" {
				b.lastSyncBuf = result.SyncBuf
			}

			// Handle messages (with in-process deduplication)
			for _, msg := range result.Msgs {
				b.handleMessage(msg)
			}

			// Send acknowledgment in background with retry.
			go func(syncBuf, getBuf string) {
				for i := 0; i < 3; i++ {
					ackClient := &http.Client{Timeout: 5 * time.Second}
					if err := ackUpdates(ackClient, b.baseURL, token, botID, userID, syncBuf, getBuf); err == nil {
						return
					}
					time.Sleep(200 * time.Millisecond)
				}
			}(result.SyncBuf, result.GetUpdatesBuf)
		} else {
			// No messages: still update cursors for the next poll
			if result.GetUpdatesBuf != "" {
				b.cursor = result.GetUpdatesBuf
			}
			if result.SyncBuf != "" {
				b.lastSyncBuf = result.SyncBuf
			}
		}
	}
}

func (b *WeixinBot) handleMessage(msg inboundMessage) {
	if msg.MessageType != MessageTypeUser {
		return
	}
	if msg.MessageState == MessageStateGenerating {
		return
	}

	// Deduplicate by message_id: the server re-delivers messages until explicitly acked.
	// Since the cursor mechanism doesn't prevent re-delivery, we dedupe in-memory.
	msgKey := msg.MessageID.String()
	b.sessionMu.Lock()
	if b.seenMsgs[msgKey] {
		b.sessionMu.Unlock()
		return
	}
	b.seenMsgs[msgKey] = true
	// Prune old entries to keep the map bounded.
	if len(b.seenMsgs) > 1000 {
		for k := range b.seenMsgs {
			delete(b.seenMsgs, k)
			break
		}
	}
	b.contextTokens[msg.FromUserID] = msg.ContextToken
	b.clientToUser[msg.ClientID] = msg.FromUserID
	b.sessionMu.Unlock()

	text := extractInboundText(msg.ItemList)
	if text == "" {
		return
	}

	b.sessionMu.Lock()
	handler := b.messageHandler
	b.sessionMu.Unlock()

	if handler == nil {
		return
	}

	handler(BotMessage{
		Platform:  "weixin",
		UserID:    msg.FromUserID,
		Channel:   msg.FromUserID,
		MessageID: msg.ClientID,
		Content:   text,
		Timestamp: time.Unix(msg.CreateTimeMs/1000, 0),
	})
}

func extractInboundText(items []inboundMessageItem) string {
	var parts []string
	for _, item := range items {
		switch item.Type {
		case MessageItemTypeText:
			if item.TextItem != nil && item.TextItem.Text != "" {
				parts = append(parts, item.TextItem.Text)
			}
		case MessageItemTypeImage:
			if item.ImageItem != nil && item.ImageItem.ImageURL != "" {
				parts = append(parts, "[image]")
			}
		case MessageItemTypeVoice:
			if item.VoiceItem != nil && item.VoiceItem.ImageURL != "" {
				parts = append(parts, "[voice]")
			}
		case MessageItemTypeFile:
			if item.FileItem != nil && item.FileItem.ImageURL != "" {
				parts = append(parts, "[file]")
			}
		case MessageItemTypeVideo:
			if item.VideoItem != nil && item.VideoItem.ImageURL != "" {
				parts = append(parts, "[video]")
			}
		}
	}
	return strings.Join(parts, "")
}

func chunkMessage(msg string, maxLen int) []string {
	if len(msg) <= maxLen {
		return []string{msg}
	}
	var chunks []string
	for i := 0; i < len(msg); i += maxLen {
		end := i + maxLen
		if end > len(msg) {
			end = len(msg)
		}
		chunks = append(chunks, msg[i:end])
	}
	return chunks
}

func (b *WeixinBot) SendMessage(channel, message string) error {
	b.sessionMu.RLock()
	contextToken, ok := b.contextTokens[channel]
	if !ok {
		b.sessionMu.RUnlock()
		return errors.New("no context_token found for user, message may be out of context")
	}
	token := b.credentials.Token
	baseURL := b.baseURL
	b.sessionMu.RUnlock()

	chunks := chunkMessage(message, MaxChunkLength)
	client := &http.Client{Timeout: APITimeout}

	for _, chunk := range chunks {
		body := weixinSendMessageBody{
			Msg: weixinOutboundMsg{
				FromUserID:   "",
				ToUserID:     channel,
				ClientID:     uuid.New().String(),
				MessageType:  MessageTypeBot,
				MessageState: MessageStateFinish,
				ContextToken: contextToken,
				ItemList: []weixinOutboundItem{
					{
						Type:     MessageItemTypeText,
						TextItem: &outboundTextItem{Text: chunk},
					},
				},
			},
			BaseInfo: weixinBaseInfo{ChannelVersion: DefaultBaseVersion},
		}
		if err := sendMessage(client, baseURL, token, body); err != nil {
			return fmt.Errorf("send message chunk: %w", err)
		}
	}
	return nil
}

func (b *WeixinBot) AddTypingIndicator(messageID string) bool {
	b.sessionMu.RLock()
	userID, ok := b.clientToUser[messageID]
	if !ok {
		b.sessionMu.RUnlock()
		return false
	}
	contextToken, ok := b.contextTokens[userID]
	if !ok {
		b.sessionMu.RUnlock()
		return false
	}
	token := b.credentials.Token
	baseURL := b.baseURL
	b.sessionMu.RUnlock()

	client := &http.Client{Timeout: APITimeout}
	cfg, err := getConfig(client, baseURL, token, userID, contextToken)
	if err != nil {
		return false
	}
	if cfg.TypingTicket == "" {
		return false
	}
	if err := sendTyping(client, baseURL, token, userID, cfg.TypingTicket, 1); err != nil {
		return false
	}
	return true
}

func (b *WeixinBot) RemoveTypingIndicator(messageID string) error {
	b.sessionMu.RLock()
	userID, ok := b.clientToUser[messageID]
	if !ok {
		b.sessionMu.RUnlock()
		return nil
	}
	contextToken, ok := b.contextTokens[userID]
	if !ok {
		b.sessionMu.RUnlock()
		return nil
	}
	token := b.credentials.Token
	baseURL := b.baseURL
	b.sessionMu.RUnlock()

	client := &http.Client{Timeout: APITimeout}
	cfg, err := getConfig(client, baseURL, token, userID, contextToken)
	if err != nil {
		return err
	}
	if cfg.TypingTicket == "" {
		return nil
	}
	return sendTyping(client, baseURL, token, userID, cfg.TypingTicket, 2)
}

func (b *WeixinBot) Stop() error {
	b.cancel()
	b.sessionMu.Lock()
	b.contextTokens = make(map[string]string)
	b.clientToUser = make(map[string]string)
	b.seenMsgs = make(map[string]bool)
	b.credentials = nil
	b.cursor = ""
	b.lastSyncBuf = ""
	b.sessionMu.Unlock()
	return nil
}
