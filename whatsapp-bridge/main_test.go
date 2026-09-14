package main

import (
	"database/sql"
	"os"
	"testing"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

func TestIsBareJIDUser(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"phone number", "393487436929", true},
		{"lid user part", "145565165821969", true},
		{"legacy group user part", "393471390814-1414891358", true},
		{"device suffix", "393406271310:38", true},
		{"contact name", "Paola Mamma Marco Sebastiani", false},
		{"group name", "JCP 2016 Sez. Fidenza", false},
		{"group fallback", "Group 120363421665321322", false},
		{"name with digits", "Marco Allenatore 2017 Alseno Calcio", false},
		{"empty", "", false},
		{"separators only", "-:.", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBareJIDUser(tt.in); got != tt.want {
				t.Errorf("isBareJIDUser(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractDirectPathFromURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"keeps the query string",
			"https://mmg.whatsapp.net/v/t62.7119-24/802309742_1113672974664032_288117231437204654_n.enc?ccb=11-4&oh=01_Q5Aa5gFv&oe=6ACB3A24&_nc_sid=5e03e0&mms3=true",
			"/v/t62.7119-24/802309742_1113672974664032_288117231437204654_n.enc?ccb=11-4&oh=01_Q5Aa5gFv&oe=6ACB3A24&_nc_sid=5e03e0&mms3=true",
		},
		{
			"no query string",
			"https://mmg.whatsapp.net/v/t62.7118-24/13812002_698058036224062_n.enc",
			"/v/t62.7118-24/13812002_698058036224062_n.enc",
		},
		{
			"host outside .net",
			"https://media-mxp1-1.cdn.whatsapp.com/v/t62.7118-24/file.enc?ccb=11-4",
			"/v/t62.7118-24/file.enc?ccb=11-4",
		},
		{"already a direct path", "/v/t62.7118-24/file.enc?ccb=11-4", "/v/t62.7118-24/file.enc?ccb=11-4"},
		{"no path", "https://mmg.whatsapp.net", "https://mmg.whatsapp.net"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractDirectPathFromURL(tt.in); got != tt.want {
				t.Errorf("extractDirectPathFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractMediaInfoKeepsProtobufDirectPath(t *testing.T) {
	const directPath = "/v/t62.7119-24/802309742_1113672974664032_n.enc?ccb=11-4&oh=01_Q5Aa5gFv&oe=6ACB3A24"

	tests := []struct {
		name         string
		msg          *waProto.Message
		wantType     string
		wantFilename string
	}{
		{
			"document",
			&waProto.Message{DocumentMessage: &waProto.DocumentMessage{
				URL:        proto.String("https://mmg.whatsapp.net" + directPath + "&mms3=true"),
				DirectPath: proto.String(directPath),
				FileName:   proto.String("JCP_U11_Convocazione.pdf"),
			}},
			"document",
			"JCP_U11_Convocazione.pdf",
		},
		{
			"image",
			&waProto.Message{ImageMessage: &waProto.ImageMessage{
				URL:        proto.String("https://mmg.whatsapp.net" + directPath + "&mms3=true"),
				DirectPath: proto.String(directPath),
			}},
			"image",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaType, filename, _, gotDirectPath, _, _, _, _ := extractMediaInfo(tt.msg)
			if mediaType != tt.wantType {
				t.Errorf("mediaType = %q, want %q", mediaType, tt.wantType)
			}
			if tt.wantFilename != "" && filename != tt.wantFilename {
				t.Errorf("filename = %q, want %q", filename, tt.wantFilename)
			}
			if gotDirectPath != directPath {
				t.Errorf("directPath = %q, want %q", gotDirectPath, directPath)
			}
		})
	}
}

func TestStoreMessageRoundTripsDirectPath(t *testing.T) {
	t.Chdir(t.TempDir())

	store, err := NewMessageStore()
	if err != nil {
		t.Fatalf("NewMessageStore() = %v", err)
	}
	defer store.Close()

	const directPath = "/v/t62.7119-24/802309742_n.enc?ccb=11-4&oh=01_Q5Aa5gFv"
	if err := store.StoreChat("120363163183245972@g.us", "GENITORI PU U11", time.Now()); err != nil {
		t.Fatalf("StoreChat() = %v", err)
	}
	err = store.StoreMessage(
		"4A87EBE3CE8B6B840B57", "120363163183245972@g.us", "someone", "", time.Now(), false,
		"document", "convocazione.pdf", "https://mmg.whatsapp.net"+directPath+"&mms3=true", directPath,
		[]byte("key"), []byte("sha"), []byte("encsha"), 1573863,
	)
	if err != nil {
		t.Fatalf("StoreMessage() = %v", err)
	}

	_, _, _, got, _, _, _, _, err := store.GetMediaInfo("4A87EBE3CE8B6B840B57", "120363163183245972@g.us")
	if err != nil {
		t.Fatalf("GetMediaInfo() = %v", err)
	}
	if got != directPath {
		t.Errorf("direct_path = %q, want %q", got, directPath)
	}
}

func TestNewMessageStoreMigratesLegacySchema(t *testing.T) {
	t.Chdir(t.TempDir())

	// A store written before direct_path existed: opening it must add the
	// column and leave the rows readable, so history downloads keep working
	// without a resync.
	if err := os.MkdirAll("store", 0755); err != nil {
		t.Fatalf("create store dir = %v", err)
	}

	legacy, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open legacy db = %v", err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE chats (jid TEXT PRIMARY KEY, name TEXT, last_message_time TIMESTAMP);
		CREATE TABLE messages (
			id TEXT, chat_jid TEXT, sender TEXT, content TEXT, timestamp TIMESTAMP,
			is_from_me BOOLEAN, media_type TEXT, filename TEXT, url TEXT, media_key BLOB,
			file_sha256 BLOB, file_enc_sha256 BLOB, file_length INTEGER,
			PRIMARY KEY (id, chat_jid), FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);
		INSERT INTO chats VALUES ('120363163183245972@g.us', 'GENITORI PU U11', '2026-09-11');
		INSERT INTO messages (id, chat_jid, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length)
		VALUES ('OLD1', '120363163183245972@g.us', 'document', 'vecchia.pdf',
			'https://mmg.whatsapp.net/v/t62.7119-24/old_n.enc?ccb=11-4&oh=xyz&mms3=true',
			x'01', x'02', x'03', 42);
	`); err != nil {
		t.Fatalf("seed legacy db = %v", err)
	}
	legacy.Close()

	store, err := NewMessageStore()
	if err != nil {
		t.Fatalf("NewMessageStore() on legacy schema = %v", err)
	}
	defer store.Close()

	_, _, url, directPath, _, _, _, _, err := store.GetMediaInfo("OLD1", "120363163183245972@g.us")
	if err != nil {
		t.Fatalf("GetMediaInfo() = %v", err)
	}
	if directPath != "" {
		t.Errorf("direct_path = %q, want empty for a legacy row", directPath)
	}

	want := "/v/t62.7119-24/old_n.enc?ccb=11-4&oh=xyz&mms3=true"
	if got := extractDirectPathFromURL(url); got != want {
		t.Errorf("fallback direct path = %q, want %q", got, want)
	}
}

func TestGetHistoryAnchor(t *testing.T) {
	t.Chdir(t.TempDir())

	store, err := NewMessageStore()
	if err != nil {
		t.Fatalf("NewMessageStore() = %v", err)
	}
	defer store.Close()

	const chat = "120363315193425422@g.us"
	if err := store.StoreChat(chat, "GENITORI PC U9", time.Now()); err != nil {
		t.Fatalf("StoreChat() = %v", err)
	}

	// The store as it looked around the missing 12/09 convocation: a gap sits
	// between the evening of the 12th and the morning of the 13th.
	seed := []struct {
		id       string
		ts       string
		isFromMe bool
	}{
		{"OLDEST", "2026-06-19T15:42:15Z", false},
		{"BEFOREGAP", "2026-09-12T18:50:18Z", false},
		{"AFTERGAP", "2026-09-13T09:02:50Z", true},
	}
	for _, s := range seed {
		ts, err := time.Parse(time.RFC3339, s.ts)
		if err != nil {
			t.Fatalf("parse %s = %v", s.ts, err)
		}
		if err := store.StoreMessage(s.id, chat, "someone", "testo", ts, s.isFromMe, "", "", "", "", nil, nil, nil, 0); err != nil {
			t.Fatalf("StoreMessage(%s) = %v", s.id, err)
		}
	}

	t.Run("defaults to the oldest message", func(t *testing.T) {
		anchor, err := store.GetHistoryAnchor(chat, "")
		if err != nil {
			t.Fatalf("GetHistoryAnchor() = %v", err)
		}
		if anchor.ID != "OLDEST" {
			t.Errorf("anchor = %q, want OLDEST", anchor.ID)
		}
	})

	t.Run("anchors on the requested message to fill a gap", func(t *testing.T) {
		anchor, err := store.GetHistoryAnchor(chat, "AFTERGAP")
		if err != nil {
			t.Fatalf("GetHistoryAnchor() = %v", err)
		}
		if anchor.ID != "AFTERGAP" {
			t.Errorf("anchor = %q, want AFTERGAP", anchor.ID)
		}
		if !anchor.IsFromMe {
			t.Error("IsFromMe = false, want true: the request tells the phone which side sent the anchor")
		}
		if want := "2026-09-13 09:02:50"; anchor.Timestamp.UTC().Format("2006-01-02 15:04:05") != want {
			t.Errorf("timestamp = %v, want %s", anchor.Timestamp.UTC(), want)
		}
	})

	t.Run("unknown message", func(t *testing.T) {
		if _, err := store.GetHistoryAnchor(chat, "NOPE"); err == nil {
			t.Error("GetHistoryAnchor() = nil error, want a not-found error")
		}
	})

	t.Run("chat with no messages", func(t *testing.T) {
		if _, err := store.GetHistoryAnchor("120363999999999999@g.us", ""); err == nil {
			t.Error("GetHistoryAnchor() = nil error, want a not-found error")
		}
	})
}

func TestExtractsDocumentSentWithACaption(t *testing.T) {
	// A PDF sent with a message body arrives wrapped in
	// DocumentWithCaptionMessage. Before it was unwrapped both the text and
	// the media came back empty and the whole message was dropped on the
	// floor, taking the attachment with it.
	const (
		caption    = "Buonasera, mi scuso per il ritardo.\nCONVOCAZIONI PER DOMANI gara torneo della bassa parmense."
		directPath = "/v/t62.7119-24/convocazione_n.enc?ccb=11-4&oh=abc"
	)

	msg := &waProto.Message{
		DocumentWithCaptionMessage: &waProto.FutureProofMessage{
			Message: &waProto.Message{
				DocumentMessage: &waProto.DocumentMessage{
					URL:        proto.String("https://mmg.whatsapp.net" + directPath + "&mms3=true"),
					DirectPath: proto.String(directPath),
					FileName:   proto.String("Convocazioni U9.pdf"),
					Caption:    proto.String(caption),
				},
			},
		},
	}

	if got := extractTextContent(msg); got != caption {
		t.Errorf("extractTextContent() = %q, want the caption", got)
	}

	mediaType, filename, _, gotDirectPath, _, _, _, _ := extractMediaInfo(msg)
	if mediaType != "document" {
		t.Errorf("mediaType = %q, want document", mediaType)
	}
	if filename != "Convocazioni U9.pdf" {
		t.Errorf("filename = %q, want the document's name", filename)
	}
	if gotDirectPath != directPath {
		t.Errorf("directPath = %q, want %q", gotDirectPath, directPath)
	}
}

func TestUnwrapMessageEnvelopes(t *testing.T) {
	image := &waProto.Message{ImageMessage: &waProto.ImageMessage{
		Caption: proto.String("foto della distinta"),
	}}

	tests := []struct {
		name string
		msg  *waProto.Message
	}{
		{"view once", &waProto.Message{ViewOnceMessage: &waProto.FutureProofMessage{Message: image}}},
		{"view once v2", &waProto.Message{ViewOnceMessageV2: &waProto.FutureProofMessage{Message: image}}},
		{"ephemeral", &waProto.Message{EphemeralMessage: &waProto.FutureProofMessage{Message: image}}},
		{
			"ephemeral wrapping view once",
			&waProto.Message{EphemeralMessage: &waProto.FutureProofMessage{
				Message: &waProto.Message{ViewOnceMessageV2: &waProto.FutureProofMessage{Message: image}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if mediaType, _, _, _, _, _, _, _ := extractMediaInfo(tt.msg); mediaType != "image" {
				t.Errorf("mediaType = %q, want image", mediaType)
			}
			if got := extractTextContent(tt.msg); got != "foto della distinta" {
				t.Errorf("extractTextContent() = %q, want the caption", got)
			}
		})
	}
}

func TestUnwrapMessageStopsOnSelfReference(t *testing.T) {
	loop := &waProto.Message{}
	loop.EphemeralMessage = &waProto.FutureProofMessage{Message: loop}

	done := make(chan *waProto.Message, 1)
	go func() { done <- unwrapMessage(loop) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("unwrapMessage did not terminate on a self-referencing envelope")
	}
}
