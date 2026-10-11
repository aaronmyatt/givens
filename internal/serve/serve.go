// Package serve is `givens serve`: internal/app over JSON lines on stdin and stdout, for the
// macOS app. The wire format is written down in
// macos/Sources/GivensUI/Core/ServeProtocol.swift; each method mirrors one internal/app call,
// and every reply carries the whole app snapshot.
// Ref: https://jsonlines.org
package serve

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"givens/internal/app"
	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// LineLengthBytesMax matches ServeClient.lineLengthMax in Swift.
	LineLengthBytesMax = 1 << 24
	// EffectsPerCallMax bounds what one call asks the app to do.
	EffectsPerCallMax = 64
	// seamPrefix starts the methods only the acceptance suites may call.
	seamPrefix = "test."
	// pushInterval is how often serve looks for a snapshot the app hasn't seen: files another
	// tool wrote, or setup output. Under watch.Interval's 500 ms, so a change shows within 2 s.
	pushInterval = 250 * time.Millisecond
)

// Error codes. Swift's CoreError carries them.
const (
	CodeNotBuilt      = "not_built"
	CodeNotFound      = "not_found"
	CodeBadRequest    = "bad_request"
	CodeUnknownMethod = "unknown_method"
	CodeNoTestSeams   = "no_test_seams"
	CodeFailed        = "failed"
)

// ErrBadRequest marks a request whose params the method can't use.
var ErrBadRequest = errors.New("bad request")

// request is one line from the app.
type request struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// reply is one line back. Exactly one of Error and Snapshot is set.
type reply struct {
	ID       int              `json:"id"`
	Error    *coreError       `json:"error,omitempty"`
	Snapshot *app.AppSnapshot `json:"snapshot,omitempty"`
	Effects  []Effect         `json:"effects,omitempty"`
	Value    any              `json:"value,omitempty"`
}

// coreError is an operating error the app shows. Swift's CoreError.
type coreError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// handler runs one method and returns its value, or nil when it has none.
type handler func(params params) (any, error)

// Server answers one app's requests, and pushes snapshots the app hasn't asked for.
type Server struct {
	core     *app.App
	effects  *recorder
	clock    *clock
	seams    bool
	handlers map[string]handler

	// mutex keeps the request loop and the pusher from using the app or out at once: the app
	// isn't safe for concurrent use.
	mutex sync.Mutex
	// sent is the last snapshot the app got, as JSON; nil before the first request.
	sent []byte
}

// New makes a server over a new app whose data lives in appData. seams lets "test.*"
// methods through; only the acceptance suites set it, through GIVENS_TEST_SEAMS=1.
func New(appData string, seams bool) (*Server, error) {
	assert.That(appData != "", "the app has a data folder")
	effects := &recorder{}
	clock := &clock{}
	core, err := app.New(app.Config{
		AppDataDir: appData, Clipboard: effects, Trash: effects, Clock: clock,
	})
	if err != nil {
		return nil, err
	}
	if err := core.UseURLOpener(effects); err != nil {
		return nil, err
	}
	server := &Server{core: core, effects: effects, clock: clock, seams: seams}
	server.handlers = server.table()
	assert.That(len(server.handlers) > 0, "the server answers some methods")
	return server, nil
}

// Serve answers requests from in, one per line, until in ends. Meanwhile it pushes a
// snapshot event whenever the app's snapshot changes without a request.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	assert.That(s != nil, "serve runs on a server")
	assert.That(in != nil && out != nil, "serve has both pipes")
	// Buffer raises Scanner's line limit to LineLengthBytesMax.
	// Ref: https://pkg.go.dev/bufio#Scanner.Buffer
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64<<10), LineLengthBytesMax)
	encoder := json.NewEncoder(out)
	stop := make(chan struct{})
	defer close(stop)
	go s.push(encoder, stop)
	// bound: Scan stops when the app closes serve's stdin.
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := s.respond(encoder, line); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// respond answers one line and remembers the snapshot sent with it.
func (s *Server) respond(encoder *json.Encoder, line string) error {
	assert.That(encoder != nil, "respond writes somewhere")
	assert.That(line != "", "a request line has text")
	s.mutex.Lock()
	defer s.mutex.Unlock()
	answer := s.answer(line)
	if answer.Snapshot != nil {
		sent, err := json.Marshal(answer.Snapshot)
		if err != nil {
			return err
		}
		s.sent = sent
	}
	return encoder.Encode(answer)
}

