package indentfile

import (
	"bufio"
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
)

// Some errors that may be returned from Tokenizer.Next().
// When `errors.Is(err, ErrLex)`,
// the returned token is also defined
// and will have `token.Type() == ErrorToken`.
var (
	ErrLex            = errors.New("Token error")
	ErrEarlyEOF       = fmt.Errorf("%w: Unexpected EOF", ErrLex)
	ErrBadIndent      = fmt.Errorf("%w: Unexpected indent", ErrLex)
	ErrBadOutdent     = fmt.Errorf("%w: Unmatched indent", ErrLex)
	ErrNoUnquote      = fmt.Errorf("%w: Unexpected end-of-line", ErrLex)
	ErrLexJSON        = fmt.Errorf("JSON %w", ErrLex)
	ErrJSONNotAllowed = fmt.Errorf("%w: Unexpected start of JSON", ErrLexJSON)
)

// Tokenizer provides a stream of tokens from an input stream.
type Tokenizer struct {
	r              *bufio.Reader
	lineno, offset int
	line           []byte
	lastToken      TokenType
	lastWordEnd    int
	indentStack    list.List
	outdenting     bool
}

// NewTokenizer creates and initialises a new Tokenizer.
func NewTokenizer(r io.Reader) (t *Tokenizer) {
	t = &Tokenizer{
		r:      bufio.NewReader(r),
		lineno: 0, offset: 0,
		line:        nil,
		lastToken:   nilToken,
		indentStack: list.List{},
	}

	t.indentStack.PushBack([]byte{})

	return
}

// Next returns the next token in the stream.
// It returns an error of io.EOF at the end of the file.
func (t *Tokenizer) Next() (tok Token, err error) {
	if t.lastToken == ErrorToken {
		return nil, io.EOF
	}

	tok = nil
	err = nil

	if t.line == nil {
		t.lineno++
		t.offset = 1
		t.line, err = t.r.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
			} else {
				return
			}
		}
	}

	if len(t.line) <= t.offset-1 {
		switch t.lastToken {
		case WordToken:
			t.lastToken = TerminatorToken
			tok = &terminatorToken{
				info: LineInfo{t.lineno, t.offset, t.line},
				skip: 0,
			}
			return

		case TerminatorToken, OutdentToken:
			if t.indentStack.Len() > 1 {
				// Ensure all indented blocks are closed.
				// Don't close the root block though.

				tailIndent := t.indentStack.Remove(
					t.indentStack.Back()).([]byte)
				t.lastToken = OutdentToken
				tok = &outdentToken{
					LineInfo{t.lineno, t.offset, t.line},
					tailIndent,
				}
				return
			}

		}

		t.lastToken = ErrorToken
		return nil, io.EOF
	}

	switch t.line[t.offset-1] {
	case ' ', '\t':
		t.offset++
		return t.Next()

	case '\n':
		if t.lastToken == nilToken || t.lastToken == TerminatorToken {
			t.line = nil
			return t.Next()
		}

		t.lastToken = TerminatorToken
		tok = &terminatorToken{
			info: LineInfo{t.lineno, t.offset, t.line},
			skip: 0,
		}
		t.line = nil
		return

	case '#':
		if t.lastToken == WordToken {
			eol := len(t.line)
			if t.line[eol-1] == '\n' && t.line[eol-2] == '\r' {
				eol--
			}

			t.lastToken = TerminatorToken
			tok = &terminatorToken{
				info: LineInfo{t.lineno, t.lastWordEnd, t.line},
				skip: eol - t.lastWordEnd,
			}
			return
		}

		tok = &commentToken{LineInfo{
			t.lineno,
			t.offset,
			t.line,
		}}
		t.line = nil
		return

	case '{', '[':
		if t.lastToken != WordToken {
			t.lastToken = ErrorToken
			tok = &errorToken{LineInfo{t.lineno, t.offset, t.line}}
			err = ErrJSONNotAllowed
			return
		}

		return t.nextJSON()

	default:
		if t.lastToken == nilToken && t.offset != 1 {
			t.lastToken = ErrorToken
			tok = &errorToken{LineInfo{t.lineno, t.offset, t.line}}
			err = ErrBadIndent
			return
		} else if t.lastToken == TerminatorToken {
			indent := t.line[:t.offset-1]
			tail := t.indentStack.Back()
			tailData := tail.Value.([]byte)
			if bytes.HasPrefix(indent, tailData) {
				if t.outdenting {
					// Didn't recognise indent!
					t.lastToken = ErrorToken
					tok = &errorToken{LineInfo{t.lineno, t.offset, t.line}}
					err = ErrBadOutdent
					return
				}

				if len(indent) != len(tailData) {
					// More stuff in indent than tailData,
					// so we've indented.
					t.indentStack.PushBack(indent)
					t.lastToken = IndentToken
					tok = &indentToken{LineInfo{
						t.lineno, t.offset, t.line,
					}}
					return
				}

				// indent & tailData are the same;
				// no indents or outdents,
				// so carry on with the word.

			} else if bytes.HasPrefix(tailData, indent) {
				// More stuff in tailData than indent,
				// so we've outdented.

				t.indentStack.Remove(tail)
				tail = t.indentStack.Back()
				tailData = tail.Value.([]byte)
				if !bytes.Equal(tailData, indent) {
					t.outdenting = true
					return t.Next()
				}

				t.lastToken = OutdentToken
				tok = &outdentToken{
					LineInfo{t.lineno, t.offset, t.line},
					tailData,
				}
				return

			} else {
				// Didn't recognise indent!
				t.lastToken = ErrorToken
				tok = &errorToken{LineInfo{t.lineno, t.offset, t.line}}
				err = ErrBadOutdent
				return
			}
		}

		return t.nextWord()
	}
}

