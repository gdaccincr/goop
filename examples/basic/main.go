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
			if len(args) == 0 {
				return nil
			}
			return ctx.Set("name", args[0])
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
		Name:   "Dog",
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

	dog, err := goop.New(Dog, "Rex")
	if err != nil {
		log.Fatal(err)
	}
	sound, err := dog.Call("Speak")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sound)
}
