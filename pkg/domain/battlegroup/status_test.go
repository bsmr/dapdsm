package battlegroup

import "testing"

const statusSampleCR = `{
  "status": {
    "serverGroupPhase": "Running",
    "phase": "Running",
    "size": 3,
    "startTimestamp": "2026-05-29T08:00:00Z",
    "database": {"phase": "Ready"},
    "utilities": {"director": {"phase": "Ready"}},
    "servers": [
      {"partitionMap": "DeepDesert_1", "phase": "Running", "ready": true,  "restarts": 0, "gamePort": 7777, "igwPort": 7888},
      {"partitionMap": "Overmap",      "phase": "Running", "ready": true,  "restarts": 2, "gamePort": 7779, "igwPort": 7890, "exitReason": "SIGSEGV", "exitCode": 139},
      {"partitionMap": "Survival_1",   "phase": "Stopped", "ready": false, "restarts": 0, "exitCode": 0}
    ]
  }
}`

func TestParseStatus_FullStatus(t *testing.T) {
	t.Parallel()
	st, err := ParseStatus([]byte(statusSampleCR))
	if err != nil {
		t.Fatalf("ParseStatus err = %v", err)
	}
	if st.Phase != "Running" {
		t.Errorf("Phase = %q, want Running", st.Phase)
	}
	if st.ServerGroupPhase != "Running" {
		t.Errorf("ServerGroupPhase = %q, want Running", st.ServerGroupPhase)
	}
	if st.DBPhase != "Ready" || st.DirectorPhase != "Ready" {
		t.Errorf("DBPhase=%q DirectorPhase=%q, want Ready/Ready", st.DBPhase, st.DirectorPhase)
	}
	if st.Size != 3 {
		t.Errorf("Size = %d, want 3", st.Size)
	}
	if st.StartedAt.IsZero() {
		t.Error("StartedAt is zero, want parsed timestamp")
	}
	if len(st.Servers) != 3 {
		t.Fatalf("len(Servers) = %d, want 3", len(st.Servers))
	}
	// DeepDesert_1: running, never exited → empty ExitReason.
	if got := st.Servers[0].ExitReason; got != "" {
		t.Errorf("Servers[0].ExitReason = %q, want empty", got)
	}
	if !st.Servers[0].Ready || st.Servers[0].GamePort != 7777 || st.Servers[0].IgwPort != 7888 {
		t.Errorf("Servers[0] = %+v, want ready DeepDesert ports 7777/7888", st.Servers[0])
	}
	// Overmap: crashed → "SIGSEGV(139)".
	if got := st.Servers[1].ExitReason; got != "SIGSEGV(139)" {
		t.Errorf("Servers[1].ExitReason = %q, want SIGSEGV(139)", got)
	}
	if st.Servers[1].Restarts != 2 {
		t.Errorf("Servers[1].Restarts = %d, want 2", st.Servers[1].Restarts)
	}
	// Survival_1: exited cleanly (exitCode present, 0, no reason) → "clean".
	if got := st.Servers[2].ExitReason; got != "clean" {
		t.Errorf("Servers[2].ExitReason = %q, want clean", got)
	}
	if st.Servers[2].Map != "Survival_1" || st.Servers[2].Ready {
		t.Errorf("Servers[2] = %+v, want Survival_1 not-ready", st.Servers[2])
	}
}

func TestParseStatus_NoStatusBlock(t *testing.T) {
	t.Parallel()
	st, err := ParseStatus([]byte(`{"spec":{}}`))
	if err != nil {
		t.Fatalf("ParseStatus err = %v", err)
	}
	if st.ServerGroupPhase != "" || len(st.Servers) != 0 {
		t.Errorf("empty-status CR parsed as %+v, want zero value", st)
	}
}

func TestParseStatus_EmptyServers(t *testing.T) {
	t.Parallel()
	st, err := ParseStatus([]byte(`{"status":{"serverGroupPhase":"Stopped","servers":[]}}`))
	if err != nil {
		t.Fatalf("ParseStatus err = %v", err)
	}
	if st.ServerGroupPhase != "Stopped" || len(st.Servers) != 0 {
		t.Errorf("got %+v, want Stopped/0 servers", st)
	}
}

func TestParseStatus_InvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := ParseStatus([]byte(`{not json`)); err == nil {
		t.Error("ParseStatus(invalid) err = nil, want decode error")
	}
}
