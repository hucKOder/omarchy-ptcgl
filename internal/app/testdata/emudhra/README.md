# Public compromised-CA regression fixtures

These files contain public certificates only. They are never installed as system
trust anchors and contain no private keys.

- Chain source: <https://emudhra.we-have-a-problem.com/chains.zip>, retrieved
  2026-10-22; archive SHA-256
  `8a2edc81b6c51e3ad27d45ca1e441ec84c961e2dee0fc547f2a02c0f74315f95`.
- Root source: <https://emudhra.we-have-a-problem.com/roots.json>, retrieved
  2026-10-22. Included roots are emSign ECC Root CA - G3 (certificate SHA-256
  `86a1ecba089c4a8d3bbe2734c612ba341d813e043cf9e8a862cd5c57a36bbe6b`)
  and AAA Certificate Services
  (`d7a7a0fb5d7e2731d771e9484ebcdef71d5f0c3e0a2948782bc83ee0ea699ef4`).

Offline verification uses explicit roots and a fixed otherwise-valid time.
It requires Go's compromised-key chain-filtering error, so certificate expiry,
hostname mismatch, unrelated policy/usage errors, or system root removal cannot
produce a passing result. The optional live downloader test uses the same
test-only roots and rejection check, with current time; refresh the public leaf
fixtures/test-service assessment when the service rotates or expires its chain.
