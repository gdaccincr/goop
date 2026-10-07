# goop

`goop` is a small Go library for building dynamic class-based object models. It
provides single inheritance, constructors, encapsulated fields and methods,
method overriding, virtual dispatch, and parent-method calls (`super`).

Go itself does not have class syntax; this package models these features with
Go structs, functions, and runtime checks. The class API is dynamic: values and
method arguments use `any`, so callers should type-assert values where needed.

## Install and import

The module path matches this repository's URL:

```go
module github.com/gdaccincr/goop
```

Then consumers can import it normally:

```go
import "github.com/gdaccincr/goop"
```

The preprocessor has its own `-runtime` flag for the runtime import path. When
using it after changing the module path, pass the same repository path:

```powershell
go run ./cmd/goopc -in app.oop -out app.go -runtime github.com/acme/goop
```

For a private GitHub module, configure access on each development machine:

```powershell
go env -w GOPRIVATE=github.com/gdaccincr/*
```

Make sure Git is authenticated (SSH or a credential manager) and the user has
repository access. During local development, another module can use a `replace`
directive pointing to this checkout instead of fetching it remotely.

## Quick start

```go
package main

import (
	"fmt"
	"log"

	"github.com/gdaccincr/goop"
)

func main() {
	Animal, err := goop.Define(goop.ClassSpec{
		Name: "Animal",
		Fields: []goop.FieldSpec{{
			Name: "name", Visibility: goop.Protected, Default: "unknown",
		}},
		Constructor: func(ctx *goop.Context, args ...any) error {
			if len(args) > 0 {
				return ctx.Set("name", args[0])
			}
			return nil
		},
		Methods: []goop.MethodSpec{{
			Name: "Speak", Visibility: goop.Public,
			Handler: func(ctx *goop.Context, _ ...any) (any, error) {
				name, err := ctx.Get("name")
				if err != nil {
					return nil, err
				}
				return fmt.Sprintf("%v makes a sound", name), nil
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	Dog, err := goop.Define(goop.ClassSpec{
		Name: "Dog",
		Parent: Animal,
		Methods: []goop.MethodSpec{{
			Name: "Speak", Visibility: goop.Public,
			Handler: func(ctx *goop.Context, _ ...any) (any, error) {
				parentSound, err := ctx.Super("Speak")
				if err != nil {
					return nil, err
				}
				return fmt.Sprintf("%v and barks", parentSound), nil
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	dog, err := goop.New(Dog, "Rex") // Animal constructor runs automatically.
	if err != nil {
		log.Fatal(err)
	}
	sound, err := dog.Call("Speak") // Calls Dog.Speak, then Animal.Speak via super.
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sound) // Rex makes a sound and barks
}
```

## API and behavior

- `Define(ClassSpec)` creates an immutable class definition. A class can have a
  parent, fields, methods, and an optional constructor.
- `New(class, args...)` allocates an object, copies field defaults, then invokes
  constructors in parent-to-child order. Each constructor receives the same
  arguments. A constructor error aborts creation.
- `Object.Get` and `Object.Set` access public fields. Methods and constructors
  use `Context.Get` and `Context.Set` to access fields permitted to their class.
- `Object.Call` invokes public methods using the most-derived implementation.
  `Context.Call` applies the current class's visibility permissions while
  retaining virtual dispatch.
- `Context.Super` explicitly invokes the nearest parent implementation of a
  method. It returns `ErrMethodNotFound` when no parent implementation exists.
- `Public` fields and methods are available to all callers. `Protected` members
  are available to the declaring class and its descendants. `Private` members
  are available only to the exact declaring class and are not inherited.
- Inherited field names cannot be redeclared. Methods can be overridden,
  including with a different visibility; normal access checks still apply.
- `Class.IsA` checks a class's ancestry. `Object.Class` returns the concrete
  class.

Errors wrap sentinel errors such as `ErrAccessDenied` and `ErrFieldNotFound`,
which can be checked with `errors.Is`.

## Limitations

This library does not change Go syntax or provide compile-time enforcement of
class relationships. Go interfaces and ordinary typed structs remain the best
choice when static typing is important. Field defaults that are maps or slices
are shallow-copied per object; pointers and other reference-valued defaults are
shared as they normally are in Go. Synchronize access to mutable reference
values yourself.

## Class syntax preprocessor

The `goopc` command adds class-like syntax to `.oop` source files and translates
them into ordinary Go. Method and constructor bodies are Go code; only class
declarations and member visibility are new syntax. The current deliberately
small syntax is:

```text
class Animal {
    protected name = "unknown"

    constructor {
        if len(args) > 0 {
            return this.Set("name", args[0])
        }
        return nil
    }

    public Speak {
        name, err := this.Get("name")
        if err != nil { return nil, err }
        return fmt.Sprintf("%v speaks", name), nil
    }
}

class Dog extends Animal {
    public Speak {
        sound, err := this.Super("Speak")
        if err != nil { return nil, err }
        return fmt.Sprintf("%v barks", sound), nil
    }
}
```

Fields use `visibility name = default`; the default value supplies the dynamic
field's value and type. Methods and constructors receive implicit `this` and
`args ...any` variables. Methods return `(any, error)`, and constructors return
`error`, just like the runtime API. `this.Get` and `this.Set` enforce field
visibility. `this.Call` dispatches methods virtually, and `this.Super("Name")`
calls the parent implementation. Parent classes must appear earlier in the
same source file. The generated class variables are package-level values.

Translate and run the checked-in sample:

```powershell
go run ./cmd/goopc -in examples/basic.oop -out examples/basic_generated.go
go run ./examples/basic_generated.go
Remove-Item examples/basic_generated.go
```

Or write generated code to stdout with `-out -`. The preprocessor does not
modify the Go compiler or allow arbitrary new Go syntax: it handles class
declarations and expects the rest of the file and all method bodies to be valid
Go. Its source is the readable input; generated `.go` output is a build artifact.

## Verify

```sh
go test ./...
go run ./examples/basic
go run ./cmd/goopc -in examples/basic.oop -out -
```

## VSCodium extension

A separate local extension for `.oop` highlighting, snippets, completions, and
running `goopc` is in [`vscode-goop`](vscode-goop/README.md). Open that folder
in VSCodium and press `F5` to test it in an Extension Development Host.