Indentfile text format
======================

[![Go Reference](https://pkg.go.dev/badge/github.com/nelsonxb/indentfile.svg)](https://pkg.go.dev/github.com/nelsonxb/indentfile)

Indentfile is a syntax-light configuration format.
This repo contains the reference parser for Go.


Usage
-----

See the [package documentation on pkg.go.dev][pkg doc]
for more on using the reference parser.

[pkg doc]: https://pkg.go.dev/github.com/nelsonxb/indentfile


Writing indentfiles
-------------------

An indentfile consists of newline-separated directives.
You might use these to directly set properties:

```
some-property the-value
other-property different-value
```

Each directive consists of a name,
and optional arguments.
Each of these parts are specified using shell-like syntax:

```
set greeting 'Hello, World!'
```

Some directives might need more detailed configuration.
An indented block following a directive
allows specifying further sub-directives.
For example:

```
# This user will have all default values
user me

# This user needs special settings
user you
    is rad
    is reading
        object "indentfile documentation"
```

We can see a few extra things in this example:

- Comments begin with a `#`.
  Note that - though it's not shown -
  comments can occur at the end of a line, too.
- A directive might be specified multiple times,
  for example if each one specifies a new object.
- Indented blocks can have further indented blocks.

The exact set of directives, and what they do,
are determined by the application.
This is almost everything you need to know
to start writing an indentfile.

There is one last piece to this puzzle.
Sometimes, you'll need to embed documents
in a more traditional serialization form.
For this, indentfiles support
specifying a JSON argument:

```
set-fields target {
    "key": "value",
    "number": 3
}
```

Until the top-level JSON object is closed,
all indentation becomes insignificant
(so if you wanted to paste a whole mess,
you most certainly could).
JSON arrays are also possible,
and the full JSON syntax is acceptable.
Again, see the documentation for a given directive
to know what kind of JSON it expects.
Note that, currently,
comments are not supported inside JSON.

And that's it!
See the documentation for your specific application for more,
or keep reading on for more details.


Designing indentfiles
---------------------

If you're writing an application that consumes indentfiles,
you should be mindful of what kind of files
your syntax is going to create.
The format was created specifically to reduce
the amount of extra syntax a user has to specify.
The philosophy is that the application
should just do the right thing most of the time,
without being pedantic to the user.

With that in mind, here are some guidelines
when you're figuring out your indentfile syntax:

1. Write a few example indentfiles first.
   Write some for simple, common scenarios,
   and some for the more complex scenarios.
2. Try to use sensible defaults.
   The most common situations
   should require the fewest directives
   and the fewest arguments.
3. Try to use obvious directives.
   What is the user most likely to try
   before looking at the documentation?
4. Read your example files, and ask yourself:
   what's the most obvious behaviour
   for this given file?
   Is it going to behave how I'd expect?
   If not, why?
5. The design you first implement doesn't have to be final.
   See where it breaks, and revise if it's not working right.


Indentfile syntax
-----------------

An indentfile consists of a sequence of directives.
Each directive has a name, zero or more arguments,
a new-line to indicate its end,
and an optional indented block.
The indented block consists of further directives.

Comments can begin anywhere within a line
(except - currently - within JSON arguments).
When an `#` is found (and not part of a quoted word - see below),
it is the start of a comment.
A comment runs until the end of a line.
A tokenizer may choose to return a comment as a token,
but it should usually be syntactically ignored.

A directive name, and most of its arguments,
are all syntactic "words".
A word starts at a non-whitespace character,
and runs until a whitespace character.
If a word contains either a quote (`'`) or double quote (`"`),
then it begins a quoted section.
Everything until the next matching quote is included literally,
including any whitespace (newlines are not currently permitted).
Quoting may begin or end in the middle of a word.
Following is a table of example word syntax,
and how they are returned as a string:

| Syntax | Parsed word |
| ------ | ----------- |
| `Hello!` | `"Hello!"` |
| `'Hello, World!'` | `"Hello, World!"` |
| `hel'lo wor'ld!` | `"hello world!"` |
| `"'this word has internal quotes'"` | `"'this word has internal quotes'"` |
| `"This "'"word"'" isn't very "'"nice" to 'read.` | `"This \"word\" isn't very \"nice\" to read."` |
| `word# but this comment isn't part of it` | `"word"` |
| `"This quoted word can have a #, and it won't become a comment."` | `"This quoted word can have a #, and it won't become a comment."` |

When a newline is encountered
(or a comment, which implicitly ends in a newline),
and one or more words have been found,
a complete directive has been formed.
The directive name is the first word,
and all remaining words are its arguments.
Following a newline, a new directive may begin.

If a directive is indented relative to the previous directive,
it is the start of an indented block.

> TODO: Explain indentation

If an unquoted open brace (`{`) or open bracket (`[`) is encountered
at the start of a word,
then instead this is considered the start of a JSON argument.

> TODO: Explain JSON
