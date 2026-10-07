// Package transpiler translates goop's small class-declaration syntax into Go.
// Method and constructor bodies remain ordinary Go code.
package transpiler

import (
	"bytes"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"strconv"
	"strings"
)

const runtimeImport = "github.com/gdaccincr/goop"

// Options configures source generation.
type Options struct {
	// RuntimeImport is the import path of the goop runtime package. When empty,
	// the module's default path is used.
	RuntimeImport string
}

type sourceToken struct {
	kind       token.Token
	literal    string
	start, end int
}

type field struct {
	name, visibility, defaultValue string
}

type method struct {
	name, visibility, body string
}

type class struct {
	name, parent string
	fields       []field
	methods      []method
	constructor  string
	start, end   int
}

// Compile translates a Go source file containing class declarations into
// formatted Go. Class declarations use the syntax documented in README.md;
// everything outside them, and every method body, is ordinary Go source.
func Compile(source []byte) ([]byte, error) {
	return CompileWithOptions(source, Options{})
}

// CompileWithOptions translates a source file with the supplied options.
func CompileWithOptions(source []byte, options Options) ([]byte, error) {
	importPath := options.RuntimeImport
	if importPath == "" {
		importPath = runtimeImport
	}
	if strings.ContainsAny(importPath, "\r\n\" ") {
		return nil, fmt.Errorf("goopc: invalid runtime import path %q", importPath)
	}
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	classes, err := parseClasses(source, tokens)
	if err != nil {
		return nil, err
	}
	if len(classes) == 0 {
		return nil, fmt.Errorf("goopc: no class declarations found")
	}

	var output bytes.Buffer
	position := 0
	for _, declaration := range classes {
		output.Write(source[position:declaration.start])
		output.WriteString(generateClass(declaration))
		position = declaration.end
	}
	output.Write(source[position:])

	generated := output.Bytes()
	packageEnd, err := findPackageLineEnd(generated)
	if err != nil {
		return nil, err
	}
	withImport := make([]byte, 0, len(generated)+len(runtimeImport)+32)
	withImport = append(withImport, generated[:packageEnd]...)
	withImport = append(withImport, []byte("\nimport gooprt "+strconv.Quote(importPath)+"\n")...)
	withImport = append(withImport, generated[packageEnd:]...)

	formatted, err := format.Source(withImport)
	if err != nil {
		return nil, fmt.Errorf("goopc: generated Go is invalid: %w", err)
	}
	return formatted, nil
}

func lex(source []byte) ([]sourceToken, error) {
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("input.oop", fileSet.Base(), len(source))
	var scannerErrors []string
	var s scanner.Scanner
	s.Init(file, source, func(pos token.Position, msg string) {
		scannerErrors = append(scannerErrors, fmt.Sprintf("%s: %s", pos, msg))
	}, scanner.ScanComments)

	var result []sourceToken
	for {
		pos, kind, literal := s.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.COMMENT {
			continue
		}
		start := file.Offset(pos)
		end := start + len(literal)
		if literal == "" {
			literal = kind.String()
			end = start + len(literal)
		}
		result = append(result, sourceToken{kind: kind, literal: literal, start: start, end: end})
	}
	if len(scannerErrors) != 0 {
		return nil, fmt.Errorf("goopc: %s", strings.Join(scannerErrors, "; "))
	}
	return result, nil
}

func parseClasses(source []byte, tokens []sourceToken) ([]class, error) {
	var result []class
	known := make(map[string]bool)
	depth := 0
	for i := 0; i < len(tokens); i++ {
		if depth == 0 && isIdent(tokens[i], "class") && isDeclarationPosition(source, tokens[i].start) {
			declaration, next, err := parseClass(source, tokens, i)
			if err != nil {
				return nil, err
			}
			if known[declaration.name] {
				return nil, at(source, tokens[i].start, "duplicate class %q", declaration.name)
			}
			if declaration.parent != "" && !known[declaration.parent] {
				return nil, at(source, tokens[i].start, "parent class %q must be declared earlier in this file", declaration.parent)
			}
			known[declaration.name] = true
			result = append(result, declaration)
			i = next - 1
			continue
		}
		switch tokens[i].kind {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			if depth > 0 {
				depth--
			}
		}
	}
	return result, nil
}

func isDeclarationPosition(source []byte, offset int) bool {
	lineStart := bytes.LastIndex(source[:offset], []byte{'\n'}) + 1
	return len(bytes.TrimSpace(source[lineStart:offset])) == 0
}

