const path = require('path');
const { spawn } = require('child_process');
const vscode = require('vscode');

function activate(context) {
	const compile = vscode.commands.registerCommand('goop.compileDocument', async (uri) => {
		const document = uri ? await vscode.workspace.openTextDocument(uri) : vscode.window.activeTextEditor?.document;
		if (!document || document.languageId !== 'goop') {
			vscode.window.showErrorMessage('Open a .oop file before compiling.');
			return;
		}
		if (document.isUntitled) {
			vscode.window.showErrorMessage('Save the .oop file before compiling it.');
			return;
		}
		if (document.isDirty && !(await document.save())) {
			vscode.window.showErrorMessage('Could not save the .oop file; compilation cancelled.');
			return;
		}

		const config = vscode.workspace.getConfiguration('goop');
		const compiler = config.get('goopcPath', 'goopc');
		const runtimeImport = config.get('runtimeImport', '').trim();
		const inputPath = document.uri.fsPath;
		const outputPath = path.join(
			path.dirname(inputPath),
			`${path.basename(inputPath, path.extname(inputPath))}.generated.go`,
		);
		const cwd = vscode.workspace.getWorkspaceFolder(document.uri)?.uri.fsPath ?? path.dirname(inputPath);
		const args = ['-in', inputPath, '-out', outputPath];
		if (runtimeImport) {
			args.push('-runtime', runtimeImport);
		}

		try {
			await runCompiler(compiler, args, cwd);
			const generated = await vscode.workspace.openTextDocument(vscode.Uri.file(outputPath));
			await vscode.window.showTextDocument(generated, { preview: false });
			vscode.window.showInformationMessage(`Compiled ${path.basename(inputPath)} to ${path.basename(outputPath)}.`);
		} catch (error) {
			const detail = error instanceof Error ? error.message : String(error);
			vscode.window.showErrorMessage(`Goop compilation failed: ${detail}`);
		}
	});

	const completion = vscode.languages.registerCompletionItemProvider('goop', {
		provideCompletionItems(document, position) {
			const line = document.lineAt(position.line).text.slice(0, position.character);
			if (/^\s*(public|protected|private)\s+[\w]*$/.test(line)) {
				return ['constructor'].map((label) => new vscode.CompletionItem(label, vscode.CompletionItemKind.Keyword));
			}
			return ['class', 'extends', 'public', 'protected', 'private', 'constructor'].map((label) => {
				const item = new vscode.CompletionItem(label, vscode.CompletionItemKind.Keyword);
				item.insertText = label;
				return item;
			});
		},
	});

	context.subscriptions.push(compile, completion);
}

function runCompiler(command, args, cwd) {
	return new Promise((resolve, reject) => {
		const child = spawn(command, args, { cwd, windowsHide: true });
		let stderr = '';
		child.stderr.setEncoding('utf8');
		child.stderr.on('data', (chunk) => { stderr += chunk; });
		child.on('error', (error) => {
			if (error.code === 'ENOENT') {
				reject(new Error(`Cannot find "${command}". Build goopc and set Goop › Goopc Path.`));
				return;
			}
			reject(error);
		});
		child.on('close', (code) => {
			if (code === 0) {
				resolve();
				return;
			}
			reject(new Error(stderr.trim() || `goopc exited with status ${code}`));
		});
	});
}

function deactivate() {}

module.exports = { activate, deactivate };