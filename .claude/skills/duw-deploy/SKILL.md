---
name: duw-deploy-to-kubernetes
description: Deploy service updates to Kubernetes dev or production clusters for DUW Kolejka Checker project. Handles version updates, deployment verification, and health checks.
alwaysApply: false
---

# DUW Deploy to Kubernetes

## Overview
This workflow defines the complete deployment process for updating services in the DUW Kolejka Checker project to Kubernetes clusters. It handles context selection, version updates, deployment application, and verification.

---

## Prerequisites
- Kubernetes access (dev and/or prod clusters)
- `kubectl` CLI tool installed
- `kubectx` CLI tool installed (for context switching)
- Service/component name must be clear
- New version number to deploy
- Target environment (dev/prd)

---

## Important Note
**If the context of the deployed component/service is not clear, you MUST explicitly ask the user to provide:**
- Service name (queue-monitor, telegram-bot, etc.)
- Version to deploy
- Target environment (dev/prd)

**DO NOT assume or guess these values.**

---

## Available Services
- **queue-monitor**: Queue monitoring service (Deployment)
- **telegram-bot**: Telegram notification bot (Deployment)
- **queue-stats-reports**: Queue statistics reporter (CronJob — daily/weekly/monthly)

---

## Available Environments
- **dev**: Development environment (aks-duw-dev-plc)
- **prd**: Production environment (aks-duw-prd-plc)

---

## Deployment Files Reference

| Service | Environment | File Path | Type |
|---------|-------------|-----------|------|
| queue-monitor | dev | `infra/k8s/queue-monitor-deployment-dev.yml` | Deployment |
| queue-monitor | prd | `infra/k8s/queue-monitor-deployment.yml` | Deployment |
| telegram-bot | dev/prd | `infra/k8s/telegram-bot-deployment.yml` | Deployment |
| queue-stats-reports | dev/prd | `infra/k8s/queue-stats-reports-cronjob.yml` | CronJob |
| queue-stats-reports | dev/prd | `infra/k8s/queue-stats-reports-external-secret.yml` | ExternalSecret |

---

## Workflow Steps

### Step 0: Gather Context (If Needed)

**ACTION**: Verify all required information is available.

**Required Information**:
- Service name
- Version to deploy
- Target environment

**If ANY information is missing or unclear, STOP and ASK USER**:
```
To proceed with deployment, I need the following information:
- Service/component name: {queue-monitor, telegram-bot, other?}
- Version to deploy: {e.g., 1.1.1}
- Target environment: {dev or prd}

Please provide the missing information.
```

**DO NOT proceed until all information is clear.**

---

### Step 1: Select Kubernetes Context

**ACTION**: Identify available contexts and switch to the correct environment.

#### 1.1: List Available Contexts

**Command**:
```bash
kubectx
```

**Expected Output**:
```
aks-duw-dev-plc
aks-duw-prd-plc
docker-desktop
saas-cluster-euwest-dev1
...
```

**DUW Contexts**:
- `aks-duw-dev-plc` - Development environment
- `aks-duw-prd-plc` - Production environment

#### 1.2: Ask Human Approval for Environment

**CRITICAL: STOP and ASK USER**

**Question to User**:
```
Available Kubernetes contexts for DUW:
- aks-duw-dev-plc (Development environment)
- aks-duw-prd-plc (Production environment)

Deployment Details:
- Service: {service-name}
- New version: {version}
- Change: {brief-description}

Which environment should I deploy to? Please type **dev** or **prd**:
```

**Wait for user response before proceeding.**

#### 1.3: Switch Context

**Commands**:
```bash
# For dev
kubectx aks-duw-dev-plc

# For prd
kubectx aks-duw-prd-plc
```

**Expected Output**:
```
Switched to context "aks-duw-dev-plc".
```

---

### Step 2: Identify Deployment File and Current Version

**ACTION**: Find the deployment file and read current version.

#### 2.1: Determine Deployment File

**File Naming Pattern**:
- Dev: `infra/k8s/{service}-deployment-dev.yml`
- Prod: `infra/k8s/{service}-deployment.yml`

**Examples**:
```bash
# queue-monitor dev
infra/k8s/queue-monitor-deployment-dev.yml

# queue-monitor prd
infra/k8s/queue-monitor-deployment.yml

# telegram-bot (same for dev and prd)
infra/k8s/telegram-bot-deployment.yml
```

