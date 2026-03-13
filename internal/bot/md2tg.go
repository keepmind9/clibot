package bot

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// tgHTMLRenderer builds a string while walking the goldmark AST
type tgHTMLRenderer struct {
	buf          bytes.Buffer
	src          []byte
	listPrefixes []string
	listCounters []int

	// Table rendering: collect cells, then render aligned columns on table exit
	inTable      bool
	tableRows    [][]string // rows of cell-text slices
	currentRow   []string
	currentCell  strings.Builder
}

// latexSubscripts maps LaTeX subscript sequences to Unicode
var latexSubscripts = map[string]string{
	"0": "₀", "1": "₁", "2": "₂", "3": "₃", "4": "₄",
	"5": "₅", "6": "₆", "7": "₇", "8": "₈", "9": "₉",
	"+": "₊", "-": "₋", "=": "₌", "(": "₍", ")": "₎",
	"a": "ₐ", "e": "ₑ", "o": "ₒ", "x": "ₓ", "h": "ₕ",
	"k": "ₖ", "l": "ₗ", "m": "ₘ", "n": "ₙ", "p": "ₚ",
	"s": "ₛ", "t": "ₜ", "i": "ᵢ", "j": "ⱼ", "r": "ᵣ",
	"u": "ᵤ", "v": "ᵥ",
	"→": "→", "∞": "∞", // Preserve these in subscripts as best as possible
}

// latexSuperscripts maps LaTeX superscript sequences to Unicode
var latexSuperscripts = map[string]string{
	"0": "⁰", "1": "¹", "2": "²", "3": "³", "4": "⁴",
	"5": "⁵", "6": "⁶", "7": "⁷", "8": "⁸", "9": "⁹",
	"+": "⁺", "-": "⁻", "=": "⁼", "(": "⁽", ")": "⁾",
	"n": "ⁿ", "i": "ⁱ", "a": "ᵃ", "b": "ᵇ", "c": "ᶜ",
	"d": "ᵈ", "e": "ᵉ", "f": "ᶠ", "g": "ᵍ", "h": "ʰ",
	"j": "ʲ", "k": "ᵏ", "l": "ˡ", "m": "ᵐ", "o": "ᵒ",
	"p": "ᵖ", "r": "ʳ", "s": "ˢ", "t": "ᵗ", "u": "ᵘ",
	"v": "ᵛ", "w": "ʷ", "x": "ˣ", "y": "ʸ", "z": "ᶻ",
}

// latexSymbols maps common LaTeX commands to Unicode
var latexSymbols = map[string]string{
	"\\alpha": "α", "\\beta": "β", "\\gamma": "γ", "\\delta": "δ",
	"\\epsilon": "ε", "\\zeta": "ζ", "\\eta": "η", "\\theta": "θ",
	"\\iota": "ι", "\\kappa": "κ", "\\lambda": "λ", "\\mu": "μ",
	"\\nu": "ν", "\\xi": "ξ", "\\omicron": "ο", "\\pi": "π",
	"\\rho": "ρ", "\\sigma": "σ", "\\tau": "τ", "\\upsilon": "υ",
	"\\phi": "φ", "\\chi": "χ", "\\psi": "ψ", "\\omega": "ω",
	"\\Gamma": "Γ", "\\Delta": "Δ", "\\Theta": "Θ", "\\Lambda": "Λ",
	"\\Xi": "Ξ", "\\Pi": "Π", "\\Sigma": "Σ", "\\Upsilon": "Φ",
	"\\Phi": "Φ", "\\Psi": "Ψ", "\\Omega": "Ω",
	"\\infty": "∞", "\\pm": "±", "\\times": "×", "\\div": "÷",
	"\\neq": "≠", "\\leq": "≤", "\\geq": "≥", "\\approx": "≈",
	"\\partial": "∂", "\\nabla": "∇", "\\sum": "∑", "\\prod": "∏",
	"\\int": "∫", "\\sqrt": "√", "\\angle": "∠", "\\cap": "∩",
	"\\cup": "∪", "\\sub": "⊂", "\\sup": "⊃", "\\in": "∈",
	"\\notin": "∉", "\\forall": "∀", "\\exists": "∃",
	"\\quad": "  ", "\\qquad": "    ",
	"\\to": "→", "\\rightarrow": "→", "\\leftarrow": "←",
	"\\lim": "lim", "\\log": "log", "\\sin": "sin", "\\cos": "cos", "\\tan": "tan",
}

