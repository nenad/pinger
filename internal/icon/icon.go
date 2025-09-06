package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	pinger "github.com/nenad/pinger/internal/ping"
)

const (
	width  = 24
	height = 24
	padX   = 3
	padY   = 4
)

var (
	colNeutral = color.RGBA{R: 220, G: 220, B: 220, A: 255}
	colYellow  = color.RGBA{R: 255, G: 200, B: 0, A: 255}
	colOrange  = color.RGBA{R: 255, G: 120, B: 0, A: 255}
	colRed     = color.RGBA{R: 230, G: 30, B: 30, A: 255}
)

// BorderColorFor returns the color representing the current state for the inner border.
// If inFlight is true, ageMs decides the color thresholding.
// Otherwise, the average of the last up to 5 pings controls the color; any failure forces red.
func BorderColorFor(history *pinger.History, inFlight bool, ageMs int64) color.Color {
	if inFlight {
		switch {
		case ageMs >= 300:
			return colOrange
		case ageMs >= 100:
			return colYellow
		default:
			return colNeutral
		}
	}
	last := history.Latest(5)
	if len(last) == 0 {
		return colNeutral
	}
	var sumMs float64
	var countOK int
	anyFail := false
	for _, s := range last {
		if s.Failed {
			anyFail = true
			continue
		}
		sumMs += float64(s.Latency.Microseconds()) / 1000.0
		countOK++
	}
	if anyFail {
		return colRed
	}
	if countOK == 0 {
		return colNeutral
	}
	avg := sumMs / float64(countOK)
	switch {
	case avg >= 300:
		return colOrange
	case avg >= 100:
		return colYellow
	default:
		return colNeutral
	}
}

// Render generates a PNG icon representing the latency history as a sparkline.
// The line color reflects current state via ColorFor.
func Render(history *pinger.History, inFlight bool, ageMs int64) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Transparent background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 0}}, image.Point{}, draw.Src)

	series := history.Snapshot()
	n := len(series)
	if n == 0 {
		return encode(img)
	}

	// Determine vertical scale. Use max of observed latency and a floor for better visuals.
	var maxMs float64 = 300.0
	for _, s := range series {
		if s.Failed {
			continue
		}
		ms := float64(s.Latency.Microseconds()) / 1000.0
		if ms > maxMs {
			maxMs = ms
		}
	}
	// Compute x positions
	plotW := float64(width - 2*padX)
	plotH := float64(height - 2*padY)
	if plotW <= 0 {
		plotW = float64(width)
	}
	if plotH <= 0 {
		plotH = float64(height)
	}

	// We'll plot last up to 20 samples for density
	if n > 20 {
		series = series[n-20:]
		n = 20
	}

	stepX := 0.0
	if n > 1 {
		stepX = plotW / float64(n-1)
	}

	// Sparkline stroke is always neutral; the border indicates status
	stroke := colNeutral

	// Convert samples to points (oldest to newest)
	points := make([]image.Point, n)
	for i := 0; i < n; i++ {
		s := series[i]
		var y float64
		if s.Failed {
			// Place failure marker at top; sparkline point at top as a gap indicator
			y = 0
		} else {
			ms := float64(s.Latency.Microseconds()) / 1000.0
			// Baseline at bottom for low/consistent latency: y grows with ms
			ratio := math.Min(1.0, ms/maxMs)
			y = plotH * ratio
		}
		x := float64(padX) + float64(i)*stepX
		yy := float64(height-padY) - y
		points[i] = image.Pt(int(x+0.5), int(yy+0.5))
	}

	// Draw axes baseline (optional faint) - skipped to keep visual clean

	// Draw the sparkline with thickness 3
	for i := 1; i < n; i++ {
		drawThickLine(img, points[i-1], points[i], stroke, 3)
	}

	// Mark failures as red dots at top
	for i := 0; i < n; i++ {
		if series[i].Failed {
			p := points[i]
			drawDot(img, image.Pt(p.X, padY), colRed)
		}
	}

	// Draw inner border (3px) indicating current status color only if not neutral
	borderColor := BorderColorFor(history, inFlight, ageMs)
	if borderColor != colNeutral {
		drawInnerBorder(img, borderColor, 3)
	}

	return encode(img)
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawDot(img *image.RGBA, p image.Point, c color.Color) {
	r := image.Rect(p.X-1, p.Y-1, p.X+2, p.Y+2)
	draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Over)
}

// Bresenham-like line drawing
func drawLine(img *image.RGBA, p0, p1 image.Point, c color.Color) {
	x0, y0 := p0.X, p0.Y
	x1, y1 := p1.X, p1.Y
	dx := int(math.Abs(float64(x1 - x0)))
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	dy := -int(math.Abs(float64(y1 - y0)))
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		img.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// drawThickLine draws a simple thick line by drawing multiple offset lines around the center.
func drawThickLine(img *image.RGBA, p0, p1 image.Point, c color.Color, thickness int) {
	if thickness <= 1 {
		drawLine(img, p0, p1, c)
		return
	}
	// Offsets distribute around center; naive but effective at small sizes
	half := thickness / 2
	for off := -half; off <= half; off++ {
		// Offset in both axes to approximate roundness
		drawLine(img, image.Pt(p0.X+off, p0.Y), image.Pt(p1.X+off, p1.Y), c)
		drawLine(img, image.Pt(p0.X, p0.Y+off), image.Pt(p1.X, p1.Y+off), c)
	}
}

// drawInnerBorder draws a border along the inside edges of the icon with given thickness.
func drawInnerBorder(img *image.RGBA, c color.Color, thickness int) {
	if thickness <= 0 {
		return
	}
	for t := 0; t < thickness; t++ {
		// Top
		drawRect(img, image.Rect(0+t, 0+t, width-t, 1+t), c)
		// Bottom
		drawRect(img, image.Rect(0+t, height-1-t, width-t, height-t), c)
		// Left
		drawRect(img, image.Rect(0+t, 0+t, 1+t, height-t), c)
		// Right
		drawRect(img, image.Rect(width-1-t, 0+t, width-t, height-t), c)
	}
}

func drawRect(img *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Over)
}
