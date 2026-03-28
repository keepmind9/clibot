package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/keepmind9/clibot/internal/logger"
	"github.com/keepmind9/clibot/internal/proxy"
)

// QQBot implements BotAdapter interface for QQ Bot Platform (QQ群机器人开放平台)
// Official API: https://bots.qq.com
type QQBot struct {
	DefaultTypingIndicator
	mu             sync.RWMutex
	appID          string
	appSecret      string
	accessToken    string
	tokenExpiresAt time.Time
	wsConn         *websocket.Conn
	gatewayURL     string
	messageHandler func(BotMessage)
	ctx            context.Context
	cancel         context.CancelFunc
	proxyMgr       proxy.Manager
	msgSeqMap      map[string]int // Track message sequences for passive reply
	lastSequence   *int
}

// QQ Bot API endpoints
const (
	QQTokenURL   = "https://bots.qq.com/app/getAppAccessToken"
	QQAPIBase    = "https://api.sgroup.qq.com"
	QQGatewayURL = QQAPIBase + "/gateway"

	// QQ Bot constants
	maxMsgSeqMapSize = 500 // Maximum number of message sequences to track

	// Timeouts
	qqWebSocketHandshakeTimeout = 10 * time.Second // WebSocket handshake timeout
	qqAPIRequestTimeout         = 10 * time.Second // Timeout for token and gateway requests
	qqMessageSendTimeout        = 15 * time.Second // Timeout for sending messages

	// Token management
	qqTokenExpirationBuffer = 60 // Buffer in seconds before token expiration

	// Message limits
	qqMaxMessageLength = 2000 // Maximum message length for QQ (characters)

	// Message types
	qqMessageTypeText = 0 // Text message type

	// Message splitting
	qqSplitMinNewlineIndex = 2 // Minimum index for newline split (maxLen / 2)
)

// WebSocket OP codes (https://bots.qq.com/docs/gateway/gateway-events)
const (
	OPDispatch       = 0  // Event dispatch
	OPHeartbeat      = 1  // Heartbeat
	OPIdentify       = 2  // Identify
	OPResume         = 6  // Resume
	OPReconnect      = 7  // Reconnect request
	OPInvalidSession = 9  // Invalid session
	OPHello          = 10 // Hello
	OPHeartbeatAck   = 11 // Heartbeat acknowledgement
)

// Intents for subscribing to events
const (
	IntentPublicMessages = 1 << 25 // Public message events (1 << 25)
)

// Shard configuration
const (
	qqShardID    = 0 // Shard ID (0 = first shard)
	qqShardTotal = 1 // Total number of shards (1 = no sharding)
)

// GatewayPayload represents a WebSocket gateway message
type GatewayPayload struct {
	OP int         `json:"op"`
	D  interface{} `json:"d,omitempty"`
	S  *int        `json:"s,omitempty"`
	T  string      `json:"t,omitempty"`
}

// C2CMessageData represents C2C (private chat) message data from WebSocket event
type C2CMessageData struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		UserOpenID string `json:"user_openid"`
	} `json:"author"`
	Content string `json:"content"`
}

// HelloData contains heartbeat_interval from OP Hello
type HelloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

// IdentifyData contains identify payload
type IdentifyData struct {
	Token   string `json:"token"`
	Intents int    `json:"intents"`
	Shard   []int  `json:"shard"`
}

// QQTokenResponse represents the token response from QQ API
type QQTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   string `json:"expires_in"` // API returns as string
}

// QQGatewayResponse represents the gateway URL response
type QQGatewayResponse struct {
	URL string `json:"url"`
}

// SendMessageRequest represents the request payload for sending messages
type SendMessageRequest struct {
	Content string `json:"content"`
	MsgType int    `json:"msg_type"`
	MsgID   string `json:"msg_id,omitempty"`
	MsgSeq  int    `json:"msg_seq,omitempty"`
}

// SendMessageResponse represents the response from sending a message
type SendMessageResponse struct {
	ID string `json:"id"`
}

