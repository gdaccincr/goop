const assert = require('assert');
const Module = require('module');
const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
const grammar = JSON.parse(fs.readFileSync(path.join(root, 'syntaxes', 'goop.tmLanguage.json'), 'utf8'));
const languageConfiguration = JSON.parse(fs.readFileSync(path.join(root, 'language-configuration.json'), 'utf8'));
const snippets = JSON.parse(fs.readFileSync(path.join(root, 'snippets', 'goop.json'), 'utf8'));

assert.strictEqual(manifest.main, './extension.js');
assert(manifest.contributes.languages.some((language) => language.id === 'goop' && language.extensions.includes('.oop')));
assert.strictEqual(grammar.scopeName, 'source.goop');
assert.strictEqual(manifest.contributes.grammars[0].embeddedLanguages['source.go'], 'go');
assert(grammar.repository['class-declaration'].end.includes('\\1'));
assert(languageConfiguration.brackets.some(([open, close]) => open === '{' && close === '}'));
assert(snippets.Class.body.some((line) => line.includes('class ')));
const syntaxCheck = spawnSync(process.execPath, ['--check', path.join(root, 'extension.js')], { encoding: 'utf8' });
assert.strictEqual(syntaxCheck.status, 0, syntaxCheck.stderr);

const registeredCommands = new Map();
const registeredLanguages = [];
const vscodeMock = {
	commands: {
		registerCommand(name, callback) {
			registeredCommands.set(name, callback);
			return { dispose() {} };
		},
	},
	languages: {
		registerCompletionItemProvider(language, provider) {
			registeredLanguages.push({ language, provider });
			return { dispose() {} };
		},
	},
	CompletionItem: class CompletionItem {
		constructor(label, kind) { this.label = label; this.kind = kind; }
	},
	CompletionItemKind: { Keyword: 14 },
};
const originalLoad = Module._load;
Module._load = function (request, parent, isMain) {
	if (request === 'vscode') return vscodeMock;
	return originalLoad.call(this, request, parent, isMain);
};
try {
	const extensionPath = path.join(root, 'extension.js');
	delete require.cache[require.resolve(extensionPath)];
	const extension = require(extensionPath);
	const subscriptions = [];
	extension.activate({ subscriptions });
	assert(registeredCommands.has('goop.compileDocument'));
	assert.strictEqual(registeredLanguages.length, 1);
	assert.strictEqual(registeredLanguages[0].language, 'goop');
	assert.strictEqual(subscriptions.length, 2);
} finally {
	Module._load = originalLoad;
}

console.log('Goop editor extension manifest, grammar, and activation are valid.');