# goop

`goop` is a small Go library for building dynamic class-based object models. It
provides single inheritance, constructors, encapsulated fields and methods,
method overriding, virtual dispatch, and parent-method calls (`super`).

Go itself does not have class syntax; this package models these features with
Go structs, functions, and runtime checks. The class API is dynamic: values and
method arguments use `any`, so callers should type-assert values where needed.

## Install and import

Install the runtime package in a Go project:

```powershell
go get github.com/gdaccincr/goop
```

Then import it normally:

```go
import "github.com/gdaccincr/goop"
```

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
  without reducing the visibility of inherited public or protected methods.
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

## Verify

```sh
go test ./...
go run ./examples/basic
```