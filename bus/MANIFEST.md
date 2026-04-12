# Bus Manifest Format (`bus.json`)

Every tool that participates in the bus ships a small `bus.json`
manifest declaring what topics it publishes and what topics it
subscribes to. The bundle install script reads all the manifests at
install time, validates the flow graph, and warns about dangling
subscribers (subscribed to a topic nobody publishes) or orphan
publishers (publishing a topic nobody listens to — fine, but worth
seeing).

The manifest is **descriptive, not prescriptive**: nothing in the bus
runtime enforces it. A tool can publish topics not in its manifest
without anything breaking. The manifest exists for humans, install-time
validation, and tooling — not for runtime safety.

## Schema

```json
{
  "tool": "dossier",
  "version": "1.0.0",
  "publishes": [
    {
      "topic": "contacts.created",
      "description": "A new contact was added to the dossier database.",
      "payload_example": {
        "id": "1234567890",
        "name": "Bob Bakery",
        "email": "bob@example.com",
        "created_at": "2026-04-09T12:00:00Z"
      }
    },
    {
      "topic": "contacts.updated",
      "description": "An existing contact was modified.",
      "payload_example": { "id": "1234567890", "name": "Bob's Bakery" }
    },
    {
      "topic": "contacts.deleted",
      "description": "A contact was removed.",
      "payload_example": { "id": "1234567890" }
    }
  ],
  "subscribes": []
}
```

## Field reference

### Top level

- `tool` (string, required) — the tool's slug, must match the `source`
  this tool passes to `bus.Open()`.
- `version` (string, required) — semver of the manifest itself, not
  the tool. Bump on any breaking change to a published topic shape.
- `publishes` (array, required) — list of topic declarations this tool
  publishes. Empty array if pure subscriber.
- `subscribes` (array, required) — list of topics this tool listens
  to. Empty array if pure publisher.

### `publishes[]`

- `topic` (string, required) — flat topic string, lowercase, dot-
  separated, conventionally `<tool>.<event>` (e.g. `contacts.created`).
- `description` (string, recommended) — one-line human description.
- `payload_example` (object, recommended) — sample payload showing
  what consumers should expect. Not enforced at runtime, but used by
  the validator to flag downstream subscribers that obviously expect
  a different shape.

### `subscribes[]`

- `topic` (string, required) — topic this tool listens to. Should
  exist in some other tool's `publishes` array, or the validator
  warns.
- `reason` (string, recommended) — one-line human description of why
  this tool cares about the topic. e.g. "translates contact creation
  into a funnel event".

### Wildcard subscribers

A tool that subscribes to **every** topic via `bus.SubscribeAll(...)`
(currently just the audit log) is a special case. Listing every
possible topic in `subscribes[]` would be wrong — the wildcard
subscriber catches topics that don't exist yet, and listing the
universe of topics would also defeat the validator's "no tool
subscribes to this topic" check (every publish would suddenly have a
"subscriber" so the orphan-publisher info would never fire).

Such tools set `"wildcard": true` at the top level of their manifest
and leave `publishes` and `subscribes` as empty arrays:

```json
{
  "tool": "audit",
  "version": "1.0.0",
  "wildcard": true,
  "publishes": [],
  "subscribes": []
}
```

The validator special-cases these:

- They emit an info-level "wildcard subscriber" line in the report so
  they're visible (rather than silently appearing as a no-op tool).
- They are NOT added to the publish/subscribe graph, so the validator
  still flags real orphan publishers and dangling subscribers
  correctly.

A wildcard tool may still declare specific `subscribes[]` entries
(mixed mode — explicit topics it especially cares about plus the
wildcard fallback). The validator treats those entries normally.

### Top-level fields, continued

- `wildcard` (bool, optional) — see above. Defaults to false. Only the
  audit log uses this currently.

## Topic naming convention

- Lowercase, ASCII letters, digits, dots.
- First segment is the publishing tool's slug.
- Second segment is the event in past tense for facts (`created`,
  `updated`, `deleted`, `paid`, `confirmed`) or imperative for
  commands (which are rare in this system).
- Avoid more than three segments. If you find yourself writing
  `orders.refunds.partial.processed`, that's a sign the second segment
  should be the noun and the verb a property in the payload. A trailing
  version suffix (`.v2`, `.v3`, …) does not count against the segment
  budget — `contacts.created.v2` is fine.
- Reserve the prefix `_bus.` for internal bus events (we don't have
  any yet, but if we ever publish `_bus.handler_failed` or similar,
  this convention prevents collisions).

## Schema versioning policy

Additive changes (new fields in the payload) are not breaking.
Subscribers MUST unmarshal permissively and ignore fields they don't
recognize — strict decoding (`DisallowUnknownFields`) is a contract
violation.

Breaking changes (renamed fields, removed fields, changed types)
require a new topic name with an explicit version suffix:

- `orders.created` — the existing topic, understood to be v1 implicitly
- `orders.created.v2` — the new shape, running alongside v1

v1 is never written explicitly. Topics start unsuffixed; the first
breaking change introduces `.v2`. The validator rejects explicit `.v1`
suffixes as redundant.

The publisher should keep emitting both during a migration window,
and remove the old topic only after every subscriber in the bundle has
been updated. The validator will eventually grow logic to detect a
publisher that has dropped a topic some subscriber still listens to.

Full rationale and the alternatives considered are in
`docs/adr/001-schema-versioning.md`. If this policy turns out to be
wrong the ADR has explicit revisit triggers.

## Validation

A small validator (`cmd/bus-validate/`) reads all `bus.json` files in
a directory tree and reports:

1. **Dangling subscribers**: a tool subscribes to a topic no other
   tool publishes. Warning, not error — could be intentional if the
   subscriber is preparing for a future publisher.

2. **Orphan publishers**: a tool publishes a topic no other tool
   subscribes to. Info, not warning — perfectly valid, the publisher
   just doesn't have any consumers right now.

3. **Topic name violations**: lowercase, dot-separated, no special
   characters. Error.

4. **Missing required fields**: `tool`, `version`, `publishes`,
   `subscribes`. Error.

5. **Slug mismatch**: `tool` field doesn't match the directory name
   or the binary name. Warning.

The validator runs as part of the bundle install script. It exits 0
on success or warnings, exits 1 on errors. The install script blocks
on errors and prints warnings without blocking.
