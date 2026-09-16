# Recommendations

The general reference this project follows for Go is Google's
[Go Style Guide](https://google.github.io/styleguide/go/). It is a good document, it is long, and it is
not restated here — what is here is only what this project adds to it, or sharpens into something a test
can hold.

The rest of the rules live next door:

- [ARCHITECTURE.md](ARCHITECTURE.md) — the layers, what may import what, package names, file roles, and
  how to add a feature.
- [CLAUDE.md](../CLAUDE.md) — how we work: what a comment is for, how long a line may be, what a test
  looks like, how secrets are handled.

## Names of functions and methods

A name carries the noun, not the act of getting it: `JobName`, not `GetJobName`. A name that says "get"
says nothing the return value has not already said, and it is noise at every call site.

`internal/archtest` rejects any `Get`-prefixed declaration. There is one honest exception, kept in
`allowedGetPrefix` with its reason: a method on an interface this app implements but does not own, whose
method set is not ours to choose.

## Avoid repetition

A declaration must not spell its own package out again. `postmanType` in package `postman` reads as
`postman.postmanType` everywhere it is used: the package name has already said it, and the repetition
costs the reader a word for nothing.

The test catches a name *longer* than its package and only an exported one, which leaves room for the one
name that has to repeat: `record.Record` is the aggregate the package is about. The noun is not a prefix
that adds nothing — it is the name of the thing.

## Import order

Imports go in three groups, in this order, separated by blank lines:

1. the standard library;
2. everything else — third-party code;
3. this module. (`json-inspector/…`)

A group holds one kind and is sorted within itself; a file that has nothing for a group simply has no
group. Which kind an import is, is decided the way `goimports` decides it — a dot in the first path
segment means it came from somebody else. That is deliberate: an ordinary file with the first two groups
is already correct as `goimports` writes it, so the only rule a formatter cannot apply for you is the
third group's existence, and that is exactly what the test looks at.

## Test doubles

A fake answers the question its port asks, and no more. It is not a second implementation of the thing
it stands in for: if a fake grows the real grammar — parsing `{{tokens}}`, formatting a URL — then a test
of the real code can pass while the fake's copy of the rules is the only thing being exercised. The
cheap version in which a fake scans for a substring is not laziness; it is the point, because it cannot
be wrong about rules it does not know.

The consequence, and the reason to write it down: when a fake becomes the second implementation, the
honest fix is to test through a real one instead — a database in a temp directory rather than a map, the
real `pkg/migrate` rather than a stub — not to make the fake more faithful.

## A rule that is not in a test is a rule that drifts

Each of the rules above names the test that holds it, and that is why they are written in this voice
rather than as advice. The layering rules are checked the same way: a violation fails `go test`, not a
review. A rule nobody can check is a rule that quietly stops being true — which has happened here
before, and is the reason `internal/archtest` exists at all.
