package breaker

import "time"

type Option func(o *Breaker)

type StrategyOption struct {
	Strategy                int
	FailThreshold           uint64
	ContinuousFailThreshold uint64
	FailRate                float64
	MinCall                 uint64
}

// WithName returns a function to set the name of a Breaker
func WithName(s string) Option {
	return func(options *Breaker) {
		options.name = s
	}
}

// WithWindowInterval returns a function to set the windowInterval of a Breaker
func WithWindowInterval(d time.Duration) Option {
	return func(options *Breaker) {
		options.windowInterval = d
	}
}

// WithStrategyOption returns a function to set the strategy function of a Breaker
func WithStrategyOption(o StrategyOption) Option {
	switch o.Strategy {
	case StrategyFail:
		if o.FailThreshold <= 0 {
			o.FailThreshold = DefaultFailThreshold
		}
		return func(options *Breaker) {
			options.strategyFn = FailStrategyFn(o.FailThreshold)
		}
	case StrategyContinuousFail:
		if o.ContinuousFailThreshold <= 0 {
			o.ContinuousFailThreshold = DefaultContinuousFailThreshold
		}
		return func(options *Breaker) {
			options.strategyFn = ContinuousFailStrategyFn(o.ContinuousFailThreshold)
		}
	case StrategyFailRate:
		if o.FailRate <= 0 || o.MinCall <= 0 {
			o.FailRate = DefaultFailRate
			o.MinCall = DefaultMinCall
		}
		return func(options *Breaker) {
			options.strategyFn = FailRateStrategyFn(o.FailRate, o.MinCall)
		}
	default:
		panic("unknown breaker strategy")
	}
}
