---
name: duw-develop-and-publish-workflow
description: Complete workflow for developing, testing, and publishing Docker images to Azure Container Registry with GitHub PR workflow for DUW Kolejka Checker project.
allowed-tools: bash, gh, git, docker, go
---

# DUW Develop and Publish Workflow

## Overview
This workflow defines the complete development, testing, and publishing process for the DUW Kolejka Checker project. It automates building Docker images, creating PRs, running CI checks, and publishing new versions to Azure Container Registry.
Project code is written in Go.

---

## Prerequisites
- Git repository access (duw-kolejka-checker)
- Docker installed locally
- GitHub CLI (`gh`) installed and authenticated
- Access to Azure Container Registry (acrduwshared.azurecr.io)
- Kubernetes cluster access (optional, for deployment)

---

## Workflow Steps

### Step 0: Preparation
**ACTION**: Set up Git environment and create feature branch. If a Linear ticket is provided,
add ticket name in branch name prefix.

**Branch Naming Format**: `[{ticket}-]{type}-{short-description}`

**Branch Naming Examples**:
```bash
build-add-ca-certs-to-image
feat-add-telegram-notifications
duw-73-use-sa-token-in-k8s-provider
```

**Commands**:
```bash
git checkout main
git pull
git checkout -b {branch-name}
```

---

### Step 1: Code Changes and Testing
**ACTION**: Implement changes, verify that Go project builds successfully, Go tests pass and verify Docker image builds successfully.

#### 1.1: Make Code Changes
- Implement the required changes based on requirements
- Modify application code as necessary

###  1.2: Check Go build and tests
- Run go build for the whole project
- Run go test for the whole project 

#### (Optional) 1.3: Build Docker Image
**Purpose**: Verify changes don't break the build, if there are changes to Dockerfile.

**Docker Build Command**:
```bash
docker build -t {service}-test:latest -f cmd/{service}/Dockerfile .
```

#### 1.4: Verify Image
**Purpose**: Ensure image starts correctly and contains expected changes

**Verification Commands**:
```bash
# Check if image built successfully
docker images | grep {service}-test

# Test image starts without errors
docker run --rm {service}-test:latest {command}

```

---

### Step 2: Git Version Control
**ACTION**: Commit changes with conventional commit message and push to remote.

#### 2.1: Stage Changes
```bash
git add {files}
```

#### 2.2: Create Commit
**Conventional Commit Format**: `{type}({scope}): {description}`

**Commit Message Examples**:
```bash
build(queue-monitor): add ca certs to image
```

**Common Types**:
- `build`: Build system or Docker changes
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `docs`: Documentation changes
- `test`: Adding or modifying tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements

**Commit Command**:
```bash
git commit -m "{type}({scope}): {description}"
```

**Guidelines**:
- Keep description concise and clear
- Use imperative mood ("add" not "added")
- Don't end with period
- Scope should be component name (queue-monitor, telegram-bot, status-collector, etc.)

#### 2.3: Push to Remote
```bash
git push -u origin {branch-name}
```

---

### Step 3: GitHub Pull Request
**ACTION**: Create PR, monitor CI checks, and merge after approval.

#### 3.1: Create Pull Request

**PR Title Format**: `{Action} {component/feature} {description}`

**PR Title Examples**:
```
Add CA certificates to queue-monitor Docker image
Implement retry logic for Telegram notifications
Fix Redis connection timeout in queue monitor
Refactor error handling in status collector
Update Kubernetes deployment documentation
```

**PR Body Template**:
```markdown
Fixes {issue description or problem}

## Changes
- {Bullet point of what changed}
- {Another change}

## Problem
{Describe the problem this PR solves}

## Solution
{Describe how the changes solve the problem}

## Testing
- [x] Local Docker build successful
- [x] Image starts without errors
- [ ] {Other relevant tests}
```

**Create PR Command**:
```bash
gh pr create --title "{PR_TITLE}" --body "$(cat <<'EOF'
{PR_BODY_CONTENT}
EOF
)"
```

**Full Example**:
```bash
gh pr create --title "Add CA certificates to queue-monitor Docker image" --body "$(cat <<'EOF'
Fixes SSL certificate verification issue when connecting to https://rezerwacje.duw.pl from Kubernetes cluster.

## Changes
- Added `ca-certificates` package to Alpine base image in queue-monitor Dockerfile

## Problem
The Alpine-based container was missing CA certificates, causing "x509: certificate signed by unknown authority" errors when making HTTPS requests.

## Solution
Install ca-certificates package in the final Alpine stage to include the standard Mozilla CA certificate bundle.

## Testing
- [x] Local Docker build successful
- [x] Image starts without errors
- [x] CA certificates present in /etc/ssl/certs/
EOF
)"
```

**Expected Output**:
```
https://github.com/UladzK/duw-kolejka-checker/pull/{PR-NUMBER}
```

#### 3.2: Monitor CI Checks

**Check PR Status Command**:
```bash
gh pr checks {PR-NUMBER}
```

