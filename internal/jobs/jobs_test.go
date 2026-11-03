package jobs

import (
	"context"
	"testing"
	"time"
)

func TestJobLifecycle(t *testing.T) {
	m := NewManager()
	release := make(chan struct{})
	job := m.Start("test", "demo", func(ctx context.Context, j *Job) error {
		j.Append("hello")
		<-release
		j.Append("world")
		return nil
	})
	time.Sleep(20 * time.Millisecond)
	history, ch := job.Subscribe()
	if len(history) == 0 || history[0].Line != "hello" {
		t.Fatalf("unexpected history: %#v", history)
	}
	close(release)
	var done Event
	for event := range ch {
		if event.Type == "done" {
			done = event
		}
	}
	if done.Status != StatusSuccess {
		t.Fatalf("unexpected done event: %#v", done)
	}
	snapshot := job.Snapshot()
	if snapshot.Status != StatusSuccess {
		t.Fatalf("unexpected status: %s", snapshot.Status)
	}
	if len(snapshot.Lines) != 2 || snapshot.Lines[1] != "world" {
		t.Fatalf("unexpected lines: %#v", snapshot.Lines)
	}
}

func TestJobFailure(t *testing.T) {
	m := NewManager()
	job := m.Start("test", "fail", func(ctx context.Context, j *Job) error {
		return context.Canceled
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if job.Snapshot().Status != StatusRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot := job.Snapshot()
	if snapshot.Status != StatusCanceled {
		t.Fatalf("unexpected status: %s", snapshot.Status)
	}
}

func TestManagerGetAndList(t *testing.T) {
	m := NewManager()
	job := m.Start("test", "list", func(ctx context.Context, j *Job) error { return nil })
	if _, ok := m.Get(job.ID); !ok {
		t.Fatal("expected job to be registered")
	}
	if len(m.List()) != 1 {
		t.Fatalf("unexpected job list: %#v", m.List())
	}
}
