---
name: duw-release-workflow
description: Complete Git workflow for production releases - creates deployment branch, commits changes, creates PR, and GitHub release. Use with duw-deploy skill for full production deployment.
---

# DUW Release Workflow

## Overview
This workflow handles the complete Git and GitHub workflow for production releases in the DUW Kolejka Checker project. It creates a deployment branch, commits deployment file changes, creates a Pull Request, and creates a GitHub release after successful deployment.

**Use this skill in combination with `duw-deploy` for production deployments.**

---

## Prerequisites
- Git repository with write access
- GitHub CLI (`gh`) installed and authenticated
- Service name and version to release
- Deployment file changes ready (or will be made during workflow)
- Successful deployment in dev environment (optional but recommended)

---

## Important Note
**This workflow assumes:**
- You are deploying to PRODUCTION
- The new version has been tested in dev environment
- You have the service name and version number

**If ANY information is missing, STOP and ASK USER:**
- Service name (queue-monitor, telegram-bot)
- Version to release (e.g., 1.1.2)
- Brief description of what changed

---

## Available Services
- **queue-monitor**: Queue monitoring service
- **telegram-bot**: Telegram notification bot

---

## Deployment Files Reference

| Service | Environment | File Path |
|---------|-------------|-----------|
| queue-monitor | prd | `infra/k8s/queue-monitor-deployment.yml` |
| telegram-bot | prd | `infra/k8s/telegram-bot-deployment.yml` |

---

## Workflow Steps

### Step 0: Gather Context

**ACTION**: Verify all required information is available.

**Required Information**:
- Service name (queue-monitor, telegram-bot)
- Version to release (e.g., 1.1.2)
- Current production version (will read from file)
- Brief description of changes/fixes

**If ANY information is missing, STOP and ASK USER**:
```
To proceed with the release workflow, I need:
- Service name: {queue-monitor, telegram-bot}
- Version to release: {e.g., 1.1.2}
- Brief description of changes

Please provide the missing information.
```

---

### Step 1: Verify Current Versions

**ACTION**: Check current version in dev and production deployment files.

**Commands**:
```bash
# Read dev deployment file
# Use Read tool: infra/k8s/queue-monitor-deployment-dev.yml

# Read production deployment file
# Use Read tool: infra/k8s/queue-monitor-deployment.yml
```

**Look for image line**:
```yaml
- image: acrduwshared.azurecr.io/queue-monitor:{version}
```

**Present to User**:
```
Version Check:
- Dev version: {dev-version}
- Production version: {prd-version}
- Target version: {new-version}

Confirm:
- Deploying: {service} v{new-version} to production
- Version tested in dev: {yes/no}

Is this correct? Please confirm to proceed.
```

**Wait for user confirmation.**

---

### Step 2: Create Deployment Branch

**ACTION**: Create a feature branch for the deployment changes.

**Branch Naming Convention**: `deploy/{service}-{version}-to-prd`

**Command**:
```bash
git checkout -b deploy/queue-monitor-1.1.2-to-prd
```

**Examples**:
```bash
# For queue-monitor v1.1.2
git checkout -b deploy/queue-monitor-1.1.2-to-prd

# For telegram-bot v2.0.0
git checkout -b deploy/telegram-bot-2.0.0-to-prd
```

**Expected Output**:
```
Switched to a new branch 'deploy/queue-monitor-1.1.2-to-prd'
```

---

### Step 3: Update Production Deployment File

**ACTION**: Update the production deployment YAML file with the new version.

**Use Edit tool** to replace the image line.

**File Paths**:
- queue-monitor: `infra/k8s/queue-monitor-deployment.yml`
- telegram-bot: `infra/k8s/telegram-bot-deployment.yml`

**Example Edit**:
```yaml
# Before (line 20)
- image: acrduwshared.azurecr.io/queue-monitor:1.0.0

# After
- image: acrduwshared.azurecr.io/queue-monitor:1.1.2
```

**Verify**: The Edit tool will confirm the change was applied.

---

### Step 4: Gather Changelog Information

**ACTION**: Collect information about what changed in this version.

**Commands**:
```bash
# Check recent releases
gh release list --limit 10

# Check recent tags for the service
git tag --list "queue-monitor-*" | tail -10

# Get commits between versions
git log --oneline queue-monitor-1.0.1..queue-monitor-1.1.2 --no-merges
```

