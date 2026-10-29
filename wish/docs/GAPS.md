# Gaps

Open questions and explicit assumptions for wish. **Questions** are for the user to answer.
**Assumptions** are what the documents currently take for granted because they seemed clear enough
not to ask; check them and correct any that are wrong. The rules for this file are in
[SDLC](../../docs/SDLC.md#work-items). Status: 2026-10-27 14:52.

- Answers carry date, time and author: `**Answer** (2026-10-27 15:10, Gregor Heinrich): …`. Short
  answers are fine. A changed answer goes on top; earlier ones stay below it, struck through.
- Once answered, the decision moves into the document it affects, and the entry gets a
  `**Resolved**` line saying where. Resolved entries stay as a record.
- A question that blocks a TODO item is marked **blocking**. Design questions that a spike is meant
  to answer (just-bash, the notebook UI) live in [TODO](TODO.md), not here.

## Questions

### Q1. When does wish implementation start?

wish has been design-only since 2026-10-14, while ush was built and is heading for its first
release ([ush ROAD](../../ush/py/docs/ROAD.md)).

- **Proposed:** after the ush 0.1 release, starting with TODO 1.1 and the just-bash spike (2.1).

### Q2. Which browsers does phase 1 support? — blocking for TODO 1.1

The persistence, worker and isolation choices depend on it (IndexedDB, cross-origin isolation,
Pyodide).

- **Proposed:** current Chrome and Firefox on the desktop; Safari once phase 1 works, because its
  storage limits and eviction rules differ most.

### Q3. Which AI providers does phase 1 offer? — blocking for TODO 6.1

Calling a provider directly from the browser needs the provider to allow it; otherwise a broker
carries the key, which changes the security profile ([SEC](SEC.md)).

- **Proposed:** Anthropic with direct browser access and the key held in session memory, as ush
  does; a local Ollama as the second option, behind the same loopback rules as in ush.

### Q4. Which licence?

The repository has no licence file. The same question for ush is [ush GAPS](../../ush/py/docs/GAPS.md)
Q1; one answer covers both.

**Answer** (2026-10-28 21:20, Gregor Heinrich): MIT. **Resolved:** [LICENSE](../../LICENSE).

## Assumptions

| ID | Assumption | Where it matters | Correction |
| --- | --- | --- | --- |
| A1 | Phase 1 needs no server: static files, with all workspace data in the browser. | [ARCH](ARCH.md), [ROAD](ROAD.md) | |
| A2 | The user brings their own AI key and pays for it; wish holds no key of its own. | [IDEA](IDEA.md), [SEC](SEC.md) | |
| A3 | One user per workspace in phase 1; sharing comes only with remote storage (TODO 7.1). | [ARCH](ARCH.md) | |
