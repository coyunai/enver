package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Status string

const (
	StatusRunning  Status = "running"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// Event is one message in a job stream.
type Event struct {
	Type   string `json:"type"` // log | done
	Line   string `json:"line,omitempty"`
	Status Status `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Time   string `json:"time,omitempty"`
}

// Snapshot is a serializable view of a job.
type Snapshot struct {
	ID      string     `json:"id"`
	Kind    string     `json:"kind"`
	Title   string     `json:"title"`
	Status  Status     `json:"status"`
	Error   string     `json:"error,omitempty"`
	Started time.Time  `json:"startedAt"`
	Ended   *time.Time `json:"endedAt,omitempty"`
	Lines   []string   `json:"lines,omitempty"`
}

// Job is a long running operation with a bounded log buffer.
type Job struct {
	ID    string
	Kind  string
	Title string

	mu     sync.Mutex
	status Status
	errMsg string
	lines  []string
	subs   map[chan Event]struct{}
	cancel context.CancelFunc
	start  time.Time
	end    *time.Time
	max    int
}

// Manager owns all jobs for the process.
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewManager() *Manager {
	return &Manager{jobs: make(map[string]*Job)}
}

// Start launches fn in the background and returns its job handle.
func (m *Manager) Start(kind, title string, fn func(ctx context.Context, j *Job) error) *Job {
	m.prune()
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		ID:     newID(),
		Kind:   kind,
		Title:  title,
		status: StatusRunning,
		subs:   make(map[chan Event]struct{}),
		cancel: cancel,
		start:  time.Now(),
		max:    4000,
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.mu.Unlock()

	go func() {
		err := fn(ctx, j)
		j.finish(err)
		cancel()
	}()
	return j
}

func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// prune drops finished jobs older than 24h so a long-running local server does
// not accumulate log buffers forever.
func (m *Manager) prune() {
	cutoff := time.Now().Add(-24 * time.Hour)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, job := range m.jobs {
		job.mu.Lock()
		status := job.status
		ended := job.end
		job.mu.Unlock()
		if status == StatusRunning {
			continue
		}
		when := job.start
		if ended != nil {
			when = *ended
		}
		if when.Before(cutoff) {
			delete(m.jobs, id)
		}
	}
}

func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	list := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		list = append(list, j)
	}
	m.mu.RUnlock()
	out := make([]Snapshot, 0, len(list))
	for _, j := range list {
		out = append(out, j.Snapshot())
	}
	return out
}

func (j *Job) Append(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != StatusRunning {
		return
	}
	j.lines = append(j.lines, line)
	if len(j.lines) > j.max {
		j.lines = j.lines[len(j.lines)-j.max:]
	}
	ev := Event{Type: "log", Line: line, Time: time.Now().Format(time.RFC3339Nano)}
	for ch := range j.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (j *Job) finish(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status != StatusRunning {
		return
	}
	now := time.Now()
	j.end = &now
	switch {
	case err == nil:
		j.status = StatusSuccess
	case err == context.Canceled:
		j.status = StatusCanceled
	default:
		j.status = StatusFailed
		j.errMsg = err.Error()
		j.lines = append(j.lines, err.Error())
	}
	ev := Event{Type: "done", Status: j.status, Error: j.errMsg, Time: now.Format(time.RFC3339Nano)}
	for ch := range j.subs {
		select {
		case ch <- ev:
		default:
		}
		close(ch)
	}
	j.subs = make(map[chan Event]struct{})
}

func (j *Job) Cancel() {
	j.mu.Lock()
	cancel := j.cancel
	j.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (j *Job) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	lines := append([]string(nil), j.lines...)
	return Snapshot{
		ID:      j.ID,
		Kind:    j.Kind,
		Title:   j.Title,
		Status:  j.status,
		Error:   j.errMsg,
		Started: j.start,
		Ended:   j.end,
		Lines:   lines,
	}
}

// Subscribe returns buffered history plus a channel of live events.
func (j *Job) Subscribe() ([]Event, <-chan Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	history := make([]Event, 0, len(j.lines))
	for _, line := range j.lines {
		history = append(history, Event{Type: "log", Line: line})
	}
	ch := make(chan Event, 256)
	if j.status != StatusRunning {
		history = append(history, Event{Type: "done", Status: j.status, Error: j.errMsg})
		close(ch)
		return history, ch
	}
	j.subs[ch] = struct{}{}
	return history, ch
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
