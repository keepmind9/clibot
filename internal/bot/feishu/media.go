package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/keepmind9/clibot/internal/bot"
	"github.com/keepmind9/clibot/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/sirupsen/logrus"
)

const defaultMediaTTL = 24 * time.Hour

// messageResourceGetter abstracts the Feishu message resource API for testability.
type messageResourceGetter interface {
	Get(ctx context.Context, req *larkim.GetMessageResourceReq, options ...larkcore.RequestOptionFunc) (*larkim.GetMessageResourceResp, error)
}

// SetMediaConfig configures media download parameters.
func (b *Bot) SetMediaConfig(dir string, ttl time.Duration, maxSize int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mediaDir = dir
	b.mediaTTL = ttl
	b.maxMediaSize = maxSize
}

// DownloadMedia downloads media files from Feishu and attaches them to the message.
// Skips text and sticker messages. Returns nil on skip or error (non-blocking).
func (b *Bot) DownloadMedia(ctx context.Context, msg *bot.BotMessage) error {
	if msg.MessageType == "text" || msg.MessageType == "post" || msg.MessageType == "sticker" || msg.MessageType == "interactive" {
		return nil
	}

	b.mu.RLock()
	larkClient := b.larkClient
	mediaDir := b.mediaDir
	maxSize := b.maxMediaSize
	b.mu.RUnlock()

	if larkClient == nil || msg.MessageID == "" {
		return nil
	}

	return downloadMedia(ctx, larkClient.Im.MessageResource, msg, mediaDir, maxSize)
}

func downloadMedia(ctx context.Context, getter messageResourceGetter, msg *bot.BotMessage, mediaDir string, maxSize int64) error {
	fileKey, fileName := parseMediaContent(msg.MessageType, msg.Content)
	if fileKey == "" {
		return nil
	}

	// Determine resource type for API call
	resourceType := "file"
	if msg.MessageType == "image" {
		resourceType = "image"
	}

	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(msg.MessageID).
		FileKey(fileKey).
		Type(resourceType).
		Build()

	resp, err := getter.Get(ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"message_id": msg.MessageID,
			"file_key":   fileKey,
			"error":      err,
		}).Warn("feishu-media-download-failed")
		return err
	}

	if resp == nil || !resp.Success() {
		logger.WithFields(logrus.Fields{
			"message_id": msg.MessageID,
			"file_key":   fileKey,
		}).Warn("feishu-media-download-api-error")
		return fmt.Errorf("media download API error")
	}

	// Check size limit
	if maxSize > 0 {
		if f, ok := resp.File.(interface{ Len() int }); ok {
			if int64(f.Len()) > maxSize {
				logger.WithFields(logrus.Fields{
					"file_key": fileKey,
					"size":     f.Len(),
					"max":      maxSize,
				}).Warn("feishu-media-exceeds-size-limit")
				return fmt.Errorf("media file exceeds size limit")
			}
		}
	}

	// Save to disk
	if mediaDir == "" {
		mediaDir = filepath.Join(os.Getenv("HOME"), ".clibot", "media")
	}
	chatDir := filepath.Join(mediaDir, msg.Channel)
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		return err
	}

	if fileName == "" {
		fileName = fileKey
		if msg.MessageType == "image" {
			fileName += ".png"
		}
	}
	localPath := filepath.Join(chatDir, fileName)

	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.File); err != nil {
		os.Remove(localPath)
		return err
	}

	msg.Attachments = append(msg.Attachments, bot.Attachment{
		Type:     msg.MessageType,
		FileName: fileName,
		FilePath: localPath,
		FileKey:  fileKey,
	})

	// Set placeholder content
	if strings.TrimSpace(msg.Content) == "" || strings.HasPrefix(msg.Content, "{") {
		switch msg.MessageType {
		case "image":
			msg.Content = "[image]"
		case "file":
			msg.Content = fmt.Sprintf("[file: %s]", fileName)
		case "audio":
			msg.Content = "[audio]"
		case "video":
			msg.Content = "[video]"
		}
	}

	logger.WithFields(logrus.Fields{
		"file_key":  fileKey,
		"file_name": fileName,
		"local":     localPath,
	}).Info("feishu-media-downloaded")

	return nil
}

// parseMediaContent extracts file_key and file_name from message content JSON based on type.
func parseMediaContent(messageType, content string) (fileKey, fileName string) {
	switch messageType {
	case "image":
		var img ImageContent
		if err := json.Unmarshal([]byte(content), &img); err == nil {
			return img.ImageKey, ""
		}
	case "file", "audio", "video", "media":
		var fc FileContent
		if err := json.Unmarshal([]byte(content), &fc); err == nil {
			return fc.FileKey, fc.FileName
		}
	}
	return "", ""
}

// startMediaGC starts a goroutine that periodically cleans expired media files.
func (b *Bot) startMediaGC() {
	b.mu.RLock()
	mediaDir := b.mediaDir
	ttl := b.mediaTTL
	b.mu.RUnlock()

	if mediaDir == "" {
		mediaDir = filepath.Join(os.Getenv("HOME"), ".clibot", "media")
	}
	if ttl == 0 {
		ttl = defaultMediaTTL
	}

	interval := ttl / 4
	if interval < 5*time.Minute {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			cleanExpiredMedia(mediaDir, ttl)
		}
	}
}

func cleanExpiredMedia(root string, ttl time.Duration) {
	cutoff := time.Now().Add(-ttl)
	count := 0

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
			count++
		}
		return nil
	})

	if count > 0 {
		logger.WithFields(logrus.Fields{
			"root":  root,
			"count": count,
		}).Info("feishu-media-gc-cleaned")
	}
}
