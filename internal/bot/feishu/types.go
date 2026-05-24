package feishu

import (
	"encoding/json"

	"github.com/keepmind9/clibot/internal/logger"
)

// TextContent represents the JSON structure of Feishu text message content
type TextContent struct {
	Text string `json:"text"`
}

// extractTextContent extracts actual text from Feishu message content
// Feishu text message format: {"text":"actual message"}
func extractTextContent(content string) string {
	var tc TextContent
	if err := json.Unmarshal([]byte(content), &tc); err != nil {
		logger.WithField("error", err).Debug("failed-to-parse-text-content-json")
		return content
	}
	return tc.Text
}

// escapeJSONString escapes special characters for JSON string content
func escapeJSONString(s string) string {
	result := ""
	for _, c := range s {
		switch c {
		case '"':
			result += "\\\""
		case '\\':
			result += "\\\\"
		case '\n':
			result += "\\n"
		case '\r':
			result += "\\r"
		case '\t':
			result += "\\t"
		default:
			result += string(c)
		}
	}
	return result
}
