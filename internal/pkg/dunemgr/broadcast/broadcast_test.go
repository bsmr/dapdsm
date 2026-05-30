package broadcast

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeEnvelope(t *testing.T) {
	inner := []byte(`{"ServerCommand":"X"}`)
	b64 := EncodeEnvelope(inner, "TOKEN123")

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	var outer struct {
		Version        int    `json:"Version"`
		AuthToken      string `json:"AuthToken"`
		MessageContent string `json:"MessageContent"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("json: %v", err)
	}
	if outer.Version != 2 {
		t.Errorf("Version=%d, want 2", outer.Version)
	}
	if outer.AuthToken != "TOKEN123" {
		t.Errorf("AuthToken=%q, want TOKEN123", outer.AuthToken)
	}
	if outer.MessageContent != string(inner) {
		t.Errorf("MessageContent=%q, want %q", outer.MessageContent, string(inner))
	}
}

func TestNoticePayload(t *testing.T) {
	got := NoticePayload("Hello", "World", 30)
	if !strings.Contains(got, `"ServerCommand":"ServiceBroadcast"`) {
		t.Errorf("missing ServerCommand: %s", got)
	}
	if !strings.Contains(got, `"BroadcastType":"Generic"`) {
		t.Errorf("missing BroadcastType: %s", got)
	}
	if !strings.Contains(got, `"BroadcastDuration":30`) {
		t.Errorf("missing BroadcastDuration: %s", got)
	}
	if !strings.Contains(got, `"Title":"Hello"`) {
		t.Errorf("missing Title: %s", got)
	}
}

func TestShutdownAnnouncePayload(t *testing.T) {
	got := ShutdownAnnouncePayload(ShutdownAnnounce{
		Kind:               "Restart",
		AtUnix:             1717000000,
		NowUnix:            1716999700,
		ShutdownDurationS:  30,
		BroadcastFrequency: 60,
		BroadcastDuration:  10,
	})
	if !strings.Contains(got, `"ShutdownType":"Restart"`) {
		t.Errorf("missing ShutdownType: %s", got)
	}
	if !strings.Contains(got, `"ShutdownTimestamp":1717000000`) {
		t.Errorf("missing ShutdownTimestamp: %s", got)
	}
	if !strings.Contains(got, `"DateTimestamp":1716999700`) {
		t.Errorf("missing DateTimestamp: %s", got)
	}
}

func TestShutdownCancelPayload(t *testing.T) {
	got := ShutdownCancelPayload()
	if !strings.Contains(got, `"ShouldCancel":true`) {
		t.Errorf("missing ShouldCancel:true: %s", got)
	}
}

func TestBuildErlangPublishLabelSafety(t *testing.T) {
	const safe = "abc"
	const unsafe = "bad label"
	gotSafe := BuildErlangPublish("PAYLOAD", safe)
	gotUnsafe := BuildErlangPublish("PAYLOAD", unsafe)
	if !strings.Contains(gotSafe, "label="+safe) {
		t.Errorf("safe label not propagated: %s", gotSafe)
	}
	if strings.Contains(gotUnsafe, unsafe) {
		t.Errorf("unsafe label leaked into Erlang: %s", gotUnsafe)
	}
	if !strings.Contains(gotUnsafe, "label=smgmt") {
		t.Errorf("unsafe label not replaced with 'smgmt': %s", gotUnsafe)
	}
}