// latexBlockRe matches display math $$...$$  (may span multiple lines)
var latexBlockRe = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`)

// latexInlineRe matches inline math $...$  (single line, non-greedy)
var latexInlineRe = regexp.MustCompile(`\$([^\n$]+?)\$`)

// preprocessLaTeX converts common LaTeX symbols and constructs to Unicode
// to improve readability in Telegram.
func preprocessLaTeX(md string) string {
	convertMath := func(math string) string {
		// Handle \sqrt{...} -> √( ... )
		// We do this first so \frac can capture it without brace confusion
		math = regexp.MustCompile(`\\sqrt\{([^}]+)\}`).ReplaceAllStringFunc(math, func(s string) string {
			content := s[6 : len(s)-1]
			// We handle symbols inside sqrt here too if needed, but convertMath is recursive-like
			return "√(" + content + ")"
		})

		// Handle \frac{num}{den} -> [num]/[den]
		math = regexp.MustCompile(`\\frac\{([^}]+)\}\{([^}]+)\}`).ReplaceAllStringFunc(math, func(s string) string {
			m := regexp.MustCompile(`\\frac\{([^}]+)\}\{([^}]+)\}`).FindStringSubmatch(s)
			if len(m) == 3 {
				num := m[1]
				den := m[2]

				// Format numerator
				if len(num) > 1 {
					num = "[" + num + "]"
				}
				// Format denominator
				if len(den) > 1 {
					den = "(" + den + ")"
				}
				return num + "/" + den
			}
			return s
		})

		// Replace common symbols
		for cmd, unicode := range latexSymbols {
			math = strings.ReplaceAll(math, cmd, unicode)
		}

		// Handle subscripts: x_2 or x_{2} or \lim_{...}
		math = regexp.MustCompile(`_{([^}]+)}`).ReplaceAllStringFunc(math, func(s string) string {
			content := s[2 : len(s)-1]
			// Recursively handle symbols inside the script first
			for cmd, unicode := range latexSymbols {
				content = strings.ReplaceAll(content, cmd, unicode)
			}
			var res strings.Builder
			for _, r := range content {
				if v, ok := latexSubscripts[string(r)]; ok {
					res.WriteString(v)
				} else {
					res.WriteRune(r)
				}
			}
			return res.String()
		})

		// Handle superscripts: x^2 or x^{2}
		math = regexp.MustCompile(`\^{([^}]+)}`).ReplaceAllStringFunc(math, func(s string) string {
			content := s[2 : len(s)-1]
			// Recursively handle symbols inside the script first
			for cmd, unicode := range latexSymbols {
				content = strings.ReplaceAll(content, cmd, unicode)
			}
			var res strings.Builder
			for _, r := range content {
				if v, ok := latexSuperscripts[string(r)]; ok {
					res.WriteString(v)
				} else {
					res.WriteRune(r)
				}
			}
			return res.String()
		})

		// Single char scripts
		math = regexp.MustCompile(`\^([^{])`).ReplaceAllStringFunc(math, func(s string) string {
			char := s[1:]
			if v, ok := latexSuperscripts[char]; ok {
				return v
			}
			return char
		})
		math = regexp.MustCompile(`_([^{])`).ReplaceAllStringFunc(math, func(s string) string {
			char := s[1:]
			for cmd, unicode := range latexSymbols {
				char = strings.ReplaceAll(char, cmd, unicode)
			}
			if v, ok := latexSubscripts[char]; ok {
				return v
			}
			return char
		})

		return math
	}

	// Process block math
	md = latexBlockRe.ReplaceAllStringFunc(md, func(s string) string {
		content := s[2 : len(s)-2]
		return "```\n" + strings.TrimSpace(convertMath(content)) + "\n```"
	})

	// Process inline math
	md = latexInlineRe.ReplaceAllStringFunc(md, func(s string) string {
		content := s[1 : len(s)-1]
		return "<code>" + convertMath(content) + "</code>"
	})

	return md
}

// ConvertMarkdownToTelegramHTML parses Markdown and generates a Telegram-compatible HTML string.
func ConvertMarkdownToTelegramHTML(mdText string) string {
	if mdText == "" {
		return ""
	}

	// Pre-process LaTeX
	mdText = preprocessLaTeX(mdText)

	src := []byte(mdText)
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Strikethrough,
			extension.Table,
			extension.TaskList,
		),
	)

	doc := md.Parser().Parse(text.NewReader(src))

	r := &tgHTMLRenderer{
		src:          src,
		listPrefixes: make([]string, 0),
		listCounters: make([]int, 0),
	}

	err := ast.Walk(doc, r.Walk)
	if err != nil {
		// Fallback parsing failed; should be rare.
		return html.EscapeString(mdText)
	}

	return strings.TrimSpace(r.buf.String())
}

// Walk implements the goldmark ast.Walker interface
func (r *tgHTMLRenderer) Walk(n ast.Node, entering bool) (ast.WalkStatus, error) {
	switch v := n.(type) {
	case *ast.Document:
		// Do nothing
	case *ast.Heading:
		if entering {
			r.buf.WriteString("<b>")
		} else {
			r.buf.WriteString("</b>\n\n")
		}
	case *ast.Paragraph, *ast.TextBlock:
		if !entering {
			// Only add newlines if we are not tightly inside a list item that already handles it.
			if n.NextSibling() != nil {
				if n.NextSibling().Kind() == ast.KindList {
					r.buf.WriteString("\n")
				} else {
					r.buf.WriteString("\n\n")
				}
			} else if n.Parent() != nil && n.Parent().Kind() == ast.KindListItem {
				r.buf.WriteString("\n")
			} else {
				r.buf.WriteString("\n\n")
			}
		}
	case *ast.Text:
		if entering {
			if r.inTable {
				val := string(v.Segment.Value(r.src))
				r.currentCell.WriteString(val)
			} else {
				val := string(v.Segment.Value(r.src))
				r.buf.WriteString(html.EscapeString(val))
			}
			if v.SoftLineBreak() || v.HardLineBreak() {
				if r.inTable {
					r.currentCell.WriteString(" ")
				} else {
					r.buf.WriteString("\n")
				}
			}
		}
	case *ast.String:
		if entering {
			if r.inTable {
				r.currentCell.WriteString(string(v.Value))
			} else {
				r.buf.WriteString(html.EscapeString(string(v.Value)))
			}
		}
	case *ast.Emphasis:
		if entering {
			if v.Level == 2 {
				r.writeOrCell("<b>")
			} else {
				r.writeOrCell("<i>")
			}
		} else {
			if v.Level == 2 {
				r.writeOrCell("</b>")
			} else {
				r.writeOrCell("</i>")
			}
		}
	case *extast.Strikethrough:
		if entering {
			r.writeOrCell("<s>")
		} else {
			r.writeOrCell("</s>")
		}
	case *ast.CodeSpan:
		if entering {
			r.writeOrCell("<code>")
		} else {
			r.writeOrCell("</code>")
		}
	case *ast.FencedCodeBlock:
		if entering {
			lang := string(v.Language(r.src))
			if lang != "" {
				r.buf.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">", html.EscapeString(lang)))
			} else {
				r.buf.WriteString("<pre><code>")
			}
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				r.buf.WriteString(html.EscapeString(string(line.Value(r.src))))
			}
		} else {
			r.buf.WriteString("</code></pre>\n\n")
		}
	case *ast.CodeBlock:
		if entering {
			r.buf.WriteString("<pre><code>")
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				r.buf.WriteString(html.EscapeString(string(line.Value(r.src))))
			}
		} else {
			r.buf.WriteString("</code></pre>\n\n")
		}
	case *ast.List:
		if entering {
			if v.IsOrdered() {
				r.listPrefixes = append(r.listPrefixes, "ordered")
				r.listCounters = append(r.listCounters, v.Start)
			} else {
				r.listPrefixes = append(r.listPrefixes, "bullet")
				r.listCounters = append(r.listCounters, 0)
			}
		} else {
			r.listPrefixes = r.listPrefixes[:len(r.listPrefixes)-1]
			r.listCounters = r.listCounters[:len(r.listCounters)-1]
			if len(r.listPrefixes) == 0 {
				r.buf.WriteString("\n")
			}
		}
	case *ast.ListItem:
		if entering {
			indentLevel := len(r.listPrefixes) - 1
			if indentLevel < 0 {
				indentLevel = 0
			}
			indent := strings.Repeat("  ", indentLevel)
			prefix := "• "

			if len(r.listPrefixes) > 0 && r.listPrefixes[len(r.listPrefixes)-1] == "ordered" {
				counterIndex := len(r.listCounters) - 1
				counter := r.listCounters[counterIndex]
				prefix = fmt.Sprintf("%d. ", counter)
				r.listCounters[counterIndex]++
			}
			r.buf.WriteString(indent + prefix)
		}
	case *extast.TaskCheckBox:
		// GFM task list checkbox: - [ ] or - [x]
		if entering {
			if v.IsChecked {
				r.buf.WriteString("☑ ")
			} else {
				r.buf.WriteString("☐ ")
			}
		}
	case *ast.Link:
		if entering {
			r.writeOrCell(fmt.Sprintf("<a href=\"%s\">", html.EscapeString(string(v.Destination))))
		} else {
			r.writeOrCell("</a>")
		}
	case *ast.AutoLink:
		if entering {
			url := html.EscapeString(string(v.URL(r.src)))
			r.writeOrCell(fmt.Sprintf("<a href=\"%s\">%s</a>", url, url))
		}
	case *ast.Image:
		// Telegram doesn't support inline images in HTML parse mode.
		// Render as a clickable link with image emoji.
		if entering {
			alt := extractTextFromNode(v, r.src)
			dest := html.EscapeString(string(v.Destination))
			if alt == "" {
				alt = "image"
			}
			r.buf.WriteString(fmt.Sprintf("🖼 <a href=\"%s\">%s</a>", dest, html.EscapeString(alt)))
			return ast.WalkSkipChildren, nil
		}
	case *ast.Blockquote:
		if entering {
			r.buf.WriteString("<blockquote>")
		} else {
			r.buf.WriteString("</blockquote>\n\n")
		}
	case *extast.Table:
		if entering {
			r.inTable = true
			r.tableRows = nil
			r.currentRow = nil
		} else {
			r.inTable = false
			r.renderAlignedTable()
		}
	case *extast.TableHeader:
		// Handled via TableRow/TableCell inside it
	case *extast.TableRow:
		if entering {
			r.currentRow = nil
		} else {
			r.tableRows = append(r.tableRows, r.currentRow)
			r.currentRow = nil
		}
	case *extast.TableCell:
		if entering {
			r.currentCell.Reset()
		} else {
			cellText := strings.TrimSpace(r.currentCell.String())
			r.currentRow = append(r.currentRow, cellText)
		}
	case *ast.ThematicBreak:
		// Horizontal rule — render as unicode line
		if entering {
			r.buf.WriteString("\n————————————————\n\n")
		}
	case *ast.RawHTML:
		// Pass through raw HTML tags like <u>, <ins>, etc.
		if entering {
			for i := 0; i < v.Segments.Len(); i++ {
				seg := v.Segments.At(i)
				r.buf.Write(seg.Value(r.src))
			}
		}
	case *ast.HTMLBlock:
		// Pass through HTML blocks
		if entering {
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				r.buf.Write(line.Value(r.src))
			}
		}
	}

	return ast.WalkContinue, nil
}

// writeOrCell writes to the table cell buffer if inside a table, otherwise to the main buffer
func (r *tgHTMLRenderer) writeOrCell(s string) {
	if r.inTable {
		r.currentCell.WriteString(s)
	} else {
		r.buf.WriteString(s)
	}
}

// renderAlignedTable renders collected table rows as a properly aligned
// plain-text table inside <pre> tags.
func (r *tgHTMLRenderer) renderAlignedTable() {
	if len(r.tableRows) == 0 {
		return
	}

	// Determine max column count and max width per column
	maxCols := 0
	for _, row := range r.tableRows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	if maxCols == 0 {
		return
	}

	// First pass: extract plain text (unescaped) for width calculation
	plainRows := make([][]string, len(r.tableRows))
	for i, row := range r.tableRows {
		plainRows[i] = make([]string, maxCols)
		for j := 0; j < maxCols; j++ {
			if j < len(row) {
				// strip any internal HTML tags used for styling inside cells
				plainRows[i][j] = html.UnescapeString(stripHTMLTags(row[j]))
			}
		}
	}

	colWidths := make([]int, maxCols)
	for _, row := range plainRows {
		for j, cell := range row {
			w := runeWidth(cell)
			if w > colWidths[j] {
				colWidths[j] = w
			}
		}
	}

	r.buf.WriteString("<pre>")
	for i, row := range plainRows {
		for j := 0; j < maxCols; j++ {
			cell := row[j]
			w := runeWidth(cell)
			padding := colWidths[j] - w
			if padding < 0 {
				padding = 0
			}
			if j > 0 {
				r.buf.WriteString(" │ ")
			}
			// Escape again for HTML safety inside <pre>
			r.buf.WriteString(html.EscapeString(cell))
			r.buf.WriteString(strings.Repeat(" ", padding))
		}
		r.buf.WriteString("\n")

		// After header row, add separator
		if i == 0 {
			for j := 0; j < maxCols; j++ {
				if j > 0 {
					r.buf.WriteString("─┼─")
				}
				r.buf.WriteString(strings.Repeat("─", colWidths[j]))
			}
			r.buf.WriteString("\n")
		}
	}
	r.buf.WriteString("</pre>\n\n")
}

// runeWidth returns the display width of a string in runes, CJK aware
func runeWidth(s string) int {
	return runewidth.StringWidth(s)
}

// stripHTMLTags removes HTML tags from a string for width calculation
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractTextFromNode extracts plain text content recursively from an AST node
func extractTextFromNode(n ast.Node, src []byte) string {
	var sb strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Segment.Value(src))
		} else {
			sb.WriteString(extractTextFromNode(child, src))
		}
	}
	return sb.String()
}
