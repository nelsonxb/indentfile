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

// Parse reads an indentfile from the stream r and processes its directives.
// Context determines what directives are available and how they behave.
// It returns a non-nil error if r.Read() has an error,
// for any syntactic problems,
// or if any of the directive handlers return an error.
// If the returned error value did not come from r.Read(),
// line and column information can be retrieved
// using ErrorLocation().
//
// If context is a DirectiveHandler,
// it will be interpreted differently to what's laid out here.
// See that type's documentation for details.
//
// Otherwise, for most types, the set of defined methods
// determines which top-level directives exist.
// When a directive is encountered,
// it is converted from "kebab-case" to "UpperCamelCase".
// Context is then checked for a method by that converted name.
// If it exists, and has the right signature,
// then it is called.
// Each argument on the directive is passed as a string
// argument to the method (except for JSON arguments - detailed later).
// The return value of these methods is then checked.
//
// The signature of a method must be valid for the directive search to succeed.
// If there are no expected JSON arguments,
// then all arguments must be strings.
// If this is not the case,
// then it is assumed that the method
// isn't actually meant to be a directive.
// A method may have variadic string arguments -
// they will be filled as you might expect.
// If a method has one non-string argument (at any position),
// then it is assumed that this directive expects a JSON argument.
// Assuming the JSON argument is provided,
// it is unmarshalled into a new instance of the argument type
// before calling the method (using encoding/json).
//
// If a type is an EndDirectiveHandler,
// then the End() method is not considered a valid directive.
// It will be called once all directives have been processed
// (no other methods will be called on the context
// after the call to End()).
//
// A method may have zero, one, or two return values.
// If there are two, then the first is used as a sub-context
// and the second is used as an error type.
// If the error is non-nil, it is wrapped with details
// on where the error occurred, and is returned from Parse.
// If the error is nil, and the sub-context is non-nil,
// then it used in an identical manner to the context argument to Parse
// for any indented directives below the active directive.
// If the sub-context is nil,
// then sub-directives will not be permitted
// and will produce an error.
//
// If there is one return value,
// and it is declared in the signature as an error type,
// then it is interpreted as with the second return value above.
// Otherwise, it is interpreted as for the first error value.
func Parse(r io.Reader, context interface{}) error {
	return ParseTokens(NewTokenizer(r), context)
}

// ParseFile behaves exactly like Parse,
// but gets the reader from the named file.
// Errors with line information are automatically
// passed to ErrorInFile().
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

// ParseTokens behaves exactly like Parse,
// except that it takes an existing token stream.
// Unlike Parse, it can be used if the stream has already
// been partially consumed.
//
// It stops if it encounters the end of the stream,
// or if it finds the end of a block that it didn't start.
// This could be useful if you want to use Parse to handle an indented block -
// call this function just after tok.Next() returns an IndentToken.
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
		err := results[1].Interface().(error)
		if err != nil {
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
