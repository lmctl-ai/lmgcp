# Pinned lmcloud compatibility patch

Upstream: lmctl-ai/lmcloud v0.1.0, commit
`2b8e50055e20cee93f51a3f96c552a964a968b12`. UPSTREAM.json records its Go
module checksum and original/current SHA256 for included files. This copy
contains the hostkit packages used by lmgcp, their tests, unchanged schema,
license and module/SDK locks. Standalone upstream commands are omitted.

WP230 removes implicit response limits and total deadlines while retaining the
existing cloud schema. Default operation/list durations, HTTP client timeout
and exhaustive-list page count are zero (unlimited). ObjectGet passes limit=0
to its adapter and returns the complete body. ReadBounded and adapter GetObject
interpret nonpositive byte bounds as unlimited; explicitly supplied positive
bounds remain enforced. Context cancellation, pagination-cycle checks and
same-origin redirect/authentication rules keep their existing protections.
The caller's long-poll wait is passed to the queue operation and does not become
a total transport deadline. Domain input validation uses the existing v0.1.0 contract.

Verify with `python3 scripts/check-hostkit-patch.py` from the lmgcp root and the
hostkit package tests in test.sh. Review this patch with future dependency
updates so an upstream replacement cannot silently restore the retired limits.
The reviewed Go SDK pin and Windows journal files are unchanged.

The copied invocation test fixture JSON-encodes native filenames; its Windows
path and quoted-name controls verify valid JSON without weakening validation.
