# core

The sourced.net spec (`sourced/1`) in code, shared by every other project: the types (records, bundles, manifests, keys, citations), canonical signing (JCS, Ed25519, content-addressed IDs), verification rules and their failure reasons, change states, and the chunker. It has no network code.

Spec: `../../Docs/sourced.net — Core Spec v1.md`. Go 1.24 or later.

```
make            # lint and test
make vectors    # regenerate the golden test vectors after a deliberate format change
```

## Packages

- `core` (this directory): types, signing, verification, chunker, citations.
- `htmllink`: finds a page's `sourced-record` link in its HTML or HTTP headers.

## Test vectors

`testdata/vectors/` holds sample records, bundles, and manifests with known IDs and signatures, so an independent implementation can check itself against this one.