func parseClass(source []byte, tokens []sourceToken, startIndex int) (class, int, error) {
	start := tokens[startIndex].start
	i := startIndex + 1
	if i >= len(tokens) || tokens[i].kind != token.IDENT {
		return class{}, 0, at(source, start, "expected class name")
	}
	declaration := class{name: tokens[i].literal, start: start}
	i++
	if i < len(tokens) && isIdent(tokens[i], "extends") {
		i++
		if i >= len(tokens) || tokens[i].kind != token.IDENT {
			return class{}, 0, at(source, start, "expected parent class name after extends")
		}
		declaration.parent = tokens[i].literal
		i++
	}
	if i >= len(tokens) || tokens[i].kind != token.LBRACE {
		return class{}, 0, at(source, start, "expected { after class declaration")
	}
	i++
	for i < len(tokens) && tokens[i].kind != token.RBRACE {
		if tokens[i].kind == token.SEMICOLON {
			i++
			continue
		}
		memberStart := tokens[i].start
		if isIdent(tokens[i], "constructor") {
			if declaration.constructor != "" {
				return class{}, 0, at(source, memberStart, "class %q has more than one constructor", declaration.name)
			}
			bodyStart, bodyEnd, next, err := parseBody(source, tokens, i+1, memberStart, "constructor")
			if err != nil {
				return class{}, 0, err
			}
			declaration.constructor = string(source[bodyStart:bodyEnd])
			i = next
			continue
		}
		visibility := visibilityName(tokens[i])
		if visibility == "" {
			return class{}, 0, at(source, memberStart, "expected public, protected, private, or constructor")
		}
		i++
		if i >= len(tokens) || tokens[i].kind != token.IDENT {
			return class{}, 0, at(source, memberStart, "expected field or method name")
		}
		name := tokens[i].literal
		i++
		if i < len(tokens) && tokens[i].kind == token.LPAREN {
			return class{}, 0, at(source, tokens[i].start, "method parameters are supplied as the implicit args ...any; omit a parameter list")
		}
		if i < len(tokens) && tokens[i].kind == token.LBRACE {
			bodyStart, bodyEnd, next, err := parseBody(source, tokens, i, memberStart, "method")
			if err != nil {
				return class{}, 0, err
			}
			declaration.methods = append(declaration.methods, method{
				name: name, visibility: visibility, body: string(source[bodyStart:bodyEnd]),
			})
			i = next
			continue
		}
		parsedField, next, err := parseField(source, tokens, i, memberStart, name, visibility)
		if err != nil {
			return class{}, 0, err
		}
		declaration.fields = append(declaration.fields, parsedField)
		i = next
	}
	if i >= len(tokens) || tokens[i].kind != token.RBRACE {
		return class{}, 0, at(source, start, "unterminated class %q", declaration.name)
	}
	declaration.end = tokens[i].end
	return declaration, i + 1, nil
}

func parseBody(source []byte, tokens []sourceToken, index, memberStart int, memberKind string) (int, int, int, error) {
	if index >= len(tokens) || tokens[index].kind != token.LBRACE {
		return 0, 0, 0, at(source, memberStart, "%s body must start with { (do not declare parameters)", memberKind)
	}
	depth := 1
	for i := index + 1; i < len(tokens); i++ {
		switch tokens[i].kind {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
			if depth == 0 {
				return tokens[index].end, tokens[i].start, i + 1, nil
			}
		}
	}
	return 0, 0, 0, at(source, memberStart, "unterminated %s body", memberKind)
}

func parseField(source []byte, tokens []sourceToken, index, memberStart int, name, visibility string) (field, int, error) {
	start := index
	depth := 0
	end := index
	for end < len(tokens) {
		kind := tokens[end].kind
		if kind == token.SEMICOLON && depth == 0 {
			break
		}
		switch kind {
		case token.LBRACE, token.LPAREN, token.LBRACK:
			depth++
		case token.RBRACE, token.RPAREN, token.RBRACK:
			if depth > 0 {
				depth--
			}
		}
		end++
	}
	if start == end {
		return field{}, 0, at(source, memberStart, "field %q must have a type and default value", name)
	}
	assign := -1
	depth = 0
	for j := start; j < end; j++ {
		switch tokens[j].kind {
		case token.LBRACE, token.LPAREN, token.LBRACK:
			depth++
		case token.RBRACE, token.RPAREN, token.RBRACK:
			depth--
		case token.ASSIGN:
			if depth == 0 {
				assign = j
			}
		}
	}
	if assign < start || assign == end-1 {
		return field{}, 0, at(source, memberStart, "field %q must use `name = default`", name)
	}
	defaultValue := strings.TrimSpace(string(source[tokens[assign+1].start:tokens[end-1].end]))
	if defaultValue == "" {
		return field{}, 0, at(source, memberStart, "field %q has an empty default", name)
	}
	next := end
	if next < len(tokens) && tokens[next].kind == token.SEMICOLON {
		next++
	}
	return field{name: name, visibility: visibility, defaultValue: defaultValue}, next, nil
}

