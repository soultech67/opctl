package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
)

// fakeNode stands in for a running node's /logging endpoint. It echoes the
// configured state and, for POSTs, records the decoded request.
type fakeNode struct {
	server   *httptest.Server
	state    model.LogState
	lastPost *model.SetLogStateReq
}

func newFakeNode(t *testing.T, state model.LogState) *fakeNode {
	t.Helper()
	fn := &fakeNode{state: state}
	fn.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			req := model.SetLogStateReq{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fn.lastPost = &req
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fn.state)
	}))
	t.Cleanup(fn.server.Close)
	return fn
}

func (fn *fakeNode) addr() string {
	return strings.TrimPrefix(fn.server.URL, "http://")
}

func runDoctor(t *testing.T, addr string, args ...string) (string, error) {
	t.Helper()
	nodeConfig := &local.NodeConfig{APIListenAddress: addr}
	cmd := NewDoctorCmd(nodeConfig)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestParseOnOff(t *testing.T) {
	on := []string{"on", "ON", "true", "enable", "enabled", "1", " on "}
	off := []string{"off", "false", "disable", "disabled", "0"}
	for _, v := range on {
		got, err := parseOnOff(v)
		if err != nil || !got {
			t.Errorf("parseOnOff(%q) = (%v, %v), want (true, nil)", v, got, err)
		}
	}
	for _, v := range off {
		got, err := parseOnOff(v)
		if err != nil || got {
			t.Errorf("parseOnOff(%q) = (%v, %v), want (false, nil)", v, got, err)
		}
	}
	if _, err := parseOnOff("maybe"); err == nil {
		t.Error("parseOnOff(\"maybe\") should error")
	}
}

func TestPrintLogState(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewDoctorCmd(&local.NodeConfig{})
	cmd.SetOut(&buf)
	printLogState(cmd, model.LogState{Enabled: true, Level: "warn", Format: "json", Filepath: "/x/node.log"})

	out := buf.String()
	for _, want := range []string{"logging: on", "level:   warn", "format:  json", "file:    /x/node.log"} {
		if !strings.Contains(out, want) {
			t.Errorf("printLogState output missing %q; got:\n%s", want, out)
		}
	}
}

func TestUnreachableNodeErr(t *testing.T) {
	cause := errors.New("connection refused")
	err := unreachableNodeErr(&local.NodeConfig{APIListenAddress: "127.0.0.1:42224"}, cause)
	if !errors.Is(err, cause) {
		t.Error("unreachableNodeErr should wrap the cause")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:42224") {
		t.Errorf("error should mention the address; got %q", err.Error())
	}
}

func TestLogsStatus(t *testing.T) {
	fn := newFakeNode(t, model.LogState{Enabled: true, Level: "info", Format: "text", Filepath: "/d/logs/node.log"})

	out, err := runDoctor(t, fn.addr(), "logs")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fn.lastPost != nil {
		t.Error("bare `logs` should be read-only (no POST)")
	}
	if !strings.Contains(out, "logging: on") || !strings.Contains(out, "level:   info") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestLogsOff(t *testing.T) {
	fn := newFakeNode(t, model.LogState{Enabled: false, Level: "info", Format: "text", Filepath: "/d/logs/node.log"})

	out, err := runDoctor(t, fn.addr(), "logs", "off")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fn.lastPost == nil || fn.lastPost.Enabled == nil || *fn.lastPost.Enabled {
		t.Fatalf("expected POST with enabled=false, got %+v", fn.lastPost)
	}
	if !strings.Contains(out, "logging: off") {
		t.Errorf("expected 'logging: off' in output:\n%s", out)
	}
	if !strings.Contains(out, "suppresses ALL daemon logging") {
		t.Errorf("expected the kill-path paper-trail caution in output:\n%s", out)
	}
}

func TestLogsBadArg(t *testing.T) {
	fn := newFakeNode(t, model.LogState{})
	_, err := runDoctor(t, fn.addr(), "logs", "maybe")
	if err == nil {
		t.Fatal("expected error for invalid on/off arg")
	}
	if fn.lastPost != nil {
		t.Error("invalid arg should be rejected before contacting the node")
	}
}

func TestLogLevelSet(t *testing.T) {
	fn := newFakeNode(t, model.LogState{Enabled: true, Level: "debug", Format: "text", Filepath: "/d/logs/node.log"})

	out, err := runDoctor(t, fn.addr(), "log-level", "DEBUG")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fn.lastPost == nil || fn.lastPost.Level == nil || *fn.lastPost.Level != model.LogLevelDebug {
		t.Fatalf("expected POST with level=debug, got %+v", fn.lastPost)
	}
	if !strings.Contains(out, "level:   debug") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestLogLevelInvalid(t *testing.T) {
	fn := newFakeNode(t, model.LogState{})
	_, err := runDoctor(t, fn.addr(), "log-level", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("unexpected error: %v", err)
	}
	if fn.lastPost != nil {
		t.Error("invalid level should be rejected client-side before contacting the node")
	}
}

func TestLogLevelStatus(t *testing.T) {
	fn := newFakeNode(t, model.LogState{Enabled: true, Level: "warn", Format: "text", Filepath: "/d/logs/node.log"})

	out, err := runDoctor(t, fn.addr(), "log-level")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fn.lastPost != nil {
		t.Error("bare `log-level` should be read-only (no POST)")
	}
	if !strings.Contains(out, "level:   warn") {
		t.Errorf("unexpected output:\n%s", out)
	}
}
