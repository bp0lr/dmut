package dnsmanager

import (
	"errors"
	"sync"
	"testing"
)

func TestPoolOwnershipAndConcurrentUpdates(t *testing.T) {
	p, err := New([]string{" 127.0.0.1\r", " "})
	if err != nil {
		t.Fatal(err)
	}
	other, err := New([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.ReportError("127.0.0.1:53", 200)
			if _, err := p.Pick(); err != nil {
				t.Error(err)
			}
			_ = p.Snapshot()
		}()
	}
	wg.Wait()
	snapshot := p.Snapshot()
	if len(snapshot) != 1 || snapshot[0].Errors != 100 {
		t.Fatal(snapshot)
	}
	snapshot[0].Errors = 999
	if p.Snapshot()[0].Errors != 100 || other.Snapshot()[0].Errors != 0 {
		t.Fatal("state escaped pool ownership")
	}
}

func TestUnavailablePool(t *testing.T) {
	if _, err := New([]string{" "}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	p, err := New([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	p.ReportError("127.0.0.1:53", 1)
	if _, err := p.Pick(); err != nil {
		t.Fatal("threshold changed", err)
	}
	p.ReportError("127.0.0.1:53", 1)
	if _, err := p.Pick(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
