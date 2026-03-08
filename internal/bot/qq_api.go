package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// QQTokenResponse represents the token response from QQ API
type QQTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
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

// getAccessToken retrieves and caches the access token
func (q *QQBot) getAccessToken() (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return cached token if still valid
	if q.accessToken != "" && time.Now().Before(q.tokenExpiresAt) {
		return q.accessToken, nil
	}

	// Request new token
	reqBody := fmt.Sprintf(`{"appId":"%s","clientSecret":"%s"}`, q.appID, q.appSecret)
	req, err := http.NewRequest("POST", QQTokenURL, strings.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	if q.proxyMgr != nil {
		if proxyClient, proxyErr := q.proxyMgr.GetHTTPClient("qq"); proxyErr == nil {
			client = proxyClient
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}

	var tokenResp QQTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}

	// Cache token with 60s buffer before expiration
	q.accessToken = tokenResp.AccessToken
	q.tokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return q.accessToken, nil
}

// getGatewayURL retrieves the WebSocket gateway URL
func (q *QQBot) getGatewayURL(token string) (string, error) {
	req, err := http.NewRequest("GET", QQGatewayURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("QQBot %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
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

	// Split long messages (QQ limit is typically 2000-4000 chars)
	const maxLen = 2000
	if len(message) > maxLen {
		parts := splitMessage(message, maxLen)
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
		MsgType: 0, // Text message
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

	client := &http.Client{Timeout: 15 * time.Second}
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
		if nlIdx := strings.LastIndex(msg[:maxLen], "\n"); nlIdx > maxLen/2 {
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