// NewQQBot creates a new QQ bot instance
func NewQQBot(appID, appSecret string) *QQBot {
	return &QQBot{
		appID:     appID,
		appSecret: appSecret,
		msgSeqMap: make(map[string]int),
	}
}

// SetProxyManager sets the proxy manager for the QQ bot
func (q *QQBot) SetProxyManager(mgr proxy.Manager) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.proxyMgr = mgr
}

// SupportsTypingIndicator returns false (QQ doesn't support typing indicators)
func (q *QQBot) SupportsTypingIndicator() bool {
	return false
}

// AddTypingIndicator does nothing (not supported)
func (q *QQBot) AddTypingIndicator(messageID string) bool {
	return false
}

// RemoveTypingIndicator does nothing (not supported)
func (q *QQBot) RemoveTypingIndicator(messageID string) error {
	return nil
}

// nextMsgSeq returns the next message sequence number for tracking passive replies
func (q *QQBot) nextMsgSeq(inboundMsgID string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	seq := q.msgSeqMap[inboundMsgID] + 1
	q.msgSeqMap[inboundMsgID] = seq

	// Prevent unbounded growth
	if len(q.msgSeqMap) > maxMsgSeqMapSize {
		for key := range q.msgSeqMap {
			delete(q.msgSeqMap, key)
			break
		}
	}

	return seq
}

// SetMessageHandler sets the message handler in a thread-safe manner
func (q *QQBot) SetMessageHandler(handler func(BotMessage)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messageHandler = handler
}

// GetMessageHandler gets the message handler in a thread-safe manner
func (q *QQBot) GetMessageHandler() func(BotMessage) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.messageHandler
}

// sendGateway sends a payload to the WebSocket gateway
func (q *QQBot) sendGateway(payload GatewayPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.wsConn == nil {
		return fmt.Errorf("websocket not connected")
	}

	return q.wsConn.WriteMessage(websocket.TextMessage, data)
}

// startHeartbeat starts the heartbeat loop
func (q *QQBot) startHeartbeat(intervalMs int) {
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-q.ctx.Done():
				return
			case <-ticker.C:
				heartbeat := GatewayPayload{OP: OPHeartbeat, D: q.lastSequence}
				if err := q.sendGateway(heartbeat); err != nil {
					logger.Debugf("Heartbeat failed: %v", err)
				}
			}
		}
	}()
}

// Start establishes connection to QQ gateway and begins listening for messages
func (q *QQBot) Start(messageHandler func(BotMessage)) error {
	q.SetMessageHandler(messageHandler)

	logger.Infof("[QQ] Starting...")
	q.ctx, q.cancel = context.WithCancel(context.Background())

	logger.Debugf("[QQ] Fetching access token...")
	token, err := q.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}
	logger.Debugf("[QQ] Access token obtained")

	logger.Debugf("[QQ] Fetching gateway URL...")
	gatewayURL, err := q.getGatewayURL(token)
	if err != nil {
		return fmt.Errorf("get gateway: %w", err)
	}

	q.mu.Lock()
	q.gatewayURL = gatewayURL
	q.mu.Unlock()

	logger.Debugf("[QQ] Gateway URL: %s", gatewayURL)

	if err := q.connectGateway(token); err != nil {
		return fmt.Errorf("connect gateway: %w", err)
	}

	logger.Infof("[QQ] Started")
	return nil
}

// connectGateway establishes WebSocket connection to QQ gateway
func (q *QQBot) connectGateway(token string) error {
	logger.Infof("[QQ] Connecting to gateway: %s", q.gatewayURL)

	// Create dialer with timeout
	dialer := &websocket.Dialer{
		HandshakeTimeout: qqWebSocketHandshakeTimeout,
	}

	ws, _, err := dialer.Dial(q.gatewayURL, nil)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	logger.Infof("[QQ] WebSocket connected")
	q.wsConn = ws
	go q.handleWebSocketMessages(token)
	return nil
}

