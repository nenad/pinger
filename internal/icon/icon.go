package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os/exec"
	"strings"
	"sync"
	"time"

	pinger "github.com/nenad/pinger/internal/ping"
)

const (
	width  = 24
	height = 24
	padX   = 0
	padY   = 0
)

var (
	colLine    = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	colNeutral = color.RGBA{R: 220, G: 220, B: 220, A: 255}
	colYellow  = color.RGBA{R: 255, G: 200, B: 0, A: 255}
	colOrange  = color.RGBA{R: 255, G: 120, B: 0, A: 255}
	colRed     = color.RGBA{R: 230, G: 30, B: 30, A: 255}
)

var (
	appearanceMu          sync.Mutex
	cachedIsDark          bool
	lastAppearanceCheck   time.Time
	appearanceCacheWindow = 5 * time.Second
)

// isDarkAppearance returns true when macOS is in dark mode. Result is cached briefly.
func isDarkAppearance() bool {
	appearanceMu.Lock()
	defer appearanceMu.Unlock()
	if time.Since(lastAppearanceCheck) < appearanceCacheWindow {
		return cachedIsDark
	}
	lastAppearanceCheck = time.Now()
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// Key is absent in light mode
		cachedIsDark = false
		return cachedIsDark
	}
	cachedIsDark = strings.Contains(strings.ToLower(string(out)), "dark")
	return cachedIsDark
}

// BorderColorFor returns the color representing the current state for the inner border.
// If inFlight is true, ageMs decides the color thresholding.
// Otherwise, the average of the last up to 5 pings controls the color; any failure forces red.
func BorderColorFor(history *pinger.History, inFlight int64) color.Color {
	last := history.Latest(5)
	if len(last) == 0 {
		return colNeutral
	}
	// We start with the in-flight current travel
	var sumMs = float64(inFlight)
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
func Render(history *pinger.History, inFlightAge int64) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Paint background color
	borderColor := BorderColorFor(history, inFlightAge)
	if borderColor != colNeutral {
		draw.Draw(img, img.Bounds(), &image.Uniform{borderColor}, image.Point{}, draw.Src)
	}

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

	// Sparkline stroke adapts to appearance: white on dark, black on light
	var stroke color.Color = colLine
	if isDarkAppearance() {
		stroke = color.White
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
			// Baseline at bottom for low/consistent latency: y grows with ms
			ratio := math.Min(1.0, ms/maxMs)
			y = plotH * ratio
		}
		x := float64(padX) + float64(i)*stepX
		yy := float64(height-padY) - y
		points[i] = image.Pt(int(x+0.5), int(yy+0.5))
	}

	// Draw axes baseline (optional faint) - skipped to keep visual clean

	// Draw the sparkline with some thickness
	for i := 1; i < n; i++ {
		drawLine(img, points[i-1], points[i], stroke)
	}

	// Mark failures as red dots at top
	for i := 0; i < n; i++ {
		if series[i].Failed {
			p := points[i]
			drawDot(img, image.Pt(p.X, padY), colRed)
		}
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
