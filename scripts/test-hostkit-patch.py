#!/usr/bin/env python3
"""Platform-independent controls for the dependency integrity gate."""
import hashlib
import importlib.util
import json
from pathlib import Path, PurePosixPath, PureWindowsPath
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('patch_check', Path(__file__).with_name('check-hostkit-patch.py'))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class PatchIntegrity(unittest.TestCase):
    def test_windows_and_posix_keys_agree(self):
        for cls, root in [(PurePosixPath, '/source'), (PureWindowsPath, 'D:/source')]:
            self.assertEqual(check.manifest_key(cls(root) / 'hostkit/endpoint.go', cls(root)), 'hostkit/endpoint.go')

    def test_digest_and_file_set_tampering_fail(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'endpoint.go'
            source.write_bytes(b'reviewed\n')
            digest = hashlib.sha256(source.read_bytes()).hexdigest()
            (root / 'UPSTREAM.json').write_text(json.dumps({'commit': '2b8e50055e20cee93f51a3f96c552a964a968b12', 'files': {'endpoint.go': {'sha256': digest}}}))
            check.verify(root)
            source.write_bytes(b'mutated\n')
            with self.assertRaisesRegex(AssertionError, 'differs from reviewed copy: endpoint.go'):
                check.verify(root)
            source.write_bytes(b'reviewed\n')
            (root / 'unexpected.go').write_bytes(b'extra\n')
            with self.assertRaisesRegex(AssertionError, 'file set differs'):
                check.verify(root)


if __name__ == '__main__':
    unittest.main()
