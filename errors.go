package indentfile

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrSyntax = errorWrap("syntax error", nil)

	ErrToken       = errorWrap("syntax error", ErrSyntax)
	ErrEOF         = errorWrap("unexpected eof", ErrToken)
	ErrCRLF        = errorWrap("line ending error", ErrToken)
	ErrIndent      = errorWrap("unexpected indent", ErrToken)
	ErrOutdent     = errorWrap("unmatched indent", ErrToken)
	ErrUnquote     = errorWrap("unclosed quotes", ErrToken)
	ErrJSONBracket = errorWrap("unmatched JSON syntax", ErrToken)

	ErrDirective    = errorWrap("directive error", ErrSyntax)
	ErrUnknown      = errorWrap("unknown directive", ErrDirective)
	ErrArguments    = errorWrap("bad argument", ErrDirective)
	ErrArgumentJSON = errorWrap("unexpected JSON", ErrArguments)
)

func ErrorLocation(err error) LineInfo {
	if errWithLine, is := err.(lineError); is {
		return errWithLine.Location()
	}

	return LineInfo{}
}

func ErrorInFile(err error, filename string) error {
	if locErr, is := err.(errWithLocation); is {
		return errWithLocation{
			Err:      locErr.Err,
			Detail:   locErr.Detail,
			File:     filename,
			LineInfo: locErr.LineInfo,
		}
	}

	return err
}

func DirectiveErrorf(format string, v ...interface{}) error {
	err := ErrDirective

	if strings.HasPrefix(format, "%w") {
		err = v[0].(error)
		v = v[1:]
		stopAfter := strings.Index(format, " ")
		if stopAfter > 0 {
			format = format[stopAfter+1:]
		} else {
			format = format[2:]
		}
	}

	detail := fmt.Sprintf(format, v...)

	return errorArg{
		Err:    err,
		Index:  0,
		Detail: detail,
	}
}

func ArgumentErrorf(index int, format string, v ...interface{}) error {
	err := ErrArguments

	if strings.HasPrefix(format, "%w") {
		err = v[0].(error)
		v = v[1:]
		stopAfter := strings.Index(format, " ")
		if stopAfter > 0 {
			format = format[stopAfter+1:]
		} else {
			format = format[2:]
		}
	}

	detail := fmt.Sprintf(format, v...)

	if index < 0 {
		index = -1
	} else {
		index++
	}

	return errorArg{
		Err:    err,
		Index:  index,
		Detail: detail,
	}
}

type lineError interface {
	Location() LineInfo
}

type errSimpleWrapper struct {
	Err     error
	Message string
}

func errorWrap(msg string, err error) error {
	return errSimpleWrapper{err, msg}
}

func (err errSimpleWrapper) Error() string {
	return err.Message
}

func (err errSimpleWrapper) Unwrap() error {
	return err.Err
}

type errWithLocation struct {
	Err       error
	DetailErr error
	Detail    string
	File      string
	LineInfo
}

func errorAt(err error, info LineInfo) error {
	if locErr, is := err.(errWithLocation); is {
		return errWithLocation{
			Err:      locErr.Err,
			Detail:   locErr.Detail,
			File:     locErr.File,
			LineInfo: info,
		}
	}

	return errWithLocation{
		Err:      err,
		LineInfo: info,
	}
}

func errorAtf(err error, info LineInfo, format string, v ...interface{}) error {
	if locErr, is := err.(*errWithLocation); is {
		return errWithLocation{
			Err:      locErr.Err,
			Detail:   fmt.Sprintf(format, v...),
			File:     locErr.File,
			LineInfo: info,
		}
	}

	return errWithLocation{
		Err:      err,
		Detail:   fmt.Sprintf(format, v...),
		LineInfo: info,
	}
}

func (err errWithLocation) Error() string {
	detail := err.Detail
	if detail != "" {
		detail = ": " + detail
	}
	if err.DetailErr != nil {
		detail = ": " + err.DetailErr.Error()
	}

	if err.File == "" {
		return fmt.Sprintf("%s at line %d:%d%s",
			err.Err.Error(), err.Lineno, err.Offset, detail)
	}

	return fmt.Sprintf("%s in file %s (%d:%d)%s",
		err.Err.Error(), err.File, err.Lineno, err.Offset, detail)
}

func (err errWithLocation) Unwrap() error {
	return err.Err
}

func (err errWithLocation) Is(target error) bool {
	return errors.Is(err.DetailErr, target)
}

type errLocatable interface {
	IntoLocation(tokens []Token) error
}

type errorArg struct {
	Err    error
	Index  int
	Detail string
}

func (err errorArg) Error() string {
	return err.Err.Error()
}

func (err errorArg) Unwrap() error {
	return err.Err
}

func (err errorArg) IntoLocation(tokens []Token) error {
	var problemToken Token
	if err.Index >= 0 && err.Index < len(tokens) {
		problemToken = tokens[err.Index]
	} else if err.Index < 0 {
		lastToken := tokens[len(tokens)-2]
		if lastToken.Type() == ObjectToken {
			problemToken = lastToken
		}
	}

	if problemToken == nil {
		problemToken = tokens[len(tokens)-1]
	}

	actualErr := err.Err
	var detailErr error
	if !errors.Is(actualErr, ErrSyntax) {
		detailErr = err.Err
		if err.Index == 0 {
			actualErr = ErrDirective
		} else {
			actualErr = ErrArguments
		}
	}

	return errWithLocation{
		Err:       actualErr,
		DetailErr: detailErr,
		Detail:    err.Detail,
		LineInfo:  problemToken.LineInfo(0),
	}
}