**Example Output**:
```bash
# Tags
queue-monitor-1.0.0
queue-monitor-1.0.1
queue-monitor-1.1.0
queue-monitor-1.1.1
queue-monitor-1.1.2

# Commits
3ff212c Fix queue monitor certificate update (#36)
dc35945 build(queue-monitor): add ca certs to image (#35)
```

**Extract**:
- Previous version tag
- Commit messages with PR numbers
- Brief description of fixes/features

---

### Step 5: Commit Deployment File Changes

**ACTION**: Commit the deployment file changes with proper commit message format.

**Commit Message Format**:
```
ops({service}): deploy v{version} to production

Update production deployment to version {version} which includes:
- {Brief description of changes/fixes}
- {Reference to PRs if available}

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Command Template**:
```bash
git add infra/k8s/{service}-deployment.yml && git commit -m "$(cat <<'EOF'
ops({service}): deploy v{version} to production

Update production deployment to version {version} which includes:
- {Description of change 1} (#{pr-number})
- {Description of change 2} (#{pr-number})

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Example**:
```bash
git add infra/k8s/queue-monitor-deployment.yml && git commit -m "$(cat <<'EOF'
ops(queue-monitor): deploy v1.1.2 to production

Update production deployment to version 1.1.2 which includes:
- Certificate fix for queue monitor (#36)
- CA certificates added to Docker image (#35)

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
EOF
)"
```

**Expected Output**:
```
[deploy/queue-monitor-1.1.2-to-prd 6bd1eae] ops(queue-monitor): deploy v1.1.2 to production
 1 file changed, 1 insertion(+), 1 deletion(-)
```

---

### Step 6: Push Branch and Create Pull Request

**ACTION**: Push the branch to GitHub and create a Pull Request.

**PR Title Format**: `ops({service}): deploy v{version} to production`

**PR Body Template**:
```markdown
## Summary
Deploy {service} version {version} to production environment

## Changes
- Update production deployment image from `{old-version}` → `{new-version}`

## What's in v{version}
- {List of changes/fixes with PR references}

## Deployment checklist
- [x] Version tested in dev environment
- [x] Deployment file updated
- [ ] Applied to Kubernetes production cluster
- [ ] Deployment verified (pod running)
- [ ] Logs checked (no errors)
- [ ] GitHub release created

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

**Command Template**:
```bash
git push -u origin deploy/{service}-{version}-to-prd && gh pr create --title "ops({service}): deploy v{version} to production" --body "$(cat <<'EOF'
## Summary
Deploy {service} version {version} to production environment

## Changes
- Update production deployment image from `{old-version}` → `{new-version}`

## What's in v{version}
- {List of changes/fixes}
- {Reference to PRs}

## Deployment checklist
- [x] Version tested in dev environment
- [x] Deployment file updated
- [ ] Applied to Kubernetes production cluster
- [ ] Deployment verified (pod running)
- [ ] Logs checked (no errors)
- [ ] GitHub release created

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

**Example**:
```bash
git push -u origin deploy/queue-monitor-1.1.2-to-prd && gh pr create --title "ops(queue-monitor): deploy v1.1.2 to production" --body "$(cat <<'EOF'
## Summary
Deploy queue-monitor version 1.1.2 to production environment

## Changes
- Update production deployment image from `1.0.0` → `1.1.2`

## What's in v1.1.2
- Fix queue monitor certificate update (#36)
- Add CA certificates to Docker image (#35)

## Deployment checklist
- [x] Version tested in dev environment
- [x] Deployment file updated
- [ ] Applied to Kubernetes production cluster
- [ ] Deployment verified (pod running)
- [ ] Logs checked (no errors)
- [ ] GitHub release created

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

**Expected Output**:
```
branch 'deploy/queue-monitor-1.1.2-to-prd' set up to track 'origin/deploy/queue-monitor-1.1.2-to-prd'.
https://github.com/UladzK/duw-kolejka-checker/pull/37
```

**Save the PR URL** - you'll need it for the final summary.

**Present to User**:
```
Pull Request Created: {pr-url}

Next Steps:
1. Use the `duw-deploy` skill to deploy to Kubernetes production
2. After successful deployment, return here to create GitHub release

Ready to proceed with Kubernetes deployment?
```

---

### Step 7: Create GitHub Release (Post-Deployment)

**ACTION**: Create a GitHub release after successful Kubernetes deployment.

**IMPORTANT**: Only perform this step AFTER successful deployment to production via `duw-deploy` skill.

**Release Tag Format**: `{service}-{version}`

**Release Format**:
```markdown
## What's Changed
* {Description of change} by @{author} in {pr-url}
* {Description of change} by @{author} in {pr-url}

{Brief description of what this release includes/fixes}

**Full Changelog**: https://github.com/UladzK/duw-kolejka-checker/compare/{previous-tag}...{new-tag}
```

**Command Template**:
```bash
gh release create {service}-{version} --title "{service}-{version}" --notes "$(cat <<'EOF'
## What's Changed
* {Description} by @{author} in {pr-url}
* {Description} by @{author} in {pr-url}

{Brief summary of what this release addresses}

**Full Changelog**: https://github.com/UladzK/duw-kolejka-checker/compare/{previous-tag}...{new-tag}
EOF
)" --latest
```

**Example**:
```bash
gh release create queue-monitor-1.1.2 --title "queue-monitor-1.1.2" --notes "$(cat <<'EOF'
## What's Changed
* Fix queue monitor certificate update by @UladzK in https://github.com/UladzK/duw-kolejka-checker/pull/36
* build(queue-monitor): add ca certs to image by @UladzK in https://github.com/UladzK/duw-kolejka-checker/pull/35

This release fixes certificate validation issues that were preventing the queue monitor from properly connecting to the DUW API.

**Full Changelog**: https://github.com/UladzK/duw-kolejka-checker/compare/queue-monitor-1.0.1...queue-monitor-1.1.2
EOF
)" --latest
```

**Expected Output**:
```
https://github.com/UladzK/duw-kolejka-checker/releases/tag/queue-monitor-1.1.2
```

**Save the release URL** for the final summary.

---

### Step 8: Final Release Summary

**ACTION**: Provide comprehensive summary of the release workflow.

**Summary Template**:
```markdown
## Release Workflow Completed! ✓

