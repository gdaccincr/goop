# Goop Language Support for VSCodium / VS Code

This local extension adds `.oop` file recognition, syntax highlighting for
Goop class declarations, snippets, and a command that compiles the active file
using the `goopc` executable. It is a separate extension; it does not patch the
Go extension or the Go language server.

## Build the compiler

From the root of the goop repository, build the CLI:

```powershell
go build -o goopc.exe ./cmd/goopc
```

Either add that executable to `PATH`, or set the VSCodium setting **Goop: Goopc
Path** to its full path, for example `C:\Users\you\goop\goopc.exe`.

The runtime import defaults to `github.com/gdaccincr/goop`. If you later rename
or move the module, configure **Goop: Runtime Import** with its new path.

## Run the extension from source

1. Open `vscode-goop` as a folder in VSCodium.
2. Press `F5` to start an Extension Development Host.
3. Open a `.oop` file in the new window. Snippets and highlighting should be
   available; run **Goop: Compile Current File** from the Command Palette to
   generate and open a neighboring `.generated.go` file.

The command is also available from the editor title and right-click menus. The
generated Go file is understood by the standard Go extension and `gopls`.

## IntelliSense scope

This first version provides keyword completions and snippets, not full Go
semantic IntelliSense in `.oop` files. The Go language server only understands
Go syntax. Full type-aware completions and diagnostics in the custom syntax
would require a Goop language server or a source-mapping integration with
`gopls`; the generated `.go` file can already use normal Go IntelliSense.

## Validate

```powershell
npm test
```