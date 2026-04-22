package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Gradient colors - purplish, pinkish, yellow, and lime
var gradientColors = []string{
	"#FF6B6B", // Coral red
	"#FACC15", // Yellow
	"#50fa7b", // Lime green
	"#7D56F4", // Purple
	"#EC4899", // Pink
}

// NanoBananaBanner is the block-style ASCII art for "BANANA CLI"
var NanoBananaBanner = []string{
	"██████╗  █████╗ ███╗   ██╗ █████╗ ███╗   ██╗ █████╗      ██████╗██╗     ██╗",
	"██╔══██╗██╔══██╗████╗  ██║██╔══██╗████╗  ██║██╔══██╗    ██╔════╝██║     ██║",
	"██████╔╝███████║██╔██╗ ██║███████║██╔██╗ ██║███████║    ██║     ██║     ██║",
	"██╔══██╗██╔══██║██║╚██╗██║██╔══██║██║╚██╗██║██╔══██║    ██║     ██║     ██║",
	"██████╔╝██║  ██║██║ ╚████║██║  ██║██║ ╚████║██║  ██║    ╚██████╗███████╗██║",
	"╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝     ╚═════╝╚══════╝╚═╝",
}

// hexToRGB converts a hex color string to RGB values
func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b int
	if len(hex) == 6 {
		r = hexVal(hex[0:2])
		g = hexVal(hex[2:4])
		b = hexVal(hex[4:6])
	}
	return r, g, b
}

func hexVal(s string) int {
	var val int
	for _, c := range s {
		val *= 16
		if c >= '0' && c <= '9' {
			val += int(c - '0')
		} else if c >= 'a' && c <= 'f' {
			val += int(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			val += int(c - 'A' + 10)
		}
	}
	return val
}

// interpolateColor blends two colors based on t (0.0 to 1.0)
func interpolateColor(c1, c2 string, t float64) string {
	r1, g1, b1 := hexToRGB(c1)
	r2, g2, b2 := hexToRGB(c2)

	r := int(float64(r1) + t*(float64(r2)-float64(r1)))
	g := int(float64(g1) + t*(float64(g2)-float64(g1)))
	b := int(float64(b1) + t*(float64(b2)-float64(b1)))

	return toHexColor(r, g, b)
}

func toHexColor(r, g, b int) string {
	hexChars := "0123456789abcdef"
	hex := []byte{'#', 0, 0, 0, 0, 0, 0}
	hex[1] = hexChars[r/16]
	hex[2] = hexChars[r%16]
	hex[3] = hexChars[g/16]
	hex[4] = hexChars[g%16]
	hex[5] = hexChars[b/16]
	hex[6] = hexChars[b%16]
	return string(hex)
}

// BannerWidth is the visual width of the banner for centering
const BannerWidth = 75

// RenderGradientBanner renders the banner with a horizontal gradient
func RenderGradientBanner() string {
	if len(NanoBananaBanner) == 0 {
		return ""
	}

	// Find the max width of the banner
	maxWidth := 0
	for _, line := range NanoBananaBanner {
		lineWidth := len([]rune(line))
		if lineWidth > maxWidth {
			maxWidth = lineWidth
		}
	}

	colors := gradientColors
	var lines []string

	for _, line := range NanoBananaBanner {
		var lineBuilder strings.Builder
		runes := []rune(line)
		for j, r := range runes {
			// Calculate position in gradient (0.0 to 1.0)
			t := float64(j) / float64(maxWidth)

			// Find which color segment we're in
			segmentCount := len(colors) - 1
			segment := int(t * float64(segmentCount))
			if segment >= segmentCount {
				segment = segmentCount - 1
			}

			// Calculate position within segment
			segmentT := (t * float64(segmentCount)) - float64(segment)

			// Interpolate between colors
			colorHex := interpolateColor(colors[segment], colors[segment+1], segmentT)

			// Apply color to character
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorHex))
			lineBuilder.WriteString(style.Render(string(r)))
		}
		lines = append(lines, lineBuilder.String())
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

// RenderSubtitle renders the subtitle text
func RenderSubtitle() string {
	return SubtleStyle.Render("Gemini AI Image Generator")
}