#### 2.2: Read Current Version

**Action**: Use Read tool to view the deployment file and identify current image version.

**Look for line**:
```yaml
containers:
- image: acrduwshared.azurecr.io/{service}:{version}
```

**Example**:
```yaml
containers:
- image: acrduwshared.azurecr.io/queue-monitor:1.1.0
```

#### 2.3: Confirm Deployment Details

**CRITICAL: STOP and ASK USER for confirmation**

**Present to User**:
```
Deployment to Update:
- File: infra/k8s/{service}-deployment-{env}.yml
- Service: {service-name}
- Environment: {dev/prd}

Version Change:
- Current version: {current-version}
- New version: {new-version}

Image Update:
```yaml
# Before:
- image: acrduwshared.azurecr.io/{service}:{old-version}

# After:
- image: acrduwshared.azurecr.io/{service}:{new-version}
```

Confirm:
- ✓ File: {filename}
- ✓ Version change: {old} → {new}
- ✓ Environment: {env}

Is this correct? Please confirm to proceed.
```

**Wait for user confirmation before proceeding.**

---

### Step 3: Update Deployment File

**ACTION**: Update the image tag in the deployment YAML file.

**Use Edit tool** to replace the image line:

**Old String**:
```yaml
      - image: acrduwshared.azurecr.io/{service}:{old-version}
```

**New String**:
```yaml
      - image: acrduwshared.azurecr.io/{service}:{new-version}
```

**Example**:
```yaml
# Before (line 20)
- image: acrduwshared.azurecr.io/queue-monitor:1.1.0

# After (line 20)
- image: acrduwshared.azurecr.io/queue-monitor:1.1.1
```

**Verify**: The Edit tool will show the updated snippet confirming the change.

---

### Step 4: Apply Deployment to Kubernetes

**ACTION**: Apply the updated deployment to the cluster.

#### 4.1: Show Deployment Summary

**Present to User**:
```
Ready to Deploy to Kubernetes

Deployment Summary:
- Context: aks-duw-{env}-plc ({environment} environment)
- File: infra/k8s/{service}-deployment-{env}.yml
- Service: {service-name}
- New image: acrduwshared.azurecr.io/{service}:{new-version}
- Strategy: {Recreate/RollingUpdate}

This will:
1. {Strategy-specific action - e.g., "Terminate the current pod before starting new one"}
2. Pull the new image ({version}) from Azure Container Registry
3. Start a new pod with the updated image

Command to execute:
```bash
kubectl apply -f infra/k8s/{service}-deployment-{env}.yml
```
```

#### 4.2: Ask Human Approval

**CRITICAL: STOP and ASK USER**

```
Do you approve this deployment? Please confirm to proceed.
```

**Wait for user approval before executing.**

#### 4.3: Execute Deployment

**Command**:
```bash
kubectl apply -f infra/k8s/{service}-deployment-{env}.yml
```

**Expected Output**:
```
deployment.apps/{service} configured
```
or
```
deployment.apps/{service} created
```

---

### Step 5: Verify Deployment Success

**ACTION**: Verify the deployment was successful and pod is running.

#### 5.1: Check Deployment Status

**Command**:
```bash
kubectl get deployment {service}
```

**Expected Output**:
```
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
queue-monitor   1/1     1            1           30s
```

**Success Criteria**:
- READY: 1/1
- UP-TO-DATE: 1
- AVAILABLE: 1

#### 5.2: Check Pod Status

**Command**:
```bash
kubectl get pods -l app={service} -o wide
```

**Expected Output**:
```
NAME                            READY   STATUS    RESTARTS   AGE   IP             NODE
queue-monitor-cc4fffcb5-xgr2q   1/1     Running   0          30s   10.244.0.248   aks-default-...
```

**Success Criteria**:
- READY: 1/1
- STATUS: Running
- RESTARTS: 0 (or low number)

#### 5.3: Present Verification Results

**Show to User**:
```
Deployment Status: ✓ READY (1/1)
```
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
{service}       1/1     1            1           {age}
```

Pod Status: ✓ Running
```
NAME                            READY   STATUS    RESTARTS   AGE
{pod-name}                      1/1     Running   0          {age}
```

✓ Deployment is READY
✓ Pod is Running
✓ No restarts
✓ New pod created successfully
```

---

### Step 6: Check Pod Logs

**ACTION**: Verify application is running correctly by examining logs.

#### 6.1: Get Recent Logs

