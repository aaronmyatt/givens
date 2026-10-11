package serve

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// call sends lines to a new server and decodes each reply.
func call(t *testing.T, seams bool, lines ...string) []map[string]any {
	t.Helper()
	server, err := New(t.TempDir(), seams)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := server.Serve(strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatal(err)
	}
	replies := make([]map[string]any, 0, len(lines))
	scanner := bufio.NewScanner(strings.NewReader(out.String()))
	// bound: out is the finished output, so Scan ends.
	for scanner.Scan() {
		var reply map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &reply); err != nil {
			t.Fatalf("reply %q: %v", scanner.Text(), err)
		}
		replies = append(replies, reply)
	}
	return replies
}

// errorCode is a reply's error code, or "".
func errorCode(reply map[string]any) string {
	failure, _ := reply["error"].(map[string]any)
	code, _ := failure["code"].(string)
	return code
}

func TestOpenAndCapability(t *testing.T) {
	shop, err := filepath.Abs("../../testdata/sample-shop")
	if err != nil {
		t.Fatal(err)
	}
	replies := call(t, false,
		`{"id":1,"method":"open","params":{"path":"`+shop+`"}}`,
		`{"id":2,"method":"window.capability","params":{"window":"w1","name":"Sign in"}}`,
	)
	if len(replies) != 2 || errorCode(replies[0]) != "" || errorCode(replies[1]) != "" {
		t.Fatalf("replies = %v", replies)
	}
	windows := replies[0]["snapshot"].(map[string]any)["windows"].([]any)
	sidebar := windows[0].(map[string]any)["sidebar"].([]any)
	capabilities := make([]any, 0, 3)
	for _, item := range sidebar {
		if item.(map[string]any)["kind"] == "capability" {
			capabilities = append(capabilities, item.(map[string]any)["name"])
		}
	}
	if len(capabilities) != 3 || capabilities[2] != "Sign in" {
		t.Errorf("sidebar = %v", sidebar)
	}
	value := replies[1]["value"].(map[string]any)
	if value["name"] != "Sign in" || len(value["scenarios"].([]any)) != 2 {
		t.Errorf("capability = %v", value)
	}
}

func TestErrors(t *testing.T) {
	replies := call(t, false,
		`{"id":1,"method":"launch"}`,
		`{"id":2,"method":"test.advanceClock","params":{"seconds":5}}`,
		`{"id":3,"method":"nope"}`,
		`{"id":4,"method":"open","params":{"path":"relative"}}`,
		`{"id":5,"method":"window.select","params":{"window":"w9","name":"Checkout"}}`,
		`not json`,
	)
	want := []string{CodeNotBuilt, CodeNoTestSeams, CodeUnknownMethod, CodeBadRequest,
		CodeBadRequest, CodeBadRequest}
	for index, reply := range replies {
		if errorCode(reply) != want[index] {
			t.Errorf("reply %d = %v, want code %s", index+1, reply, want[index])
		}
	}
}

func TestAdvanceClockSeam(t *testing.T) {
	replies := call(t, true, `{"id":1,"method":"test.advanceClock","params":{"seconds":"60"}}`)
	if len(replies) != 1 || errorCode(replies[0]) != "" {
		t.Fatalf("replies = %v", replies)
	}
}

// TestPushOnChange opens a copy of the sample shop, edits it behind serve's back, and expects
// a snapshot event without asking.
func TestPushOnChange(t *testing.T) {
	root := filepath.Join(t.TempDir(), "capabilities")
	if err := os.CopyFS(root, os.DirFS("../../testdata/sample-shop/capabilities")); err != nil {
		t.Fatal(err)
	}
	server, err := New(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	requests, writeRequests := io.Pipe()
	replies, writeReplies := io.Pipe()
	go func() { _ = server.Serve(requests, writeReplies) }()
	defer func() { _ = writeRequests.Close() }()
	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(replies)
		scanner.Buffer(make([]byte, 0, 1<<16), LineLengthBytesMax)
		// bound: the pipe closes when the test ends.
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	if _, err := io.WriteString(writeRequests,
		`{"id":1,"method":"open","params":{"path":"`+root+`"}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	<-lines
	feature := filepath.Join(root, "invites", "invites.feature")
	text, err := os.ReadFile(feature)
	if err != nil {
		t.Fatal(err)
	}
	added := string(text) + "\n  Scenario: Resend invite\n    Then Sam gets it again\n"
	if err := os.WriteFile(feature, []byte(added), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-lines:
		if !strings.HasPrefix(line, `{"event":"snapshot"`) || !strings.Contains(line, `"count":3`) {
			t.Errorf("pushed %s", line)
		}
	case <-time.After(2 * time.Second):
		t.Error("no snapshot was pushed within 2 seconds")
	}
}
