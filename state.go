package breaker

import (
	"errors"
	"fmt"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen

	TimeFormat = "2006-01-02 15:04:05"
)

var (
	ErrStateOpen     = errors.New("circuit breaker is open, drop request")
	ErrStateHalfOpen = errors.New("circuit breaker is half-open, too many calls")
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return fmt.Sprintf("unknown state: %d", s)
	}
}