### Release Information

**Service**: {service}
**Version**: {version}
**Previous Version**: {previous-version}

---

### Git Workflow Completed

**Branch Created**: `deploy/{service}-{version}-to-prd`
**Deployment File Updated**: `infra/k8s/{service}-deployment.yml`
  - Image: `acrduwshared.azurecr.io/{service}:{old-version}` → `{new-version}`

**Commit**: {commit-hash}
```
ops({service}): deploy v{version} to production
```

**Pull Request**: {pr-url}
**GitHub Release**: {release-url} (marked as latest)

---

### What's in This Release

{List of changes/fixes with descriptions}

---

### Next Steps

**If you haven't deployed to Kubernetes yet:**
1. Use the `duw-deploy` skill to deploy to production
2. Specify service: {service}
3. Specify version: {version}
4. Specify environment: prd

**If deployment is already complete:**
✓ All release tasks completed successfully!
✓ Production deployment file updated
✓ Pull Request created for tracking
✓ GitHub release published

---

### Complete Release Workflow Summary

✓ All steps completed:
1. Verified versions (dev: {dev-ver}, prd: {old-ver} → {new-ver})
2. Created deployment branch
3. Updated production deployment file
4. Committed changes with proper format
5. Pushed branch and created PR
6. Created GitHub release (if post-deployment)

**The release workflow has been completed successfully!**
```

---

## Workflow Integration with duw-deploy

### Option 1: Release First, Then Deploy

**Use this when you want to prepare everything before deploying:**

1. **Run `duw-release` skill**
   - Creates branch
   - Updates deployment file
   - Creates PR
   - Stops before creating GitHub release

2. **Run `duw-deploy` skill**
   - Switch to production context
   - Apply deployment
   - Verify pod is running
   - Check logs

3. **Return to `duw-release` skill**
   - Create GitHub release (Step 7)
   - Provide final summary

### Option 2: Deploy First, Then Release

**Use this when deployment file is already updated:**

1. **Run `duw-deploy` skill**
   - Deploy to production
   - Verify deployment

2. **Run `duw-release` skill**
   - Provide service name and version
   - Skill will create PR and release

---

## Command Reference

### Git Workflow Commands

**Branch Management**:
```bash
# Create deployment branch
git checkout -b deploy/{service}-{version}-to-prd

