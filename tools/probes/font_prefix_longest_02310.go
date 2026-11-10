//go:build ignore

// Public longest-prefix oracle: enumerate independently measured prefixes.
// tools/testenv.sh go run -tags font_emoji tools/probes/font_prefix_longest_02310.go
// Also runnable against a pinned v0.23.9 module for genuine before/after.
package main

import (
	"flag"
	"fmt"
	"goframework.io/desktop-ui/style"
	"goframework.io/graphics-paint2d"
	"unicode/utf8"
)

func main() {
	explicit := flag.Bool("explicit-providers", false, "name installed script lenders for an older default-list baseline")
	fixedOnly := flag.Bool("fixed-only", false, "run only the exact measured-width controls")
	flag.Parse()
	style.SetFallbackFamilies()
	if *explicit {
		style.SetFallbackFamilies("Noto Sans Arabic", "Noto Sans Hebrew", "Noto Sans Devanagari", "Noto Sans Thai")
	}
	defer style.SetFallbackFamilies()
	f := style.BakeFont(24, paint2d.White)
	fmt.Printf("primary=%q arabicInstalled=%t indicInstalled=%t thaiInstalled=%t\n", f.Family, style.FontInstalled("Noto Sans Arabic"), style.FontInstalled("Noto Sans Devanagari"), style.FontInstalled("Noto Sans Thai"))
	for _, c := range []struct {
		text  string
		limit float32
	}{{"مرحبا", 12.5}, {"क्षि", 17.4}, {"किरण", 35}} {
		p := f.Prefix(c.text, c.limit)
		rs := []rune(c.text)
		best := ""
		for i := 1; i <= len(rs); i++ {
			if candidate := string(rs[:i]); f.Advance(candidate) <= c.limit {
				best = candidate
			}
		}
		fmt.Printf("FIXED text=%q budget=%g actual=%q actualWidth=%g longest=%q longestWidth=%g\n", c.text, c.limit, p, f.Advance(p), best, f.Advance(best))
	}
	if *fixedOnly {
		return
	}
	for _, text := range []string{"هي", "هل", "هنا", "هيئة", "مرحبا", "سلام", "لا", "لاآ", "क्षि", "नमस्ते", "किरण", "अक्षर", "संगीत", "สวัสดี", "ภาษาไทย", "office", "ffi", "AVATAR"} {
		face := f
		if text == "office" || text == "ffi" || text == "AVATAR" {
			face = style.BakeFamily("Noto Sans", style.WeightRegular, 24, paint2d.White)
		}
		rs := []rune(text)
		width := face.Advance(text)
		found := false
		for step := 1; step < 200; step++ {
			limit := width * float32(step) / 200
			best := ""
			for i := 1; i <= len(rs); i++ {
				candidate := string(rs[:i])
				if face.Advance(candidate) <= limit+1.0/32 {
					best = candidate
				}
			}
			got := face.Prefix(text, limit)
			if utf8.RuneCountInString(got) < utf8.RuneCountInString(best) {
				fmt.Printf("UNDERSHOOT text=%q family=%q full=%g budget=%g actual=%q actualWidth=%g longest=%q longestWidth=%g\n", text, face.Family, width, limit, got, face.Advance(got), best, face.Advance(best))
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("PASS longest text=%q width=%g\n", text, width)
		}
	}
}
