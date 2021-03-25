package indentfile

import (
	"container/list"
	"strings"
	"testing"
)

func TestParseSimple(t *testing.T) {
	messages := list.New()
	ctx := &msgCtx{messages, ""}

	err := ParseFile("test_files/parse/simple.txt", ctx)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	expect := []string{
		"hello world",
		"* says hello",
		"* waves",
		"* looks at you",
		"<* looks end>",
		"* unnervingly",
		"<* end>",
		"uhhh lets just go",
		"<end>",
	}

	if len(expect) != messages.Len() {
		t.Fatalf("Got %d messages; want %d", messages.Len(), len(expect))
	}

	node := messages.Front()
	for i, msg := range expect {
		val := node.Value.(string)
		if val != msg {
			t.Fatalf("Got %d = %q; want %q", i, val, msg)
		}

		node = node.Next()
	}
}

func TestParseJSON(t *testing.T) {
	messages := list.New()
	ctx := &msgCtx{messages, ""}

	err := ParseFile("test_files/parse/json.txt", ctx)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	expect := []string{
		"some object (hello)",
		"<some object end>",
		"<end>",
	}

	if len(expect) != messages.Len() {
		t.Fatalf("Got %d messages; want %d", messages.Len(), len(expect))
	}

	node := messages.Front()
	for i, msg := range expect {
		val := node.Value.(string)
		if val != msg {
			t.Fatalf("Got %d = %q; want %q", i, val, msg)
		}

		node = node.Next()
	}
}

type msgCtx struct {
	messages *list.List
	prefix   string
}

func (m msgCtx) Msg(words ...string) {
	line := strings.Join(words, " ")
	if m.prefix != "" {
		line = m.prefix + " " + line
	}

	m.messages.PushBack(line)
}

func (m msgCtx) Prefix(words ...string) msgCtx {
	newPrefix := strings.Join(words, " ")
	if m.prefix != "" {
		newPrefix = m.prefix + " " + newPrefix
	}

	// Intentionally not returning a pointer -
	// this should be valid too!
	return msgCtx{m.messages, newPrefix}
}

func (m msgCtx) Object(obj msgObject) {
	if obj.Surround == "" {
		m.Msg(obj.Text)
	} else if len(obj.Surround) == 2 {
		m.Msg(obj.Surround[:1] + obj.Text + obj.Surround[1:])
	} else {
		m.Msg(obj.Surround + obj.Text + obj.Surround)
	}
}

func (m msgCtx) End() error {
	if m.prefix == "" {
		m.messages.PushBack("<end>")
	} else {
		m.messages.PushBack("<" + m.prefix + " end>")
	}

	return nil
}

type msgObject struct {
	Text     string `json:"text"`
	Surround string `json:"sur,omitempty"`
}
