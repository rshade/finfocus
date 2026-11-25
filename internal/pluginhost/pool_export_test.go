package pluginhost

import "time"

// PoolHoldsForTest sums the holds of every entry the pool still tracks, so a
// test can wait until requesters have joined a launch instead of sleeping.
func PoolHoldsForTest(p *Pool) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for entry := range p.live {
		total += entry.holds
	}
	return total
}

// PoolClosedForTest reports whether Close has started.
func PoolClosedForTest(p *Pool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

// PoolLearnWaitersForTest counts requests waiting for a binary's first launch
// to report its settings.
func PoolLearnWaitersForTest(p *Pool) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.learnWaiters
}

// SetBindTimeoutForTest shortens how long launcher waits for a plugin to bind
// and to announce its port.
func SetBindTimeoutForTest(launcher *ProcessLauncher, timeout time.Duration) {
	launcher.bindTimeout = timeout
}
