package feishu

import (
	"context"
	"sync"
	"time"

	"github.com/keepmind9/clibot/internal/logger"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	"github.com/sirupsen/logrus"
)

const senderCacheTTL = 30 * time.Minute

type nameEntry struct {
	Name      string
	ExpiresAt time.Time
}

// userGetter abstracts the Feishu contact API for testability.
type userGetter interface {
	Get(ctx context.Context, req *larkcontact.GetUserReq, options ...larkcore.RequestOptionFunc) (*larkcontact.GetUserResp, error)
}

// resolveSenderName resolves an open_id to a display name using the Contact API.
// Results are cached for senderCacheTTL. Returns empty string on error.
func resolveSenderName(ctx context.Context, getter userGetter, openID string, cache *sync.Map) string {
	if openID == "" {
		return ""
	}

	if cached, ok := cache.Load(openID); ok {
		entry := cached.(nameEntry)
		if time.Now().Before(entry.ExpiresAt) {
			return entry.Name
		}
		cache.Delete(openID)
	}

	req := larkcontact.NewGetUserReqBuilder().
		UserId(openID).
		UserIdType("open_id").
		Build()

	resp, err := getter.Get(ctx, req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"open_id": openID,
			"error":   err,
		}).Debug("feishu-sender-name-resolve-failed")
		return ""
	}

	if !resp.Success() || resp.Data == nil || resp.Data.User == nil {
		return ""
	}

	name := pickDisplayName(resp.Data.User)
	if name != "" {
		cache.Store(openID, nameEntry{
			Name:      name,
			ExpiresAt: time.Now().Add(senderCacheTTL),
		})
	}

	return name
}

func pickDisplayName(user *larkcontact.User) string {
	if user.Name != nil && *user.Name != "" {
		return *user.Name
	}
	if user.Nickname != nil && *user.Nickname != "" {
		return *user.Nickname
	}
	if user.EnName != nil && *user.EnName != "" {
		return *user.EnName
	}
	if user.OpenId != nil {
		return *user.OpenId
	}
	return ""
}
