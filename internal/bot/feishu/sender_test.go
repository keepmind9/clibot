package feishu

import (
	"context"
	"sync"
	"testing"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	"github.com/stretchr/testify/assert"
)

type mockUserGetter struct {
	resp *larkcontact.GetUserResp
	err  error
}

func (m *mockUserGetter) Get(_ context.Context, _ *larkcontact.GetUserReq, _ ...larkcore.RequestOptionFunc) (*larkcontact.GetUserResp, error) {
	return m.resp, m.err
}

func TestResolveSenderName(t *testing.T) {
	tests := []struct {
		name       string
		openID     string
		getter     *mockUserGetter
		cachePre   map[string]nameEntry
		wantName   string
		wantCached bool
	}{
		{
			name:     "empty open_id returns empty",
			openID:   "",
			getter:   &mockUserGetter{},
			wantName: "",
		},
		{
			name:   "cache hit returns cached name",
			openID: "ou_cached",
			getter: &mockUserGetter{},
			cachePre: map[string]nameEntry{
				"ou_cached": {Name: "CachedUser", ExpiresAt: time.Now().Add(10 * time.Minute)},
			},
			wantName:   "CachedUser",
			wantCached: true,
		},
		{
			name:   "expired cache triggers re-fetch",
			openID: "ou_expired",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: &larkcontact.User{
							Name: strPtr("FreshName"),
						},
					},
				},
			},
			cachePre: map[string]nameEntry{
				"ou_expired": {Name: "StaleName", ExpiresAt: time.Now().Add(-1 * time.Minute)},
			},
			wantName:   "FreshName",
			wantCached: true,
		},
		{
			name:   "API success with Name field",
			openID: "ou_name",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: &larkcontact.User{
							Name:     strPtr("ZhangSan"),
							Nickname: strPtr("ZS"),
							EnName:   strPtr("zs"),
						},
					},
				},
			},
			wantName:   "ZhangSan",
			wantCached: true,
		},
		{
			name:   "Name priority: Nickname over EnName",
			openID: "ou_nick",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: &larkcontact.User{
							Nickname: strPtr("NickUser"),
							EnName:   strPtr("en_user"),
						},
					},
				},
			},
			wantName:   "NickUser",
			wantCached: true,
		},
		{
			name:   "Name priority: EnName when no Name/Nickname",
			openID: "ou_en",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: &larkcontact.User{
							EnName: strPtr("EnUser"),
						},
					},
				},
			},
			wantName:   "EnUser",
			wantCached: true,
		},
		{
			name:   "Name priority: OpenId as last resort",
			openID: "ou_fallback",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: &larkcontact.User{
							OpenId: strPtr("ou_fallback"),
						},
					},
				},
			},
			wantName:   "ou_fallback",
			wantCached: true,
		},
		{
			name:   "API error returns empty string",
			openID: "ou_err",
			getter: &mockUserGetter{
				err: context.DeadlineExceeded,
			},
			wantName:   "",
			wantCached: false,
		},
		{
			name:   "API non-success returns empty string",
			openID: "ou_fail",
			getter: &mockUserGetter{
				resp: func() *larkcontact.GetUserResp {
					r := &larkcontact.GetUserResp{}
					r.Code = 999
					r.Msg = "forbidden"
					return r
				}(),
			},
			wantName:   "",
			wantCached: false,
		},
		{
			name:   "nil user returns empty string",
			openID: "ou_nil",
			getter: &mockUserGetter{
				resp: &larkcontact.GetUserResp{
					Data: &larkcontact.GetUserRespData{
						User: nil,
					},
				},
			},
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &sync.Map{}
			for k, v := range tt.cachePre {
				cache.Store(k, v)
			}

			got := resolveSenderName(context.Background(), tt.getter, tt.openID, cache)
			assert.Equal(t, tt.wantName, got)

			if tt.wantCached && tt.openID != "" {
				cached, ok := cache.Load(tt.openID)
				assert.True(t, ok, "expected cache entry")
				if ok {
					entry := cached.(nameEntry)
					assert.Equal(t, tt.wantName, entry.Name)
					assert.True(t, entry.ExpiresAt.After(time.Now()))
				}
			}
		})
	}
}