// handleWebSocketMessages receives and processes WebSocket messages
func (q *QQBot) handleWebSocketMessages(token string) {
	for {
		select {
		case <-q.ctx.Done():
			return
		default:
			q.mu.RLock()
			ws := q.wsConn
			q.mu.RUnlock()

			if ws == nil {
				return
			}

			_, message, err := ws.ReadMessage()
			if err != nil {
				logger.Errorf("[QQ] WebSocket error: %v", err)
				q.scheduleReconnect()
				return
			}

			var payload GatewayPayload
			if err := json.Unmarshal(message, &payload); err != nil {
				logger.Errorf("[QQ] Invalid payload: %v", err)
				continue
			}

			q.handleGatewayPayload(payload, token)
		}
	}
}

// handleGatewayPayload processes gateway payloads based on OP code
func (q *QQBot) handleGatewayPayload(payload GatewayPayload, token string) {
	switch payload.OP {
	case OPHello:
		// Server sent hello with heartbeat interval
		if helloData, ok := payload.D.(map[string]interface{}); ok {
			if interval, ok := helloData["heartbeat_interval"].(float64); ok {
				q.startHeartbeat(int(interval))
			}
		}

		// Send identify
		identify := GatewayPayload{
			OP: OPIdentify,
			D: IdentifyData{
				Token:   fmt.Sprintf("QQBot %s", token),
				Intents: IntentPublicMessages,
				Shard:   []int{qqShardID, qqShardTotal},
			},
		}
		if err := q.sendGateway(identify); err != nil {
			logger.Errorf("[QQ] Identify failed: %v", err)
		}

	case OPDispatch:
		// Update sequence number
		if payload.S != nil {
			q.lastSequence = payload.S
		}

		// Handle event types
		switch payload.T {
		case "READY":
			logger.Infof("[QQ] Gateway READY")
		case "C2C_MESSAGE_CREATE":
			q.handleC2CMessage(payload.D)
		}

	case OPHeartbeatAck:
		// Heartbeat acknowledged, nothing to do
	case OPReconnect:
		logger.Infof("[QQ] Server requested reconnection")
		q.scheduleReconnect()
	}
}

// handleC2CMessage processes C2C (private chat) messages
func (q *QQBot) handleC2CMessage(data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Errorf("[QQ] Marshal error: %v", err)
		return
	}

	var msg C2CMessageData
	if err := json.Unmarshal(jsonData, &msg); err != nil {
		logger.Errorf("[QQ] Parse error: %v", err)
		return
	}

	// Create bot message and call handler
	botMsg := BotMessage{
		Platform:  "qq",
		UserID:    msg.Author.UserOpenID,
		Channel:   msg.Author.UserOpenID,
		Content:   msg.Content,
		Timestamp: time.Now(),
		MessageID: msg.ID,
	}

	if q.messageHandler != nil {
		q.messageHandler(botMsg)
	}
}

// scheduleReconnect schedules a reconnection attempt with exponential backoff
func (q *QQBot) scheduleReconnect() {
	// TODO: Implement exponential backoff reconnection
	// For now, just log and stop
	logger.Warn("Connection lost, manual restart required")
}

// Stop stops the QQ bot and cleans up resources
func (q *QQBot) Stop() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.cancel != nil {
		q.cancel()
	}

	if q.wsConn != nil {
		q.wsConn.Close()
		q.wsConn = nil
	}

	logger.Infof("[QQ] Stopped")
	return nil
}

