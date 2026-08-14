package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Nord palette (https://nordtheme.com) — the same palette the managed fleet
// (Pharos/Harbor/Waypoint) uses.
const (
	nord0  = 0x2E3440 // polar night
	nord1  = 0x3B4252
	nord2  = 0x434C5E
	nord3  = 0x4C566A
	nord4  = 0xD8DEE9 // snow storm
	nord5  = 0xE5E9F0
	nord6  = 0xECEFF4
	nord7  = 0x8FBCBB // frost
	nord8  = 0x88C0D0
	nord9  = 0x81A1C1
	nord10 = 0x5E81AC
	nord11 = 0xBF616A // aurora
	nord12 = 0xD08770
	nord13 = 0xEBCB8B
	nord14 = 0xA3BE8C
	nord15 = 0xB48EAD
)

func nordRGB(v uint32) color.Color {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xFF}
}

// cOr returns the dark or light Nord value depending on the theme variant, so
// dark mode uses the polar-night palette and light mode the snow-storm one.
func cOr(dark, light uint32, v fyne.ThemeVariant) color.Color {
	if v == theme.VariantDark {
		return nordRGB(dark)
	}
	return nordRGB(light)
}

// nordTheme is a Fyne theme re-skinned with the Nord palette, delegating
// fonts/sizes/icons to the default theme.
type nordTheme struct{ base fyne.Theme }

// newNordTheme returns a Nord-skinned theme (dark + light variants).
func newNordTheme() fyne.Theme {
	return nordTheme{base: theme.DefaultTheme()}
}

func (t nordTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		// Dark = nord0; light = a deeper-than-nord6 gray so the white list card
		// visibly pops off the window (not a near-identical off-white).
		return cOr(nord0, nord5, variant)
	case theme.ColorNameForeground:
		return cOr(nord4, nord0, variant)
	case theme.ColorNameButton:
		return cOr(nord1, nord5, variant)
	case theme.ColorNameDisabled:
		return cOr(nord3, nord4, variant)
	case theme.ColorNameDisabledButton:
		return cOr(nord2, nord4, variant)
	case theme.ColorNameInputBackground:
		return cOr(nord1, nord5, variant)
	case theme.ColorNamePlaceHolder:
		return cOr(nord3, nord4, variant)
	case theme.ColorNameMenuBackground:
		return cOr(nord1, nord5, variant)
	case theme.ColorNameHover:
		return cOr(nord2, nord4, variant)
	case theme.ColorNamePressed:
		return cOr(nord0, nord6, variant)
	case theme.ColorNameFocus:
		return nordRGB(nord9)
	case theme.ColorNameSelection:
		return nordRGB(nord9)
	case theme.ColorNameScrollBar:
		return nordRGB(nord3)
	case theme.ColorNameSeparator:
		// Row dividers: a soft-but-visible gray in both themes. Fyne's dark
		// default is pure black (harsh on nord0) and its light default is
		// #E3E3E3 (invisible on the nord5 window) — neither fits Nord.
		return nordRGB(nord3)
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0x33}
	case theme.ColorNamePrimary:
		// Frost-blue accent (Harbor/Nord identity) used for primary buttons and
		// selection. Dark label on it to keep the text at >=4.5:1.
		return nordRGB(nord9)
	case theme.ColorNameForegroundOnPrimary:
		return nordRGB(nord0)
	case theme.ColorNameSuccess:
		return nordRGB(nord14)
	case theme.ColorNameError:
		return nordRGB(nord11)
	case theme.ColorNameWarning:
		return nordRGB(nord13)
	case theme.ColorNameHyperlink:
		return nordRGB(nord8)
	}
	return t.base.Color(name, variant)
}

func (t nordTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t nordTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t nordTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}