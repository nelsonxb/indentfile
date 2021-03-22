/*
Package indentfile implements a parser for
the indentfile configuration syntax.
Documentation for this syntax
can be found in the README.


Using the Reflection API

This API is similar to many other file format APIs,
such as encoding/json.
The primary entry point to this API is the Parse function.
There is also the extra convenience function ParseFile.

The second argument to these functions is an arbitrary object.
Provided that this object does not implement DirectiveHandler,
its set of methods are used as the top-level directives.
The name of a top-level directive
is converted from "kebab-case" to "UpperCamelCase".
If a method by that name exists -
and all its parameters are strings -
then it is called using the directive arguments.

A directive method may have a non-string argument.
In this case, a JSON argument is required.
The JSON argument will be unmarshalled into a new instance of that type.

The return type of the method determines
how to proceed with the next directive.
If the method has a void return,
then no further processing is done.
Sub-directives will not be expected,
and will cause a parse error.
If the directive has one return,
and it is an error type,
then the parsing will fail if it is non-nil.
If the directive otherwise has one return,
and it is not an error and non-nil,
then the returned object is used as the context
for any sub-directives.
If the single return is nil,
then the behaviour is as for a void return.
If the directive has two return values,
they should be a context and an error type.
The error is checked first - as above -
and the context is used also as described above.

See the reflection example for a demonstration of this API.


Using the Interface API

If the reflection API is not sufficiently flexible,
you may instead implement the DirectiveHandler interface.
Conveniently, the interface API can be
mixed-and-matched with the reflection API;
anywhere that a directive context is expected,
a DirectiveHandler can be used instead.

When a DirectiveHandler is used as the context,
the Directive method will be called for every directive in the indentfile.
The directive name and arguments will be passed in the obvious manner.
The return values are the same as in
the two-return mode of the reflection API.

When accepting JSON arguments,
implement the ObjectDirectiveHandler.
The Directive method will still be called
whenever there is no JSON argument,
but ObjectDirective will be called if a JSON argument is present.
The source of the JSON argument will be passed
as a separate argument to the function.
This argument is suitable to pass directly to json.UnmarshalJSON.


Using the Tokenizer API

For even more low-level control,
you may instead use the Tokenizer type
to iterate over a raw token stream.
See the documentation of that type
for more details.


*/
package indentfile
