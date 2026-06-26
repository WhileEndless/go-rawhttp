package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// progressBar renders a live, IDM-style download progress bar on stderr. It is
// driven by an atomic byte counter updated by the download workers.
type progressBar struct {
	total      int64
	done       int64 // atomic
	start      time.Time
	width      int
	enabled    bool
	mu         sync.Mutex
	stop       chan struct{}
	wg         sync.WaitGroup
	lastDone   int64
	lastSample time.Time
	speed      float64 // bytes/sec, smoothed
}

func newProgressBar(total int64, enabled bool) *progressBar {
	return &progressBar{
		total:   total,
		width:   40,
		enabled: enabled,
		stop:    make(chan struct{}),
	}
}

// add records n freshly downloaded bytes.
func (p *progressBar) add(n int64) { atomic.AddInt64(&p.done, n) }

// start begins the render loop on a ticker.
func (p *progressBar) startRender() {
	if !p.enabled {
		return
	}
	p.start = time.Now()
	p.lastSample = p.start
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				p.render()
				fmt.Fprintln(os.Stderr)
				return
			case <-ticker.C:
				p.render()
			}
		}
	}()
}

// finish stops the render loop and prints the final state.
func (p *progressBar) finish() {
	if !p.enabled {
		return
	}
	close(p.stop)
	p.wg.Wait()
}

func (p *progressBar) render() {
	done := atomic.LoadInt64(&p.done)

	// Smooth the speed using the instantaneous sample since last render.
	now := time.Now()
	dt := now.Sub(p.lastSample).Seconds()
	if dt > 0 {
		inst := float64(done-p.lastDone) / dt
		if p.speed == 0 {
			p.speed = inst
		} else {
			p.speed = 0.7*p.speed + 0.3*inst // EWMA
		}
	}
	p.lastDone = done
	p.lastSample = now

	var line string
	if p.total > 0 {
		ratio := float64(done) / float64(p.total)
		if ratio > 1 {
			ratio = 1
		}
		filled := int(ratio * float64(p.width))
		bar := strings.Repeat("=", filled)
		if filled < p.width {
			bar += ">" + strings.Repeat(" ", p.width-filled-1)
		}
		eta := "--:--"
		if p.speed > 0 {
			remaining := float64(p.total-done) / p.speed
			eta = fmtDuration(time.Duration(remaining * float64(time.Second)))
		}
		line = fmt.Sprintf("\r[%s] %5.1f%%  %s / %s  %s/s  ETA %s",
			bar, ratio*100, humanBytes(done), humanBytes(p.total), humanBytes(int64(p.speed)), eta)
	} else {
		// Unknown total: show downloaded amount and speed only.
		line = fmt.Sprintf("\r%s downloaded  %s/s", humanBytes(done), humanBytes(int64(p.speed)))
	}

	// Pad to clear any leftover characters from a longer previous line.
	fmt.Fprintf(os.Stderr, "%-78s", line)
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
