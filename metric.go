package breaker

import "time"

type Metric struct {
	WindowBatch       uint64
	WindowStartTime   time.Time
	CountAll          uint64
	CountSuccess      uint64
	CountFail         uint64
	ContinuousSuccess uint64
	ContinuousFail    uint64
}

// NewWindowBatch new window batch
func (m *Metric) NewWindowBatch() {
	m.WindowBatch++
}

// onSuccess on success call
func (m *Metric) onSuccess() {
	m.CountAll++
	m.CountSuccess++
	m.ContinuousSuccess++
	m.CountFail = 0
}

// onFail on fail call
func (m *Metric) onFail() {
	m.CountAll++
	m.CountFail++
	m.ContinuousFail++
	m.ContinuousSuccess = 0
}

// OnReset reset window
func (m *Metric) OnReset() {
	m.CountAll = 0
	m.CountSuccess = 0
	m.CountFail = 0
	m.ContinuousSuccess = 0
	m.ContinuousFail = 0
}
