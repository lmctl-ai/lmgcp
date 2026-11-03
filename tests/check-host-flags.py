#!/usr/bin/env python3
"""Hard-cutover CLI contract. Retired spellings must fail before opening a file."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

schema = Path(sys.argv[1]).resolve()
command = sys.argv[2:]
assert command, 'schema path and host command required'
old = '--' + 'dsl'
with tempfile.TemporaryDirectory(prefix='host flag control ') as directory:
    scratch = Path(directory)
    missing = scratch / 'must-not-open.dslb'
    env = dict(os.environ, HOME=str(scratch), LMCTL_HOME=str(scratch / '.lmctl'))
    def run(args, env=env):
        return subprocess.run([*command, *map(str, args)], capture_output=True, env=env)
    # Collect refusal controls so a baseline proves every retired spelling fails.
    failures = []
    for flag, rest in [(old, [missing]), (old + '-schema', ['--json']),
                       (old + '-signatures', []), ('--first', [missing]),
                       ('--last', [missing]), ('cloud', ['--first', missing]),
                       ('cloud', ['--last', missing])]:
        result = run([flag, *rest])
        if result.returncode != 2: failures.append((flag, result.returncode, 'retired option accepted or reached execution'))
        if str(missing).encode() in result.stdout + result.stderr: failures.append((flag, 'opened retired-flag input'))
    assert not failures, failures
    assert not list(scratch.iterdir()), 'retired option created scratch-HOME state'
    result = run(['--lmcode-schema', '--json'])
    assert (result.returncode, result.stdout, result.stderr) == (0, schema.read_bytes(), b''), result
    json.loads(result.stdout)
    result = run(['--lmcode-signatures'])
    assert result.returncode == 0 and result.stdout and not result.stderr, result
    # The selector must recognize execution, yielding a loader error rather than usage.
    result = run(['--lmcode', missing])
    assert result.returncode == 3, ('new execution option did not reach the loader', result)
    assert not missing.exists()

    # A present invocation manifest is strict. Missing and non-boolean dry
    # values must fail as CLI errors before the host opens bytecode or dispatches.
    manifest = scratch / 'invocation.json'
    for label, body in [
        ('missing', {'version': 1, 'mode': 'single', 'parts': [{'index': 0, 'file': str(missing)}]}),
        ('string', {'version': 1, 'mode': 'single', 'dry': 'false', 'parts': [{'index': 0, 'file': str(missing)}]}),
        ('null', {'version': 1, 'mode': 'single', 'dry': None, 'parts': [{'index': 0, 'file': str(missing)}]}),
        ('number', {'version': 1, 'mode': 'single', 'dry': 0, 'parts': [{'index': 0, 'file': str(missing)}]}),
    ]:
        manifest.write_text(json.dumps(body), encoding='utf-8')
        result = run(['--lmcode', missing], env={**env, 'LMCODE_INVOCATION_FILE': str(manifest)})
        assert result.returncode == 2, (label, result.returncode, result.stdout, result.stderr)
        assert b'LMCODE_RETURN_BEGIN' in result.stdout and b'"phase":"cli"' in result.stdout, (label, result.stdout)
        assert b'LOAD_BAD_BYTECODE' not in result.stdout, (label, 'host dispatched before manifest rejection', result.stdout)

    manifest.write_text(json.dumps({
        'version': 1, 'mode': 'single', 'dry': True,
        'parts': [{'index': 0, 'file': str(missing)}],
    }), encoding='utf-8')
    result = run(['--lmcode', missing], env={**env, 'LMCODE_INVOCATION_FILE': str(manifest)})
    assert result.returncode == 3 and b'"dry":true' in result.stdout, ('valid dry manifest did not reach dry-aware load failure', result)
    manifest.write_text(json.dumps({
        'version': 1, 'mode': 'single', 'dry': False,
        'parts': [{'index': 0, 'file': str(missing)}],
    }), encoding='utf-8')
    result = run(['--lmcode', missing], env={**env, 'LMCODE_INVOCATION_FILE': str(manifest)})
    assert result.returncode == 3 and b'"dry":true' not in result.stdout, ('valid real manifest was marked dry', result)
print('PASS hard flag cutover: exact schema/signatures, new loader route, retired options refused before file effects')
