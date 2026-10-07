// Package goop provides a small, dynamic class system for Go programs.
//
// Go does not have user-defined class syntax or runtime inheritance. This
// package models classes, single inheritance, visibility, constructors, and
// virtual method dispatch using ordinary Go values and functions.
package goop

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var (
	ErrInvalidClass      = errors.New("goop: invalid class")
	ErrInvalidDefinition = errors.New("goop: invalid class definition")
	ErrNilObject         = errors.New("goop: nil object")
	ErrFieldNotFound     = errors.New("goop: field not found")
	ErrMethodNotFound    = errors.New("goop: method not found")
	ErrAccessDenied      = errors.New("goop: access denied")
	ErrInvalidVisibility = errors.New("goop: invalid visibility")
)

// Visibility controls access to a field or method.
type Visibility uint8

const (
	Public Visibility = iota + 1
	Protected
	Private
)

func (v Visibility) String() string {
	switch v {
	case Public:
		return "public"
	case Protected:
		return "protected"
	case Private:
		return "private"
	default:
		return "invalid"
	}
}

// FieldSpec describes a field declared by a class. Map and slice defaults are
// shallow-copied for each instance; pointer defaults are shared as usual in Go.
type FieldSpec struct {
	Name       string
	Visibility Visibility
	Default    any
}

// MethodFunc implements a class method. Arguments and return values are
// dynamically typed; applications should type-assert them at their boundary.
type MethodFunc func(*Context, ...any) (any, error)

// ConstructorFunc initializes an object. Constructors run from the oldest
// ancestor to the concrete class and each receives the arguments passed to New.
type ConstructorFunc func(*Context, ...any) error

// MethodSpec describes a method declared by a class.
type MethodSpec struct {
	Name       string
	Visibility Visibility
	Handler    MethodFunc
}

// ClassSpec describes a class and its own declarations. Parent may be nil.
type ClassSpec struct {
	Name        string
	Parent      *Class
	Fields      []FieldSpec
	Methods     []MethodSpec
	Constructor ConstructorFunc
}

type fieldDefinition struct {
	visibility Visibility
	defaultVal any
}

type methodDefinition struct {
	visibility Visibility
	handler    MethodFunc
}

// Class is an immutable class definition created by Define.
type Class struct {
	name        string
	parent      *Class
	fields      map[string]fieldDefinition
	methods     map[string]methodDefinition
	constructor ConstructorFunc
}

// Define validates and creates a class. A child may override an inherited
// method, but field names must be unique across the inheritance chain.
func Define(spec ClassSpec) (*Class, error) {
	if strings.TrimSpace(spec.Name) == "" {
		return nil, fmt.Errorf("%w: class name cannot be empty", ErrInvalidDefinition)
	}
	if spec.Parent != nil && spec.Parent.name == "" {
		return nil, fmt.Errorf("%w: parent class is invalid", ErrInvalidClass)
	}

	c := &Class{
		name:        spec.Name,
		parent:      spec.Parent,
		fields:      make(map[string]fieldDefinition, len(spec.Fields)),
		methods:     make(map[string]methodDefinition, len(spec.Methods)),
		constructor: spec.Constructor,
	}
	for _, field := range spec.Fields {
		if strings.TrimSpace(field.Name) == "" {
			return nil, fmt.Errorf("%w: field name cannot be empty", ErrInvalidDefinition)
		}
		if !validVisibility(field.Visibility) {
			return nil, fmt.Errorf("%w: field %q", ErrInvalidVisibility, field.Name)
		}
		_, exists := c.fields[field.Name]
		var inherited *fieldDefinition
		if spec.Parent != nil {
			_, inherited = spec.Parent.findField(field.Name)
		}
		if exists || inherited != nil {
			return nil, fmt.Errorf("%w: duplicate field %q", ErrInvalidDefinition, field.Name)
		}
		c.fields[field.Name] = fieldDefinition{visibility: field.Visibility, defaultVal: field.Default}
	}
	for _, method := range spec.Methods {
		if strings.TrimSpace(method.Name) == "" || method.Handler == nil {
			return nil, fmt.Errorf("%w: method name and handler are required", ErrInvalidDefinition)
		}
		if !validVisibility(method.Visibility) {
			return nil, fmt.Errorf("%w: method %q", ErrInvalidVisibility, method.Name)
		}
		if _, exists := c.methods[method.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate method %q", ErrInvalidDefinition, method.Name)
		}
		if spec.Parent != nil {
			_, inherited := spec.Parent.findMethod(method.Name)
			if inherited != nil && method.Visibility > inherited.visibility {
				return nil, fmt.Errorf("%w: override %q cannot reduce visibility from %s to %s", ErrInvalidDefinition, method.Name, inherited.visibility, method.Visibility)
			}
		}
		c.methods[method.Name] = methodDefinition{visibility: method.Visibility, handler: method.Handler}
	}
	return c, nil
}