**Important**: Do not proceed to merge until all checks pass.

#### 3.3: Address PR Review Comments

**ACTION**: Check for review comments from Copilot or other reviewers and address them before merging.

**Fetch Review Comments**:
```bash
gh api repos/UladzK/duw-queue-monitor/pulls/{PR-NUMBER}/comments
```

**For each comment, decide one of**:

**Fix** — The remark is valid and requires a code change:
1. Make the fix locally
2. Commit and push
3. Reply to the comment explaining the fix:
   ```bash
   gh api repos/UladzK/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
     -f body="Fixed — {brief description of what was changed}." \
     -F in_reply_to={COMMENT-ID}
   ```
4. Wait for CI to pass again (`gh pr checks {PR-NUMBER} --watch`)

**Acknowledge** — The remark is valid but out of scope or deferred:
```bash
gh api repos/UladzK/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
  -f body="Good point. Tracked in {ticket/issue} for follow-up." \
  -F in_reply_to={COMMENT-ID}
```

**Discard** — The remark is not applicable or incorrect:
```bash
gh api repos/UladzK/duw-queue-monitor/pulls/{PR-NUMBER}/comments \
  -f body="Not applicable — {brief reasoning}." \
  -F in_reply_to={COMMENT-ID}
```

**Important**: Always reply to review comments before merging. Do not leave comments unaddressed.

#### 3.4: Merge Pull Request

**CRITICAL**: **STOP and ASK HUMAN APPROVAL before merging**

**Question to Human**:
```
PR #{PR-NUMBER} is ready for merge:
- Title: {PR_TITLE}
- URL: {PR_URL}
- CI Checks: ✓ All passed

Do you approve merging this PR to main?
```

**After Approval - Merge Command**:
```bash
gh pr merge {PR-NUMBER} --squash --delete-branch
```

**Example**:
```bash
gh pr merge 35 --delete-branch
```

**Expected Outcome**:
- PR is merged to main
- Feature branch is deleted
- Main branch now contains your changes

---

### Step 4: Build New Version
**ACTION**: Publish new Docker image version to Azure Container Registry.

#### 4.1: Find Publish Workflow

**List Workflows Command**:
```bash
gh workflow list
```

**Expected Output**:
```
Publish workflow          active    174585094
Pull request workflow     active    167106561
```

**Workflow Details**:
- **Name**: "Publish workflow"
- **Inputs**:
  - `service`: Service name (queue-monitor or telegram-bot)
  - `version`: Semantic version (e.g., 1.1.1)

#### 4.2: Get Human Approval and Version Number

**CRITICAL**: **STOP and ASK HUMAN for approval and version number**

**Questions to Human**:
```
Ready to publish {SERVICE_NAME}:

Current version in deployment: {CURRENT_VERSION}
What's new: {BRIEF_SUMMARY_OF_CHANGES}

The Publish workflow will:
1. Build and push Docker image with the new version to Azure Container Registry
2. Create a git tag {service}-{version}

What version number should I use for this release?
- Patch (e.g., 1.1.1): Bug fixes, minor updates
- Minor (e.g., 1.2.0): New features, backwards compatible
- Major (e.g., 2.0.0): Breaking changes

Do you approve starting the Publish workflow?
```

**Semantic Versioning Guide**:
- **Patch** (x.x.1): Bug fixes, minor updates (e.g., 1.1.0 → 1.1.1)
- **Minor** (x.1.0): New features, backwards compatible (e.g., 1.1.1 → 1.2.0)
- **Major** (1.0.0): Breaking changes (e.g., 1.2.0 → 2.0.0)

#### 4.3: Trigger Publish Workflow

**Command**:
```bash
gh workflow run "Publish workflow" -f service={SERVICE_NAME} -f version={VERSION}
```

**Examples**:
```bash
# Publish queue-monitor version 1.1.1
gh workflow run "Publish workflow" -f service=queue-monitor -f version=1.1.1

# Publish telegram-bot version 1.2.0
gh workflow run "Publish workflow" -f service=telegram-bot -f version=1.2.0
```

#### 4.4: Get Workflow Run ID

**Command**:
```bash
gh run list --workflow="Publish workflow" --limit 1
```

**Expected Output**:
```
queued    Publish workflow    main    workflow_dispatch    18761382838    7s    2025-10-23T20:39:37Z
```

**Extract Run ID**: `18761382838`

#### 4.5: Monitor Workflow Execution

**Watch Workflow Command**:
```bash
gh run watch {WORKFLOW-RUN-ID}
```

**Example**:
```bash
gh run watch 18761382838
```

**Expected Output** (updates every 3 seconds):
```
Refreshing run status every 3 seconds. Press Ctrl+C to quit.

* main Publish workflow · 18761382838
Triggered via workflow_dispatch about 1 minute ago

JOBS
* build-and-publish (ID 53526530082)
  ✓ Set up job
  ✓ Checkout code
  ✓ Azure login
  ✓ Login to ACR using CLI to allow log in using service principal
  ✓ Verify image with version does not exist
  ✓ Get service name in Go code style
  * Build and push Docker image
  * Create tag
  ...
```

