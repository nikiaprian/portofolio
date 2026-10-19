package handler

import (
	"regexp"
	"strings"
)

var inlineLogProvidersRE = regexp.MustCompile(`provider_setting=([^\s]+)\s+provider_used=([^\s]+)\s*`)

// AugmentInlineLogProviders fills provider_used from legacy message text and removes that inline fragment.
func AugmentInlineLogProviders(msg, used string) (outUsed, outMsg string) {
	outUsed = strings.TrimSpace(used)
	outMsg = strings.TrimSpace(msg)
	if m := inlineLogProvidersRE.FindStringSubmatch(msg); len(m) == 3 {
		if outUsed == "" {
			outUsed = strings.ToLower(strings.TrimSpace(m[2]))
		}
		outMsg = strings.TrimSpace(inlineLogProvidersRE.ReplaceAllString(msg, ""))
	}
	return outUsed, outMsg
}
