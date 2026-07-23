---
name: duw-git-workflow
description: Git, commit, PR, force-push, and merge conventions for the DUW Queue Monitor project. Use this skill whenever performing git operations (branching, committing, opening or merging PRs, addressing review comments, rewriting history) against the `uladzk/duw-queue-monitor` repo. Required reading for all agents dispatched to work on this project.
allowed-tools: bash, gh, git
---

# DUW Git Workflow

Authoritative conventions for git, commits, pull requests, and merges in the DUW Queue Monitor repository. Follow exactly — these are the user's established preferences and have been corrected through real-world iteration.

**Repo:** `uladzk/duw-queue-monitor` (lowercase username after rename from `UladzK`).

---

## 1. Branching

**Never commit directly to `main`.** Every change goes through a feature branch + PR.

**Branch naming:**

- With Linear/GitHub ticket: `duw-{ticket-number}-{kebab-slug}` — e.g., `duw-92-phase-3-ovh-platform-shared`, `duw-73-use-sa-token-in-k8s-provider`.
- Without ticket: `{type}-{kebab-slug}` — e.g., `feat-earlier-working-hour-start`, `refactor-working-hours-to-config`, `fix-cronjob-suspend-default`.
- Multi-phase work: include phase number — e.g., `duw-92-phase-3-fix-tfstate-bucket-policy`.

**Mandatory steps before creating a branch:**

```bash
git checkout main
git pull --ff-only
git checkout -b {branch-name}
```

Check `git branch -r` if unsure about naming style — match existing conventions, do not invent new ones.

---

## 2. Commit conventions

**Format:** `{type}({scope}): {imperative description}`

### Types

| Type | Use for |
|---|---|
| `feat` | New user-visible feature or behavior |
| `fix` | Bug fix |
| `refactor` | Code rearrangement, no functional change |
| `ops` | **Infrastructure provisioning, new IaC resources, new scripts, deployment changes.** Use this — not `feat` — when adding new Terraform modules, k8s manifests, CI workflows, etc. |
| `build` | Build system, Dockerfile, CI/CD pipeline changes |
| `docs` | Documentation, README, CLAUDE.md updates |
| `test` | Adding or modifying tests |
| `chore` | Maintenance, dependency bumps |
| `perf` | Performance improvements |

### Scope

**Be specific. Use the module name, not the parent directory.**

- ✅ `ops(ovh-platform-shared): add terraform module for tf state bucket`
- ✅ `ops(queue-monitor): bump image to 1.4.0 in dev overlay`
- ✅ `fix(ovh-platform-shared): use explicit s3 actions in policy instead of wildcard`
- ❌ `ops(infra): add terraform module for tf state bucket` — too generic, harder to grep history by module

### Subject line rules

- Imperative mood: "add" not "added", "fix" not "fixes"
- No trailing period
- ≤ ~72 chars for the subject line
- Lowercase except for proper nouns (PR titles can be normal-case if longer)

### Body rules (when present)

- Optional. Use for the "why" if non-obvious — never the "what" (the diff shows that).
- Wrap at ~72 chars.
- Empty line between subject and body.

### Absolute prohibitions

- ❌ **NEVER** include `Co-Authored-By:` lines.
- ❌ **NEVER** include any Claude / AI-tool authorship attribution (`🤖 Generated with...`, `Generated-By:`, etc.).
- ❌ **NEVER** prefix `[DUW-N]` to commit messages. (For GRAFX/CHILI repos a hook adds the prefix; DUW has no such hook and manual prefixing pollutes the log.)

### Splitting commits

Prefer logical splits when a single PR touches multiple concerns:

```
ops(ovh-platform-shared): add terraform module for tf state bucket
ops(ovh-platform-shared): add provision-ovh.sh wrapper script
docs(ovh-platform-shared): document module and provision-ovh.sh in CLAUDE.md
```

Each commit should pass `terraform validate` / `go build` / tests independently. If unsure, one bundled commit is fine — but never a "WIP" / "fix things" commit.

---

## 3. Pull request creation

