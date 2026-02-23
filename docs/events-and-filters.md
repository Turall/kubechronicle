# Event Flow, Ignore, and Block Logic

This page explains **how kubechronicle decides which events to store**, how ignore/block patterns work, and what the event lifecycle looks like.

## What is an event?

An **event** is a Kubernetes API operation that kubechronicle records as a `ChangeEvent`:

- **Admission webhook**:
  - Operations: `CREATE`, `UPDATE`, `DELETE`
  - Resources: core resources (Deployments, StatefulSets, DaemonSets, Services, ConfigMaps, Secrets (hashed), Ingress) and any CRDs you add to the webhook rules
- **Audit processor (optional)**:
  - Operations: `EXEC` (pod exec and node exec), if you run the audit processor

Each event captures:

- Who: user/service account, groups, source IP
- What: resource kind / namespace / name
- When: timestamp
- How: diff (for UPDATE) or snapshot (for DELETE)
- Whether it was allowed or blocked (`allowed` + `block_pattern`)

## Event lifecycle (webhook)

For `CREATE` / `UPDATE` / `DELETE`:

1. **Kubernetes API server** sends an `AdmissionReview` to the kubechronicle webhook.
2. The **decoder** extracts:
   - Operation (`CREATE` / `UPDATE` / `DELETE`)
   - Resource kind, namespace, name
   - Actor (user, groups, service account, source IP)
   - For `UPDATE`: old and new objects → JSON Patch diff
   - For `DELETE`: filtered snapshot of the object
3. The handler builds a `ChangeEvent` and then:
   - Loads the current **ignore** and **block** configuration
   - Checks **block** rules first
   - If not blocked, checks **ignore** rules
4. If **blocked**:
   - `allowed = false`
   - `block_pattern` is set to the matching pattern
   - The event is queued for persistence (if store is configured)
   - The webhook response is `Allowed: false` (Kubernetes request is denied)
5. If **ignored**:
   - The event is **not queued** or stored
   - The webhook response is `Allowed: true` (request proceeds)
6. Otherwise:
   - `allowed = true`
   - `block_pattern = ""`
   - The event is queued and saved asynchronously to PostgreSQL
   - The webhook always returns `Allowed: true` (unless blocked by rules)

If the store is unavailable or the queue is full, kubechronicle **logs a warning and drops events**, but it **never blocks Kubernetes** (fail-open design).

## Ignore patterns

Ignore patterns let you **skip noisy events** while still allowing the underlying Kubernetes operation.

Configured via:

- `IGNORE_CONFIG` environment variable (JSON), or
- Individual env vars (`IGNORE_NAMESPACES`, `IGNORE_NAMES`, etc.), or
- Mounted ConfigMap that the handler reloads periodically.

An event is ignored if **any** of these patterns match:

- `namespace_patterns`
- `name_patterns`
- `resource_kind_patterns`

**Behavior:**

- Webhook response: **Allowed** (request proceeds)
- Event: **not stored**, no row in `change_events`
- Logs: informational entry noting that the event matched an ignore pattern

See also:

- High-level description in the main `README.md` under **Ignore Patterns**

## Block patterns

Block patterns let you **actively deny** certain operations (optional enforcement mode).

Configured via `BLOCK_CONFIG` (JSON) with:

- `namespace_patterns`
- `name_patterns`
- `resource_kind_patterns`
- `operation_patterns` (e.g. `["DELETE"]`; empty = all operations that match other patterns)
- Optional `message` returned to the user when blocked

**Evaluation:**

- Block rules are evaluated **before** ignore rules.
- Case-insensitive matching for operations.

**Behavior when blocked:**

- Webhook response:
  - `Allowed: false`
  - HTTP status `403`
  - Message from `BLOCK_CONFIG.message` (or a default)
- Event:
  - Stored with `allowed = false`
  - `block_pattern` set to the pattern that matched
  - Everything else (who/what/when) is captured as usual

This lets you audit **attempted** forbidden actions as well as successful ones.

## Auto-ignored fields in diffs/snapshots

To keep diffs readable and efficient, kubechronicle ignores Kubernetes “noise” fields:

- `metadata.managedFields`
- `metadata.resourceVersion`
- `metadata.generation`
- `metadata.creationTimestamp`
- Entire `status` subtree
- `kubectl.kubernetes.io/last-applied-configuration`

For **Secrets**, values in `data` and `stringData` are **hashed (SHA‑256)** so that plaintext secrets are never stored.

## Event lifecycle (exec tracking)

If you enable the **audit processor**:

1. Kubernetes audit logs are sent to kubechronicle (file or webhook).
2. The audit processor filters events where:
   - Subresource is `exec`, or
   - `requestURI` contains `/exec`
3. It creates a `ChangeEvent` with:
   - `operation = "EXEC"`
   - `resource_kind` = `Pod` or `Node`
   - `exec_metadata` (command, container, stdin, TTY, target type, node name)
4. The event is saved to PostgreSQL in the same `change_events` table.

There are **no ignore/block rules applied at this stage**; if you run the audit processor, all successfully parsed exec events are stored.

