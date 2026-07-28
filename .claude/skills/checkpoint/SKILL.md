---
name: checkpoint
description: Write a high-fidelity handoff of the current task state to a scratchpad file BEFORE the user compacts the conversation, so no task-critical detail is lost to the lossy auto-summary. Invoke when the user says the conversation has grown, worries your performance is dropping, wants to "save where we are" / "checkpoint" / "not forget the task" before compacting and continuing. Produces a transient CHECKPOINT.md the post-compaction session re-reads — it does NOT write to the persistent memory/ system.
---

# Checkpoint before compaction

The user is about to compact this conversation and wants the current task state preserved at full fidelity. Compaction already auto-summarizes, but that summary is lossy and silently drops task-critical detail (the exact file:line being edited, a decision made long ago and its rationale, a gotcha just discovered, the precise next step). Your job is to write a durable, structured handoff that the post-compaction session re-reads instead of trusting the summary.

## Scope

- **This is a TRANSIENT, task-scoped checkpoint.** Write it to the session scratchpad, NOT to the persistent `memory/` system. Do not add MEMORY.md lines or frontmatter memory files — that system is for durable cross-conversation facts, and task-state pollutes it.
- If, while writing the checkpoint, you notice a genuinely durable fact (a lasting user preference, a project invariant not derivable from code) worth persisting forever, mention it to the user in one line and let THEM decide — do not write it to `memory/` as part of this skill.

## What to do

1. **Locate the scratchpad directory** — it is named in your system prompt under "Scratchpad Directory" (a `.../scratchpad` path). Write the checkpoint to `<scratchpad>/CHECKPOINT.md`. If a CHECKPOINT.md already exists there, overwrite it (the latest state is what matters).

2. **Reconstruct the task state from the whole conversation**, not just the last few messages. Walk back through what was actually done, decided, and discovered. Be concrete: real file paths, real `file:line` anchors, real symbol names, real command invocations. Vague handoffs ("was working on the auth stuff") defeat the purpose.

3. **Write `CHECKPOINT.md` with exactly this structure** (omit a section only if it is genuinely empty):

```markdown
# Checkpoint — <one-line task title>

_Written before compaction. Read this in full before continuing._

## Goal
The overall objective in 1–3 sentences — what "done" looks like.

## Done
- Concrete completed steps, each with the file(s)/symbol(s) touched.

## In progress
- The exact thing being worked on right now, with `path:line` anchors and
what state it is in (compiles? tested? half-edited?).

## Next steps
1. Ordered, specific actions to resume. First item = the very next keystroke-level move.

## Key decisions & rationale
- Decision → why it was chosen over the alternative. Capture the "why" the
summary will drop.

## Open questions
- Unresolved forks, things waiting on the user, ambiguities.

## Gotchas & constraints
- Non-obvious facts discovered this session (a failing edge case, a lint rule,
a cross-service contract, an AWS region guard, a "don't touch X" from CLAUDE.md).

## Mental model
- The 2–5 sentences of context that make the above make sense — how the pieces
fit, what you now understand that you didn't at the start.
```

4. **After writing, tell the user (in your reply, not just the file):**
- The absolute path to `CHECKPOINT.md`.
- This exact instruction to give the fresh session: **"Read `<path>/CHECKPOINT.md` in full, then continue the task."** Explain that they should paste this as their first message after compacting, because the post-compaction session won't otherwise know the file exists.
- Any durable-fact suggestion from the Scope note above, if applicable.

## Notes

- Do not start doing the actual task work here — this skill only captures state. The user will resume the work after compacting.
- Keep the checkpoint honest: if something is broken or uncertain, say so plainly. A handoff that overstates progress is worse than none.