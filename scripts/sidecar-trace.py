#!/usr/bin/env python3
"""Derive Linux syscall names from pinned sidecars and the generated PM-1 corpus.
Run via scripts/heavy.sh; STRACE, TYPST and TINYMIST are absolute executable paths.
Raw traces are retained in the supplied output directory for review in CI.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

out = Path(sys.argv[1])
out.mkdir(parents=True, exist_ok=True)
root = Path('testdata/perf/generated').resolve()
main = root / 'text/main.typ'
strace = os.environ.get('STRACE', 'strace')
typst = os.environ['TYPST']
tinymist = os.environ['TINYMIST']

def trace(name, command):
    return [strace, '-f', '-qq', '-o', str(out / (name + '.trace')), *command]

for action, args in [('compile', [str(main), str(out / 'main.pdf')]),
                     ('query', [str(main), 'heading']),
                     ('eval', ['--in', str(main), 'query(heading)'])]:
    subprocess.run(trace(action, [typst, action, *args]), check=True, stdout=subprocess.DEVNULL)

p = subprocess.Popen(trace('lsp', [tinymist, 'lsp']), stdin=subprocess.PIPE,
                     stdout=subprocess.PIPE, stderr=(out / 'lsp.log').open('wb'))
def send(message):
    body = json.dumps(dict(jsonrpc='2.0', **message)).encode()
    p.stdin.write(b'Content-Length: ' + str(len(body)).encode() + b'\r\n\r\n' + body)
    p.stdin.flush()
def response(identifier):
    while True:
        headers = {}
        while True:
            line = p.stdout.readline()
            if not line:
                raise RuntimeError('LSP closed before response')
            if line == b'\r\n':
                break
            key, value = line.decode().split(':', 1)
            headers[key.lower()] = value.strip()
        message = json.loads(p.stdout.read(int(headers['content-length'])))
        if message.get('id') == identifier:
            if 'error' in message:
                raise RuntimeError(message)
            return message
send(dict(id=1, method='initialize', params=dict(processId=None, rootUri=root.as_uri(), capabilities={})))
response(1)
send(dict(method='initialized', params={}))
send(dict(method='textDocument/didOpen', params=dict(textDocument=dict(uri=main.as_uri(), languageId='typst', version=1, text=main.read_text()))))
send(dict(id=2, method='textDocument/documentSymbol', params=dict(textDocument=dict(uri=main.as_uri()))))
response(2)
send(dict(id=3, method='shutdown', params=None))
response(3)
send(dict(method='exit', params=None))
p.stdin.close()
if p.wait(timeout=60):
    raise RuntimeError('LSP failed')
names = set()
for path in out.glob('*.trace'):
    names.update(re.findall(r'^\s*(?:\d+\s+)?([a-z][a-z0-9_]*)\(', path.read_text(), re.M))
(out / 'syscalls.txt').write_text('\n'.join(sorted(names)) + '\n')
print(out / 'syscalls.txt')
