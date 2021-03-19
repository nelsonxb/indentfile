package indentfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestSimpleIndents(t *testing.T) {
	expect := []expectToken{
		{WordToken, LineInfo{3, 1, nil}, []byte("simple"), nil},
		{WordToken, LineInfo{3, 8, nil}, []byte("directive"), nil},
		{TerminatorToken, LineInfo{3, 17, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{4, 1, nil}, []byte("block"), nil},
		{WordToken, LineInfo{4, 7, nil}, []byte("directive"), nil},
		{TerminatorToken, LineInfo{4, 16, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{5, 5, nil}, []byte("    "), nil},
		{WordToken, LineInfo{5, 5, nil}, []byte("indented"), nil},
		{WordToken, LineInfo{5, 14, nil}, []byte("directive"), nil},
		{TerminatorToken, LineInfo{5, 23, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{7, 5, nil}, []byte("simple"), nil},
		{WordToken, LineInfo{7, 12, nil}, []byte("directive"), nil},
		{WordToken, LineInfo{7, 22, nil}, []byte("with"), nil},
		{WordToken, LineInfo{7, 27, nil}, []byte("multiple"), nil},
		{WordToken, LineInfo{7, 36, nil}, []byte("arguments"), nil},
		{TerminatorToken, LineInfo{7, 45, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{11, 1, nil}, []byte{}, nil},
		{WordToken, LineInfo{11, 1, nil}, []byte("outdented"), nil},
		{TerminatorToken, LineInfo{11, 10, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{13, 5, nil}, []byte("    "), nil},
		{WordToken, LineInfo{13, 5, nil}, []byte("suffix"), nil},
		{WordToken, LineInfo{13, 12, nil}, []byte("indented"), nil},
		{TerminatorToken, LineInfo{13, 20, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{14, 1, nil}, []byte{}, nil},
	}

	testTokenSequence(t, "tokens/simple_indents.txt", expect)
	// TODO: Test CRLF
}

func TestWeirdIndents(t *testing.T) {
	testTokenSequence(t, "tokens/weird_indents.txt", []expectToken{
		{WordToken, LineInfo{1, 1, nil}, []byte("outer"), nil},
		{TerminatorToken, LineInfo{1, 6, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{2, 3, nil}, []byte("  "), nil},
		{WordToken, LineInfo{2, 3, nil}, []byte("inner"), nil},
		{TerminatorToken, LineInfo{2, 8, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{3, 9, nil}, []byte("        "), nil},
		{WordToken, LineInfo{3, 9, nil}, []byte("more"), nil},
		{WordToken, LineInfo{3, 14, nil}, []byte("inner"), nil},
		{TerminatorToken, LineInfo{3, 19, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{4, 3, nil}, []byte("  "), nil},
		{WordToken, LineInfo{4, 3, nil}, []byte("sub"), nil},
		{TerminatorToken, LineInfo{4, 6, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{6, 1, nil}, []byte{}, nil},
		{WordToken, LineInfo{6, 1, nil}, []byte("next"), nil},
		{WordToken, LineInfo{6, 6, nil}, []byte("outer"), nil},
		{TerminatorToken, LineInfo{6, 11, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{7, 9, nil}, []byte("        "), nil},
		{WordToken, LineInfo{7, 9, nil}, []byte("level"), nil},
		{WordToken, LineInfo{7, 15, nil}, []byte("1"), nil},
		{TerminatorToken, LineInfo{7, 16, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{8, 10, nil}, []byte("         "), nil},
		{WordToken, LineInfo{8, 10, nil}, []byte("level"), nil},
		{WordToken, LineInfo{8, 16, nil}, []byte("2"), nil},
		{TerminatorToken, LineInfo{8, 17, nil}, []byte{'\n'}, nil},
		{ErrorToken, LineInfo{10, 5, nil}, []byte("    bad\n"),
			ErrBadOutdent},
	})
}

func TestShellSyntax(t *testing.T) {
	testTokenSequence(t, "tokens/shell_syntax.txt", []expectToken{
		{CommentToken, LineInfo{1, 1, nil}, []byte("# Initial comment"), nil},
		{WordToken, LineInfo{2, 2, nil}, []byte("some"), nil},
		{WordToken, LineInfo{2, 8, nil}, []byte("directive"), nil},
		{TerminatorToken, LineInfo{2, 17, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{3, 1, nil}, []byte("some"), nil},
		{WordToken, LineInfo{3, 7, nil}, []byte("directive"), nil},
		{TerminatorToken, LineInfo{3, 21, nil}, []byte{'\n'}, nil},
		{CommentToken, LineInfo{4, 1, nil}, []byte("# Later comment"), nil},
		{WordToken, LineInfo{5, 1, nil}, []byte("some"), nil},
		{WordToken, LineInfo{5, 7, nil}, []byte("quoted directive"), nil},
		{TerminatorToken, LineInfo{5, 24, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{6, 1, nil}, []byte("some"), nil},
		{WordToken, LineInfo{6, 9, nil}, []byte("weird 'directive'"), nil},
		{TerminatorToken, LineInfo{6, 27, nil}, []byte{'\n'}, nil},
		{CommentToken, LineInfo{6, 31, nil}, []byte("# EOL comment"), nil},
		{WordToken, LineInfo{7, 1, nil}, []byte("some"), nil},
		{WordToken, LineInfo{7, 7, nil}, []byte("stranger \"directive\""), nil},
		{TerminatorToken, LineInfo{7, 34, nil}, []byte{'\n'}, nil},
		{CommentToken, LineInfo{9, 5, nil}, []byte("# Also check that EOF ~ EOL"), nil},
		{IndentToken, LineInfo{10, 5, nil}, []byte("    "), nil},
		{WordToken, LineInfo{10, 5, nil}, []byte("eof"), nil},
		{TerminatorToken, LineInfo{10, 8, nil}, []byte{}, nil},
		{OutdentToken, LineInfo{10, 8, nil}, []byte{}, nil},
	})
}

func TestStartIndented(t *testing.T) {
	testTokenSequence(t, "tokens/start_indented.txt", []expectToken{
		{CommentToken, LineInfo{1, 5, nil}, []byte("# This comment shouldn't matter..."), nil},
		{CommentToken, LineInfo{2, 1, nil}, []byte("# And neither should this one..."), nil},
		{ErrorToken, LineInfo{4, 5, nil}, []byte("    This line should produce an error.\n"), ErrBadIndent},
	})
}

func TestJsonSyntax(t *testing.T) {
	expectObject := `{
        "key": "value",
    "obj": {
        "id": 1,
        "value": null
    }
}`

	expectArray := `[
        {"id": 1},
        {"id": 2},
        [{"id": 3}],
        {"id": 4, "values": [1, 2, 3]}
    ]`

	testTokenSequence(t, "tokens/json_syntax.txt", []expectToken{
		{WordToken, LineInfo{2, 1, nil}, []byte("block"), nil},
		{TerminatorToken, LineInfo{2, 6, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{3, 5, nil}, []byte("    "), nil},
		{WordToken, LineInfo{3, 5, nil}, []byte("indented"), nil},
		{WordToken, LineInfo{3, 14, nil}, []byte("json"), nil},
		{ObjectToken, LineInfo{3, 19, nil}, []byte(expectObject), nil},
		{TerminatorToken, LineInfo{9, 2, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{11, 5, nil}, []byte("indented"), nil},
		{WordToken, LineInfo{11, 14, nil}, []byte("array"), nil},
		{ObjectToken, LineInfo{11, 20, nil}, []byte(expectArray), nil},
		{TerminatorToken, LineInfo{16, 6, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{17, 1, nil}, []byte{}, nil},
	})
}

func TestMessyWhitespace(t *testing.T) {
	contents := "\n" +
		"\t# Notice: this file is re-generated by TestMessyWhitespace.\n" +
		"        # Modify the source in that function instead of this file.\r\n" +
		"\r\n" +
		"block\n" +
		"   level 1\n" +
		"   block\r\n" +
		"   \tlevel\t2      threeee\t\t\t\t\tfour \t \r\n" +
		"   \tblock\t  \t #\tLine comments!\n" +
		"   \t  \tlevel 3\r\n" +
		"   \toutdent 1\n" +
		"\r\n" +
		"root\r\n" +
		"\r\n" +
		"\t\t\tstatement"

	err := os.WriteFile(
		"test_files/tokens/messy_whitespace.txt",
		[]byte(contents),
		0666,
	)

	if err != nil {
		t.Fatalf("Failed to generate messy_whitespace.txt: %v", err)
	}

	testTokenSequence(t, "tokens/messy_whitespace.txt", []expectToken{
		{CommentToken, LineInfo{2, 2, nil}, []byte("# Notice: this file is re-generated by TestMessyWhitespace."), nil},
		{CommentToken, LineInfo{3, 9, nil}, []byte("# Modify the source in that function instead of this file."), nil},
		{WordToken, LineInfo{5, 1, nil}, []byte("block"), nil},
		{TerminatorToken, LineInfo{5, 6, nil}, []byte{'\n'}, nil},
		{IndentToken, LineInfo{6, 4, nil}, []byte("   "), nil},
		{WordToken, LineInfo{6, 4, nil}, []byte("level"), nil},
		{WordToken, LineInfo{6, 10, nil}, []byte("1"), nil},
		{TerminatorToken, LineInfo{6, 11, nil}, []byte{'\n'}, nil},
		{WordToken, LineInfo{7, 4, nil}, []byte("block"), nil},
		{TerminatorToken, LineInfo{7, 9, nil}, []byte{'\r', '\n'}, nil},
		{IndentToken, LineInfo{8, 5, nil}, []byte("   \t"), nil},
		{WordToken, LineInfo{8, 5, nil}, []byte("level"), nil},
		{WordToken, LineInfo{8, 11, nil}, []byte("2"), nil},
		{WordToken, LineInfo{8, 18, nil}, []byte("threeee"), nil},
		{WordToken, LineInfo{8, 30, nil}, []byte("four"), nil},
		{TerminatorToken, LineInfo{8, 37, nil}, []byte{'\r', '\n'}, nil},
		{WordToken, LineInfo{9, 5, nil}, []byte("block"), nil},
		{TerminatorToken, LineInfo{9, 10, nil}, []byte{'\n'}, nil},
		{CommentToken, LineInfo{9, 15, nil}, []byte("#\tLine comments!"), nil},
		{IndentToken, LineInfo{10, 8, nil}, []byte("   \t  \t"), nil},
		{WordToken, LineInfo{10, 8, nil}, []byte("level"), nil},
		{WordToken, LineInfo{10, 14, nil}, []byte("3"), nil},
		{TerminatorToken, LineInfo{10, 15, nil}, []byte{'\r', '\n'}, nil},
		{OutdentToken, LineInfo{11, 5, nil}, []byte("   \t"), nil},
		{WordToken, LineInfo{11, 5, nil}, []byte("outdent"), nil},
		{WordToken, LineInfo{11, 13, nil}, []byte("1"), nil},
		{TerminatorToken, LineInfo{11, 14, nil}, []byte{'\n'}, nil},
		{OutdentToken, LineInfo{13, 1, nil}, []byte{}, nil},
		{WordToken, LineInfo{13, 1, nil}, []byte("root"), nil},
		{TerminatorToken, LineInfo{13, 5, nil}, []byte{'\r', '\n'}, nil},
		{IndentToken, LineInfo{15, 4, nil}, []byte("\t\t\t"), nil},
		{WordToken, LineInfo{15, 4, nil}, []byte("statement"), nil},
		{TerminatorToken, LineInfo{15, 13, nil}, []byte{}, nil},
		{OutdentToken, LineInfo{15, 13, nil}, []byte{}, nil},
	})
}

type expectToken struct {
	Type TokenType
	Info LineInfo
	Text []byte
	Err  error
}

func testTokenSequence(t *testing.T, respath string, expectTokens []expectToken) {
	fd, err := os.Open("test_files/" + respath)
	if err != nil {
		panic(err)
	}

	defer fd.Close()

	problems := 0
	tokens := NewTokenizer(fd)

	for i, expect := range expectTokens {
		if problems >= 5 {
			t.Fatalf("Stopping after %d non-matches", problems)
		}

		actual, err := tokens.Next()
		if err != nil {
			if expect.Err == nil || !errors.Is(err, expect.Err) {
				problems++
				t.Errorf("Token %d error %q; want %s",
					i, err, tokenTypeName(expect.Type))
			} else if expect.Err != nil && expect.Type == ErrorToken {
				if actual == nil {
					problems++
					t.Errorf("Token %d = nil; want ErrorToken", i)
				} else if actual.Type() != ErrorToken {
					problems++
					t.Errorf("Token %d type = %s; want ErrorToken",
						i, tokenTypeName(actual.Type()))
				} else if !cmpLineInfo(actual.LineInfo(0), expect.Info) {
					problems++
					t.Errorf("Token %d line info = %v; want %v",
						i, actual.LineInfo(0),
						expect.Info)
				} else if !bytes.Equal(actual.Text(), expect.Text) {
					problems++
					t.Errorf("Token %d text = %q; want %q",
						i, string(actual.Text()),
						string(expect.Text))
				}
			}

		} else if expect.Err != nil {
			problems++
			t.Errorf("Token %d type = %s; want error %q",
				i, tokenTypeName(actual.Type()), expect.Err)

		} else if actual.Type() != expect.Type {
			problems++
			t.Errorf("Token %d type = %s; want %s",
				i, tokenTypeName(actual.Type()),
				tokenTypeName(expect.Type))

		} else if !cmpLineInfo(actual.LineInfo(0), expect.Info) {
			problems++
			t.Errorf("Token %d line info = %v; want %v",
				i, actual.LineInfo(0), expect.Info)

		} else if !bytes.Equal(actual.Text(), expect.Text) {
			problems++
			t.Errorf("Token %d text = %q; want %q",
				i, string(actual.Text()), string(expect.Text))
		}
	}

	actual, err := tokens.Next()
	if err == io.EOF {
		return
	}

	if err != nil {
		t.Errorf("Unexpected error instead of EOF: %q", err)
	}

	t.Errorf("Unexpected token %d type = %s; want EOF",
		len(expectTokens), tokenTypeName(actual.Type()))
}

func tokenTypeName(t TokenType) string {
	switch t {
	case nilToken:
		return "nilToken"
	case ErrorToken:
		return "ErrorToken"
	case WordToken:
		return "WordToken"
	case ObjectToken:
		return "ObjectToken"
	case TerminatorToken:
		return "TerminatorToken"
	case IndentToken:
		return "IndentToken"
	case OutdentToken:
		return "OutdentToken"
	case CommentToken:
		return "CommentToken"
	default:
		panic(fmt.Errorf("Unknown token type %v", t))
	}
}

func cmpLineInfo(actual, expect LineInfo) bool {
	return actual.Lineno == expect.Lineno &&
		actual.Offset == expect.Offset &&
		(expect.Text == nil || bytes.Equal(actual.Text, expect.Text))
}