func generateClass(declaration class) string {
	var out strings.Builder
	fmt.Fprintf(&out, "\nvar %s *gooprt.Class\n\nfunc init() {\n\tvar err error\n\t%s, err = gooprt.Define(gooprt.ClassSpec{\n\t\tName: %s,\n",
		declaration.name, declaration.name, strconv.Quote(declaration.name))
	if declaration.parent != "" {
		fmt.Fprintf(&out, "\t\tParent: %s,\n", declaration.parent)
	}
	if len(declaration.fields) != 0 {
		out.WriteString("\t\tFields: []gooprt.FieldSpec{\n")
		for _, item := range declaration.fields {
			fmt.Fprintf(&out, "\t\t\t{Name: %s, Visibility: gooprt.%s, Default: %s},\n",
				strconv.Quote(item.name), titleVisibility(item.visibility), item.defaultValue)
		}
		out.WriteString("\t\t},\n")
	}
	if len(declaration.methods) != 0 {
		out.WriteString("\t\tMethods: []gooprt.MethodSpec{\n")
		for _, item := range declaration.methods {
			fmt.Fprintf(&out, "\t\t\t{Name: %s, Visibility: gooprt.%s, Handler: func(this *gooprt.Context, args ...any) (any, error) {\n%s\n\t\t\t}},\n",
				strconv.Quote(item.name), titleVisibility(item.visibility), indent(item.body, "\t\t\t\t"))
		}
		out.WriteString("\t\t},\n")
	}
	if declaration.constructor != "" {
		fmt.Fprintf(&out, "\t\tConstructor: func(this *gooprt.Context, args ...any) error {\n%s\n\t\t},\n",
			indent(declaration.constructor, "\t\t\t"))
	}
	out.WriteString("\t})\n\tif err != nil {\n\t\t panic(err)\n\t}\n}\n")
	return out.String()
}

func indent(text, prefix string) string {
	lines := strings.Split(strings.Trim(text, "\r\n"), "\n")
	for i := range lines {
		lines[i] = prefix + strings.TrimRight(lines[i], "\r")
	}
	return strings.Join(lines, "\n")
}

func findPackageLineEnd(source []byte) (int, error) {
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("generated.go", fileSet.Base(), len(source))
	var s scanner.Scanner
	s.Init(file, source, nil, 0)
	seenPackage := false
	for {
		pos, kind, _ := s.Scan()
		if kind == token.EOF {
			break
		}
		if !seenPackage && kind == token.PACKAGE {
			seenPackage = true
			continue
		}
		if seenPackage && kind == token.IDENT {
			// The package name ends at the next whitespace after its token.
			nameEnd := file.Offset(pos)
			for nameEnd < len(source) && isIdentifierByte(source[nameEnd]) {
				nameEnd++
			}
			for nameEnd < len(source) && source[nameEnd] != '\n' {
				nameEnd++
			}
			if nameEnd < len(source) {
				return nameEnd + 1, nil
			}
			return len(source), nil
		}
	}
	return 0, fmt.Errorf("goopc: input must begin with a Go package declaration")
}

func isIdentifierByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func isIdent(item sourceToken, value string) bool {
	return item.kind == token.IDENT && item.literal == value
}

func visibilityName(item sourceToken) string {
	switch item.literal {
	case "public", "protected", "private":
		if item.kind == token.IDENT {
			return item.literal
		}
	}
	return ""
}

func titleVisibility(visibility string) string {
	return strings.ToUpper(visibility[:1]) + visibility[1:]
}

func at(source []byte, offset int, format string, args ...any) error {
	line := bytes.Count(source[:offset], []byte{'\n'}) + 1
	lastNewline := bytes.LastIndex(source[:offset], []byte{'\n'})
	column := offset - lastNewline
	return fmt.Errorf("goopc: line %d, column %d: %s", line, column, fmt.Sprintf(format, args...))
}
