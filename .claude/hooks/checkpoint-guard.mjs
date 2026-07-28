#!/usr/bin/env node
// Checkpoint guard for compaction. Wired to PreCompact + PostCompact in
// .claude/settings.json. See .claude/skills/checkpoint/SKILL.md.
//
// Why a script and not a jq one-liner: jq is not installed in this
// environment (verified), and the scratchpad path has to be discovered by
// session_id rather than hardcoded.
//
// What each mode does, and what it deliberately does NOT do:
//
//   pre   Runs just before the conversation is compacted. A hook is a shell
//         command, not a model turn, so it CANNOT author CHECKPOINT.md itself
//         — only Claude can do that, by invoking the `checkpoint` skill. So
//         `pre` does the two things a shell process actually can: warn the
//         user when no checkpoint exists, and inject the checkpoint skill's
//         section headings into the compaction so the lossy auto-summary at
//         least preserves those dimensions.
//
//   post  Runs after compaction, when the fresh context has no idea a
//         CHECKPOINT.md exists. Injects its absolute path with an
//         instruction to read it. This automates step 4 of the skill, which
//         otherwise requires the user to hand-paste the path every time.
//
// Usage: node checkpoint-guard.mjs (pre|post)   [hook JSON on stdin]

import { existsSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

const mode = process.argv[2];

/** Read all of stdin; return {} rather than throwing on empty/garbage. */
async function readInput() {
  try {
    const chunks = [];
    for await (const c of process.stdin) chunks.push(c);
    const raw = Buffer.concat(chunks).toString("utf8").trim();
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

/**
 * Locate this session's scratchpad CHECKPOINT.md.
 *
 * Scratchpads live at <tmp>/claude/<sanitized-cwd>/<session_id>/scratchpad.
 * We glob for <session_id> instead of re-deriving the cwd sanitization, since
 * the session id is unique and the sanitization rule is not ours to own.
 * Fallback stays inside the SAME project directory so a checkpoint from an
 * unrelated project can never be picked up.
 */
function findCheckpoint(sessionId) {
  const root = join(tmpdir(), "claude");
  if (!existsSync(root)) return null;

  let projects;
  try {
    projects = readdirSync(root, { withFileTypes: true })
      .filter((d) => d.isDirectory())
      .map((d) => join(root, d.name));
  } catch {
    return null;
  }

  // Primary: exact session match.
  if (sessionId) {
    for (const p of projects) {
      const cp = join(p, sessionId, "scratchpad", "CHECKPOINT.md");
      if (existsSync(cp)) return cp;
      // Session dir exists but no checkpoint in it -> fall back within this
      // project only (a checkpoint written before a session-id change).
      if (existsSync(join(p, sessionId))) return newestIn(p);
    }
  }
  return null;
}

/** Newest CHECKPOINT.md across the session dirs of one project. */
function newestIn(projectDir) {
  let best = null;
  let bestTime = 0;
  let sessions;
  try {
    sessions = readdirSync(projectDir, { withFileTypes: true }).filter((d) => d.isDirectory());
  } catch {
    return null;
  }
  for (const s of sessions) {
    const cp = join(projectDir, s.name, "scratchpad", "CHECKPOINT.md");
    try {
      const t = statSync(cp).mtimeMs;
      if (t > bestTime) {
        bestTime = t;
        best = cp;
      }
    } catch {
      /* no checkpoint in this session dir */
    }
  }
  return best;
}

function emit(obj) {
  process.stdout.write(JSON.stringify(obj));
}

const input = await readInput();
const checkpoint = findCheckpoint(input.session_id);
const trigger = input.trigger ? ` (${input.trigger})` : "";

if (mode === "post") {
  if (!checkpoint) process.exit(0); // nothing to point at; stay silent
  emit({
    systemMessage: `Checkpoint found — pointing the fresh context at ${checkpoint}`,
    hookSpecificOutput: {
      hookEventName: "PostCompact",
      additionalContext:
        `A checkpoint file written before this compaction exists at:\n${checkpoint}\n\n` +
        `Read it IN FULL with the Read tool before doing anything else, then ` +
        `continue the task from its "Next steps" section. It is higher fidelity ` +
        `than the compaction summary and takes precedence where the two disagree.`,
    },
  });
  process.exit(0);
}

// mode === "pre"
if (checkpoint) {
  emit({
    systemMessage: `Compacting${trigger} — checkpoint present at ${checkpoint}`,
    hookSpecificOutput: {
      hookEventName: "PreCompact",
      additionalContext:
        `A high-fidelity checkpoint for this task exists at ${checkpoint}. ` +
        `It survives compaction; prefer it over re-deriving task state.`,
    },
  });
} else {
  emit({
    systemMessage:
      `Compacting${trigger} with NO checkpoint written. Task state will only ` +
      `survive as the lossy auto-summary. To capture it at full fidelity, run ` +
      `/checkpoint BEFORE /compact next time.`,
    hookSpecificOutput: {
      hookEventName: "PreCompact",
      additionalContext:
        `No CHECKPOINT.md was written for this session, so this summary is the ` +
        `only record of task state. Per .claude/skills/checkpoint/SKILL.md, ` +
        `preserve these dimensions concretely (real paths, file:line anchors, ` +
        `symbol names, exact commands) rather than in the abstract:\n` +
        `  - Goal (what "done" looks like)\n` +
        `  - Done (completed steps + files/symbols touched)\n` +
        `  - In progress (exact path:line, and whether it compiles / is tested)\n` +
        `  - Next steps (ordered; first item = the very next move)\n` +
        `  - Key decisions & rationale (the "why", not just the what)\n` +
        `  - Open questions (waiting on the user, unresolved forks)\n` +
        `  - Gotchas & constraints (non-obvious findings, "don't touch X")\n` +
        `Do not overstate progress: if something is broken or unverified, say so.`,
    },
  });
}
