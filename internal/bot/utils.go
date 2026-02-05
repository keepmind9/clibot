package bot

import (
	"github.com/keepmind9/clibot/pkg/constants"
)

// maskSecret masks sensitive information for logging
func maskSecret(s string) string {
	if len(s) <= constants.MinSecretLengthForMasking {
		return "***"
	}
	return s[:constants.SecretMaskPrefixLength] + "***" + s[len(s)-constants.SecretMaskSuffixLength:]
}
