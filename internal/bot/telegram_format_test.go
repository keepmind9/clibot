package bot

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertMarkdownToTelegramHTML_Headings(t *testing.T) {
	md := "# Heading 1\n## Heading 2"
	expected := "<b>Heading 1</b>\n\n<b>Heading 2</b>"

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Equal(t, expected, result)
}

func TestConvertMarkdownToTelegramHTML_Lists(t *testing.T) {
	md := "- Item 1\n- Item 2\n  - Nested 1\n  - Nested 2\n- Item 3"
	expected := "• Item 1\n• Item 2\n  • Nested 1\n  • Nested 2\n• Item 3"

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Equal(t, expected, result)

	mdOrdered := "1. First\n2. Second"
	expectedOrdered := "1. First\n2. Second"

	resultOrdered := ConvertMarkdownToTelegramHTML(mdOrdered)
	assert.Equal(t, expectedOrdered, resultOrdered)
}

func TestConvertMarkdownToTelegramHTML_CodeBlocks(t *testing.T) {
	md := "Here is some code:\n```go\nfunc main() {}\n```\nAnd `inline` code."

	result := ConvertMarkdownToTelegramHTML(md)
	assert.True(t, strings.Contains(result, "<pre><code class=\"language-go\">func main() {}"))
	assert.True(t, strings.Contains(result, "<code>inline</code>"))
}

func TestConvertMarkdownToTelegramHTML_MixedFormatting(t *testing.T) {
	md := "This is **bold**, *italic*, and ~~strikethrough~~."
	expected := "This is <b>bold</b>, <i>italic</i>, and <s>strikethrough</s>."

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Equal(t, expected, result)
}

func TestConvertMarkdownToTelegramHTML_Links(t *testing.T) {
	md := "Click [here](https://example.com) for more info."
	expected := "Click <a href=\"https://example.com\">here</a> for more info."

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Equal(t, expected, result)
}

func TestConvertMarkdownToTelegramHTML_Tables(t *testing.T) {
	md := "| Name | Age |\n|------|-----|\n| Alice | 30 |\n| Bob | 25 |"

	result := ConvertMarkdownToTelegramHTML(md)
	// Should render as <pre> with aligned columns and separator
	assert.Contains(t, result, "<pre>")
	assert.Contains(t, result, "</pre>")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Bob")
	assert.Contains(t, result, "│")
	assert.Contains(t, result, "─")
}

func TestConvertMarkdownToTelegramHTML_TaskList(t *testing.T) {
	md := "- [ ] unchecked\n- [x] checked"

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Contains(t, result, "☐ unchecked")
	assert.Contains(t, result, "☑ checked")
}

func TestConvertMarkdownToTelegramHTML_ThematicBreak(t *testing.T) {
	md := "Above\n\n---\n\nBelow"

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Contains(t, result, "Above")
	assert.Contains(t, result, "Below")
	assert.Contains(t, result, "————")
	// Should NOT contain raw "---"
	assert.NotContains(t, result, "---")
}

func TestConvertMarkdownToTelegramHTML_Images(t *testing.T) {
	md := "![alt text](https://example.com/image.png)"

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Contains(t, result, "🖼")
	assert.Contains(t, result, `<a href="https://example.com/image.png">`)
	assert.Contains(t, result, "alt text")
}

func TestConvertMarkdownToTelegramHTML_LaTeX(t *testing.T) {
	md := "Inline math $x^2 + y^2 = z^2$ here."

	result := ConvertMarkdownToTelegramHTML(md)
	// LaTeX should be wrapped in code tags
	assert.Contains(t, result, "<code>")
	assert.Contains(t, result, "x^2 + y^2 = z^2")
}

func TestConvertMarkdownToTelegramHTML_DisplayLaTeX(t *testing.T) {
	md := "Display:\n$$E = mc^2$$\nDone."

	result := ConvertMarkdownToTelegramHTML(md)
	// Display LaTeX should be in a code block
	assert.Contains(t, result, "<pre>")
	assert.Contains(t, result, "E = mc^2")
}
