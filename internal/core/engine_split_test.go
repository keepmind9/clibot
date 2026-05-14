package core

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestSplitMessageByLines_NoSplitNeeded(t *testing.T) {
	msg := "hello world"
	chunks := splitMessageByLines(msg, 100)
	assert.Equal(t, []string{msg}, chunks)
}

func TestSplitMessageByLines_Disabled(t *testing.T) {
	msg := strings.Repeat("a\n", 1000)
	chunks := splitMessageByLines(msg, 0)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, msg, chunks[0])
}

func TestSplitMessageByLines_SplitByLines(t *testing.T) {
	// 5 lines, each 10 chars, maxLen=25 -> expect 3 chunks
	lines := []string{"line1: 1234", "line2: 5678", "line3: abcd", "line4: efgh", "line5: ijkl"}
	msg := strings.Join(lines, "\n")

	chunks := splitMessageByLines(msg, 25)

	assert.Greater(t, len(chunks), 1)
	// Verify no line is split
	for _, chunk := range chunks {
		for _, line := range strings.Split(chunk, "\n") {
			assert.Contains(t, lines, line)
		}
	}
	// Verify reassembly (chunks are split at line boundaries, join with \n)
	assert.Equal(t, msg, strings.Join(chunks, "\n"))
}

func TestSplitMessageByLines_SingleLongLine(t *testing.T) {
	longLine := strings.Repeat("x", 100)
	msg := "short\n" + longLine + "\nanother"

	chunks := splitMessageByLines(msg, 50)

	// Long line should be its own chunk even though it exceeds maxLen
	assert.Greater(t, len(chunks), 1)
	found := false
	for _, chunk := range chunks {
		if chunk == longLine {
			found = true
		}
	}
	assert.True(t, found, "long line should be preserved as its own chunk")
}

func TestSplitMessageByLines_EmptyMessage(t *testing.T) {
	chunks := splitMessageByLines("", 10)
	assert.Equal(t, []string{""}, chunks)
}

func TestSplitMessageByLines_EachLineExactLimit(t *testing.T) {
	// Lines exactly at maxLen
	msg := "12345\n12345\n12345"
	chunks := splitMessageByLines(msg, 5)
	// Each line is 5 chars, with newline separator adding 1, first two lines fit 5+1+5=11 > 5
	// So each line becomes its own chunk
	assert.Equal(t, 3, len(chunks))
}

func TestSplitMessageByLines_ManyShortLines(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "short")
	}
	msg := strings.Join(lines, "\n")

	chunks := splitMessageByLines(msg, 30)

	assert.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, utf8.RuneCountInString(chunk), 30+6, "chunk should be close to maxLen (allowing for last line)")
	}
}

func TestSplitMessageByLines_MultibyteUTF8(t *testing.T) {
	// Chinese characters: each rune is 3 bytes but counts as 1 rune
	// "你好世界" = 4 runes, 12 bytes
	msg := "你好世界\n第二行内容\n第三行测试"
	chunks := splitMessageByLines(msg, 6)

	assert.Greater(t, len(chunks), 1)
	// Verify reassembly
	assert.Equal(t, msg, strings.Join(chunks, "\n"))
	// Verify no rune is split (each chunk is valid UTF-8)
	for _, chunk := range chunks {
		assert.True(t, utf8.ValidString(chunk))
	}
}