func validVisibility(v Visibility) bool { return v == Public || v == Protected || v == Private }

// Name returns the class name.
func (c *Class) Name() string {
	if c == nil {
		return ""
	}
	return c.name
}

// Parent returns the direct parent class, or nil for a root class.
func (c *Class) Parent() *Class {
	if c == nil {
		return nil
	}
	return c.parent
}

// IsA reports whether c is the requested class or derives from it.
func (c *Class) IsA(ancestor *Class) bool {
	if c == nil || ancestor == nil {
		return false
	}
	for current := c; current != nil; current = current.parent {
		if current == ancestor {
			return true
		}
	}
	return false
}

// Object is an instance of a Class. Its field store is protected against
// concurrent Get and Set calls; applications should still synchronize access
// to mutable reference values returned from fields.
type Object struct {
	class  *Class
	mu     sync.RWMutex
	values map[string]any
}

// New constructs an object, initializing ancestor fields and running
// constructors from parent to child. If a constructor fails, no object is
// returned.
func New(class *Class, args ...any) (*Object, error) {
	if class == nil || class.name == "" {
		return nil, ErrInvalidClass
	}
	chain := make([]*Class, 0, 4)
	for current := class; current != nil; current = current.parent {
		chain = append(chain, current)
	}
	object := &Object{class: class, values: make(map[string]any)}
	for i := len(chain) - 1; i >= 0; i-- {
		for name, field := range chain[i].fields {
			object.values[name] = cloneDefault(field.defaultVal)
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		current := chain[i]
		if current.constructor == nil {
			continue
		}
		ctx := &Context{object: object, owner: current, args: args}
		if err := current.constructor(ctx, args...); err != nil {
			return nil, fmt.Errorf("goop: %s constructor: %w", current.name, err)
		}
	}
	return object, nil
}

func cloneDefault(value any) any {
	if value == nil {
		return nil
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Map:
		if v.IsNil() {
			return value
		}
		copy := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			copy.SetMapIndex(iter.Key(), iter.Value())
		}
		return copy.Interface()
	case reflect.Slice:
		if v.IsNil() {
			return value
		}
		copy := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(copy, v)
		return copy.Interface()
	default:
		return value
	}
}

// Class returns the object's concrete class.
func (o *Object) Class() *Class {
	if o == nil {
		return nil
	}
	return o.class
}

// Get reads a public field. Protected and private fields are available from
// methods and constructors through Context.Get.
func (o *Object) Get(name string) (any, error) {
	return o.get(name, nil)
}

func (o *Object) get(name string, caller *Class) (any, error) {
	if o == nil || o.class == nil {
		return nil, ErrNilObject
	}
	owner, field := o.class.findField(name)
	if field == nil {
		return nil, fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	}
	if !canAccess(field.visibility, owner, caller) {
		return nil, fmt.Errorf("%w: %s field %q", ErrAccessDenied, field.visibility, name)
	}
	o.mu.RLock()
	value := o.values[name]
	o.mu.RUnlock()
	return value, nil
}

// Set changes a public field. Protected and private fields are available from
// methods and constructors through Context.Set.
func (o *Object) Set(name string, value any) error {
	return o.set(name, value, nil)
}

func (o *Object) set(name string, value any, caller *Class) error {
	if o == nil || o.class == nil {
		return ErrNilObject
	}
	owner, field := o.class.findField(name)
	if field == nil {
		return fmt.Errorf("%w: %q", ErrFieldNotFound, name)
	}
	if !canAccess(field.visibility, owner, caller) {
		return fmt.Errorf("%w: %s field %q", ErrAccessDenied, field.visibility, name)
	}
	o.mu.Lock()
	o.values[name] = value
	o.mu.Unlock()
	return nil
}