#### 4.6: View Workflow Details

**Command**:
```bash
gh run view {WORKFLOW-RUN-ID}
```

**Example**:
```bash
gh run view 18761382838
```

**Expected Output**:
```
✓ main Publish workflow · 18761382838
Triggered via workflow_dispatch about 2 minutes ago

JOBS
✓ build-and-publish in 1m33s (ID 53526530082)

View this run on GitHub: https://github.com/UladzK/duw-kolejka-checker/actions/runs/18761382838
```

#### 4.7: Verify Deployment Artifacts

**Verify Git Tag**:
```bash
git fetch --tags
git tag --list "{service-name}-*" | tail -5
```

**Example**:
```bash
git fetch --tags
git tag --list "queue-monitor-*" | tail -5
```

**Expected Output**:
```
queue-monitor-1.1.0
queue-monitor-1.1.1  ← New tag
```

**Verification Checklist**:
- [x] Workflow completed successfully
- [x] Git tag created with format `{service}-{version}`
- [x] Docker image pushed to `acrduwshared.azurecr.io/{service}:{version}`

---

## Error Handling

### CI Checks Fail
**Symptoms**: PR checks show "fail" status

**Actions**:
1. Click the check link to view failure logs
2. Identify the issue (tests failing, build errors, linting issues)
3. Fix the issue locally starting from Step 1.

### Workflow Execution Fails
**Symptoms**: Publish workflow shows failure status

**Actions**:
1. View workflow logs:
   ```bash
   gh run view {RUN-ID} --log
   ```
2. Common issues:
   - **Image already exists**: Version number already used
   - **Authentication failure**: Azure credentials issue
   - **Build error**: Docker build failed

### Image Already Exists
**Error Message**: "Image {name}:{version} already exists"

**Actions**:
1. Choose a different version number
2. Verify current versions in ACR or git tags:
   ```bash
   git tag --list "{service}-*" | tail -10
   ```
3. Use the next appropriate version number

---

## Workflow Summary Checklist

**Step 0: Preparation**
- [ ] Checkout main branch
- [ ] Pull latest changes
- [ ] Create feature branch with proper naming

**Step 1: Code Changes and Testing**
- [ ] Implement code changes
- [ ] Build Docker image locally
- [ ] Verify image builds and starts correctly

**Step 2: Git Version Control**
- [ ] Stage changes
- [ ] Create commit with conventional message
- [ ] Push branch to remote

**Step 3: GitHub Pull Request**
- [ ] Create PR with descriptive title and body
- [ ] Monitor CI checks until they pass
- [ ] Address PR review comments (fix/acknowledge/discard, reply to each)
- [ ] **ASK HUMAN APPROVAL**
- [ ] Merge PR to main

**Step 4: Build New Version**
- [ ] Find Publish workflow
- [ ] **ASK HUMAN FOR VERSION NUMBER AND APPROVAL**
- [ ] Trigger Publish workflow
- [ ] Monitor workflow execution
- [ ] Verify git tag created
- [ ] Verify Docker image published

---

## Critical Rules

### Git and Version Control
1. ✅ **ALWAYS** use conventional commit format: `{type}({scope}): {description}`
2. ✅ **ALWAYS** use descriptive branch names: `{type}-{description}`
3. ✅ **ALWAYS** pull latest changes before creating feature branch
4. ✅ **ALWAYS** push feature branch before creating PR

### Testing and Verification
5. ✅ **ALWAYS** build Docker image locally before pushing
6. ✅ **ALWAYS** verify image starts without errors
7. ✅ **ALWAYS** wait for CI checks to pass before merging
8. ❌ **NEVER** merge PR with failing CI checks

### Pull Request Workflow
9. ✅ **ALWAYS** create PR with descriptive title and body
10. ✅ **ALWAYS** include what changed, why, and how in PR description
11. ✅ **ALWAYS** monitor PR checks until completion
12. ✅ **ALWAYS** address and reply to all PR review comments before merging
13. ✅ **ALWAYS** get human approval before merging PR
14. ❌ **NEVER** merge with unaddressed review comments

### Publishing and Deployment
13. ✅ **ALWAYS** get human approval before publishing new version
14. ✅ **ALWAYS** get version number from human (don't assume)
15. ✅ **ALWAYS** use semantic versioning (MAJOR.MINOR.PATCH)
16. ✅ **ALWAYS** monitor workflow execution until completion
17. ✅ **ALWAYS** verify git tag was created
18. ❌ **NEVER** use a version number that already exists

### Progress Tracking
19. ✅ **ALWAYS** use TodoWrite tool to track workflow progress
20. ✅ **ALWAYS** mark tasks as completed immediately after finishing
21. ✅ **ALWAYS** have exactly ONE task in_progress at a time

---
**END OF WORKFLOW**
