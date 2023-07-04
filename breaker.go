package breaker

import (
	"log"
	"sync"
	"time"
)

// Breaker 熔断器结构体
type Breaker struct {
	name            string        // 熔断器名称
	state           State         // 熔断器状态
	halfOpenMaxCall uint64        // 半开期间最大请求数（半开期间，若请求前的总请求数大于此则丢弃，若请求后的连续成功数大于此则关闭熔断器）
	mu              sync.RWMutex  // 互斥锁
	openTime        time.Time     // 熔断器打开时间
	windowInterval  time.Duration // 窗口间隔
	coolDownTime    time.Duration // 冷却时间（从开到半开的时间间隔）
	metric          Metric        // 指标
	strategyFn      StrategyFn    // 熔断策略
}

const (
	DefaultWindowInterval          = time.Second // 默认窗口间隔
	DefaultCoolDownTime            = time.Second // 默认冷却时间
	DefaultHalfOpenMaxCall         = 5           // 默认半开期间最大请求数
	DefaultFailThreshold           = 10          // 默认失败数阈值
	DefaultContinuousFailThreshold = 10          // 默认连续失败数阈值
	DefaultFailRate                = 0.6         // 默认失败率阈值
	DefaultMinCall                 = 10          // 默认失败率策略的最小请求数
)

var defaultBreaker = Breaker{
	windowInterval:  DefaultWindowInterval,
	coolDownTime:    DefaultCoolDownTime,
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
	if breaker.name == "" {
		breaker.name = "breakerName"
	}
	breaker.newWindow(time.Now())
	return breaker
}

// Do do fn
func (b *Breaker) Do(fn func() error) error {
	log.Printf("start call, breaker: %s, state: %v\n", b.name, b.state)
	// before call
	if err := b.beforeCall(); err != nil {
		log.Printf("end call with error, err: %v, name: %s, state: %v, batch: %d, window start time: %v, "+
			"metric: (all: %d, success: %d, fail: %d, cSuccess: %d, cFail: %d)\n",
			err,
			b.name,
			b.state,
			b.metric.WindowBatch,
			b.metric.WindowStartTime.Format(TimeFormat),
			b.metric.TotalRequest,
			b.metric.TotalSuccess,
			b.metric.TotalFail,
			b.metric.ContinuousSuccess,
			b.metric.ContinuousFail,
		)
		return err
	}

	// recover
	defer func() {
		if err := recover(); err != nil {
			b.afterCall(false)
			//panic(err)
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
		b.metric.WindowStartTime.Format(TimeFormat),
		b.metric.TotalRequest,
		b.metric.TotalSuccess,
		b.metric.TotalFail,
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
		// 过了冷却期，更新熔断器状态为半开
		if b.openTime.Add(b.coolDownTime).Before(now) {
			b.changeState(StateHalfOpen, now)
			log.Printf("breaker: %s cool down passed, switch to half-open\n", b.name)
			return nil
		}
		log.Printf("breaker: %s is open, drop request\n", b.name)
		return ErrStateOpen
	case StateHalfOpen:
		// 请求数 ≥ 半开最大请求数，丢弃请求
		if b.metric.TotalRequest >= b.halfOpenMaxCall {
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

// newWindow create new window
func (b *Breaker) newWindow(t time.Time) {
	log.Println("newWindow....")
	b.metric.NewWindowBatch()
	b.metric.OnReset()
	switch b.state {
	case StateClosed:
		if b.windowInterval == 0 {
			b.metric.WindowStartTime = time.Now()
		} else {
			b.metric.WindowStartTime = t.Add(b.windowInterval)
		}
	case StateOpen:
		b.metric.WindowStartTime = t.Add(b.coolDownTime)
	default:
		b.metric.WindowStartTime = time.Now()
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
		log.Printf("---->%+v\n", b.metric)
		if b.strategyFn(b.metric) {
			log.Println("<------fail")
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
		b.openTime = t
	}
}