# Check current branch
git branch --show-current

# Switch back to main
git checkout main
```

**Commit and Push**:
```bash
# Stage deployment file
git add infra/k8s/{service}-deployment.yml

# Commit with message
git commit -m "ops({service}): deploy v{version} to production..."

# Push and track remote branch
git push -u origin deploy/{service}-{version}-to-prd
```

**Status Check**:
```bash
# Check git status
git status

# View recent commits
git log --oneline -5

# View file diff
git diff infra/k8s/{service}-deployment.yml
```

### GitHub CLI Commands

**Pull Request**:
```bash
# Create PR
gh pr create --title "..." --body "..."

# List recent PRs
gh pr list --limit 10

# View PR details
gh pr view {pr-number}

# Check PR status
gh pr status
```

**Release Management**:
```bash
# List recent releases
gh release list --limit 10

# View release details
gh release view {tag-name}

# Create release
gh release create {tag} --title "..." --notes "..." --latest

# Delete release (if needed)
gh release delete {tag}
```

**Git Tags**:
```bash
# List tags for service
git tag --list "{service}-*"

# List recent tags
git tag --list "{service}-*" | tail -10

# View tag details
git show {tag-name}

# Create annotated tag (manual)
git tag -a {service}-{version} -m "Release {version}"
```

**Changelog/Commits**:
```bash
# Get commits between tags
git log --oneline {old-tag}..{new-tag}

# Get commits without merges
git log --oneline {old-tag}..{new-tag} --no-merges

# View commit details
git show {commit-hash}
```

---

## Workflow Checklist

**Step 0: Gather Context**
- [ ] Service name is clear
- [ ] Version to release is known
- [ ] Brief description of changes available

**Step 1: Verify Versions**
- [ ] Read dev deployment file
- [ ] Read production deployment file
- [ ] Confirmed versions with user

**Step 2: Create Branch**
- [ ] Created deployment branch with correct naming

**Step 3: Update File**
- [ ] Updated production deployment YAML
- [ ] Verified change was applied

**Step 4: Gather Changelog**
- [ ] Listed recent releases
- [ ] Listed recent tags
- [ ] Got commits between versions
- [ ] Identified changes and PRs

**Step 5: Commit Changes**
- [ ] Staged deployment file
- [ ] Created commit with proper format
- [ ] Verified commit was created

**Step 6: Create PR**
- [ ] Pushed branch to GitHub
- [ ] Created PR with proper title and body
- [ ] Saved PR URL

**Step 7: Create Release** (post-deployment)
- [ ] Verified deployment was successful
- [ ] Created GitHub release with changelog
- [ ] Marked as latest release
- [ ] Saved release URL

**Step 8: Final Summary**
- [ ] Provided comprehensive summary
- [ ] Listed all URLs (PR, release)
- [ ] Documented what changed

---

## Critical Rules

### Information Gathering
1. ✅ **ALWAYS** ask user for service name if not clear
2. ✅ **ALWAYS** ask user for version to release if not clear
3. ✅ **ALWAYS** verify versions before proceeding
4. ❌ **NEVER** assume or guess service name
5. ❌ **NEVER** assume or guess version number

### Git Workflow
6. ✅ **ALWAYS** create deployment branch before changes
7. ✅ **ALWAYS** use proper branch naming: `deploy/{service}-{version}-to-prd`
8. ✅ **ALWAYS** use proper commit message format
9. ✅ **ALWAYS** include Co-Authored-By in commits
10. ❌ **NEVER** commit directly to main branch

### GitHub Integration
11. ✅ **ALWAYS** create PR with deployment checklist
12. ✅ **ALWAYS** save PR and release URLs
13. ✅ **ALWAYS** mark release as "latest"
14. ✅ **ALWAYS** include Full Changelog link in release
15. ❌ **NEVER** create release before successful deployment

### Progress Tracking
16. ✅ **ALWAYS** use TodoWrite tool to track workflow progress
17. ✅ **ALWAYS** mark tasks as completed immediately after finishing
18. ✅ **ALWAYS** have exactly ONE task in_progress at a time

### Safety
19. ✅ **ALWAYS** get user confirmation before pushing changes
20. ✅ **ALWAYS** verify file changes were applied correctly
21. ❌ **NEVER** skip verification steps

---

**END OF WORKFLOW**
