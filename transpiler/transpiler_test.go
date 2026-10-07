package transpiler

import (
	"strings"
	"testing"
)

func TestCompileClassesInheritanceAndMethodBodies(t *testing.T) {
	source := []byte(`package main

import "fmt"

class Animal {
  protected name = "unknown"
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
    return fmt.Sprintf("%v and barks", sound), nil
  }
}

func main() {
  rex, err := gooprt.New(Dog, "Rex")
  if err != nil { panic(err) }
  sound, err := rex.Call("Speak")
  if err != nil { panic(err) }
  fmt.Println(sound)
}
`)
	generated, err := Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	for _, expected := range []string{
		`import gooprt "github.com/gdaccincr/goop"`,
		`var Animal *gooprt.Class`,
		`var Dog *gooprt.Class`,
		`Parent: Animal`,
		`Visibility: gooprt.Protected`,
		`this.Super("Speak")`,
		`gooprt.New(Dog, "Rex")`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("generated source does not contain %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "class Animal") || strings.Contains(text, "class Dog") {
		t.Fatalf("class syntax was not removed:\n%s", text)
	}
}

func TestCompileRejectsUnknownParent(t *testing.T) {
	_, err := Compile([]byte("package main\nclass Dog extends Animal {}\n"))
	if err == nil || !strings.Contains(err.Error(), "must be declared earlier") {
		t.Fatalf("Compile error = %v, want unknown-parent error", err)
	}
}

func TestCompileRejectsMissingFieldDefault(t *testing.T) {
	_, err := Compile([]byte("package main\nclass User { private name string }\n"))
	if err == nil || !strings.Contains(err.Error(), "name = default") {
		t.Fatalf("Compile error = %v, want missing-default error", err)
	}
}

func TestCompileRejectsInvalidGoMethodBody(t *testing.T) {
	_, err := Compile([]byte("package main\nclass User { public Run { this is not go } }\n"))
	if err == nil || !strings.Contains(err.Error(), "generated Go is invalid") {
		t.Fatalf("Compile error = %v, want invalid Go error", err)
	}
}
