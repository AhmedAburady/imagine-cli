# Architecture Ethos: LEGO, not Jenga

> Apply this principle to every design and implementation decision in this codebase, regardless of language, framework, or paradigm.

## The Principle

Build with **LEGO**, never **Jenga**.

- **LEGO** — every unit of code is either a *reusable brick* (a self-contained piece with a stable, minimal interface) or it *snaps into an existing one* (it extends the system through an established connection point). You grow the system by **adding** pieces, not by reaching into and reshaping the ones already in place.
- **Jenga** — units are stacked and load-bearing. Pulling, replacing, or changing one risks toppling the rest. Behavior is added by wedging logic into existing pieces until the structure is fragile and any move is a gamble.

**Extensibility is a primary goal, not an afterthought.** The system should be cheap to extend (add a brick) and expensive to break (no piece silently depends on another's internals).

## The Decision Procedure

Before writing or modifying any code, classify the change:

1. **Is it a new brick?** A new, self-contained unit with a clear contract that could be reused or removed without disturbing unrelated code. → Build it.
2. **Does it snap into an existing brick?** It plugs into an established extension point — an interface, a registry, a hook, a plugin slot, a strategy, an event. → Plug it in.
3. **Is it neither?** If the only way forward is to modify the internals of an existing piece that others depend on, **you are about to play Jenga. Stop.**
   - Propose the brick-shaped alternative: introduce the missing extension point, then add your piece through it.
   - If a refactor is genuinely required first, surface it explicitly and do it as its own step — never bury a structural change inside a feature change.

## What Qualifies as a "Brick"

A brick must satisfy all of these, in any language:

- **Stable, minimal public contract.** It exposes the smallest surface needed and nothing about its internals. Callers depend on the contract, never the implementation.
- **Self-contained.** It owns its own state and logic. It does not reach into another piece's private internals, and nothing reaches into its.
- **Replaceable.** It can be swapped for another implementation of the same contract without callers noticing.
- **Removable.** Deleting it does not cause unrelated parts of the system to collapse. If removal causes a cascade, the coupling was Jenga.
- **Composable.** It connects to other bricks through their contracts, so the same brick can be reused in new combinations.

## LEGO Moves vs. Jenga Moves

| Goal | LEGO move (do this) | Jenga move (avoid) |
|---|---|---|
| Add a variant / case | Define a contract; register a new implementation behind it | Add another `if/switch` branch inside the core that now knows every case |
| Reuse logic | Extract a shared brick with a clear contract; all callers snap to it | Copy-paste the block into each call site; they drift over time |
| Cross-module call | Talk through the other module's published interface | Reach into its private fields / internal state |
| Select behavior | Make it data- or config-driven against a common interface | Hardcode the specific choice at the call site |
| Grow a component | Add behavior via a hook / plugin / extension point | Edit the component's internals every time something new is needed |
| Change shared logic | Change it in the one brick that owns it | Change it in three places and hope they stay in sync |

## Examples (language-agnostic)

The contrast is the same in any language. Pseudocode below; map it to whatever you're writing in.

### 1 — Adding a new case

**Jenga** — the core grows a new branch and learns about every variant. Each addition edits load-bearing code:
```
function process(item):
    if item.type == "A": ...
    elif item.type == "B": ...
    elif item.type == "C": ...   # every new type edits this function
```

**LEGO** — the core stays closed; new behavior arrives as a new brick snapped into a registry:
```
interface Handler:
    handles(item) -> bool
    process(item) -> result

registry.register(AHandler)
registry.register(BHandler)
registry.register(CHandler)   # new behavior = add a brick, core untouched

function process(item):
    return registry.resolve(item).process(item)
```

### 2 — Reusing logic

**Jenga** — the same validation logic copied into three modules; fixing a bug means finding all three.

**LEGO** — one `validate()` brick with a clear contract; all three modules call it. One fix, everywhere.

### 3 — Crossing a boundary

**Jenga** — `ModuleA` reads `ModuleB.internalCache` directly, so any change to B's internals breaks A.

**LEGO** — `ModuleA` calls `ModuleB.get(key)`. B can rewrite its internals freely as long as the contract holds.

### 4 — Choosing an implementation

**Jenga** — the storage URL and provider-specific calls are hardcoded at every call site.

**LEGO** — a `Storage` contract with interchangeable implementations (`LocalStorage`, `S3Storage`, …); the choice is wired once from config. Adding a provider is a new brick, not a sweep through the codebase.

## Self-Check (run before committing any change)

- [ ] Is this change a **new brick** or does it **snap into an existing extension point**? (If neither, it's Jenga — reconsider.)
- [ ] Does every new unit expose a **minimal contract** and hide its internals?
- [ ] Can I add this **without modifying the internals** of unrelated, depended-upon code?
- [ ] If this piece were **removed later**, would anything unrelated collapse?
- [ ] Did I **reuse** an existing brick instead of duplicating logic?
- [ ] If a structural change was unavoidable, did I **surface it and isolate it** from the feature work?

When in doubt: **add a brick, don't pull from the tower.**
