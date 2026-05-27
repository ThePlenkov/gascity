package coordstore

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestChaosClientForwardsCreateAndPersistsAckBeforeReturn(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "chaos.sock")
	ledgerPath := filepath.Join(dir, "acked-writes.jsonl")

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close() //nolint:errcheck
	requests := make(chan ChaosRequest, 2)
	go serveOneChaosCreate(t, ln, requests)

	client := NewChaosClient(ChaosClientConfig{
		SocketPath:      socketPath,
		AckedWritesPath: ledgerPath,
	})
	created, err := client.Create(ctx, Record{ID: "ack-1", Title: "created"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "ack-1" {
		t.Fatalf("created ID = %q", created.ID)
	}
	req := <-requests
	if req.Method != "Create" {
		t.Fatalf("method = %q, want Create", req.Method)
	}
	if client.LastAckTime().IsZero() {
		t.Fatalf("LastAckTime was not updated")
	}
	if got := client.AckedIDs(); len(got) != 1 || got[0] != "ack-1" {
		t.Fatalf("AckedIDs = %#v", got)
	}
	assertAckLedgerLine(t, ledgerPath, "Create", "ack-1")
}

func serveOneChaosCreate(t *testing.T, ln net.Listener, requests chan<- ChaosRequest) {
	t.Helper()
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close() //nolint:errcheck
	var req ChaosRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		t.Errorf("decode request: %v", err)
		return
	}
	requests <- req
	var args chaosCreateArgs
	if err := json.Unmarshal(req.Args, &args); err != nil {
		t.Errorf("decode create args: %v", err)
		return
	}
	data, err := json.Marshal(args.Record)
	if err != nil {
		t.Errorf("marshal result: %v", err)
		return
	}
	if err := json.NewEncoder(conn).Encode(ChaosResponse{Result: data}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func assertAckLedgerLine(t *testing.T, path, method, id string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer file.Close() //nolint:errcheck
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("ledger missing first line")
	}
	var entry AckedWrite
	if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
		t.Fatalf("decode ledger entry: %v", err)
	}
	if entry.Method != method || entry.ID != id || entry.AckedAt.IsZero() {
		t.Fatalf("ledger entry = %#v", entry)
	}
	if scanner.Scan() {
		t.Fatalf("ledger has unexpected extra line: %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan ledger: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("ledger line was not newline-terminated: %q", data)
	}
}

func TestChaosClientMapsRemoteNotFoundToErrNotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "chaos.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer ln.Close() //nolint:errcheck
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close() //nolint:errcheck
		_ = json.NewDecoder(conn).Decode(&ChaosRequest{})
		_ = json.NewEncoder(conn).Encode(ChaosResponse{Err: ErrNotFound.Error()})
	}()

	client := NewChaosClient(ChaosClientConfig{SocketPath: socketPath})
	_, err = client.Get(ctx, "missing")
	if !IsNotFound(err) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
	if !client.LastAckTime().IsZero() {
		t.Fatalf("read-only error updated LastAckTime: %s", client.LastAckTime().Format(time.RFC3339Nano))
	}
}

func TestRecordContentFingerprintDetectsCorruption(t *testing.T) {
	base := Record{
		ID:        "rec-1",
		Title:     "expected",
		Status:    "open",
		Type:      "task",
		Priority:  2,
		CreatedAt: time.Unix(10, 20),
		UpdatedAt: time.Unix(11, 21),
		Assignee:  "builder",
		Labels:    []string{"b", "a"},
		Metadata:  map[string]string{"z": "last", "a": "first"},
	}
	same := base
	same.Labels = []string{"a", "b"}
	same.Metadata = map[string]string{"a": "first", "z": "last"}
	corrupted := base
	corrupted.Metadata = map[string]string{"a": "changed", "z": "last"}

	baseFingerprint, err := recordContentFingerprint(base)
	if err != nil {
		t.Fatalf("base fingerprint: %v", err)
	}
	sameFingerprint, err := recordContentFingerprint(same)
	if err != nil {
		t.Fatalf("same fingerprint: %v", err)
	}
	corruptedFingerprint, err := recordContentFingerprint(corrupted)
	if err != nil {
		t.Fatalf("corrupted fingerprint: %v", err)
	}

	if baseFingerprint != sameFingerprint {
		t.Fatalf("fingerprint changed for equivalent content: %q != %q", baseFingerprint, sameFingerprint)
	}
	if baseFingerprint == corruptedFingerprint {
		t.Fatalf("fingerprint did not change for corrupted content: %q", baseFingerprint)
	}
}

func TestCheckAckedRecordContentReportsMissingAndCorrupted(t *testing.T) {
	ctx := context.Background()
	expectedRecord := Record{ID: "ok", Title: "expected", Status: "open", Type: "task"}
	expectedFingerprint, err := recordContentFingerprint(expectedRecord)
	if err != nil {
		t.Fatalf("expected fingerprint: %v", err)
	}
	corruptedRecord := Record{ID: "corrupt", Title: "expected", Status: "open", Type: "task"}
	corruptedExpected, err := recordContentFingerprint(corruptedRecord)
	if err != nil {
		t.Fatalf("corrupted expected fingerprint: %v", err)
	}
	store := recordGetterFunc(func(_ context.Context, id string) (Record, error) {
		switch id {
		case "ok":
			return expectedRecord, nil
		case "corrupt":
			return Record{ID: "corrupt", Title: "changed", Status: "open", Type: "task"}, nil
		default:
			return Record{}, ErrNotFound
		}
	})

	result, err := checkAckedRecordContent(ctx, store, []string{"ok", "corrupt", "missing"}, map[string]string{
		"ok":      expectedFingerprint,
		"corrupt": corruptedExpected,
		"missing": "expected",
	})
	if err != nil {
		t.Fatalf("check content: %v", err)
	}
	if len(result.Found) != 2 {
		t.Fatalf("found = %d, want 2", len(result.Found))
	}
	if len(result.Missing) != 1 || result.Missing[0] != "missing" {
		t.Fatalf("missing = %v, want [missing]", result.Missing)
	}
	if len(result.Corrupted) != 1 || result.Corrupted[0] != "corrupt" {
		t.Fatalf("corrupted = %v, want [corrupt]", result.Corrupted)
	}
}

type recordGetterFunc func(context.Context, string) (Record, error)

func (f recordGetterFunc) Get(ctx context.Context, id string) (Record, error) {
	return f(ctx, id)
}
