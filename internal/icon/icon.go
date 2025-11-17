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
	width     = 24
	height    = 24
	padX      = 0
	padY      = 2 // Top padding
	padBottom = 2 // Bottom padding to ensure low latency dots are visible
)

// Render generates a PNG icon representing the latency history as a sparkline.
// Creates a monochrome template icon (black on transparent) for proper menubar display.
func Render(history *pinger.History, inFlightAge int64) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	series := history.Snapshot()
	n := len(series)
	if n == 0 {
		// Draw a simple indicator dot when no history exists
		drawDot(img, image.Pt(width/2, height/2))
		return encode(img)
	}

	// Compute x positions and available plot height
	plotW := float64(width - 2*padX)
	if plotW <= 0 {
		plotW = float64(width)
	}
	plotH := float64(height - padY - padBottom)

	// We'll plot last up to 20 samples for density
	if n > 20 {
		series = series[n-20:]
		n = 20
	}

	stepX := 0.0
	if n > 1 {
		stepX = plotW / float64(n-1)
	}

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
			// Map into three vertical bands within the available plot height
			// Each band occupies 1/3 of plotH
			bandHeight := plotH / 3.0
			switch {
			case ms < 100:
				// 0–100ms → occupy top band; scale 0..100 to 0..bandHeight
				ratio := ms / 100.0
				y = bandHeight * ratio
			case ms < 200:
				// 100–200ms → middle band
				ratio := (ms - 100.0) / 100.0
				y = bandHeight + bandHeight*ratio
			default:
				// >=200ms → bottom band; cap at plotH
				ratio := (ms - 200.0) / 100.0
				if ratio > 1.0 {
					ratio = 1.0
				}
				y = 2.0*bandHeight + bandHeight*ratio
			}
		}
		x := float64(padX) + float64(i)*stepX
		yy := float64(padY) + (plotH - y)
		points[i] = image.Pt(int(x+0.5), int(yy+0.5))
	}

	// Draw the sparkline with some thickness
	for i := 1; i < n; i++ {
		drawLine(img, points[i-1], points[i])
	}

	// Mark failures as larger black dots at top
	for i := 0; i < n; i++ {
		if series[i].Failed {
			p := points[i]
			drawDot(img, image.Pt(p.X, padY+2))
		}
	}

	return encode(img)
}

func encode(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawDot(img *image.RGBA, p image.Point) {
	r := image.Rect(p.X-1, p.Y-1, p.X+2, p.Y+2)
	draw.Draw(img, r, &image.Uniform{color.Black}, image.Point{}, draw.Over)
}

// Bresenham-like line drawing
func drawLine(img *image.RGBA, p0, p1 image.Point) {
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
		img.Set(x0, y0, color.Black)
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
