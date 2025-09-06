package ping

import (
	"context"
	"errors"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// Sample represents a single ping result.
type Sample struct {
	Timestamp   time.Time
	Latency     time.Duration
	Failed      bool
	Description string
}

// History is a ring buffer of recent Samples.
type History struct {
	mu       sync.RWMutex
	buffer   []Sample
	capacity int
	nextIdx  int
	size     int
}

func NewHistory(capacity int) *History {
	if capacity <= 0 {
		capacity = 60
	}
	return &History{
		buffer:   make([]Sample, capacity),
		capacity: capacity,
	}
}

func (h *History) Add(sample Sample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buffer[h.nextIdx] = sample
	h.nextIdx = (h.nextIdx + 1) % h.capacity
	if h.size < h.capacity {
		h.size++
	}
}

// Latest returns up to n latest samples, most recent first.
func (h *History) Latest(n int) []Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n <= 0 || n > h.size {
		n = h.size
	}
	out := make([]Sample, 0, n)
	// start from last added (nextIdx-1)
	idx := (h.nextIdx - 1 + h.capacity) % h.capacity
	for i := 0; i < n; i++ {
		out = append(out, h.buffer[idx])
		idx = (idx - 1 + h.capacity) % h.capacity
	}
	return out
}

// Snapshot returns all samples in chronological order (oldest to newest).
func (h *History) Snapshot() []Sample {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Sample, 0, h.size)
	start := (h.nextIdx - h.size + h.capacity) % h.capacity
	for i := 0; i < h.size; i++ {
		idx := (start + i) % h.capacity
		out = append(out, h.buffer[idx])
	}
	return out
}

type Manager struct {
	target        string
	interval      time.Duration
	timeout       time.Duration
	history       *History
	mu            sync.RWMutex
	inFlightStart time.Time
	inFlight      bool
	cancel        context.CancelFunc
	resultCh      chan Sample
}

func NewManager(target string, interval time.Duration, timeout time.Duration, historyCapacity int) *Manager {
	if target == "" {
		target = "1.1.1.1"
	}
	if interval <= 0 {
		interval = time.Second
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Manager{
		target:   target,
		interval: interval,
		timeout:  timeout,
		history:  NewHistory(historyCapacity),
		resultCh: make(chan Sample, 100),
	}
}

func (m *Manager) Results() <-chan Sample { return m.resultCh }

func (m *Manager) History() *History { return m.history }

func (m *Manager) IsInFlight() (bool, time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.inFlight {
		return false, 0
	}
	return true, time.Since(m.inFlightStart)
}

func (m *Manager) markInFlight(start bool) {
	m.mu.Lock()
	if start {
		m.inFlight = true
		m.inFlightStart = time.Now()
	} else {
		m.inFlight = false
	}
	m.mu.Unlock()
}

func (m *Manager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.loop(ctx)
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *Manager) loop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	// fire immediately
	m.doPing(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doPing(ctx)
		}
	}
}

func (m *Manager) doPing(ctx context.Context) {
	m.markInFlight(true)
	defer m.markInFlight(false)

	pinger, err := probing.NewPinger(m.target)
	if err != nil {
		m.emitFailure(err)
		return
	}
	// Use unprivileged mode to avoid requiring root.
	pinger.SetPrivileged(false)
	pinger.Count = 1
	pinger.Timeout = m.timeout

	start := time.Now()
	err = pinger.Run() // Blocks until finished
	rtt := time.Since(start)
	if err != nil {
		m.emitFailure(err)
		return
	}
	stats := pinger.Statistics()
	var latency time.Duration
	if stats != nil && stats.AvgRtt > 0 {
		latency = stats.AvgRtt
	} else {
		// Fallback to measured elapsed
		latency = rtt
	}
	sample := Sample{
		Timestamp:   time.Now(),
		Latency:     latency,
		Failed:      false,
		Description: "ok",
	}
	m.history.Add(sample)
	select {
	case m.resultCh <- sample:
	default:
		// drop if full
	}
}

func (m *Manager) emitFailure(err error) {
	if err == nil {
		err = errors.New("ping failed")
	}
	sample := Sample{
		Timestamp:   time.Now(),
		Latency:     0,
		Failed:      true,
		Description: err.Error(),
	}
	m.history.Add(sample)
	select {
	case m.resultCh <- sample:
	default:
	}
}