// event is a line serve sends unasked. Swift's ServeEvent.
type event struct {
	Event    string           `json:"event"`
	Snapshot *app.AppSnapshot `json:"snapshot"`
}

// push sends a snapshot event each time the snapshot differs from the last one sent, until
// stop closes or out fails.
func (s *Server) push(encoder *json.Encoder, stop chan struct{}) {
	assert.That(encoder != nil, "push writes somewhere")
	assert.That(stop != nil, "push can be stopped")
	ticker := time.NewTicker(pushInterval)
	defer ticker.Stop()
	// bound: stop closes when Serve returns.
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		if err := s.pushIfChanged(encoder); err != nil {
			return
		}
	}
}

// pushIfChanged sends the snapshot when it differs from what the app last got.
func (s *Server) pushIfChanged(encoder *json.Encoder) error {
	assert.That(s != nil, "push runs on a server")
	assert.That(encoder != nil, "push writes somewhere")
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.sent == nil {
		return nil
	}
	snapshot := s.core.Snapshot()
	now, err := json.Marshal(snapshot)
	if err != nil || bytes.Equal(now, s.sent) {
		return err
	}
	s.sent = now
	return encoder.Encode(event{Event: "snapshot", Snapshot: &snapshot})
}

// answer runs one request line and builds its reply.
func (s *Server) answer(line string) reply {
	assert.That(s != nil, "answer runs on a server")
	assert.That(line != "", "a request line has text")
	var call request
	if err := json.Unmarshal([]byte(line), &call); err != nil {
		return reply{Error: &coreError{CodeBadRequest, "not a request: " + err.Error()}}
	}
	run, ok := s.handlers[call.Method]
	if !ok {
		return reply{ID: call.ID, Error: &coreError{CodeUnknownMethod,
			fmt.Sprintf("serve has no method %q", call.Method)}}
	}
	if strings.HasPrefix(call.Method, seamPrefix) && !s.seams {
		return reply{ID: call.ID, Error: &coreError{CodeNoTestSeams,
			call.Method + " needs GIVENS_TEST_SEAMS=1"}}
	}
	var given params
	if len(call.Params) > 0 {
		if err := json.Unmarshal(call.Params, &given); err != nil {
			return reply{ID: call.ID, Error: &coreError{CodeBadRequest, err.Error()}}
		}
	}
	s.effects.reset()
	value, err := run(given)
	if err != nil {
		return reply{ID: call.ID, Error: &coreError{codeOf(err), err.Error()}}
	}
	snapshot := s.core.Snapshot()
	return reply{ID: call.ID, Snapshot: &snapshot, Effects: s.effects.taken(), Value: value}
}

// codeOf names an error for the app.
func codeOf(err error) string {
	assert.That(err != nil, "only an error has a code")
	assert.That(err.Error() != "", "an error says what went wrong")
	switch {
	case errors.Is(err, app.ErrNotImplemented):
		return CodeNotBuilt
	case errors.Is(err, app.ErrNotFound):
		return CodeNotFound
	case errors.Is(err, ErrBadRequest):
		return CodeBadRequest
	}
	return CodeFailed
}

// clock is the real clock, moved forward by the test.advanceClock seam.
type clock struct {
	offset time.Duration
}

// Now is the wall clock plus whatever the seam added.
func (c *clock) Now() time.Time {
	assert.That(c != nil, "now runs on a clock")
	assert.That(c.offset >= 0, "the clock only moves forward")
	return time.Now().Add(c.offset)
}
