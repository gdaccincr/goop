package goop

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestInheritanceConstructorsVirtualDispatchAndSuper(t *testing.T) {
	var constructorOrder []string
	base, err := Define(ClassSpec{
		Name:   "Base",
		Fields: []FieldSpec{{Name: "name", Visibility: Protected, Default: "unknown"}},
		Constructor: func(ctx *Context, args ...any) error {
			constructorOrder = append(constructorOrder, "Base")
			if len(args) == 0 {
				return nil
			}
			return ctx.Set("name", args[0])
		},
		Methods: []MethodSpec{{Name: "Speak", Visibility: Public, Handler: func(ctx *Context, _ ...any) (any, error) {
			name, err := ctx.Get("name")
			if err != nil {
				return nil, err
			}
			return fmt.Sprintf("%v speaks", name), nil
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := Define(ClassSpec{
		Name: "Child", Parent: base,
		Constructor: func(_ *Context, _ ...any) error {
			constructorOrder = append(constructorOrder, "Child")
			return nil
		},
		Methods: []MethodSpec{{Name: "Speak", Visibility: Public, Handler: func(ctx *Context, _ ...any) (any, error) {
			parent, err := ctx.Super("Speak")
			if err != nil {
				return nil, err
			}
			return parent.(string) + " loudly", nil
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	object, err := New(child, "Rex")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Base", "Child"}; !reflect.DeepEqual(constructorOrder, want) {
		t.Fatalf("constructor order = %v, want %v", constructorOrder, want)
	}
	got, err := object.Call("Speak")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Rex speaks loudly"; got != want {
		t.Fatalf("Speak() = %v, want %q", got, want)
	}
	if !child.IsA(base) || base.IsA(child) || object.Class() != child {
		t.Fatal("class ancestry or concrete class is incorrect")
	}
}

func TestVisibilityAndPrivateMethods(t *testing.T) {
	base, err := Define(ClassSpec{
		Name: "Base",
		Fields: []FieldSpec{
			{Name: "protected", Visibility: Protected, Default: 12},
			{Name: "private", Visibility: Private, Default: 34},
		},
		Methods: []MethodSpec{
			{Name: "secret", Visibility: Private, Handler: func(_ *Context, _ ...any) (any, error) { return "base secret", nil }},
			{Name: "Reveal", Visibility: Public, Handler: func(ctx *Context, _ ...any) (any, error) { return ctx.Call("secret") }},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	child, err := Define(ClassSpec{
		Name: "Child", Parent: base,
		Methods: []MethodSpec{
			{Name: "ReadProtected", Visibility: Public, Handler: func(ctx *Context, _ ...any) (any, error) { return ctx.Get("protected") }},
			{Name: "secret", Visibility: Public, Handler: func(_ *Context, _ ...any) (any, error) { return "child method", nil }},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	object, err := New(child)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := object.Get("protected"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Get(protected) error = %v, want ErrAccessDenied", err)
	}
	if _, err := object.Get("private"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Get(private) error = %v, want ErrAccessDenied", err)
	}
	if err := object.Set("private", 99); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("Set(private) error = %v, want ErrAccessDenied", err)
	}
	value, err := object.Call("ReadProtected")
	if err != nil || value != 12 {
		t.Fatalf("ReadProtected() = %v, %v; want 12, nil", value, err)
	}
	value, err = object.Call("Reveal")
	if err != nil || value != "base secret" {
		t.Fatalf("Reveal() = %v, %v; want base private implementation", value, err)
	}
	value, err = object.Call("secret")
	if err != nil || value != "child method" {
		t.Fatalf("child public method call = %v, %v", value, err)
	}
}

func TestPublicFieldsDefaultsAndIndependentMapCopies(t *testing.T) {
	class, err := Define(ClassSpec{
		Name:   "WithMap",
		Fields: []FieldSpec{{Name: "data", Visibility: Public, Default: map[string]int{"count": 1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := New(class)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(class)
	if err != nil {
		t.Fatal(err)
	}
	firstMap, err := first.Get("data")
	if err != nil {
		t.Fatal(err)
	}
	firstMap.(map[string]int)["count"] = 2
	secondMap, err := second.Get("data")
	if err != nil {
		t.Fatal(err)
	}
	if got := secondMap.(map[string]int)["count"]; got != 1 {
		t.Fatalf("second object's map count = %d, want 1", got)
	}
	if err := first.Set("data", map[string]int{"count": 3}); err != nil {
		t.Fatal(err)
	}
	updated, err := first.Get("data")
	if err != nil || updated.(map[string]int)["count"] != 3 {
		t.Fatalf("public field Set/Get failed: %v, %v", updated, err)
	}
}

func TestDefinitionAndConstructionErrors(t *testing.T) {
	base, err := Define(ClassSpec{Name: "Base", Fields: []FieldSpec{{Name: "id", Visibility: Private}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Define(ClassSpec{Name: "Child", Parent: base, Fields: []FieldSpec{{Name: "id", Visibility: Public}}}); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("duplicate inherited field error = %v, want ErrInvalidDefinition", err)
	}
	parentWithPublicMethod, err := Define(ClassSpec{
		Name:    "PublicAPI",
		Methods: []MethodSpec{{Name: "Run", Visibility: Public, Handler: func(_ *Context, _ ...any) (any, error) { return nil, nil }}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Define(ClassSpec{
		Name: "NarrowedAPI", Parent: parentWithPublicMethod,
		Methods: []MethodSpec{{Name: "Run", Visibility: Protected, Handler: func(_ *Context, _ ...any) (any, error) { return nil, nil }}},
	}); !errors.Is(err, ErrInvalidDefinition) {
		t.Fatalf("narrowed method visibility error = %v, want ErrInvalidDefinition", err)
	}
	broken, err := Define(ClassSpec{Name: "Broken", Constructor: func(_ *Context, _ ...any) error { return errors.New("boom") }})
	if err != nil {
		t.Fatal(err)
	}
	if object, err := New(broken); object != nil || err == nil {
		t.Fatalf("New(Broken) = %v, %v; want nil object and error", object, err)
	}
	object, err := New(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := object.Get("missing"); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("missing field error = %v, want ErrFieldNotFound", err)
	}
	if _, err := object.Call("missing"); !errors.Is(err, ErrMethodNotFound) {
		t.Fatalf("missing method error = %v, want ErrMethodNotFound", err)
	}
}
