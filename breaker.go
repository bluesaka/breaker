package breaker

import (
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrStateOpen     = errors.New("circuit breaker is open")
	ErrStateHalfOpen = errors.New("circuit breaker is half-open, too many calls")
)

// Breaker represents a circuit breaker.
type Breaker struct {
	name            string        // breaker name
	state           State         // breaker state
	mu              sync.RWMutex  // rw lock
	stateOpenTime   time.Time     // time of breaker open
	windowInterval  time.Duration // metric window interval
	sleepTimeout    time.Duration // breaker cool-down period
	metric          Metric        // breaker window metric
	halfOpenMaxCall uint64        // max call when breaker is half-open
	strategyFn      StrategyFn    // breaker strategy function
}

const (
	DefaultWindowInterval          = time.Second * 5
	DefaultSleepTimeout            = time.Second * 6
	DefaultHalfOpenMaxCall         = 5
	DefaultFailThreshold           = 3
	DefaultContinuousFailThreshold = 2
	DefaultFailRate                = 0.6
	DefaultMinCall                 = 2
)

var defaultBreaker = Breaker{
	windowInterval:  DefaultWindowInterval,
	sleepTimeout:    DefaultSleepTimeout,
	halfOpenMaxCall: DefaultHalfOpenMaxCall,
	strategyFn:      FailStrategyFn(DefaultFailThreshold),
}

// NewBreaker returns a Breaker object.
// opts can be used to customize the Breaker.
func NewBreaker(opts ...Option) *Breaker {
	breaker := &defaultBreaker
	for _, opt := range opts {
		opt(breaker)
	}
	if len(breaker.name) == 0 {
		breaker.name = "rand-breaker-name"
	}
	breaker.newWindow(time.Now())
	return breaker
}

// Call call fn
func (b *Breaker) Call(fn func() error) error {
	log.Printf("start call, breaker: %s, state: %v\n", b.name, b.state)
	// before call
	if err := b.beforeCall(); err != nil {
		log.Printf("end call with error, err: %v, name: %s, state: %v, batch: %d, window start time: %v, "+
			"metric: (all: %d, success: %d, fail: %d, cSuccess: %d, cFail: %d)\n",
			err,
			b.name,
			b.state,
			b.metric.WindowBatch,
			b.metric.WindowStartTime.Format("2006-01-02 15:04:05"),
			b.metric.CountAll,
			b.metric.CountSuccess,
			b.metric.CountFail,
			b.metric.ContinuousSuccess,
			b.metric.ContinuousFail,
		)
		return err
	}

	// panic handle
	defer func() {
		if err := recover(); err != nil {
			b.afterCall(false)
			panic(err)
		}
	}()

	// call function
	err := fn()

	// after call
	b.afterCall(err == nil)
	log.Printf("end call, name: %s, state:%v, batch: %d, window start time: %v, "+
		"metric: (all: %d, success: %d, fail: %d, cSuccess: %d, cFail: %d)\n",
		b.name,
		b.state,
		b.metric.WindowBatch,
		b.metric.WindowStartTime.Format("2006-01-02 15:04:05"),
		b.metric.CountAll,
		b.metric.CountSuccess,
		b.metric.CountFail,
		b.metric.ContinuousSuccess,
		b.metric.ContinuousFail,
	)

	return err
}

// beforeCall before call
func (b *Breaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	switch b.state {
	case StateOpen:
		// cool down
		if b.stateOpenTime.Add(b.sleepTimeout).Before(now) {
			b.changeState(StateHalfOpen, now)
			log.Printf("breaker: %s cool down passed, switch to half-open\n", b.name)
			return nil
		}
		log.Printf("breaker: %s is open, drop request\n", b.name)
		return ErrStateOpen
	case StateHalfOpen:
		if b.metric.CountAll >= b.halfOpenMaxCall {
			log.Printf("breaker: %s is half-open, drop request that beyond max threshold\n", b.name)
			return ErrStateHalfOpen
		}
	default:
		if !b.metric.WindowStartTime.IsZero() && b.metric.WindowStartTime.Before(now) {
			b.newWindow(now)
		}
	}
	return nil
}

// after call
func (b *Breaker) afterCall(result bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if result {
		b.onSuccess(time.Now())
	} else {
		b.onFail(time.Now())
	}
}

// new Window create new window
func (b *Breaker) newWindow(t time.Time) {
	log.Println("newWindow....")
	b.metric.NewWindowBatch()
	b.metric.OnReset()
	switch b.state {
	case StateClosed:
		if b.windowInterval == 0 {
			b.metric.WindowStartTime = time.Time{}
		} else {
			b.metric.WindowStartTime = t.Add(b.windowInterval)
		}
	case StateOpen:
		b.metric.WindowStartTime = t.Add(b.sleepTimeout)
	default:
		b.metric.WindowStartTime = time.Time{}
	}
}

// onSuccess call on success
func (b *Breaker) onSuccess(t time.Time) {
	b.metric.onSuccess()
	if b.state == StateHalfOpen && b.metric.ContinuousSuccess >= b.halfOpenMaxCall {
		b.changeState(StateClosed, t)
	}
}

// onFail call on failure
func (b *Breaker) onFail(t time.Time) {
	b.metric.onFail()
	switch b.state {
	case StateClosed:
		if b.strategyFn(b.metric) {
			b.changeState(StateOpen, t)
		}
	case StateHalfOpen:
		b.changeState(StateOpen, t)
	}
}

// changeState change breaker state
func (b *Breaker) changeState(state State, t time.Time) {
	if b.state == state {
		return
	}
	b.state = state
	b.newWindow(t)
	if state == StateOpen {
		b.stateOpenTime = t
	}
}
