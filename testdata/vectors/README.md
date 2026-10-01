# Test vectors

A small publisher, `example-library.test`, built deterministically from the Markdown in `source/`. Any implementation of sourced/1 should reproduce these files byte for byte from the same inputs, or at least verify them.

| Path | What it is |
| --- | --- |
| `source/` | The input pages. `book-care-v2.md` corrects one paragraph of `book-care-v1.md`. |
| `example-library.test/.well-known/sourced/` | The publisher tree: `keys.json`, `manifest.json`, `records/`, `bundles/`. |
| `expected.json` | Record IDs, states, the example citation, and the key seeds. |

What the vectors cover:

- **Keys:** `2026a` active, `2025a` retired (still verifies what it signed), `2024x` revoked.
- **book-care v1 → v2:** a `correction` with a note. Three of the four chunks keep their IDs, and v1 resolves to `corrected`.
- **digitization:** signed with the retired key, with a fenced code block inside a chunk.
- **Manifest:** points each URL to its current record.

Chunking uses the default parameters (150 / 800 / 2,000 characters).

## Keys

Each private key is the Ed25519 key whose seed is the SHA-256 of its seed phrase in `expected.json`, for example `sha256("sourced.net test vector key 2026a")`. **These keys are public test fixtures. Never use them for anything else.**

## Regenerating

The vectors are checked by `go test ./core` (`TestVectors`). After a deliberate change to the format or the chunker, regenerate them with:

```
make vectors
```

and review the diff before committing.
