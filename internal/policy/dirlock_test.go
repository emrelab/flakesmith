package policy_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emrelab/flakesmith/internal/policy"
)

// The lock must really serialize different goroutines.
//
// This test exists because the first version used a refcount, and a refcount
// cannot tell "the same call chain came back" from "another goroutine arrived",
// so 8 goroutines were entering at the same time.
func TestLockSerializesGoroutines(t *testing.T) {
	dir := t.TempDir()
	var inside, maxInside int32
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, release, err := policy.Locked(context.Background(), dir)
			if err != nil {
				t.Error(err)
				return
			}
			defer release()

			n := atomic.AddInt32(&inside, 1)
			for {
				cur := atomic.LoadInt32(&maxInside)
				if n <= cur || atomic.CompareAndSwapInt32(&maxInside, cur, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(&inside, -1)
		}()
	}
	wg.Wait()

	if maxInside != 1 {
		t.Fatalf("%d holders at once: the lock is not working", maxInside)
	}
}

// A nested call must not deadlock: Runner.Run also wants the lock while the probe holds it.
func TestLockIsReentrantViaContext(t *testing.T) {
	dir := t.TempDir()
	ctx, release, err := policy.Locked(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	if !policy.Held(ctx, dir) {
		t.Fatal("context does not carry the lock")
	}

	_, inner, err := policy.Locked(ctx, dir)
	if err != nil {
		t.Fatalf("nested lock returned an error: %v", err)
	}
	inner()
}

// Different directories must not wait for each other.
func TestLockIsPerDirectory(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	_, relA, err := policy.Locked(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer relA()

	done := make(chan error, 1)
	go func() {
		_, rel, err := policy.Locked(context.Background(), b)
		if err == nil {
			rel()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("different directory blocked")
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	_, release, err := policy.Locked(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	release()
	release() // must not panic
}
