// Package core implements the data model and verification rules of the
// sourced.net protocol (spec sourced/1): hashing and IDs, canonical signing,
// records, bundles, manifests, key sets, citations, change states, and the
// chunker.
//
// Every other component (publisher tooling, resolver, validating client,
// checker, benchmark) builds on this package, so they all agree on the rules
// by construction.
package core
