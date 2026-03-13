package bot

import (
	"fmt"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestHelpRenderingRepro(t *testing.T) {
	// Simulated help message with code block and HTML link
	md := "Special Commands:\n```bash\n<a href=\"tg://msg?text=help\">help</a> - Show help\n```"
	
	result := ConvertMarkdownToTelegramHTML(md)
	fmt.Printf("Result with code block:\n%s\n", result)
	
	// Verified: inside <pre><code>, the <a href> should be escaped
	assert.Contains(t, result, "&lt;a href=")
	assert.NotContains(t, result, "<a href=")

	// Simulated help message without code block
	md2 := "Special Commands:\n\n<a href=\"tg://msg?text=help\">help</a> - Show help"
	result2 := ConvertMarkdownToTelegramHTML(md2)
	fmt.Printf("Result without code block:\n%s\n", result2)
	
	// Should contain the raw <a href> tag
	assert.Contains(t, result2, "<a href=\"tg://msg?text=help\">")
}