func (c *Class) findField(name string) (*Class, *fieldDefinition) {
	for current := c; current != nil; current = current.parent {
		if field, ok := current.fields[name]; ok {
			return current, &field
		}
	}
	return nil, nil
}

func (c *Class) findMethod(name string) (*Class, *methodDefinition) {
	for current := c; current != nil; current = current.parent {
		if method, ok := current.methods[name]; ok && method.visibility != Private {
			return current, &method
		}
	}
	return nil, nil
}

func canAccess(visibility Visibility, owner, caller *Class) bool {
	switch visibility {
	case Public:
		return true
	case Protected:
		return caller != nil && caller.IsA(owner)
	case Private:
		return caller == owner
	default:
		return false
	}
}

// Call invokes a public method with virtual dispatch. The concrete object's
// most-derived implementation is selected, even when called through a parent
// class reference (interfaces can expose that parent-facing API).
func (o *Object) Call(name string, args ...any) (any, error) {
	if o == nil {
		return nil, ErrNilObject
	}
	return o.call(name, nil, o.class, args...)
}

func (o *Object) call(name string, caller, start *Class, args ...any) (any, error) {
	if o == nil || o.class == nil {
		return nil, ErrNilObject
	}
	owner, method := lookupMethod(start, caller, name)
	if method == nil {
		return nil, fmt.Errorf("%w: %q", ErrMethodNotFound, name)
	}
	if !canAccess(method.visibility, owner, caller) {
		return nil, fmt.Errorf("%w: %s method %q", ErrAccessDenied, method.visibility, name)
	}
	ctx := &Context{object: o, owner: owner, args: args}
	return method.handler(ctx, args...)
}

func lookupMethod(start, caller *Class, name string) (*Class, *methodDefinition) {
	// A private method is lexically bound to its declaring class. A same-named
	// method in a subclass must not replace calls made from that class.
	if caller != nil {
		if method, ok := caller.methods[name]; ok && method.visibility == Private {
			return caller, &method
		}
	}
	for current := start; current != nil; current = current.parent {
		if method, ok := current.methods[name]; ok {
			// Private methods are not inherited, and cannot be called from
			// outside their declaring class.
			if method.visibility == Private && caller != current {
				continue
			}
			return current, &method
		}
	}
	return nil, nil
}

// Context is the receiver and lexical class context for a method or
// constructor. It should not be retained after the handler returns.
type Context struct {
	object *Object
	owner  *Class
	args   []any
}

// Class returns the class whose method or constructor is currently executing.
func (ctx *Context) Class() *Class {
	if ctx == nil {
		return nil
	}
	return ctx.owner
}

// Object returns the concrete receiver.
func (ctx *Context) Object() *Object {
	if ctx == nil {
		return nil
	}
	return ctx.object
}

// Args returns the arguments supplied to the current method or constructor.
func (ctx *Context) Args() []any {
	if ctx == nil {
		return nil
	}
	return append([]any(nil), ctx.args...)
}

// Get reads a field subject to the current class's visibility permissions.
func (ctx *Context) Get(name string) (any, error) {
	if ctx == nil {
		return nil, ErrNilObject
	}
	return ctx.object.get(name, ctx.owner)
}

// Set changes a field subject to the current class's visibility permissions.
func (ctx *Context) Set(name string, value any) error {
	if ctx == nil {
		return ErrNilObject
	}
	return ctx.object.set(name, value, ctx.owner)
}

// Call invokes a method with virtual dispatch, subject to the current class's
// visibility permissions.
func (ctx *Context) Call(name string, args ...any) (any, error) {
	if ctx == nil {
		return nil, ErrNilObject
	}
	return ctx.object.call(name, ctx.owner, ctx.object.class, args...)
}

// Super invokes the nearest implementation declared above the current class.
// It is intended for an overriding method to call its parent's implementation.
func (ctx *Context) Super(name string, args ...any) (any, error) {
	if ctx == nil || ctx.owner == nil || ctx.owner.parent == nil {
		return nil, fmt.Errorf("%w: no parent implementation for %q", ErrMethodNotFound, name)
	}
	return ctx.object.call(name, ctx.owner, ctx.owner.parent, args...)
}