func (t *Tokenizer) nextWord() (tok Token, err error) {
	word := &wordToken{}
	tok = word
	word.line = t.line
	word.charstops = []charstop{}
	err = nil

	var quote byte = 0
	var at int = 0
	for i := t.offset - 1; i < len(t.line); i++ {
		c := t.line[i]

		if quote == 0 {
			if c == ' ' || c == '\n' || c == '#' {
				break

			} else if c == '"' || c == '\'' {
				quote = c
				word.charstops = append(word.charstops, charstop{
					at, t.lineno, i + 2,
				})

				at-- // Undo next loop's increment

			} else {
				if len(word.charstops) == 0 {
					word.charstops = append(word.charstops, charstop{
						at, t.lineno, i + 1,
					})
				}

				word.word = append(word.word, c)
			}

		} else if c == quote {
			quote = 0
			word.charstops = append(word.charstops, charstop{
				at, t.lineno, i + 1,
			})

			at-- // Undo next loop's increment

		} else if c == '\n' {
			// Can't have newline in quote!
			t.lastToken = ErrorToken
			tok = &errorToken{LineInfo{t.lineno, i + 1, t.line}}
			err = ErrNoUnquote
			return

		} else {

			word.word = append(word.word, c)
		}

		at++
		t.offset++
	}

	if t.offset-1 > len(t.line) {
		t.lastToken = ErrorToken
		tok = &errorToken{LineInfo{t.lineno, t.offset - 1, t.line}}
		err = ErrEarlyEOF
	} else {
		t.lastToken = WordToken
		t.lastWordEnd = t.offset
	}

	return
}

func (t *Tokenizer) nextJSON() (tok Token, err error) {
	// TODO
	t.lastToken = ErrorToken
	tok = &errorToken{LineInfo{t.lineno, t.offset, t.line}}
	err = ErrJSONNotAllowed
	return
}

// TokenType enumerates the valid types of token.
type TokenType int

// The valid token type identifiers.
const (
	nilToken = TokenType(iota)
	// Dummy token carrying error detail.
	// Text() will return the full line.
	// LineInfo(0) will return details of where the error occurred.
	ErrorToken
	// Token is a single "word", as defined in shell syntax.
	// Text() will return the shell-parsed text of a field.
	WordToken
	// Token is a complete JSON object or array.
	// Full syntactic validation is outside the scope of this interface,
	// though if the syntax is valid then this is one complete object.
	// Text() will return JSON source.
	ObjectToken
	// Token signals that a directive has ended.
	// This will be followed by either a WordToken, an IndentToken,
	// or io.EOF.
	// Text() will return []byte{'\n'}.
	TerminatorToken
	// Token signals that the source has indented to create a new block.
	// Text() will return the exact indentation sequence
	// used for the new block.
	IndentToken
	// Token signals that the source has outdented to terminate one block.
	// If multiple blocks are being terminated on one line,
	// a correct OutdentToken will be produced for each block.
	// Text() will return the exact indentation sequence
	// used for the block being outdented to.
	OutdentToken
	// Token is a comment.
	// Usually, this token will be ignored,
	// but may be useful to implement metadirectives.
	// Text() will return the full text of the comment,
	// including the leading comment character.
	CommentToken
)

