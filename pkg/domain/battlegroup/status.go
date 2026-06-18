package battlegroup

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Status is the operator-observed state of a BattleGroup, distilled from
// the CR's .status subresource. It is JSON-tagged so it serializes into
// the dunemgr status cache unchanged.
type Status struct {
	ServerGroupPhase string         `json:"server_group_phase"`
	DBPhase          string         `json:"db_phase"`
	DirectorPhase    string         `json:"director_phase"`
	Size             int            `json:"size"`
	StartedAt        time.Time      `json:"started_at"`
	Servers          []ServerStatus `json:"servers"`
}

// ServerStatus is the per-server/per-map slice of .status.servers[].
type ServerStatus struct {
	Map        string `json:"map"`
	Phase      string `json:"phase"`
	Ready      bool   `json:"ready"`
	Restarts   int    `json:"restarts"`
	GamePort   int    `json:"game_port"`
	IgwPort    int    `json:"igw_port"`
	ExitReason string `json:"exit_reason"`
}

// ParseStatus decodes a BattleGroup CR (JSON) and returns its observed
// Status. A CR with no .status block yields the zero Status (not an
// error) — the operator simply has not populated it yet.
func ParseStatus(cr []byte) (Status, error) {
	var doc struct {
		Status struct {
			ServerGroupPhase string `json:"serverGroupPhase"`
			Phase            string `json:"phase"`
			Size             int    `json:"size"`
			StartTimestamp   string `json:"startTimestamp"`
			Database         struct {
				Phase string `json:"phase"`
			} `json:"database"`
			Utilities struct {
				Director struct {
					Phase string `json:"phase"`
				} `json:"director"`
			} `json:"utilities"`
			Servers []struct {
				PartitionMap string `json:"partitionMap"`
				Phase        string `json:"phase"`
				Ready        bool   `json:"ready"`
				Restarts     int    `json:"restarts"`
				GamePort     int    `json:"gamePort"`
				IgwPort      int    `json:"igwPort"`
				ExitReason   string `json:"exitReason"`
				ExitCode     *int   `json:"exitCode"`
			} `json:"servers"`
		} `json:"status"`
	}
	if err := json.Unmarshal(cr, &doc); err != nil {
		return Status{}, fmt.Errorf("decode BattleGroup status JSON: %w", err)
	}
	s := doc.Status
	out := Status{
		ServerGroupPhase: s.ServerGroupPhase,
		DBPhase:          s.Database.Phase,
		DirectorPhase:    s.Utilities.Director.Phase,
		Size:             s.Size,
	}
	if s.StartTimestamp != "" {
		if ts, err := time.Parse(time.RFC3339, s.StartTimestamp); err == nil {
			out.StartedAt = ts
		}
	}
	out.Servers = make([]ServerStatus, 0, len(s.Servers))
	for _, srv := range s.Servers {
		out.Servers = append(out.Servers, ServerStatus{
			Map:        srv.PartitionMap,
			Phase:      srv.Phase,
			Ready:      srv.Ready,
			Restarts:   srv.Restarts,
			GamePort:   srv.GamePort,
			IgwPort:    srv.IgwPort,
			ExitReason: composeExitReason(srv.ExitReason, srv.ExitCode),
		})
	}
	return out, nil
}

// composeExitReason renders the "LAST EXIT" cell:
//   - no exitCode recorded (nil)         → ""      (server never exited)
//   - reason present                     → "reason(code)" e.g. "SIGSEGV(139)"
//   - reason empty, code 0               → "clean"
//   - reason empty, code != 0            → "exit(code)"
func composeExitReason(reason string, code *int) string {
	if code == nil {
		return ""
	}
	if reason != "" {
		return reason + "(" + strconv.Itoa(*code) + ")"
	}
	if *code == 0 {
		return "clean"
	}
	return "exit(" + strconv.Itoa(*code) + ")"
}