**Command**:
```bash
kubectl logs deployment/{service} --tail=50
```

**Example Output** (queue-monitor):
```
time=2025-10-23T... level=INFO msg="Starting queue monitor..."
time=2025-10-23T... level=INFO msg="Queue monitor started. Waiting for shutdown signal..."
time=2025-10-23T... level=INFO msg="Initializing monitor state..."
time=2025-10-23T... level=INFO msg="Started monitor loop. Status check interval is set to 5 seconds"
```

**Look for**:
- Application startup messages
- Successful initialization
- No critical errors

#### 6.2: Search for Errors

**Commands**:
```bash
# General error search
kubectl logs deployment/{service} | grep -i error

# Specific searches (if relevant)
kubectl logs deployment/{service} | grep -i "certificate\|ssl\|x509\|tls"
kubectl logs deployment/{service} | grep -i "failed\|exception\|panic"
```

**Expected**: No critical errors found (empty output or only expected warnings)

#### 6.3: Present Log Analysis

**Show to User**:
```
Application Logs: ✓ Started Successfully
- ✓ "Starting {service}..." - Application initialized
- ✓ "{service} started..." - Running
- ✓ "{Monitoring/Service} active" - Core functionality working
- ✓ No critical errors found

{If errors found, list them here}
```

---

### Step 7: Final Summary and Completion

**ACTION**: Provide comprehensive deployment summary.

**Final Summary Template**:
```markdown
## Deployment Completed Successfully! ✓

### Summary

**Environment**: {env} (aks-duw-{env}-plc)
**Service**: {service-name}
**Version**: {new-version}
**Deployment File**: infra/k8s/{service}-deployment-{env}.yml

---

### What Changed
{Brief description of what this version includes}

---

### Verification Results

**Deployment Status**: ✓ READY (1/1)
```
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
{service}       1/1     1            1           {age}
```

**Pod Status**: ✓ Running
```
NAME                            READY   STATUS    RESTARTS   AGE
{pod-name}                      1/1     Running   0          {age}
```

**Application Logs**: ✓ Started Successfully
{List key success indicators from logs}

**Issues Fixed/Addressed**: ✓ {List what this deployment fixed}
{If applicable, note before/after comparison}

---

### Current Status
{Describe current state of the application}

{If any notes or warnings, include them here}

---

### Next Steps (Optional)
{Suggest follow-up actions if needed, such as:}
1. Monitor during peak hours to verify performance
2. Deploy to production if dev testing successful
3. Address any related issues found

---

### Complete Workflow Summary

✓ All steps completed successfully:
1. Selected Kubernetes context (aks-duw-{env}-plc)
2. Updated deployment file ({old-version} → {new-version})
3. Applied changes to cluster
4. Verified deployment is running
5. Confirmed application logs show healthy status
6. {Issue/feature} successfully deployed

The deployment has been completed successfully!
```

---

## CronJob Deployment (queue-stats-reports)

CronJob deployment differs from Deployment resources. Use these steps instead of Steps 2-6 when deploying queue-stats-reports.

### CronJob: Apply Manifests

**First-time deployment** (apply both ExternalSecret and CronJob):
```bash
kubectl apply -f infra/k8s/queue-stats-reports-external-secret.yml
kubectl apply -f infra/k8s/queue-stats-reports-cronjob.yml
```

**Version update** (update image tag in CronJob manifest, then apply):
```bash
kubectl apply -f infra/k8s/queue-stats-reports-cronjob.yml
```

### CronJob: Version Update

The CronJob manifest contains three CronJob resources (daily, weekly, monthly). To update the version, use Edit tool to replace the image tag in all three:
```yaml
image: acrduwshared.azurecr.io/queue-stats-reports:{old-version}
→
image: acrduwshared.azurecr.io/queue-stats-reports:{new-version}
```

### CronJob: Verify

```bash
# Check CronJob status
kubectl get cronjobs | grep queue-stats-reports

# Check recent jobs
kubectl get jobs | grep queue-stats-reports

# Manual trigger for testing
kubectl create job --from=cronjob/queue-stats-reports-daily test-daily-run

# Check job logs
kubectl logs job/test-daily-run
```

### CronJob: Suspend/Unsuspend

Weekly and monthly CronJobs start suspended. Unsuspend when ready:
```bash
kubectl patch cronjob queue-stats-reports-weekly -p '{"spec":{"suspend":false}}'
kubectl patch cronjob queue-stats-reports-monthly -p '{"spec":{"suspend":false}}'
```

