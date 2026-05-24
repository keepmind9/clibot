package feishu

import (
	"context"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/sirupsen/logrus"
)

const (
	keepaliveCheckInterval  = 30 * time.Second
	keepaliveStaleThreshold = 5 * time.Minute
	keepaliveMaxFailures    = 3
	keepaliveProbeTimeout   = 10 * time.Second
)

// startKeepaliveMonitor runs a background goroutine that probes Feishu API
// liveness when no events have been received for a while, and forces a
// WebSocket reconnect after consecutive probe failures.
func (b *Bot) startKeepaliveMonitor() {
	b.startKeepaliveMonitorWith(keepaliveCheckInterval)
}

// startKeepaliveMonitorWith is the testable variant that accepts a custom check interval.
func (b *Bot) startKeepaliveMonitorWith(checkInterval time.Duration) {
	lastTick := time.Now()
	failures := 0

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case now := <-ticker.C:
			// Sleep detection: if the tick arrived much later than expected
			// (e.g., system was suspended), reset counters and continue.
			if now.Sub(lastTick) > checkInterval*2 {
				logger.Info("feishu-keepalive-sleep-detected-resetting")
				failures = 0
				lastTick = now
				continue
			}
			lastTick = now

			lastEventNano := b.lastEventAt.Load()
			elapsed := time.Since(time.Unix(0, lastEventNano))

			if elapsed < keepaliveStaleThreshold {
				failures = 0
				continue
			}

			// Stale: probe liveness
			if b.probeLiveness() {
				failures = 0
				continue
			}

			failures++
			logger.WithFields(logrus.Fields{
				"failures":     failures,
				"max_failures": keepaliveMaxFailures,
			}).Warn("feishu-keepalive-probe-failed")

			if failures >= keepaliveMaxFailures {
				logger.Warn("feishu-keepalive-max-failures-reconnecting")
				b.forceReconnect()
				failures = 0
			}
		}
	}
}

// probeLiveness performs a lightweight API call to verify Feishu connectivity.
func (b *Bot) probeLiveness() bool {
	b.mu.RLock()
	larkClient := b.larkClient
	botCtx := b.ctx
	b.mu.RUnlock()

	if larkClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(botCtx, keepaliveProbeTimeout)
	defer cancel()

	// Use a lightweight API: list messages with page_size=1 to check connectivity.
	req := larkim.NewListMessageReqBuilder().
		PageSize(1).
		Build()

	resp, err := larkClient.Im.Message.List(ctx, req)
	if err != nil {
		logger.WithField("error", err).Warn("feishu-keepalive-probe-error")
		return false
	}
	if !resp.Success() {
		logger.WithFields(logrus.Fields{
			"code": resp.Code,
			"msg":  resp.Msg,
		}).Warn("feishu-keepalive-probe-api-error")
		return false
	}

	return true
}

// forceReconnect tears down the current WebSocket connection and starts a new one.
func (b *Bot) forceReconnect() {
	b.mu.Lock()
	// Cancel current context to stop existing WS client
	if b.cancel != nil {
		b.cancel()
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())

	opts := []ws.ClientOption{
		ws.WithEventHandler(b.dispatcher),
		ws.WithLogLevel(larkcore.LogLevelInfo),
		ws.WithAutoReconnect(true),
	}
	b.wsClient = ws.NewClient(b.appID, b.appSecret, opts...)
	wsClient := b.wsClient
	newCtx := b.ctx
	b.mu.Unlock()

	go func() {
		if err := wsClient.Start(newCtx); err != nil {
			logger.WithFields(logrus.Fields{
				"app_id": b.appID,
				"error":  err,
			}).Error("feishu-keepalive-reconnect-failed")
		}
	}()

	logger.Info("feishu-keepalive-reconnect-initiated")
}
