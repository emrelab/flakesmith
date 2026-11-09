package synctestbubble

import (
	"testing"
	"testing/synctest"
	"time"
)

// Expected: TestSynctest marks this test. One hour of sleep costs no wall
// clock inside a bubble — the verdict cannot vary with machine load.
func TestVirtualHour(t *testing.T) {
	var slept time.Duration
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		time.Sleep(time.Hour)
		slept = time.Since(start)
	})
	if slept != time.Hour {
		t.Errorf("bubble slept %v, want 1h", slept)
	}
}

// Expected: TestSynctest marks this test too — Wait is called directly.
func TestBubbleWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan int)
		go func() { ch <- 1 }()
		synctest.Wait() // all bubble goroutines are now blocked, deterministically
		if got := <-ch; got != 1 {
			t.Errorf("got %d, want 1", got)
		}
	})
}

// Expected: NOT marked. A test with no bubble must not inherit the flag from
// its package — the fact is per test, like TestParallel.
func TestPlain(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("arithmetic broke")
	}
}
