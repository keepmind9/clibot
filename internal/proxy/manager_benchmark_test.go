package proxy

import (
	"testing"
)

func BenchmarkProxyManager_GetHTTPClient_Cached(b *testing.B) {
	config := &mockConfigProvider{
		globalEnabled: true,
		globalType:    "http",
		globalURL:     "http://127.0.0.1:8080",
	}
	pm := NewProxyManager(config)

	// Warm up cache
	pm.GetHTTPClient("telegram")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.GetHTTPClient("telegram")
	}
}

func BenchmarkProxyManager_GetHTTPClient_Uncached(b *testing.B) {
	config := &mockConfigProvider{
		globalEnabled: true,
		globalType:    "http",
		globalURL:     "http://127.0.0.1:8080",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm := NewProxyManager(config)
		pm.GetHTTPClient("telegram")
	}
}
