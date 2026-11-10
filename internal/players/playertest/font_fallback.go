//go:build font_fallback || font_complex || font_emoji

package playertest

import (
	"strings"
	"testing"

	"goframework.io/desktop-ui/style"
)

// These tags compiled fallback in; the toolkit turns it on at run time now.
func init() { style.SetFallback(true) }

// RequireCJKFallback checks the environment the shipping font path needs.
// No font is installed or application fallback preference changed by a test.
func RequireCJKFallback(t *testing.T) {
	t.Helper()
	t.Setenv(style.SystemFontsEnv, "1")
	for _, family := range style.FallbackFamilies() {
		if (strings.Contains(family, "CJK") || strings.Contains(family, "Source Han") || family == "PingFang SC" || family == "Microsoft YaHei" || family == "Meiryo" || family == "Malgun Gothic" || family == "WenQuanYi Zen Hei" || family == "Droid Sans Fallback") && style.FontInstalled(family) {
			t.Logf("installed CJK fallback: %s", family)
			return
		}
	}
	t.Skip("font_fallback is enabled but no known CJK fallback family is installed")
}
