package viewBuilder

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

var (
	FrameColor     = lipgloss.Color("#49c6f0")
	BlendColor     = lipgloss.Color("#a784e2")
	HighlightColor = lipgloss.Color("#ff7ae8")
	SuccessColor   = lipgloss.Color("#66e68f")
	WarningColor   = lipgloss.Color("#f0ad43")
	ErrorColor     = lipgloss.Color("#eb6280")
)

func ProgressBlend() []color.Color {
	const hold = 6
	blend := make([]color.Color, 0, hold+2)
	for range hold {
		blend = append(blend, FrameColor)
	}
	blend = append(blend, BlendColor, HighlightColor)
	return blend
}

var (
	TitleStyle     = lipgloss.NewStyle().Foreground(FrameColor).Bold(true)
	BorderStyle    = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(FrameColor)
	HighlightStyle = lipgloss.NewStyle().Foreground(HighlightColor)
	SuccessStyle   = lipgloss.NewStyle().Foreground(SuccessColor)
	WarningStyle   = lipgloss.NewStyle().Foreground(WarningColor)
	ErrorStyle     = lipgloss.NewStyle().Foreground(ErrorColor)
)
