package cache

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	once                   sync.Once
	stringStore, jsonStore GetRemover
	stringc                = make(chan string)
	cacheFills             AtomicInt
)

const (
	stringStoreName = "string-store"
	jsonStoreName   = "json-store"
	cacheSize       = 1 << 20
	fromChan        = "from-chan"
)

type TestMessage struct {
	Name string
	City string
}

func testSetup() {
	stringStore = NewStore(stringStoreName, cacheSize, GetterFunc(func(key string, dest Sink) error {
		if key == fromChan {
			key = <-stringc
		}
		cacheFills.Add(1)
		return dest.SetString("ECHO: " + key)
	}))

	jsonStore = NewStore(jsonStoreName, cacheSize, GetterFunc(func(key string, dest Sink) error {
		if key == fromChan {
			key = <-stringc
		}
		cacheFills.Add(1)
		return dest.SetJSON(&TestMessage{
			Name: "ECHO: " + key,
			City: "SOME-CITY",
		})
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

func TestGetDupSuppressJSON(t *testing.T) {
	once.Do(testSetup)
	resc := make(chan *TestMessage, 2)
	for i := 0; i < 2; i++ {
		go func() {
			tm := new(TestMessage)
			if err := jsonStore.Get(fromChan, JSONSink(tm)); err != nil {
				tm.Name = "ERROR: " + err.Error()
			}
			resc <- tm
		}()
	}

	time.Sleep(250 * time.Millisecond)

	stringc <- "Fluffy"
	want := &TestMessage{
		Name: "ECHO: Fluffy",
		City: "SOME-CITY",
	}
	for i := 0; i < 2; i++ {
		select {
		case v := <-resc:
			if !reflect.DeepEqual(v, want) {
				got, _ := json.Marshal(v)
				w, _ := json.Marshal(want)
				t.Errorf(" Got: %v\nWant: %v", string(got), string(w))
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

func TestRemove(t *testing.T) {
	once.Do(testSetup)
	fills := countFills(func() {
		for i := 0; i < 10; i++ {
			var s string
			if err := stringStore.Get("TestCaching-key1", StringSink(&s)); err != nil {
				t.Fatal(err)
			}
		}

		stringStore.Remove("TestCaching-key1")

		for i := 0; i < 10; i++ {
			var s string
			if err := stringStore.Get("TestCaching-key1", StringSink(&s)); err != nil {
				t.Fatal(err)
			}
		}
	})

	if fills != 2 {
		t.Errorf("expected 2 cache fill; got %d", fills)
	}
}
