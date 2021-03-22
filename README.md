Indentfile text format
======================

[![Go Reference](https://pkg.go.dev/badge/github.com/nelsonxb/indentfile.svg)](https://pkg.go.dev/github.com/nelsonxb/indentfile)

Indentfile is a format for text files, designed for configuration.
This repo contains the reference parser for Go.


Usage
-----

See the [package documentation on pkg.go.dev](https://pkg.go.dev/github.com/nelsonxb/indentfile)
for more on using the reference parser.


Indentfile syntax
-----------------

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
