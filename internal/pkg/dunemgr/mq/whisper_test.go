package mq

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeWhisperEnvelope(t *testing.T) {
	chat := `{"ChannelType":"ETextChatChannelType::Whispers","Message":{"Body":"hi"}}`
	b64 := EncodeWhisperEnvelope(chat)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("not base64: %v", err)
	}
	var outer struct {
		Content string
		Type    string
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatal(err)
	}
	if outer.Type != "ECourierMessageType::TextChat" {
		t.Fatalf("Type wrong: %q", outer.Type)
	}
	if outer.Content != chat {
		t.Fatalf("Content not the chat json: %q", outer.Content)
	}
}

func TestBuildWhisperChatJSON(t *testing.T) {
	s := buildWhisperChatJSON("Paul", "Server", "hello there", true, "")
	if !strings.Contains(s, `"ETextChatChannelType::Whispers"`) {
		t.Fatalf("missing whisper channel: %s", s)
	}
	if !strings.Contains(s, `"UserNameTo":"Paul"`) || !strings.Contains(s, `"hello there"`) {
		t.Fatalf("missing to/body: %s", s)
	}
	if !strings.Contains(s, `"AuthorName":"Server"`) {
		t.Fatalf("missing spoofed from: %s", s)
	}
}

func TestBuildErlangWhisper(t *testing.T) {
	e := BuildErlangWhisper("Yng==", "ABCDEF1234567890", "FEDCBA0987654321")
	if !strings.Contains(e, `chat.whispers`) {
		t.Fatalf("wrong exchange: %s", e)
	}
	if !strings.Contains(e, "ABCDEF1234567890") {
		t.Fatalf("routing key (target fls) missing: %s", e)
	}
	if !strings.Contains(e, "text_chat") {
		t.Fatalf("amqp type text_chat missing: %s", e)
	}
}

func TestBuildErlangWhisperDigitFirstFLS(t *testing.T) {
	e := BuildErlangWhisper("Yng==", "127AC6307755DB02", "3386A7DC456B968D")
	if !strings.Contains(e, "127AC6307755DB02") {
		t.Fatalf("digit-first target fls was dropped: %s", e)
	}
	if !strings.Contains(e, "3386A7DC456B968D") {
		t.Fatalf("digit-first sender fls was dropped: %s", e)
	}
}

func TestWhisperChatJSONSetsSenderFuncomID(t *testing.T) {
	j := buildWhisperChatJSON("Stilgar", "", "hi", false, "GM#0001")
	if !strings.Contains(j, `"FuncomIdFrom":"GM#0001"`) {
		t.Fatalf("sender funcom-id not set: %s", j)
	}
}

func TestWhisperChatJSONSpoofHasEmptySender(t *testing.T) {
	j := buildWhisperChatJSON("Stilgar", "GM", "hi", true, "")
	if !strings.Contains(j, `"FuncomIdFrom":""`) {
		t.Fatalf("spoof sender should be empty: %s", j)
	}
}
