package feishu

// DebounceWindow returns the configured debounce window in milliseconds.
// Returns 0 if debouncing is disabled.
func (b *Bot) DebounceWindow() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.debounceMs
}

// SetDebounceMs configures the debounce window in milliseconds.
// Set to 0 to disable debouncing.
func (b *Bot) SetDebounceMs(ms int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.debounceMs = ms
}
