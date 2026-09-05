const vscode = require('vscode');
const fs = require('fs');
const path = require('path');
const { kill } = require('process');

function activate(context) {
    let disposable = vscode.commands.registerCommand('eo.runCode', function () {
        const editor = vscode.window.activeTextEditor;
        if (!editor) return;
        const filePath = editor.document.fileName;
        
        if (!filePath.endsWith('.eo')) return;
        
        // تحديد مسار ملف eo.go ديناميكياً من مجلد tasks داخل مجلد الإضافة
        const eoGoPath = path.join(context.extensionPath, 'tasks', 'eo.go');
        
        editor.document.save().then(() => {
            const terminal = vscode.window.createTerminal({
                name: 'eo Terminal',
                shellPath: 'cmd.exe'
            });
            terminal.show();
            terminal.sendText(`"eo.exe" "${filePath}"`);
        });
    });

    context.subscriptions.push(disposable);

    // تسجيل المزود بحيث يعمل على أي ملف امتداده .eo أو لغة eo
    let providerDisposable = vscode.languages.registerCompletionItemProvider(
        [{ scheme: 'file', language: 'eo' }, { scheme: 'file', pattern: '**/*.eo' }],
        {
            provideCompletionItems(document, position, token, context) {
                let completionItems = [];

                // 1. الكلمات المفتاحية الأساسية
                const keywords = [
                    { name: 'print ', kind: vscode.CompletionItemKind.Function, detail: 'EO Print' },
                    { name: 'input ', kind: vscode.CompletionItemKind.Function, detail: 'EO Input'},
                    { name: 'lower ', kind: vscode.CompletionItemKind.Function, detail: 'EO Lower'},
                    { name: 'upper ', kind: vscode.CompletionItemKind.Function, detail: 'EO Upper'},
                    { name: 'ivs ', kind: vscode.CompletionItemKind.Function, detailL: 'EO Inverse'},
                    { name: 'len ', kind: vscode.CompletionItemKind.Function, detail: 'EO Length'},
                    { name: 'toInt ', kind: vscode.CompletionItemKind.Function, detail: 'EO To Integer'},
                    { name: 'toString ', kind: vscode.CompletionItemKind.Function, detail: 'EO To String'},
                    { name: 'toFloat ', kind: vscode.CompletionItemKind.Function, detail: 'EO To Float'},
                    { naem: 'toBool ', kind: vscode.CompletionItemKind.Function, detail: 'EO TO Bool'},
                    { name: 'randint ', kind: vscode.CompletionItemKind.Function, detail: 'EO Randint'},
                    { name: 'choice ', kind: vscode.CompletionItemKind.Function, detail: 'EO Choice'},
                    { name: 'randfloat ', kind: vscode.CompletionItemKind.Function, detail: 'EO Randfloat'},
                    { name: 'require ', kind: vscode.CompletionItemKind.Function, detail: 'EO Require'},
                    { name: 'round ', kind: vscode.CompletionItemKind.Function, detail: 'EO Round'},
                    { name: 'return ', kind: vscode.CompletionItemKind.Function, detail: 'EO Return'},
                    { name: 'string', kind: vscode.CompletionItemKind.Keyword, detail: 'EO String Type' },
                    { name: 'True', kind: vscode.CompletionItemKind.Keyword, detail: 'EO True'},
                    { name: 'False', kind: vscode.CompletionItemKind.Keyword, detail: 'EO False'},
                    { name: 'bool', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Bool'},
                    { name: 'int', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Integer Type' },
                    { name: 'float', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Float Type' },
                    { name: 'list', kind: vscode.CompletionItemKind.Keyword, detail: 'EO List Type' },
                    { name: 'map', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Map Type' },
                    { name: 'var ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Variable Declaration' },
                    { name: 'const ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Constant Declaration' },
                    { name: 'for ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO For Loop' },
                    { name: 'while ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO While Loop' },
                    { name: 'func ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Normal Function' },
                    { name: 'fn ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Pro Function' },
                    { name: 'alias ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Alias'},
                    { name: 'import ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Import'},
                    { name: 'struct ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Struct'},
                    { name: 'method ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Method'},
                    { name: 'constructor ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO Contstructer'},
                    { name: 'if ', kind: vscode.CompletionItemKind.Keyword, detail: 'EO If'},
                    { name: 'elif ', kind: vscode.CompletionItemKind.Keyword, detail: 'Eo Else If'},
                    { name: 'else ', kind: vscode.CompletionItemKind.Keyword, detail: 'Eo Else'}
                ];

                keywords.forEach(kw => {
                    let item = new vscode.CompletionItem(kw.name, kw.kind);
                    item.detail = kw.detail;
                    completionItems.push(item);
                });

                // 2. استخراج المتغيرات المعرفة في الملف الحالي
                const text = document.getText();
                const varMatches = text.matchAll(/(?:var|const)\s+([a-zA-Z_][a-zA-Z0-9_]*)/g);
                for (const match of varMatches) {
                    completionItems.push(new vscode.CompletionItem(match[1], vscode.CompletionItemKind.Variable));
                }

                return completionItems;
            }
        },
        // الحروف التي تظهر القائمة تلقائياً فور كتابتها
        ' ', 'v', 'c', 'p', 's', 'i', 'f', 'l', 'm'
    );

    context.subscriptions.push(providerDisposable);
}

function deactivate() {}

module.exports = {
    activate,
    deactivate
};
