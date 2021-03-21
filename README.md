Indentfile text format
======================

Indentfile is a format for text files, designed for configuration.
This repo contains the reference parser for Go.


File structure
--------------

An indentfile consists of a sequence of directives.
Each directive consists of a sequence of strings,
split using a shell-like syntax.
Indentation allows a directive to create a block of sub-directives.
Each directive may also have one JSON object or array,
which may span multiple lines
and doesn't mess with the rest of the indentation.

A simple file might look like this:

```
# Comments start with a hash

config option1 option2      # Comments can trail lines
    property key value

directive arg1 arg2
```

The first directive -
named `config` and with arguments `option1` and `option2` -
has a sub-directive within its context
named `property` with arguments `key` and `value`.
Blocks are nestable (`property` could also have sub-directives),
and there can be as many of these as needed.

JSON arguments are also supported:

```
object-directive "with json" [
    {"hello": "world"}
]

    add {
        "hello": "user"
    }
```

Although this example is kinda ugly,
it does a decent job of demonstrating the concept.
The last argument can be a complex JSON object or array,
and this doesn't restrict the other features.


Implementation
--------------

This reference implementation offers a few ways to parse these files.
The first two use the `indentfile.Parse` function family.

The most convenient option for most users should be the reflection-based method:

```go
func main() {
	context := &ExampleContext{}
	err := indentfile.Parse(rfd, context)
	// Or...
	err = indentfile.ParseFile("filename", context) // Or "-" for stdin
}

type ExampleContext struct {}

func (*ExampleContext) Config(options ...string) *ExampleConfig {
	// ...
}

func (*ExampleContext) Directive(arg1 string, arg2 string) error {
	// ...
}

// Note that this is a type that will be automatically picked up
// and unmarshalled using encoding/json.
type Greeting struct {
	Hello string `json:"hello"`
}

func (*ExampleContext) ObjectDirective(wordArg string, jsonArg []Greeting) (*ObjectConfig, error) {
	// ...
}

type ExampleConfig struct {}

func (*ExampleConfig) Property(key string, value string) {
	// ...
}

type ObjectConfig struct{}

func (*ObjectConfig) Add(greeting *Greeting) {
	// ...
}
```

It is also possible to use the interface-based API
(which still uses the same `indentfile.Parse` family of functions),
or get a token stream directly from `indentfile.NewTokenizer(io.Reader)`.
