package broadcast

import (
	"strings"
	"testing"
)

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
