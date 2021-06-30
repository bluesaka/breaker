package breaker

import (
	"errors"
	"fmt"
	"log"
	"testing"
	"time"
)

// TestBreaker
// exec `go test -v -run ^TestBreaker$` command to test
func TestBreaker(t *testing.T) {
	strategyOpt := StrategyOption{
		Strategy:      StrategyFail,
		FailThreshold: 2,
	}
	breaker := NewBreaker(WithName("my-breaker"), WithStrategyOption(strategyOpt))
	for i := 0; i < 20; i++ {
		log.Println("i:", i)
		breaker.Call(func() error {
			if i <= 2 || i >= 8 {
				return nil
			} else {
				return errors.New("error")
			}
		})
		fmt.Println()
		time.Sleep(time.Second)
	}
}
