# Multi-Agent Orchestration Model

A reusable, domain-agnostic process for carrying a task through a team of
cooperating agents so that the result is **correct, and provably so** — planned
deliberately, implemented without collisions, checked by evidence rather than by
agreement, gated by a standalone auditor, and returned to a human owner, without
ever spinning in a loop.

**Prime directive: correctness and production-grade robustness over speed.** Speed is
never traded against either. The guardrails below cut *waste*, not rigor — the
anti-over-engineering rule removes ceremony that does not buy correctness, never a
check that does.

**Scope and stance.** This document is deliberately independent of any language,
framework, repository, or model vendor. It names capability *tiers*
(frontier / capable / cheap) and *roles*, never product names, so it survives model
releases. Anything project-specific — which agent is the gate, which commands are
the deterministic checks, where lessons are persisted — is plugged in at
[Adoption](#adoption-instantiating-this-in-a-project), never hard-coded into the
model. This is a **living document**: it is expected to be exercised on real work,
and every gap that real work exposes is folded back in until the model is solid
(see [How this model evolves](#how-this-model-evolves)).

---

## The two ideas the whole model enforces

> **1. Consensus is not correctness.** Agreement among checks that share a premise,
> a lens, or a model family is not evidence — they fail together.
>
> **2. Evidence beats identity — and the system is the source of truth.** The reliable
> defeat of a wrong-but-plausible result is not a differently-shaped *reader*; it is
> *executed or computed ground truth* — a from-scratch trace of the real failure, a
> recomputed number, a replayed input, **read from the code and the running system, not
> from a name, a memory, or a plausible assumption.** Evidence arrives from outside
> every model's weights, so it cannot inherit anyone's framing. The standard does not
> scale down: a claim's *triviality* lowers the process weight, **never** the burden of
> proof — the most obvious-looking premise is fact-checked against source too.

Decorrelation (idea 1) reduces the odds that reviewers miss the same thing.
Evidence (idea 2) categorically defeats an entire class of failure —
plausibility-over-mechanism, agreement-with-the-provided-framing, uncomputed
arithmetic. **The model therefore spends its strongest, mandatory machinery on
evidence, and treats reviewer diversity as a supporting hedge, not the main
control.** Getting that ranking backwards — investing in *who reads* over *what is
executed* — is the single most common way this kind of system ships a confident
defect.

*Illustrative failure (abstracted).* A fix reset a retry back-off counter when a
connection was *accepted* rather than when it had *held*. The real failure struck
just after acceptance, so the reset re-fired on every attempt and the back-off
never advanced — the fix silently did nothing. Several readers approved it by
reasoning that it "mirrored an existing blessed pattern." One reviewer caught it by
*tracing the documented failure step by step and computing the resulting rate*.
Identity-diversity did not catch it; a method — evidence — did. Every rule below is
downstream of that lesson.

---

## Roles

Each role carries the capability tier it should run at; the reasoning is
[Model Economics](#model-economics-spend-intelligence-where-judgment-lives).
Not every role appears on every task — [Risk Tiers](#risk-tiers-scale-to-blast-radius)
governs which do.

### Owner (human) — *the judge*
Sets the goal, approves scope, and decides anything the agents cannot. Acts as
**judge** the instant the process would otherwise stall. Receives a verbatim
evidence packet on escalation (not a summary). Talks only to the Orchestrator.

### Orchestrator — *frontier* — accountable for everything
Owns the outcome end to end; delegation distributes the work, never the
responsibility. Decomposes the task, dispatches sub-agents, collects results,
routes them onward, and reports honestly. It is also **the single largest source of
correlated failure**, because every sub-agent inherits its framing. Two hard
constraints follow:

- **It must not lead its witnesses — on *scope* no less than on conclusion.** Never
  state the conclusion it expects, never hand its own reasoning/analogy to a reviewer
  as fact, never pre-answer the question it is asking — and never hand a sweeper a
  *pre-narrowed* candidate set ("here are the route-param sites") in place of the
  neutral, mechanically-generated one ("here are all N sites — dispose each").
  Narrowing the set **is** leading the witness, and it travels silently to every
  downstream agent; it is the framing error most likely to survive a whole review.
- **It dispatches through checked-in templates, not free prose** — see
  [Structural mitigations](#structural-mitigations-for-the-orchestrator-single-point-of-failure).

### Planner — *frontier* — owns the design
Determines what must be done, in what order, over which files, with which
trade-offs. Three load-bearing duties:

- **Resolves ambiguity; never delegates a decision.** An open question becomes a
  question two workers answer differently. The Planner decides it, or routes it to
  the Owner — it never lets it fan out unresolved.
- **Guarantees non-overlap.** No two delegated pieces may decide the same question
  or edit the same concern.
- **Does not self-certify the design.** Wherever a plan exists it is reviewed before a
  worker builds against it — the author of the design is not its sole grader, the same
  separation of duties the model enforces on code. See
  [Plan-stage review](#plan-stage-review-refine-then-refute).

Appears only when the task is genuinely ambiguous or decomposable; trivial or
obvious work skips it. Read-only.

### Worker(s) — *capable by default* — implement the spec
Implement the approved plan exactly, keep the build green, and report the diff plus
a **claims list** (every quantitative or behavioral assertion the change makes, each
either computed or explicitly tagged `UNVERIFIED`). Count scales with *genuine*
independence:

- **Default to one worker.** Interdependent work does not decompose; parallel
  workers only help on truly non-touching threads.
- **Parallel workers must be partitioned** so they never touch the same file or
  concern. If they would, it is one worker, or sequential ones.

(On worker tier, see [Economics](#model-economics-spend-intelligence-where-judgment-lives):
default capable, not cheap, at small scale.)

### Adversary / Falsifier — *frontier* — the primary safeguard
A reviewer whose mandate is inverted: **assume the change is wrong and prove it.**
Present on all non-trivial work. It attacks the **plan whenever one exists** — a
design-level defect caught post-implementation is caught a full cycle too late, so a
wrong *approach* is cheapest to kill here — and again attacks the **implementation**.
On critical work both passes are mandatory. Its deliverables are **evidence, not
opinion**:

- a from-scratch trace of the documented/most-likely failure, with intermediate
  state — produced *before reading* the maker's comments or narrative, then compared
  against them (see [Trace-before-reading](#the-evidence-obligations-the-heart));
- fresh failure hypotheses of its own;
- an attack on the change's **unstated assumptions**, not only its written branches — an
  Adversary that refutes only what the plan *says* inherits the plan's frame. It asks the
  questions the artifact never wrote down: *what credential does step N carry, and whose is
  it by then? what bounds this loop / timer / request? what state is assumed still true
  here, and what if it changed?* The stated branches are the maker's frame; the defect
  lives in what the frame omitted;
- for anything it approves, an explicit "I attempted falsifications A, B, C and each
  held" — an **honored** deliverable, so an Adversary that comes back empty is not
  punished into manufacturing noise.

It is [kill-scored](#keeping-the-adversary-honest): a finding exists only if it
carries a concrete failing input or a replayable trace.

### Reviewer(s) — *capable-to-frontier* — supporting decorrelation
Optional supporting lens(es) beyond the Adversary, used when a distinct
*detection target* adds real coverage (e.g. systems/load, security/authorization,
data-exposure). Engineered to fail *differently* (see
[Decorrelation](#decorrelation-a-supporting-hedge-honestly-scoped)). A second
*generalist* reviewer — same stance, same targets, same family as an existing one —
adds cost and false confidence, not coverage; do not add one.

### Gate — *frontier* — standalone **evidence auditor**
Run as a standalone final step. **It does not re-read the change as a fourth
opinion; it audits that the required evidence exists.** An approval that cannot show
its work counts for nothing. It also runs the project's **deterministic checks**
(linters, type-checkers, contract/gate scripts, CI) — a zero-marginal-token
"verify" stance — and holds authority to **ratchet the risk tier up**. Its checklist
is [below](#the-gate-checklist-evidence-auditing). On any failure the work returns to
the worker(s), bounded by the [cycle cap](#the-twin-guardrails).

### External Check — *different model family, or human* — for the highest stakes
The one thing no internal panel provides: a reviewer outside the Orchestrator's own
family and context. Same-family loops share blind spots; cross-family review catches
what they cannot. Mandatory on critical work. It is a hedge, **not a substitute for
evidence** — a cross-family reader that does not execute can still miss a
mechanism-level defect.

**Procurement — who provides it.** When the Orchestrator's harness cannot itself
spawn a different-family agent (a single-vendor harness is the common case), the
External Check is **the Owner's to procure** — a different-family model, a separate
system, or a human. The internal loop cannot manufacture its own decorrelation, so
its obligation instead is to **hand over a self-contained evidence packet** — the
from-scratch traces, the computations, the new-assumptions list, the point-of-
difference analyses, the diff — so the external pass is cheap and high-signal rather
than a cold re-read. A critical change is not "done" until that external pass has
run; if it is pending, the Orchestrator says so explicitly and the change waits
([fail-closed](#the-twin-guardrails)).

---

## The Flow

```
Owner ── states task ──▶ ORCHESTRATOR ◀── evidence-backed outcome ──┐
                              │  (dispatches via templates)         │
              ┌───────────────┤                                     │
  any plan:   ▼               ▼                                     │
  refine, then refute the Planner ──▶ plan (decisions resolved,     │
  design (decorrelated from  │         non-overlap guaranteed)      │
  the refinement dialogue     ▼                                     │
                          Worker(s) ──▶ diff + claims list          │  loop back on
                              │         (each computed / UNVERIFIED)│  any finding,
                              ▼                                     │  bounded by the
                   Adversary (REFUTE, trace-first, kill-scored)    │  cycle cap;
                   [+ optional decorrelated Reviewer lens]          │  else escalate
                              ▼                                     │  to the judge
              Gate = EVIDENCE AUDITOR + deterministic checks ───────┤  (fail-closed)
                              │  evidence complete? tier ok?        │
                              ▼                                     │
              External Check (cross-family / human)  ← critical only
                              ▼                                     │
                         ORCHESTRATOR ──▶ Owner ────────────────────┘
```

Every arrow returns to the Orchestrator, which decides the next hop.

---

## The evidence obligations (the heart)

These are **mandatory artifacts**, enforced by the Gate. They are what actually
catches mechanism-level defects, and they are cheap — prose and arithmetic, no
automation required.

- **Trace the real failure, from scratch.** For a bug fix, someone (the Adversary)
  must take the documented or most-likely failure and simulate it against the new
  code line by line, showing intermediate state — **before** reading the change's
  own comments, commit message, or narrative, then compare. This defeats
  framing-inheritance directly: the framing lives *in the artifact* (comments,
  messages), so the only framing-free input is a trace you built yourself.
- **Reproduce, don't reason.** Every quantitative or behavioral claim ("~2/sec",
  "the back-off advances", "O(1)", "idempotent") is either accompanied by an
  independent computation/hand-trace **or** tagged `UNVERIFIED`. An untagged
  unverified claim is a **Blocker** at critical tier. The cost is small; the claims
  that get skipped under a vague "verify everything" norm are exactly where subtle
  defects hide, so the obligation is *narrow and loud* rather than broad and soft.
- **An analogy is a claim, not evidence.** "This mirrors blessed pattern X" triggers
  *more* scrutiny, not less: a required **point-of-difference analysis** showing the
  analogy holds at the exact spot that matters. Near-identical patterns hide their
  bugs precisely where they differ by one detail.
- **Sweep the pattern class by *mechanical enumeration*, never by agent judgment.**
  When a change establishes a new rule ("reset only after the connection has *held*";
  "encode this value before it enters a path"), the same class must be swept across
  every sibling *before closing*. A sweep that trusts an agent to *find* the sites is
  the single most common way a whole sub-class ships while the sweep reports
  "complete" — so three forcing functions, not a reminder to be thorough:
  1. **The candidate set comes from a mechanical enumeration** (grep / AST / type
     search) the Orchestrator runs and hands over as a *fixed, complete list*. The
     agent **classifies rows; it does not decide the set.** Letting the reviewer
     enumerate is exactly how the missed site stays invisible — it was never on
     anyone's list to reject in the first place.
  2. **Every row gets an explicit disposition — *fixed* or *excluded* — with no
     silent drops.** An excluded row is a *visible claim*, not an absence.
  3. **Every exclusion carries a *source-trace*, not a name-based reason, and an
     untraced exclusion is a Blocker** — same force as an untagged unverified number.
     "It's an id, those are safe" is reasoning from the *name*; trace the value to its
     origin and the safe-list is often wrong (a value the system merely **stores** from
     user input and echoes back is safe only as far as its write-time validation
     reaches — frequently create-only, grandfathering old rows). The evidence
     obligations above apply to the safe-list with the **same** force as to the fix.

  The enumeration key must be the class's **defining property, not a proxy that correlates
  with it today.** If the class is behavioral ("optimistically reports success and swallows
  the confirmation"), enumerating by a stand-in — a URL, a symbol name, a file — sweeps only
  the members that share the stand-in and silently drops the rest: the grep passes, the
  class does not. Name the signature, enumerate by it, and **record the signature, not just
  the member list**, so the next sweep greps the pattern instead of re-running a path match
  that never described the class.

  A wrong safe-list is the sweep's most common *silent* failure — it reads as
  complete. Its deepest form is a **mis-framed sink/source class** (the enumeration
  searched for the wrong thing), invisible from inside because every agent inherited
  the frame; the only reliable catch is an **independent re-derivation of scope, done
  blind to the triage** — one agent, given only the raw finding and the code, defines
  the threat's source/sink classes *itself* and enumerates from scratch, then its set
  is diffed against the triage and every delta adjudicated. Mandatory on critical
  work; it is the internal approximation of what the External Check provides for free.
- **State new assumptions.** List what the change *newly relies on* ("depends on
  field Y being write-once", "assumes the upstream close is graceful"), each with
  the evidence that it holds. Load-bearing assumptions left implicit are how a fix
  breaks silently later.
- **Trace the artifact's lifetime, not just the diff.** A fix is reviewed against the
  finding it closes — but when a change *introduces a durable or stateful artifact* (a
  persistent UI element, a timer, a retained credential, a background loop, a cache, an
  event listener), that artifact has a life *beyond the moment it is created*, and the
  defect is usually there: it outlives, or collides with, the state it silently assumed.
  Trace it across the *whole* state machine — what becomes of it when the session ends, a
  new one begins, the user navigates away, the tab reloads, or the token / cookie / lock
  it holds is rotated or revoked? "Does the fix close the finding?" and "what does the
  fix's own artifact do for the rest of its life?" are different questions; the
  diff-focused reviewer only asks the first. *(Abstracted from real work: a signout
  retry-loop wrapping a credential-bearing retry action outlived the signed-out session it
  belonged to and silently rebound to the next session — missed by four internal roles,
  each of whom reviewed the diff, not the lifetime.)*

---

## Decorrelation — a supporting hedge, honestly scoped

Reviewer diversity reduces correlated misses, but only some axes are real, and this
model is explicit about which:

| Axis | Real? | Notes |
|---|---|---|
| **Stance** (verify vs. **refute**) | **Strong** | The Adversary's refutation stance surfaces defects verification never will. Keep it. |
| **Evidence obligation** (read vs. **execute/compute**) | **Strongest** | Not "diversity" at all — it is the primary control above. One agent reads; one must produce executed/computed truth. |
| **Detection target** (lens) | Moderate | Different targets find different bugs, but differently-targeted readers still share deep model biases. |
| **Input slice** (withholding the maker's reasoning) | **Weak / often illusory** | In codebases with rationale-dense comments, commit messages, and repo access, the maker's framing travels *in-band*; a "diff-only" reviewer reads it anyway. Replace input-withholding with the **trace-before-reading** sequencing obligation, which is enforceable. |
| **Model family** | Strong, but external only | The strongest true decorrelator, but within a single harness every spawned agent is usually one family — so family diversity exists in practice only via the **External Check**. Do not credit within-family "family diversity." |

**Anti-sycophancy (mandatory):** reviewers run with no shared history and never see
each other's conclusions; sequentially-revealed opinions manufacture false
consensus. The Orchestrator collects verdicts independently and never folds one
reviewer's finding into another's prompt. **Weight disagreement** — investigate a
lone refutation, do not average it away.

---

## Plan-stage review: refine, then refute

Wherever a plan exists, it is reviewed **before** a worker builds against it. A wrong
*approach* is the most expensive defect there is — a full implementation cycle spent
on the wrong shape — so the cheapest place to catch it is while the plan is still
prose. This fires at **every tier that has a plan** (all of Standard and Critical),
not at critical alone; trivial work has no plan to review and opts in only by
[ratcheting up](#risk-tiers-scale-to-blast-radius).

Two distinct activities live here, and conflating them silently reintroduces the very
correlation the model fights elsewhere:

- **Refinement** — collaborative improvement of the plan (completeness, ordering,
  trade-offs). Shared history is fine; the goal is a better plan, and iteration is how
  it gets one.
- **Refutation** — an independent, kill-scored attempt to break the *finalized* plan,
  run by an agent **decorrelated from the refinement dialogue** (no shared history with
  it). This is the Adversary's plan-stage pass.

The trap is running only the first and calling it review. A planner and reviewer who
iterate together to consensus have *converged*, not *verified* — they can agree
because they now share a blind spot, the exact false consensus
[Decorrelation](#decorrelation-a-supporting-hedge-honestly-scoped) warns against.
Refine collaboratively if it helps, but the **refutation of the finalized plan must
stay independent**, or the step feels like a control while buying none — the same
separation of duties the model enforces on code (the maker never grades the checker),
applied one stage earlier, to the design.

---

## Competitive generation: decorrelating the maker

The model decorrelates *checkers* (multiple lenses, a refutation stance) but by
default runs a single *maker*. Competitive generation decorrelates the maker too:
**two agents, blind to each other, produce the artifact independently, and a judge
selects or synthesizes.** Two blind attempts vary on *approach*, not merely on
reading — signal one maker cannot give. The judge is an existing frontier reviewer,
**not a new standing role**; a generalist "chooser" bolted beside the Adversary is the
redundant reader the model warns against.

Where it pays and where it does not:

- **Plan stage — strong.** The approach space is widest at design time, a wrong
  approach is the costliest defect, and plans are cheap prose. **Default it on for
  critical and large-decomposable work:** two independent plans, judged.
- **Implementation of small tasks — usually net-negative.** Two diffs mean two
  evidence passes, doubling load on the model's most expensive stage
  ([Economics](#model-economics-spend-intelligence-where-judgment-lives)) to buy
  variance you may not need. **Reach for it only when the *approach itself* is
  genuinely contested**, not as a standing default; one capable worker plus a real
  evidence pass beats two workers plus a judge on most small tasks.

Two rules keep it from *laundering* confidence instead of earning it:

1. **A synthesis is a new artifact.** "Best of both" splices two designs across a seam
   neither maker built for, and can carry a defect neither original had. A merged
   result inherits **neither parent's approval** — it runs the full evidence pass from
   scratch.
2. **The judge selects on *survived falsification*, not on which reads better.** Each
   candidate goes through the evidence obligations; the survivor wins. Choosing the
   more *plausible* candidate rebuilds the plausibility-over-mechanism failure the whole
   model exists to defeat.

**Its ceiling.** Two blind makers *of the same model family* do not escape the
[shared-framing floor](#known-limits-intellectual-honesty): if both inherited the same
blind spot, both miss it, and a same-family judge picks the better-looking wrong answer
with *higher* confidence than one wrong answer would have earned. Competitive
generation hedges **approach-variance, not framing** — it does **not** reduce the need
for the External Check on critical work.

**Convergence is not corroboration.** Two generators agreeing is *not* evidence the answer
is right — and it is worth least exactly when it feels like most. If a constraint *forces*
the move (once an invariant is accepted, the design space collapses to one shape),
agreement is near-certain and carries no information — the value was the independent
**proof** each produced, which a single generator would have bought just as well. Worse,
same-family generators converge on the same **miss** as readily as the same solution: they
share the blind spot *and* the conclusion, and the agreement then manufactures false
confidence. Credit the proof, never the consensus.

---

## Risk Tiers — scale to blast radius

Do **not** classify by apparent size — a one-line change can carry a critical
defect. Classify by **blast-radius zone**: maintain a standing list of
high-consequence areas (for example: authentication/session, retry/back-off/
reconnect, rate-limiting, caching, access-control, cryptography, data migrations,
money, anything irreversible). A change touching any zone is critical *by default*,
regardless of size.

| Tier | Team | Evidence | External |
|---|---|---|---|
| **Trivial** (comment, doc, verbatim-dictated) | do it; deterministic checks | no loop — but premises still verified against source | no |
| **Standard** | capable Worker + Adversary; plan-stage review (refine + refute) when a plan exists; deterministic checks; Gate | trace + claims-computed + analogy break-points + sweep-if-invariant + lifetime-if-durable-artifact | no |
| **Critical** (any blast-radius zone) | + plan-stage refutation on the design; competitive plans (judged on evidence); optional decorrelated Reviewer lens; Gate | all of Standard, enforced as Blockers | **yes** (cross-family / human), fail-closed |

**Tier is a ratchet:** it can rise mid-task (the Gate or any agent may reclassify
*up* on what it discovers) and never falls. When in doubt, tier up.

---

## The Gate checklist (evidence auditing)

The Gate approves only when **all** required artifacts are present and internally
consistent. Missing evidence is a Blocker, not a nit:

1. **Documented-failure trace** attached (bug fixes) and consistent with the code.
2. **Every quantitative/behavioral claim** computed, or explicitly `UNVERIFIED`
   (untagged-unverified = Blocker at critical).
3. **Every cited analogy** carries a point-of-difference analysis.
4. **Invariant sweep** run from a *mechanical* enumeration (not agent-found), keyed on
   the class's **defining signature, not a proxy** (a URL / symbol / file that merely
   correlates), with **every row dispositioned** and **every exclusion source-traced** (an
   untraced exclusion is a Blocker); the signature (not just the member list) recorded, and
   blind scope re-derivation present on critical work.
5. **New-assumptions list** present, each with its evidence.
6. **Artifact-lifetime analysis** — if the change introduces a *durable or stateful*
   artifact (persistent UI element, timer, retained credential, background loop, cache,
   event listener), its lifecycle is traced across the full state machine (what happens to
   it when the assumed state — session, token, cookie, lock, mount — changes while it is
   still alive). Absent lifetime analysis for such an artifact is a Blocker at critical.
7. **Deterministic checks** green.
8. **Tier respected**, ratchet honored; External Check present if critical.
9. **Persistence updated** — if the change establishes a new invariant or closes a
   tracked item, the Field Guide and any tracking artifacts reflect it. A lesson left
   unwritten is re-learned by re-breaking; leaving this to reviewer diligence is the
   exact reliance the model rejects everywhere else.
10. **Plan-stage review** happened wherever a plan existed — an independent refutation
   of the *finalized* plan, decorrelated from any refinement dialogue; and any
   synthesized/merged artifact from competitive generation ran its **own** full
   evidence pass (it inherits no parent's approval).

An approval that cannot produce these counts as no approval.

---

## Keeping the Adversary honest

An Adversary rewarded for "finding something" will manufacture noise — the mirror of
sycophancy. Constrain it:

- **Score kills, not swings.** A finding exists only if it carries a concrete
  failing input or a replayable trace the Gate can reproduce. Nitpicks that cannot
  produce one do not count.
- **Cap findings per pass** (e.g. top N) to force prioritization.
- **Honor the empty result.** "I attempted A, B, C and all held," with the traces,
  is a first-class deliverable.
- **Feed it primary artifacts, not the Orchestrator's description of the failure** —
  the verbatim error, the log excerpt, the user report, the diff — because the
  Orchestrator's description is exactly where framing-poison enters.
- **Track its precision over time** in the persistent store; the human holds the
  score, since agents are stateless across sessions.

---

## The twin guardrails

Both must hold at once; the model is wrong if it either ships defects **or** burns
tokens spinning.

**Anti-Catch-22 — never loop forever.**
- Cap the fix→review cycle at **two full cycles per defect class** (distinct genuine
  defects each get their own budget; do not let two real bugs exhaust one).
- On exceeding the cap, or on genuinely contradictory guidance, **stop and escalate
  to the Owner as judge.**
- **Escalation is fail-closed by tier:** if the judge is unavailable, critical work
  **does not ship**; trivial work may fail open. Never ship merely to exit a loop.
- The **escalation packet is verbatim** — both reviewers' full outputs, the diff, the
  traces — delivered without passing through the Orchestrator's summary, so the judge
  does not inherit the framing at the moment independence matters most.

**Anti-Over-Engineering — never ceremony for its own sake.**
- Most multi-agent effort is wasted on work that didn't need it; reach for the full
  apparatus only when the tier justifies it.
- **Do not parallelize interdependent work.** One good worker beats three colliding
  ones.
- Trivial and verbatim-dictated edits skip the loop; say so out loud.
- The Orchestrator must be able to state in one line why each spawned agent earns its
  tokens. If it can't, don't spawn it.

Resolution of the tension: **spend on evidence and refutation (high value per token),
not on redundant same-shaped readers (low value per token).**

---

## Model Economics — spend intelligence where judgment lives

Quality plateaus while cost varies widely; frontier intelligence is only needed at a
few moments — decomposition, design decisions, trade-offs, and refutation — after
which an explicit spec can be followed by a cheaper model. But the crossover is
**scale-dependent**, and this model is explicit about it:

- **Large, decomposable tasks:** frontier Planner + cheaper Workers wins big;
  worker-hours dominate and the spec is written once and reused across many workers.
- **Small tasks (a few files):** spec-writing effort approaches implementation
  effort, and the binding costs are *review cycles and escalations*. A cheaper worker
  raises defect inflow into exactly those most-expensive stages. **Default the Worker
  to capable-or-better, and take the savings by dropping the redundant second
  generalist reviewer instead.**
- **Frontier where judgment lives:** Orchestrator, Planner, Adversary, Gate.
- **Measure, don't assume.** Track cycles-per-task and escalation-rate by worker tier
  — the model's own *reproduce-don't-reason* value demands the model itself be tuned
  on data, not vibes.
- **Separation of duties:** the maker never grades the checker. The Adversary and Gate
  can overrule the Worker; a capable checker over a cheaper maker also yields
  debate-level quality at lower cost.

---

## Structural mitigations for the Orchestrator single-point-of-failure

Dispositional rules ("do not lead the witness") decay across stateless sessions.
These are structural instead:

- **Checked-in prompt templates.** The Orchestrator fills only artifact slots (file
  paths, verbatim error strings) and writes **no free prose** into a reviewer's
  prompt. The sent prompt is auditable by diffing it against the template.
- **Adversary fed primary artifacts only** (verbatim error, log, report, diff) — never
  the Orchestrator's narrative of the failure.
- **Gate authority to ratchet the tier up**, independent of the Orchestrator's initial
  classification.
- **Verbatim escalation packet** (above) — the judge sees primary artifacts, not the
  correlation source's summary.

---

## The Field Guide (persistence)

Agents are stateless across sessions and their weights are frozen; a lesson not
written down is re-learned by re-breaking. So every non-obvious, hard-won lesson is
written **where the next agent will automatically load it** — the project's
persistent, auto-injected instruction store (see
[Adoption](#adoption-instantiating-this-in-a-project)) — not into a standalone
document nothing loads. Capturing a surprise once converts a repeated miss into a
standing check. **Updating the Field Guide is part of "done," not an afterthought** —
and this obligation applies to *this model too*: a process that is not wired to load
will not govern a session.

---

## The sub-agent contract (every dispatch)

A sub-agent drifts if any of these is missing; the Orchestrator supplies all four,
every time:

1. **Objective** — the single outcome this agent owns.
2. **Output format** — exactly what to return (a verdict, a trace, a diff, a claims
   list, a structured finding).
3. **Inputs & tools** — which artifacts/tools to use; for the Adversary, primary
   artifacts only.
4. **Boundaries** — what is out of scope and which constraints must not be violated.

---

## Adoption (instantiating this in a project)

The model is agnostic; a project binds these slots once:

- **Deterministic checks** → the project's lint/type/contract/CI commands the Gate
  runs.
- **Gate agent** → the project's designated standalone review agent, if any.
- **Field Guide target** → the project's auto-loaded instruction file(s) where
  lessons and the blast-radius zone list live — and a pointer to *this* model from
  that same store, so it actually loads.
- **Blast-radius zones** → the project's standing list of high-consequence areas.
- **Prompt templates** → checked-in, versioned with the project.

---

## How this model evolves

This is a live process, exercised on real work. When real work exposes a gap — a
defect that slips the Gate, a role that adds no value, a guardrail that mis-fires —
the fix is folded back here, and the corresponding standing check is added to the
Field Guide. The model is never "finished"; it is *hardened*. Treat it the way it
treats a change: when it establishes a new rule, sweep the implications; when it
claims an improvement, show the evidence.

---

## Known limits (intellectual honesty)

- **Multi-agent only wins on decomposable work.** Tightly interdependent tasks often
  don't decompose; a single strong agent with real evidence obligations beats a
  swarm. Don't force parallelism onto coupling.
- **Internal decorrelation has a floor — a single-family loop cannot certify its own
  critical work.** Every internal agent descends from the Orchestrator's context and
  usually one model family, so a *shared framing error* (a wrong safe-list, a
  mis-scoped sweep) is invisible from inside: every agent inherited it, and they agree
  *because* they inherited it. A single-family loop's "no gaps found" must therefore
  **never** be read as "no gaps." The prompt raises the *odds* of catching a defect;
  the *reliance* on critical tickets comes only from outside the prompt — deterministic
  checks (which cannot inherit a frame) and the cross-family External Check (which does
  not share one). That is why both are mandated, not optional, at the top tier — and
  why the honest instruction to an owner is: for a critical ticket, do not trust this
  process alone.
- **This model is a means, not a ritual.** If a step is not buying correctness on
  *this* task, the anti-over-engineering guardrail says drop it and say why.
