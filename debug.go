package main
import (
	"fmt"
	"github.com/keepmind9/clibot/internal/bot"
)
func main() {
	md := "- Item 1\n- Item 2\n  - Nested 1\n  - Nested 2\n- Item 3"
	res := bot.ConvertMarkdownToTelegramHTML(md)
	fmt.Printf("RES: %q\n", res)
}
