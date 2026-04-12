# ADR 001: Bus event schema versioning

**Status:** Accepted (provisional, revisitable)
**Date:** 2026-04-09
**Context:** First cross-tool integration (dossier → headcount) is
shipping. Multiple tools will eventually publish and subscribe to
shared topics. Payload shapes will need to evolve.

## The decision

**Additive changes are not breaking. Breaking changes get a new topic
with a `.vN` suffix.**

Concretely:

1. Adding a new field to a payload is fine. Subscribers ignore unknown
   fields. No version bump required.

2. Renaming a field, removing a field, or changing a field's type is
   breaking. The publisher must:
   a. Start emitting a new topic name with a `.v2` suffix (or `.v3`,
      etc.) containing the new shape.
   b. Continue emitting the old topic name with the old shape during
      a migration window.
   c. Drop the old topic only after every subscriber in known bundles
      has migrated.

3. The bus runtime does not enforce any of this. The validator
   (`cmd/bus-validate`) is the only enforcement, and it only catches
   the obvious cases (subscribers listening to topics nobody publishes,
   etc.). The honor system carries the rest.

## Why this and not the alternatives

Three options were on the table:

### Option A: No versioning, additive only, and breaking changes are just bugs

The simplest possible policy. Treats every payload as forever-stable.

Rejected because: it's a lie. Real systems evolve. Telling subscribers
"the payload will never change" and then changing it is worse than
giving them an explicit version they can pin to.

### Option B: Versioned topic names (`orders.created.v1`, `orders.created.v2`) ← **chosen**

Publishers can run multiple versions in parallel during migrations.
Subscribers explicitly opt into the version they understand. The bus
treats different versions as different topics, which means no special
runtime support is required.

The cost is uglier topic names and a bit of mental overhead when
breaking changes happen. The benefit is migrations are explicit,
visible in `bus.json` manifests, and don't require any global
coordination.

### Option C: A schema registry table inside `_bus.db`

Each topic has a registered schema (JSON Schema or similar), and the
bus runtime validates payloads on publish. Subscribers declare which
schema version they understand and the bus only delivers compatible
events.

Rejected because:
- Significant runtime cost (schema validation on every event).
- Significant developer cost (every tool has to write and maintain
  JSON schemas).
- Solves a problem we don't have yet — most subscribers aren't
  validating payload shapes today, they just unmarshal what they need
  and ignore the rest.
- We can always add this later without breaking option B. Option B is
  the simpler thing that works.

## Consequences

**Good:**
- Tools can ship payload changes independently without coordination.
- Migration paths are explicit and traceable in the manifests.
- Subscribers don't break silently when publishers add fields.
- Zero runtime cost.

**Bad:**
- The honor system is the only thing enforcing this. A publisher who
  yolo-renames a field will silently break every subscriber.
- Topic names get noisier over time as `.v2`, `.v3` accumulate. We
  should eventually delete old versions, but the policy for "when is
  it safe" is itself a coordination problem.
- The validator can't tell a real breaking change from a typo. If
  someone publishes `orders.created.v2` thinking they renamed a field
  but actually broke nothing, the validator just sees a new topic.

## When to revisit

Revisit this decision if any of the following happens:

1. Two or more breaking-change incidents in a quarter where a
   subscriber broke silently and we didn't notice for >24h. That
   means the honor system isn't enough and we need runtime validation
   (option C).

2. Topic name pollution gets bad enough that someone has to write a
   "topic deprecation" tool. That means the manual process for
   retiring old versions has scaled past human attention.

3. A subscriber actually wants to subscribe to "any version of
   orders.created" rather than picking one explicitly. That means the
   versioning model is leaking into application code in a way that
   suggests a different abstraction would be cleaner.

Until any of those happen, this is the policy.

## Implementation notes

- The validator should grow logic to flag a publisher that has dropped
  a topic some subscriber still listens to. Currently it only flags
  the static state of the manifests, not the dynamic transition.
- Consider a `bus-versions` command later that lists all topics with
  multiple versions in flight, so deprecation candidates are visible.
- The MANIFEST.md spec already documents this convention. Keep it in
  sync with this ADR if either changes.

## Amendments 2026-04-09

Three clarifications added during session 2 verification, before any
versioned topics actually shipped. None of these change the core
decision — they nail down corners that were ambiguous.

### 1. The subscriber contract is "ignore unknown fields"

Additive-not-breaking only works if subscribers actually ignore fields
they don't recognize. Go's `encoding/json` does this by default when
you unmarshal into a struct, but `json.Decoder.DisallowUnknownFields()`
flips the behavior and will error on any new field. Strict decoding is
a contract violation — a subscriber that opts into it has silently
made every additive change a breaking change for itself.

**Rule:** subscribers MUST unmarshal permissively. They MUST NOT use
`DisallowUnknownFields` on bus payloads. If a subscriber needs strict
schemas for its own reasons, it should copy the payload into its own
strict type after permissive unmarshal.

### 2. v1 is implicit; the first suffix is `.v2`

Topics start unsuffixed. `contacts.created` is understood to be v1
retroactively. The first breaking change produces `contacts.created.v2`
as a new topic running alongside `contacts.created`. Writing `.v1`
explicitly is redundant and is rejected by the validator.

Rationale: the common case is a topic that never needs a version bump,
and making every topic wear a `.v1` suffix forever to support the rare
case is pure noise. Versioning is a thing you opt into when you need
it, not a thing you carry by default.

### 3. Validator strips `.vN` before counting segments

The MANIFEST.md guideline of "prefer ≤3 segments" was written before
versioning was decided. A topic like `contacts.created.v2` is 4
segments on the wire but only 2 segments of real structure plus a
version marker. The validator now strips a trailing `.v<N>` where
N ≥ 2 before applying the segment-count warning, so versioned topics
don't trip it.

The topic-name regex still accepts the full string — versioning is
just part of the topic name for dispatch purposes. Only the count
check treats the suffix specially.
