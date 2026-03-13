package bot

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
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
	md := "| Name | 城市 | Age |\n|------|-----|---|\n| Alice | New York | 30 |\n| 机器人 | 北京 | 25 |"

	result := ConvertMarkdownToTelegramHTML(md)
	// Should render as <pre> with aligned columns and separator
	assert.Contains(t, result, "<pre>")
	assert.Contains(t, result, "</pre>")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "机器人")
	assert.Contains(t, result, "北京")
	assert.Contains(t, result, "│")
	assert.Contains(t, result, "─")

	// Verify alignment roughly by checking if the separator line width matches
	lines := strings.Split(result, "\n")
	var preLines []string
	inPre := false
	for _, l := range lines {
		if strings.Contains(l, "<pre>") {
			inPre = true
			continue
		}
		if strings.Contains(l, "</pre>") {
			break
		}
		if inPre {
			preLines = append(preLines, l)
		}
	}
	// Check header, separator, and data rows
	if len(preLines) >= 3 {
		// Header and first data row should have same structure (ignoring content)
		// We use runewidth to check visual length
		hLen := runewidth.StringWidth(preLines[0])
		sLen := runewidth.StringWidth(preLines[1])
		dLen := runewidth.StringWidth(preLines[2])
		assert.Equal(t, hLen, sLen)
		assert.Equal(t, hLen, dLen)
	}
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
	md := "Inline math $E = mc^2$ and $H_2O$ and $\\alpha + \\beta$."

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Contains(t, result, "<code>E = mc²</code>")
	assert.Contains(t, result, "<code>H₂O</code>")
	assert.Contains(t, result, "<code>α + β</code>")
}

func TestConvertMarkdownToTelegramHTML_DisplayLaTeX(t *testing.T) {
	md := "Display:\n$$\\sum_{i=0}^{n} x_i$$\nDone."

	result := ConvertMarkdownToTelegramHTML(md)
	assert.Contains(t, result, "<pre>")
	assert.Contains(t, result, "∑ᵢ₌₀ⁿ xᵢ")

	// Limit test
	md2 := "$$\\lim_{x \\to \\infty} \\frac{1}{x} = 0$$"
	result2 := ConvertMarkdownToTelegramHTML(md2)
	assert.Contains(t, result2, "limₓ → ∞ 1/x = 0")

	// Quadratic formula test
	md3 := "$$x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}$$"
	result3 := ConvertMarkdownToTelegramHTML(md3)
	assert.Contains(t, result3, "x = [-b ± √(b² - 4ac)]/(2a)")

	// Summation test
	md4 := "$$\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}$$"
	result4 := ConvertMarkdownToTelegramHTML(md4)
	assert.Contains(t, result4, "∑ᵢ₌₁ⁿ i = [n(n+1)]/2")
}
