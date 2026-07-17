package ui

import "charm.land/lipgloss/v2"

// overlayAt splices box over base at cell (x, y) on a termW x termH canvas.
// Content outside the canvas is clipped; negative positions clamp to 0; base
// regions the box does not cover show through unchanged (ANSI-safe: the cell
// buffer re-emits styles per run and resets at splice boundaries).
func overlayAt(base, box string, termW, termH, x, y int) string {
	if termW < 1 || termH < 1 {
		return base
	}
	canvas := lipgloss.NewCanvas(termW, termH)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(box).X(max(x, 0)).Y(max(y, 0)).Z(1),
	))
	return canvas.Render()
}

// overlayCenter centers box over base on a termW x termH canvas.
func overlayCenter(base, box string, termW, termH int) string {
	x := (termW - lipgloss.Width(box)) / 2
	y := (termH - lipgloss.Height(box)) / 2
	return overlayAt(base, box, termW, termH, x, y)
}
