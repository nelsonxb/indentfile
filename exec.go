package indentfile

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode"
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
		path = "<stdin>"
	} else {
		r, err = os.Open(path)
		if err != nil {
			return
		}

		defer r.Close()
	}

	return ErrorInFile(Parse(r, context), path)
}

func ParseTokens(tok *Tokenizer, context interface{}) (err error) {
	handler := getDirectiveHandlerFor(context)
	var token Token

	var block interface{}
	var line []Token
	var words []string
	var json []byte

tokenLoop:
	for token, err = tok.Next(); err == nil; token, err = tok.Next() {
		switch token.Type() {
		case WordToken:
			line = append(line, token)
			words = append(words, string(token.Text()))

		case ObjectToken:
			line = append(line, token)
			json = token.Text()

		case TerminatorToken:
			line = append(line, token)
			if len(json) == 0 {
				block, err = handler.Directive(words[0], words[1:])
			} else {
				block, err = handler.ObjectDirective(words[0], words[1:], json)
			}

			if err != nil {
				if locatable, is := err.(errLocatable); is {
					err = locatable.IntoLocation(line)
				} else if errors.Is(err, ErrArgumentJSON) {
					err = errorAt(err, line[len(line)-2].LineInfo(0))
				} else {
					err = errorAt(err, line[0].LineInfo(0))
				}
				return
			}

			line = nil
			words = nil
			json = nil

		case IndentToken:
			if block == nil {
				return errorAt(ErrIndent, token.LineInfo(0))
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
	return methodDirectiveHandler(valueOf)
}

type patchedHandler struct {
	DirectiveHandler
}

func (h *patchedHandler) ObjectDirective(name string, argv []string, object []byte) (interface{}, error) {
	return nil, ErrArgumentJSON
}

type methodDirectiveHandler reflect.Value

func (ctx methodDirectiveHandler) Directive(name string, argv []string) (interface{}, error) {
	return ctx.ObjectDirective(name, argv, nil)
}

func (ctx methodDirectiveHandler) ObjectDirective(name string, argv []string, object []byte) (interface{}, error) {
	if strings.ToLower(name) != name {
		return nil, DirectiveErrorf("%w %q", ErrUnknown, name)
	}

	methodName := snakeToPascal(name)

	method := reflect.Value(ctx).MethodByName(methodName)
	if !method.IsValid() {
		return nil, DirectiveErrorf("%w %q", ErrUnknown, name)
	}

	methodType := method.Type()
	nargs := methodType.NumIn()
	nret := methodType.NumOut()
	var argValues, results []reflect.Value

	if nret > 2 {
		return nil, DirectiveErrorf("%w %q", ErrUnknown, name)
	} else if methodName == "End" && nargs == 0 && nret == 1 {
		if methodType.Out(0).Implements(
			reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, DirectiveErrorf("%w %q (.End is a reserved method)",
				ErrUnknown, name)
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
			return nil, DirectiveErrorf("%w %q (.%s has bad signature)",
				ErrUnknown, name, methodName)
		}
	}

	for i := 0; i < nargs; i++ {
		argType := methodType.In(i)
		if argType.Kind() != reflect.String {
			if objIndex == -1 {
				objIndex = i
			} else {
				return nil, DirectiveErrorf("%w %q (.%s has bad signature)",
					ErrUnknown, name, methodName)
			}
		}
	}

	if object != nil {
		nargs--
	}

	if nargv < nargs {
		return nil, ArgumentErrorf(nargv+3, "not enough arguments")
	}

	if !methodType.IsVariadic() && nargv > nargs {
		return nil, ArgumentErrorf(nargs, "too many arguments")
	}

	if objIndex == -1 {
		return nil, ArgumentErrorf(-1, "expected JSON argument")
	}

	argValues = make([]reflect.Value, nargv)
	if object != nil {
		objArgType := methodType.In(objIndex)
		unPtr := false
		if objArgType.Kind() == reflect.Ptr {
			objArgType = objArgType.Elem()
		} else {
			unPtr = true
		}
		objArgValue := reflect.New(objArgType)
		err := json.Unmarshal(object, objArgValue.Interface())
		if err != nil {
			return nil, ArgumentErrorf(-1, "%w", err)
		}

		argValues = append(argValues, reflect.Value{})
		if unPtr {
			argValues[objIndex] = objArgValue.Elem()
		} else {
			argValues[objIndex] = objArgValue
		}
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
			return nil, DirectiveErrorf("%w", err)
		} else {
			return result, nil
		}
	} else if len(results) == 2 {
		result := results[0].Interface()
		err, isErr := results[1].Interface().(error)
		if isErr && err != nil {
			err = DirectiveErrorf("%w", err)
		}
		return result, err
	} else {
		return nil, nil
	}
}

func snakeToPascal(name string) string {
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
