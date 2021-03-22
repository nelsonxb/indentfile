package indentfile_test

import (
	"strings"

	"github.com/nelsonxb/indentfile"
)

var fileSource string = `
config option1 option2
	property key value

directive arg1 arg2

object-directive "with json" [
	{"hello": "world"}
]

	add {
		"hello": "user"
	}
`

func Example_reflection() {
	context := &ExampleContext{}
	indentfile.Parse(strings.NewReader(fileSource), context)
}

type ExampleContext struct{}

func (*ExampleContext) Config(options ...string) *ExampleConfig {
	// ...
	return &ExampleConfig{}
}

// Note that - since this function doesn't properly implement
// DirectiveHandler - the type as a whole still uses the reflection API.
func (*ExampleContext) Directive(arg1 string, arg2 string) error {
	// ...
	return nil
}

// Note that this is a type that will be automatically picked up
// and unmarshalled using encoding/json.
type Greeting struct {
	Hello string `json:"hello"`
}

func (*ExampleContext) ObjectDirective(wordArg string, jsonArg []Greeting) (*ObjectConfig, error) {
	// ...
	return &ObjectConfig{}, nil
}

type ExampleConfig struct{}

func (*ExampleConfig) Property(key string, value string) {
	// ...
}

type ObjectConfig struct{}

func (*ObjectConfig) Add(greeting *Greeting) {
	// ...
}