### CronJob: Rollback

```bash
# Delete and recreate with previous version
kubectl delete -f infra/k8s/queue-stats-reports-cronjob.yml
# Edit manifest back to previous version, then:
kubectl apply -f infra/k8s/queue-stats-reports-cronjob.yml
```

---

## Error Handling

### Pod Fails to Start

**Symptoms**: Pod status shows `CrashLoopBackOff`, `Error`, or stuck in `Pending`

**Diagnostic Commands**:
```bash
# Check pod events and details
kubectl describe pod -l app={service}

# Check logs for error messages
kubectl logs deployment/{service}

# Check previous container logs (if restarting)
kubectl logs deployment/{service} --previous
```

**Common Issues**:
- **Image pull error**: Check ACR credentials and image exists
- **Application error**: Check environment variables, secrets, and configuration
- **Resource limits**: Check CPU/memory limits are sufficient
- **ConfigMap/Secret missing**: Verify required config resources exist

**Actions**:
1. Review describe output for error messages
2. Check logs for application-specific errors
3. Verify configuration and secrets
4. Consider rollback if issue cannot be quickly resolved

---

### Image Pull Error

**Symptoms**: Pod status shows `ImagePullBackOff` or `ErrImagePull`

**Diagnostic Commands**:
```bash
# Verify image exists in ACR
az acr repository show-tags --name acrduwshared --repository {service}

# Check recent tags
git tag --list "{service}-*" | tail -10

# Check pod events
kubectl describe pod -l app={service} | grep -A10 Events
```

**Common Causes**:
- Image version doesn't exist in ACR
- ACR authentication issue
- Network connectivity problem

**Actions**:
1. Verify the version exists in ACR
2. Check ACR pull secrets exist in namespace
3. Verify image name is correct (no typos)
4. If image doesn't exist, trigger Publish workflow

---

### Deployment Timeout

**Symptoms**: Deployment shows `0/1` AVAILABLE after several minutes

**Diagnostic Commands**:
```bash
# Check replica set status
kubectl get replicaset -l app={service}

# Check pod status
kubectl get pods -l app={service}

# Check deployment events
kubectl describe deployment {service}

# Check rollout status
kubectl rollout status deployment/{service}
```

**Actions**:
1. Review replica set events
2. Check if old pods are still terminating
3. Verify new pod is starting (not stuck in Pending/ContainerCreating)
4. Consider increasing timeout or checking resource constraints

---

### Application Errors in Logs

**Symptoms**: Pod is Running but logs show errors

**Diagnostic Commands**:
```bash
# View full logs
kubectl logs deployment/{service} --tail=100

# Follow logs in real-time
kubectl logs -f deployment/{service}

# Search for specific error patterns
kubectl logs deployment/{service} | grep -i "error\|exception\|failed"
```

**Actions**:
1. Identify the specific error from logs
2. Check if it's a configuration issue (environment variables, secrets)
3. Check if it's a dependency issue (Redis, database, external API)
4. Verify the new version works as expected
5. Consider rollback if errors are critical

---

### Rollback Procedure

If the new version has critical issues that cannot be quickly resolved:

**Quick Rollback (using kubectl)**:
```bash
# Rollback to previous version
kubectl rollout undo deployment/{service}

# Check rollback status
kubectl rollout status deployment/{service}

# Verify pods are running
kubectl get pods -l app={service}
```

**Manual Rollback (edit deployment file)**:
```bash
# 1. Update deployment file back to previous version
# Use Edit tool to change: {new-version} → {old-version}

# 2. Apply the previous configuration
kubectl apply -f infra/k8s/{service}-deployment-{env}.yml

# 3. Verify rollback
kubectl get deployment {service}
kubectl get pods -l app={service}
```

**After Rollback**:
1. Investigate the issue that caused the rollback
2. Fix the issue in code/configuration
3. Test thoroughly before redeploying
4. Document what went wrong for future reference

---

## Complete Command Reference

### Context Management
```bash
# List available contexts
kubectx

# Switch to dev
kubectx aks-duw-dev-plc

# Switch to prd
kubectx aks-duw-prd-plc

# Show current context
kubectl config current-context
```

