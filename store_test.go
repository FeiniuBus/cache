package cache

import (
	"sync"
	"testing"
	"time"
)

var (
	once        sync.Once
	stringStore Getter
	stringc     = make(chan string)
	cacheFills  AtomicInt
)

const (
	stringStoreName = "string-store"
	cacheSize       = 1 << 20
	fromChan        = "from-chan"
)

func testSetup() {
	stringStore = NewStore(stringStoreName, cacheSize, GetterFunc(func(key string, dest Sink) error {
		if key == fromChan {
			key = <-stringc
		}
		cacheFills.Add(1)
		return dest.SetString("ECHO: " + key)
	}))
}

func TestGetDupSuppressString(t *testing.T) {
	once.Do(testSetup)
	resc := make(chan string, 2)
	for i := 0; i < 2; i++ {
		go func() {
			var s string
			if err := stringStore.Get(fromChan, StringSink(&s)); err != nil {
				resc <- "ERROR: " + err.Error()
				return
			}
			resc <- s
		}()
	}

	time.Sleep(250 * time.Millisecond)

	stringc <- "foo"

	for i := 0; i < 2; i++ {
		select {
		case v := <-resc:
			if v != "ECHO: foo" {
				t.Errorf("got %q; want %q", v, "ECHO: foo")
			}
		case <-time.After(5 * time.Second):
			t.Errorf("timeout waiting on getter #%d of 2", i+1)
		}
	}
}

func countFills(f func()) int64 {
	fills0 := cacheFills.Get()
	f()
	return cacheFills.Get() - fills0
}

func TestCaching(t *testing.T) {
	once.Do(testSetup)
	fills := countFills(func() {
		for i := 0; i < 10; i++ {
			var s string
			if err := stringStore.Get("TestCaching-key", StringSink(&s)); err != nil {
				t.Fatal(err)
			}
		}
	})
	if fills != 1 {
		t.Errorf("expected 1 cache fill; got %d", fills)
	}
}
