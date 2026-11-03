#!/usr/bin/env python3
"""Verify the reviewed, locally patched cloud dependency tree."""
import hashlib
import json
from pathlib import Path

root = Path(__file__).resolve().parent.parent / 'third_party/lmcloud'
lock = json.loads((root / 'UPSTREAM.json').read_text())
assert lock['commit'] == '2b8e50055e20cee93f51a3f96c552a964a968b12'
files = {str(p.relative_to(root)): p for p in root.rglob('*') if p.is_file() and p.name != 'UPSTREAM.json'}
assert files.keys() == lock['files'].keys(), 'cloud dependency file set differs from reviewed copy'
for name, path in files.items():
    assert hashlib.sha256(path.read_bytes()).hexdigest() == lock['files'][name]['sha256'], 'cloud dependency differs from reviewed copy: ' + name
print('PASS pinned cloud dependency and compatibility patch')