### Deployment Operations
```bash
# Apply deployment
kubectl apply -f infra/k8s/{service}-deployment-{env}.yml

# Check deployment status
kubectl get deployment {service}

# Watch deployment (real-time)
kubectl get deployment {service} -w

# Describe deployment (events and details)
kubectl describe deployment {service}

# Check rollout status
kubectl rollout status deployment/{service}
```

### Pod Operations
```bash
# List pods for service
kubectl get pods -l app={service}

# List pods with details
kubectl get pods -l app={service} -o wide

# Watch pods (real-time)
kubectl get pods -l app={service} -w

# Describe pod
kubectl describe pod -l app={service}

# Describe specific pod
kubectl describe pod {pod-name}
```

### Log Operations
```bash
# View recent logs (last 50 lines)
kubectl logs deployment/{service} --tail=50

# View recent logs (last 100 lines)
kubectl logs deployment/{service} --tail=100

# Follow logs (real-time)
kubectl logs -f deployment/{service}

# Logs with timestamps
kubectl logs deployment/{service} --timestamps

# Previous container logs (if restarted)
kubectl logs deployment/{service} --previous

# Search logs for errors
kubectl logs deployment/{service} | grep -i error

# Search logs for specific patterns
kubectl logs deployment/{service} | grep -i "certificate\|ssl\|x509"
```

### Troubleshooting
```bash
# Check replica sets
kubectl get replicaset -l app={service}

# Check services
kubectl get service {service}

# Check secrets
kubectl get secrets

# Check configmaps
kubectl get configmaps

# View events
kubectl get events --sort-by='.lastTimestamp'

# Rollback deployment
kubectl rollout undo deployment/{service}

# Rollback to specific revision
kubectl rollout undo deployment/{service} --to-revision={revision-number}

# View rollout history
kubectl rollout history deployment/{service}
```

---

## Workflow Summary Checklist

**Step 0: Gather Context**
- [ ] Service name is clear
- [ ] Version to deploy is known
- [ ] Target environment is specified

**Step 1: Select Kubernetes Context**
- [ ] List available contexts
- [ ] Get human approval for environment
- [ ] Switch to selected context

**Step 2: Identify Deployment File**
- [ ] Find correct deployment file
- [ ] Read current version
- [ ] Get human approval for version change

**Step 3: Update Deployment File**
- [ ] Update image tag to new version
- [ ] Verify change was applied

**Step 4: Apply Deployment**
- [ ] Show deployment summary
- [ ] Get human approval to proceed
- [ ] Execute kubectl apply

**Step 5: Verify Deployment**
- [ ] Check deployment status (READY)
- [ ] Check pod status (Running)
- [ ] Verify no restart issues

**Step 6: Check Logs**
- [ ] View recent application logs
- [ ] Search for errors
- [ ] Verify successful startup

**Step 7: Final Summary**
- [ ] Provide comprehensive summary
- [ ] List all verification results
- [ ] Suggest next steps if needed

---

## Critical Rules

### Information Gathering
1. ✅ **ALWAYS** ask user for service/component if not clear from context
2. ✅ **ALWAYS** ask user for version to deploy if not clear
3. ✅ **ALWAYS** ask user for target environment if not clear
4. ❌ **NEVER** assume or guess service name
5. ❌ **NEVER** assume or guess version number
6. ❌ **NEVER** assume environment (dev/prd)

### Human Approval
7. ✅ **ALWAYS** get human approval for environment selection
8. ✅ **ALWAYS** get human approval to confirm version change
9. ✅ **ALWAYS** get human approval before applying deployment
10. ❌ **NEVER** deploy without explicit approval

### Verification
11. ✅ **ALWAYS** verify deployment status after applying
12. ✅ **ALWAYS** verify pod is Running before proceeding
13. ✅ **ALWAYS** check logs for errors
14. ✅ **ALWAYS** provide comprehensive summary at the end
15. ❌ **NEVER** skip verification steps
16. ❌ **NEVER** assume deployment succeeded without checking

### Progress Tracking
17. ✅ **ALWAYS** use TodoWrite tool to track workflow progress
18. ✅ **ALWAYS** mark tasks as completed immediately after finishing
19. ✅ **ALWAYS** have exactly ONE task in_progress at a time

### Safety
20. ✅ **ALWAYS** show what will change before applying
21. ✅ **ALWAYS** provide rollback instructions if issues occur
22. ❌ **NEVER** force deployments if verification fails
23. ❌ **NEVER** ignore errors in logs

---

**END OF WORKFLOW**