// getAccessToken retrieves and caches the access token
func (q *QQBot) getAccessToken() (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return cached token if still valid
	if q.accessToken != "" && time.Now().Before(q.tokenExpiresAt) {
		return q.accessToken, nil
	}

	logger.Debugf("[QQ] Requesting token from %s", QQTokenURL)

	// Request new token
	reqBody := fmt.Sprintf(`{"appId":"%s","clientSecret":"%s"}`, q.appID, q.appSecret)
	req, err := http.NewRequest("POST", QQTokenURL, strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create client with timeout
	client := &http.Client{
		Timeout: qqAPIRequestTimeout,
	}

	// Use proxy if available
	if q.proxyMgr != nil {
		logger.Debugf("[QQ] Using proxy")
		if proxyClient, proxyErr := q.proxyMgr.GetHTTPClient("qq"); proxyErr == nil {
			logger.Debugf("[QQ] Proxy client connected")
			client = proxyClient
		} else {
			logger.Debugf("[QQ] Proxy error: %v, using direct", proxyErr)
		}
	} else {
		logger.Debugf("[QQ] No proxy, using direct")
	}

	logger.Debugf("[QQ] Sending API request...")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch token: %w", err)
	}
	defer resp.Body.Close()

	logger.Debugf("[QQ] Token status: %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}

	var tokenResp QQTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}

	// Parse expires_in as integer (API returns it as string)
	expiresIn, err := strconv.Atoi(tokenResp.ExpiresIn)
	if err != nil {
		return "", fmt.Errorf("parse expires_in: %w", err)
	}

	logger.Debugf("[QQ] Token expires in %ds", expiresIn)

	// Cache token with buffer before expiration
	q.accessToken = tokenResp.AccessToken
	q.tokenExpiresAt = time.Now().Add(time.Duration(expiresIn-qqTokenExpirationBuffer) * time.Second)

	return q.accessToken, nil
}

// getGatewayURL retrieves the WebSocket gateway URL
func (q *QQBot) getGatewayURL(token string) (string, error) {
	req, err := http.NewRequest("GET", QQGatewayURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("QQBot %s", token))

	client := &http.Client{Timeout: qqAPIRequestTimeout}
	if q.proxyMgr != nil {
		if proxyClient, proxyErr := q.proxyMgr.GetHTTPClient("qq"); proxyErr == nil {
			client = proxyClient
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway request failed: %s", resp.Status)
	}

	var gatewayResp QQGatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gatewayResp); err != nil {
		return "", fmt.Errorf("decode gateway: %w", err)
	}

	return gatewayResp.URL, nil
}

// SendMessage sends a message to QQ (C2C private message)
func (q *QQBot) SendMessage(channel, message string) error {
	q.mu.RLock()
	token := q.accessToken
	q.mu.RUnlock()

	if token == "" {
		return fmt.Errorf("not authenticated")
	}

	// Split long messages
	if len(message) > qqMaxMessageLength {
		parts := splitMessage(message, qqMaxMessageLength)
		for _, part := range parts {
			if err := q.sendSingleMessage(channel, part, token); err != nil {
				return err
			}
		}
		return nil
	}

	return q.sendSingleMessage(channel, message, token)
}

// sendSingleMessage sends a single message (without splitting)
func (q *QQBot) sendSingleMessage(channel, message, token string) error {
	url := fmt.Sprintf("%s/v2/users/%s/messages", QQAPIBase, channel)

	reqBody := SendMessageRequest{
		Content: message,
		MsgType: qqMessageTypeText,
		// Note: QQ requires msg_id and msg_seq for passive reply
		// This is a simplified version - production should track these
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("QQBot %s", token))

	client := &http.Client{Timeout: qqMessageSendTimeout}
	if q.proxyMgr != nil {
		if proxyClient, proxyErr := q.proxyMgr.GetHTTPClient("qq"); proxyErr == nil {
			client = proxyClient
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send message failed: %s", resp.Status)
	}

	return nil
}

// splitMessage splits a long message into smaller parts
func splitMessage(msg string, maxLen int) []string {
	if len(msg) <= maxLen {
		return []string{msg}
	}

	var parts []string
	for len(msg) > maxLen {
		// Try to split at newline if possible
		splitIdx := maxLen
		if nlIdx := strings.LastIndex(msg[:maxLen], "\n"); nlIdx > maxLen/qqSplitMinNewlineIndex {
			splitIdx = nlIdx + 1
		}
		parts = append(parts, msg[:splitIdx])
		msg = msg[splitIdx:]
	}
	if len(msg) > 0 {
		parts = append(parts, msg)
	}
	return parts
}
