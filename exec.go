package indentfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode"
)

var (
	ErrDirective        = errors.New("Directive error")
	ErrNoDirective      = fmt.Errorf("%w: directive not recognised", ErrDirective)
	ErrNumArgs          = fmt.Errorf("%w: argument count incorrect", ErrDirective)
	ErrObjectNotAllowed = fmt.Errorf("%w: JSON object not used for this directive", ErrDirective)
)

type DirectiveHandler interface {
	Directive(name string, argv []string) (interface{}, error)
}

type ObjectDirectiveHandler interface {
	DirectiveHandler
	ObjectDirective(name string, argv []string, json []byte) (interface{}, error)
}

type EndDirectiveHandler interface {
	End() error
}

type HandlerFunc func(name string, argv []string) (interface{}, error)
type ObjectHandlerFunc func(name string, argv []string, json []byte) (interface{}, error)

func (fn HandlerFunc) Directive(name string, argv []string) (interface{}, error) {
	return fn(name, argv)
}

func (fn ObjectHandlerFunc) Directive(name string, argv []string) (interface{}, error) {
	return fn(name, argv, nil)
}

func (fn ObjectHandlerFunc) ObjectDirective(name string, argv []string, json []byte) (interface{}, error) {
	return fn(name, argv, json)
}

func Parse(r io.Reader, context interface{}) error {
	return ParseTokens(NewTokenizer(r), context)
}

func ParseFile(path string, context interface{}) (err error) {
	var r io.ReadCloser
	if path == "-" {
		r = os.Stdin
	} else {
		r, err = os.Open(path)
		if err != nil {
			return
		}
	}

	return Parse(r, context)
}

func ParseTokens(tok *Tokenizer, context interface{}) (err error) {
	handler := getDirectiveHandlerFor(context)
	var token Token

	var block interface{}
	var words []string
	var json []byte

tokenLoop:
	for token, err = tok.Next(); err == nil; token, err = tok.Next() {
		switch token.Type() {
		case WordToken:
			words = append(words, string(token.Text()))

		case ObjectToken:
			json = token.Text()

		case TerminatorToken:
			if len(json) == 0 {
				block, err = handler.Directive(words[0], words[1:])
			} else {
				block, err = handler.ObjectDirective(words[0], words[1:], json)
			}

			if err != nil {
				return
			}

			words = nil
			json = nil

		case IndentToken:
			if block == nil {
				return ErrBadIndent
			}

			err = ParseTokens(tok, block)
			if err != nil {
				return
			}

		case OutdentToken:
			break tokenLoop

		default:
			continue
		}
	}

	if err == io.EOF {
		err = nil
	} else if err != nil {
		return
	}

	if ender, is := context.(EndDirectiveHandler); is {
		err = ender.End()
	}

	return
}

func getDirectiveHandlerFor(context interface{}) ObjectDirectiveHandler {
	if handler, is := context.(ObjectDirectiveHandler); is {
		return handler
	} else if handler, is := context.(DirectiveHandler); is {
		return &patchedHandler{handler}
	}

	valueOf := reflect.ValueOf(context)
	if valueOf.Kind() != reflect.Ptr {
		panic(fmt.Errorf("Type %v is not a pointer", valueOf.Type()))
	}

	return methodDirectiveHandler(valueOf)
}

type patchedHandler struct {
	DirectiveHandler
}

func (h *patchedHandler) ObjectDirective(name string, argv []string, object []byte) (interface{}, error) {
	return nil, ErrObjectNotAllowed
}

type methodDirectiveHandler reflect.Value

func (ctx methodDirectiveHandler) Directive(name string, argv []string) (interface{}, error) {
	return ctx.ObjectDirective(name, argv, nil)
}

func (ctx methodDirectiveHandler) ObjectDirective(name string, argv []string, object []byte) (interface{}, error) {
	name = snakeToPascal(name)

	method := reflect.Value(ctx).MethodByName(name)
	if !method.IsValid() {
		return nil, ErrNoDirective
	}

	methodType := method.Type()
	nargs := methodType.NumIn()
	nret := methodType.NumOut()
	var argValues, results []reflect.Value

	if nret > 2 {
		return nil, ErrNoDirective
	} else if name == "End" && nargs == 0 && nret == 1 {
		if methodType.Out(0).Implements(
			reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, ErrNoDirective
		}
	}

	nargv := len(argv)
	objIndex := -1
	if object == nil {
		objIndex = -2
	}

	if methodType.IsVariadic() {
		nargs--
		if methodType.In(nargs).Elem().Kind() != reflect.String {
			return nil, ErrNoDirective
		}
	}

	for i := 0; i < nargs; i++ {
		argType := methodType.In(i)
		if argType.Kind() != reflect.String {
			if objIndex == -1 {
				objIndex = i
			} else {
				return nil, ErrNoDirective
			}
		}
	}

	if nargv < nargs {
		return nil, ErrNumArgs
	}

	if !methodType.IsVariadic() && nargv > nargs {
		return nil, ErrNumArgs
	}

	if objIndex == -1 {
		return nil, ErrNoDirective
	}

	argValues = make([]reflect.Value, nargv)
	if object != nil {
		objArgType := methodType.In(objIndex)
		if objArgType.Kind() == reflect.Ptr {
			objArgType = objArgType.Elem()
		}
		objArgValue := reflect.New(objArgType)
		err := json.Unmarshal(object, objArgValue.Interface())
		if err != nil {
			return nil, err
		}

		argValues[objIndex] = objArgValue
	} else {
		objIndex = nargv
	}

	for i, arg := range argv {
		if i >= objIndex {
			i++
		}

		argValues[i] = reflect.ValueOf(arg)
	}

	results = method.Call(argValues)

	if len(results) == 1 {
		result := results[0].Interface()
		if err, is := result.(error); is {
			return nil, err
		} else {
			return result, nil
		}
	} else if len(results) == 2 {
		result := results[0].Interface()
		err := results[1].Interface().(error)
		return result, err
	} else {
		return nil, nil
	}
}

func snakeToPascal(name string) string {
	if strings.ToLower(name) != name {
		panic(fmt.Errorf("Name %q is not lower-case", name))
	}

	isFirstOfWord := true
	isFirstOfName := true
	needPrepend := false
	name = strings.Map(func(c rune) rune {
		if c == '-' {
			isFirstOfWord = true
			return -1
		}

		if isFirstOfWord {
			k := unicode.ToUpper(c)
			if k == c && isFirstOfName {
				needPrepend = true
			}

			isFirstOfWord = false
			isFirstOfName = false
			return k
		}

		return c
	}, name)

	if needPrepend {
		return "X" + name
	}

	return name
}