**Always create as `--draft` first.** User flips to ready when satisfied.

### Title

Use the same conventional-commit subject style as the primary commit. Max ~70 chars; details go in the body.

- ✅ `feat(infra): add OVH platform-shared terraform module + provision-ovh.sh`
- ✅ `fix(ovh-platform-shared): grant tf user s3 access to state bucket`
- ❌ `Add CA certificates to queue-monitor Docker image and update the readme` (too long, not conventional-commit style)

### Body — the canonical template

Three sections in this exact order, no others:

```markdown
## Changes

- {Bullet point of what changed}
- {Another change}

## Problem

{Describe the problem this PR solves — 1–3 sentences}

## Solution

{Describe how the changes solve the problem — 1–3 sentences}
```

- **Changes** is a bulleted list of concrete diffs (added module X, renamed Y, pinned Z).
- **Problem** explains the motivation — what was broken, missing, or non-obvious.
- **Solution** explains the approach — *why* this fix vs alternatives, key trade-offs.

Reference Linear/DUW tickets at the end of Solution when applicable: "Part of DUW-92, Phase 3."

### Example (good)

```markdown
## Changes

- Replace `Action = ["s3:*"]` with the explicit action set Terraform's s3 backend needs (Get/Put/Delete object, ListBucket, GetBucketLocation, plus the multipart-upload trio).
- Scope unchanged: still bound to `arn:aws:s3:::duw-tfstate-shared` and `/*`.

## Problem

PR #74's policy applied verbatim with `Action = ["s3:*"]` (verified via `terraform state show`) but `aws s3 ls` against the bucket still returned `AccessDenied`. OVH's S3 policy engine does not expand the `s3:*` wildcard — the user evaluated to zero allowed actions even with the policy attached.

## Solution

Enumerate the explicit actions that `terraform init -migrate-state` and routine plan/apply against the s3 backend require. Matches OVH's own provider docs example, which never uses `s3:*`. Part of DUW-92, Phase 3 follow-up.
```

### What does NOT belong in PR body

- ❌ `## Testing` / `## Test plan` checklists — unless there's a genuine manual verification list a reviewer must follow before merge
- ❌ Step-by-step bootstrap / operational instructions (e.g., "now run terraform apply with these flags") — those are communicated agent→Claude→user in chat or in a comment after merge, never in the PR body itself
- ❌ `## Summary` heading (or any heading other than Changes/Problem/Solution)
- ❌ `Fixes #N` shortcuts — keep ticket references inline in Solution, not as auto-close keywords (the user wants explicit transition control over Linear)
- ❌ Any Claude / AI-tool footer (`🤖 Generated with...`, `Co-Authored-By: Claude...`)
- ❌ Checkboxes (`- [x] ...`)
- ❌ Verbose template scaffolding the PR doesn't need — if a section truly has nothing to say, write one short sentence rather than padding

### Referencing tickets

Reference DUW-N in the **body**, not the commit message. Format: "Part of DUW-92, Phase 3 follow-up." Linear sometimes auto-transitions tickets on merge; check after merging if the ticket should remain in-progress.

### Create command

```bash
gh pr create --draft --title "{TITLE}" --body "$(cat <<'EOF'
## Summary

- {bullet 1}
- {bullet 2}
- {bullet 3}
EOF
)"
```

---

## 4. Force-pushing & history rewrites

Rewriting your **own** unmerged feature branch is allowed and sometimes necessary (e.g., when commit messages turned out wrong). Use `--force-with-lease`, never plain `--force`.

```bash
git push --force-with-lease origin {branch-name}
```

`--force-with-lease` aborts if anyone else pushed to the branch since you last fetched — protects against clobbering a collaborator's work.

**To rewrite commit history without `rebase -i`** (the `-i` flag is not supported in agent contexts):

```bash
# 1. Soft reset to base, preserving all working-tree changes staged
git reset --soft main

# 2. Unstage so you can re-stage selectively per commit
git reset

# 3. Create fresh commits with correct messages and scopes
git add {files for commit 1}
git commit -m "{type}({scope}): {description}"
# repeat...

# 4. Force-push with lease
git push --force-with-lease origin {branch-name}
```

**Never** force-push to `main`, ever. Even with `--force-with-lease`.

---

## 5. Merging

**NEVER squash-merge.** The user wants individual commits preserved on `main` — this enables `git log -- {file}` to surface meaningful history per change, and makes `git blame` produce useful results.

```bash
# Mark draft → ready (no-op if already ready)
gh pr ready {N}

# Merge with explicit merge-commit + branch deletion
gh pr merge {N} --merge --delete-branch
```

**Both flags are required:**

- `--merge`: explicit merge-commit strategy. Without this, `gh` errors with "merge, rebase, or squash required".
- `--delete-branch`: cleans up the feature branch on the remote after merge.

After merge, pull main locally:

```bash
git checkout main
git pull --ff-only
```

If the user says "merge" without specifying, default to draft→ready→merge without asking again.

---

## 6. Review comments

Always address every comment before merging. For each comment, choose **fix / acknowledge / discard**:

### Fix (the comment is valid and requires changes)

```bash
# 1. Edit code, commit, push to the same branch
git add {files}
git commit -m "{type}({scope}): {description}"
git push

# 2. Reply
gh api repos/uladzk/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
  -f body="Fixed — {brief description of what changed}." \
  -F in_reply_to={COMMENT-ID}
```

### Acknowledge (valid but out of scope or deferred)

```bash
gh api repos/uladzk/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
  -f body="Good point. Tracked in {ticket/issue} for follow-up." \
  -F in_reply_to={COMMENT-ID}
```

### Discard (not applicable)

```bash
gh api repos/uladzk/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
  -f body="Not applicable — {brief reasoning}." \
  -F in_reply_to={COMMENT-ID}
```

**Approving a PR (when reviewing as separate identity):** never include text in the approval body itself. The user has explicitly said "do not include any message in the approval request — if needed, this will be made as a separate comment to a pull request and will be asked by user."

---

## 7. Agent-specific reminders

When dispatched as a subagent for a phase / task:

1. **Use the `superpowers:verification-before-completion` skill** — every PR opened must report literal command output (PR URL, `git diff --stat`, validation output), not assumptions.
2. **Open PRs as `--draft`.** The user converts to ready after review.
3. **Read recent `git log --oneline -10` before making your first commit** to confirm the style hasn't drifted since this skill was last updated.
4. **Stop and report back, don't push, if** any of: pre-existing changes in working tree you didn't make; assumed resource/attribute names don't validate; commit message convention is unclear for the change at hand.
5. **`infra/scripts/provision.sh` and `provision-ovh.sh` are tracked as `100644` (not executable)** — invoke as `bash ./infra/scripts/provision-ovh.sh ...`, do not `chmod +x` them.

---

## 8. Quick reference card

```
Branch:    git checkout main && git pull --ff-only && git checkout -b duw-{N}-{slug}
Commit:    {type}({module-scope}): {imperative description}    # types: feat fix refactor ops build docs test chore perf
PR open:   gh pr create --draft --title "..." --body "## Summary\n\n- ...\n- ...\n- ..."
PR merge:  gh pr ready {N} ; gh pr merge {N} --merge --delete-branch
Rewrite:   git reset --soft {base} && git reset && {re-commit} && git push --force-with-lease
After merge: git checkout main && git pull --ff-only
```

---

## 9. Anti-patterns — never do these

- ❌ Commit directly to `main`
- ❌ `gh pr merge --squash` (squash-merge)
- ❌ `git push --force` (without `--with-lease`)
- ❌ `git rebase -i` (interactive flag is unsupported)
- ❌ `Co-Authored-By:` or Claude attribution in commits
- ❌ Bootstrap / operational instructions in PR body (those go in chat communication)
- ❌ Generic `infra` scope on commits that are module-specific
- ❌ `feat` for new infrastructure resources (use `ops`)
- ❌ Approving PRs with text in the approval body
- ❌ Prefixing `[DUW-N]` to commit messages

---
**END OF SKILL**