// The Token interface defines the available introspection on a token.
// Each kind of token will implement this interface slightly differently.
type Token interface {
	// Type returns the type of this token.
	Type() TokenType
	// LineInfo returns details of the line's source code
	// at the given character index into Text().
	LineInfo(at int) LineInfo
	// Text returns the parsed value of this token.
	Text() []byte
}

// LineInfo carries details about the source code of a given token.
type LineInfo struct {
	// The 1-based index of the line in the source code
	Lineno int
	// The 1-based index of the character at question within the line
	Offset int
	// The full line of source code at `Lineno`
	Text []byte
}

func (l LineInfo) String() string {
	return fmt.Sprintf("LineInfo{%d, %d, ...}", l.Lineno, l.Offset)
}

type charstop struct {
	at     int
	lineno int
	offset int
}

type errorToken struct {
	info LineInfo
}

func (t *errorToken) Type() TokenType {
	return ErrorToken
}

func (t *errorToken) LineInfo(at int) LineInfo {
	return t.info
}

func (t *errorToken) Text() []byte {
	return t.info.Text
}

type wordToken struct {
	line      []byte
	word      []byte
	charstops []charstop
}

func (t *wordToken) Type() TokenType {
	return WordToken
}

func (t *wordToken) LineInfo(at int) LineInfo {
	return findCharstop(at, t.charstops, t.line)
}

func (t *wordToken) Text() []byte {
	return t.word
}

type jsonToken struct {
	src       []byte
	charstops []charstop
}

func (t *jsonToken) Type() TokenType {
	return ObjectToken
}

func (t *jsonToken) LineInfo(at int) LineInfo {
	return findCharstop(at, t.charstops, t.src)
}

func (t *jsonToken) Text() []byte {
	return t.src[t.charstops[0].offset-1:]
}

type terminatorToken struct {
	info LineInfo
	skip int
}

func (t *terminatorToken) Type() TokenType {
	return TerminatorToken
}

func (t *terminatorToken) LineInfo(at int) LineInfo {
	return t.info
}

func (t *terminatorToken) Text() []byte {
	return t.info.Text[t.info.Offset+t.skip-1:]
}

type indentToken struct {
	info LineInfo
}

func (t *indentToken) Type() TokenType {
	return IndentToken
}

func (t *indentToken) LineInfo(at int) LineInfo {
	return t.info
}

func (t *indentToken) Text() []byte {
	return t.info.Text[:t.info.Offset-1]
}

type outdentToken struct {
	info   LineInfo
	indent []byte
}

func (t *outdentToken) Type() TokenType {
	return OutdentToken
}

func (t *outdentToken) LineInfo(at int) LineInfo {
	return t.info
}

func (t *outdentToken) Text() []byte {
	return t.indent
}

type commentToken struct {
	info LineInfo
}

func (t *commentToken) Type() TokenType {
	return CommentToken
}

func (t *commentToken) LineInfo(at int) LineInfo {
	return t.info
}

func (t *commentToken) Text() []byte {
	eol := len(t.info.Text)
	if t.info.Text[eol-1] == '\n' {
		eol--
		if t.info.Text[eol-1] == '\r' {
			eol--
		}
	}

	return t.info.Text[t.info.Offset-1 : eol]
}

func findCharstop(at int, charstops []charstop, line []byte) LineInfo {
	lastline := 1
	startat := 0

	for _, stop := range charstops {
		if stop.lineno > lastline {
			lastline = stop.lineno
			startat = stop.at
		}

		if stop.at <= at {
			endl := bytes.IndexByte(line[startat:], '\n')
			if endl < 0 {
				return LineInfo{
					stop.lineno,
					stop.offset + (at - stop.at),
					line[startat:],
				}
			}

			return LineInfo{
				stop.lineno,
				stop.offset + (at - stop.at),
				line[startat:endl],
			}
		}
	}

	return LineInfo{-1, -1, nil}
}
