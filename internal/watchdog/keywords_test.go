package watchdog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessKeyWords_Tab_ConvertsToTabKey(t *testing.T) {
	result := ProcessKeyWords("tab")
	assert.Equal(t, "C-i", result)
}

func TestProcessKeyWords_TabUpperCase_ConvertsToTabKey(t *testing.T) {
	result := ProcessKeyWords("TAB")
	assert.Equal(t, "C-i", result)
}

func TestProcessKeyWords_TabMixedCase_ConvertsToTabKey(t *testing.T) {
	result := ProcessKeyWords("TaB")
	assert.Equal(t, "C-i", result)
}

func TestProcessKeyWords_TabWithSpaces_TrimsAndConverts(t *testing.T) {
	result := ProcessKeyWords("  tab  ")
	assert.Equal(t, "C-i", result)
}

func TestProcessKeyWords_Esc_ConvertsToEscapeKey(t *testing.T) {
	result := ProcessKeyWords("esc")
	assert.Equal(t, "C-[", result)
}

func TestProcessKeyWords_EscUpperCase_ConvertsToEscapeKey(t *testing.T) {
	result := ProcessKeyWords("ESC")
	assert.Equal(t, "C-[", result)
}

func TestProcessKeyWords_Stab_ConvertsToShiftTab(t *testing.T) {
	result := ProcessKeyWords("stab")
	assert.Equal(t, "\x1b[Z", result)
}

func TestProcessKeyWords_StabUpperCase_ConvertsToShiftTab(t *testing.T) {
	result := ProcessKeyWords("STAB")
	assert.Equal(t, "\x1b[Z", result)
}

func TestProcessKeyWords_STabDash_ConvertsToShiftTab(t *testing.T) {
	result := ProcessKeyWords("s-tab")
	assert.Equal(t, "\x1b[Z", result)
}

func TestProcessKeyWords_STabDashUpperCase_ConvertsToShiftTab(t *testing.T) {
	result := ProcessKeyWords("S-TAB")
	assert.Equal(t, "\x1b[Z", result)
}

func TestProcessKeyWords_STabDashWithSpaces_TrimsAndConverts(t *testing.T) {
	result := ProcessKeyWords("  s-tab  ")
	assert.Equal(t, "\x1b[Z", result)
}

func TestProcessKeyWords_Enter_ConvertsToEnterKey(t *testing.T) {
	result := ProcessKeyWords("enter")
	assert.Equal(t, "C-m", result)
}

func TestProcessKeyWords_NotAKeyword_ReturnsOriginal(t *testing.T) {
	input := "hello world"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}

func TestProcessKeyWords_KeywordInSentence_ReturnsOriginal(t *testing.T) {
	input := "press tab to autocomplete"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}

func TestProcessKeyWords_Tablet_ReturnsOriginal(t *testing.T) {
	input := "tablet"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}

func TestProcessKeyWords_EscapeWord_ReturnsOriginal(t *testing.T) {
	input := "escape"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}

func TestProcessKeyWords_WithLeadingSpace_TrimsAndConverts(t *testing.T) {
	input := " tab"
	result := ProcessKeyWords(input)
	assert.Equal(t, "C-i", result) // Trims spaces then converts
}

func TestProcessKeyWords_WithTrailingSpace_TrimsAndConverts(t *testing.T) {
	input := "tab "
	result := ProcessKeyWords(input)
	assert.Equal(t, "C-i", result) // Trims spaces then converts
}

func TestProcessKeyWords_EmptyString_ReturnsEmpty(t *testing.T) {
	input := ""
	result := ProcessKeyWords(input)
	assert.Equal(t, "", result)
}

func TestProcessKeyWords_OnlySpaces_ReturnsEmpty(t *testing.T) {
	input := "   "
	result := ProcessKeyWords(input)
	assert.Equal(t, "", result) // Trims to empty string
}

func TestProcessKeyWords_HelpMessage_ReturnsOriginal(t *testing.T) {
	input := "help me with tab completion"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}

func TestProcessKeyWords_Ctrlc_ConvertsToCtrlC(t *testing.T) {
	result := ProcessKeyWords("ctrlc")
	assert.Equal(t, "C-c", result)
}

func TestProcessKeyWords_CtrlcUpperCase_ConvertsToCtrlC(t *testing.T) {
	result := ProcessKeyWords("CTRLC")
	assert.Equal(t, "C-c", result)
}

func TestProcessKeyWords_CtrlDashC_ConvertsToCtrlC(t *testing.T) {
	result := ProcessKeyWords("ctrl-c")
	assert.Equal(t, "C-c", result)
}

func TestProcessKeyWords_CtrlDashCUpperCase_ConvertsToCtrlC(t *testing.T) {
	result := ProcessKeyWords("CTRL-C")
	assert.Equal(t, "C-c", result)
}

func TestProcessKeyWords_CtrlcWithSpaces_TrimsAndConverts(t *testing.T) {
	result := ProcessKeyWords("  ctrlc  ")
	assert.Equal(t, "C-c", result)
}

func TestProcessKeyWords_CtrlcInText_ReturnsOriginal(t *testing.T) {
	input := "press ctrlc to stop"
	result := ProcessKeyWords(input)
	assert.Equal(t, input, result)
}
