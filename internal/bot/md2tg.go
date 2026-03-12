package bot

import (
	"bytes"
	"fmt"
	"html"
	"strings"

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
}

// ConvertMarkdownToTelegramHTML parses Markdown and generates a Telegram-compatible HTML string.
func ConvertMarkdownToTelegramHTML(mdText string) string {
	if mdText == "" {
		return ""
	}

	src := []byte(mdText)
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Strikethrough,
			extension.Table,
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
			// Standard behavior for Telegram is to separate paragraphs well.
			if n.NextSibling() != nil {
				if n.NextSibling().Kind() == ast.KindList {
					r.buf.WriteString("\n")
				} else {
					r.buf.WriteString("\n\n")
				}
			} else if n.Parent() != nil && n.Parent().Kind() == ast.KindListItem {
				// No trailing newline if we are the last block in a list item, 
				// as it may introduce extra spacing. We'll handle list endings specifically.
				r.buf.WriteString("\n")
			} else {
				r.buf.WriteString("\n\n")
			}
		}
	case *ast.Text:
		if entering {
			val := string(v.Segment.Value(r.src))
			r.buf.WriteString(html.EscapeString(val))
			if v.SoftLineBreak() || v.HardLineBreak() {
				r.buf.WriteString("\n")
			}
		}
	case *ast.String:
		if entering {
			r.buf.WriteString(html.EscapeString(string(v.Value)))
		}
	case *ast.Emphasis:
		if entering {
			if v.Level == 2 {
				r.buf.WriteString("<b>")
			} else {
				r.buf.WriteString("<i>")
			}
		} else {
			if v.Level == 2 {
				r.buf.WriteString("</b>")
			} else {
				r.buf.WriteString("</i>")
			}
		}
	case *extast.Strikethrough:
		if entering {
			r.buf.WriteString("<s>")
		} else {
			r.buf.WriteString("</s>")
		}
	case *ast.CodeSpan:
		if entering {
			r.buf.WriteString("<code>")
		} else {
			r.buf.WriteString("</code>")
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
	case *ast.Link:
		if entering {
			r.buf.WriteString(fmt.Sprintf("<a href=\"%s\">", html.EscapeString(string(v.Destination))))
		} else {
			r.buf.WriteString("</a>")
		}
	case *ast.AutoLink:
		if entering {
			url := html.EscapeString(string(v.URL(r.src)))
			r.buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a>", url, url))
		}
	case *ast.Blockquote:
		if entering {
			r.buf.WriteString("<blockquote>")
		} else {
			r.buf.WriteString("</blockquote>\n\n")
		}
	case *extast.Table:
		if entering {
			r.buf.WriteString("<pre>")
		} else {
			r.buf.WriteString("</pre>\n\n")
		}
	case *extast.TableHeader:
		// We handle rows directly
	case *extast.TableRow:
		if entering {
			// Row content
		} else {
			r.buf.WriteString("\n")
		}
	case *extast.TableCell:
		if entering {
			// Add separator if it's not the first cell
			if n.PreviousSibling() != nil {
				r.buf.WriteString(" | ")
			}
		}
	case *ast.ThematicBreak:
		if !entering {
			r.buf.WriteString("\n---\n\n")
		}
	}

	return ast.WalkContinue, nil
}
