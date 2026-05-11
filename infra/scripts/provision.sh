#!/bin/bash

set -euo pipefail

# --- Validate Inputs ---

# Usage check
if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <module> <env> [--skip-infisical-login] [-destroy]"
  exit 1
fi

# --- Input Arguments ---
MODULE="$1"
ENV="$2"
shift 2

# --- Parse optional flags ---
SKIP_INFISICAL_LOGIN=false
DESTROY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-infisical-login)
      SKIP_INFISICAL_LOGIN=true
      shift
      ;;
    -destroy)
      DESTROY=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(dirname "$(realpath "$0")")"
MODULE_DIR="$SCRIPT_DIR/../terraform/azure/$MODULE"

# --- Constants ---
readonly AZURE_SUB="77a70a5e-2230-43b7-8983-61e7497498a8"
readonly INFISICAL_PROJECT_ID="145e0d1a-6378-4338-a9eb-2d77178f96e7" # terraform secrets project ID

if [[ ! -d "$MODULE_DIR" ]]; then
  echo "❌ Error: Module directory '$MODULE_DIR' does not exist."
  exit 1
fi

echo "🔧 Module dir: $MODULE_DIR"
echo "🌍 Environment: $ENV"
echo "🔨 Destroy flag: $DESTROY"
cd "$MODULE_DIR"

# --- Infisical Login ---
if [[ $SKIP_INFISICAL_LOGIN == false ]]; then
  echo "🔐 Logging in to Infisical..."
  infisical login
else
  echo "⚠️ Skipping Infisical login as per user request."
fi

# --- Check Azure CLI Login ---
if ! az account show; then
  echo "☁️ Logging in to Azure..."
  az login
fi

# --- Setting the subscription ---
az account set --subscription $AZURE_SUB

# --- Terraform Init ---
echo "📦 Initializing Terraform backend..."
terraform init -backend-config="envs/$ENV/backend.hcl"

# --- Terraform Plan ---
echo "🛠️ Planning infrastructure changes..."
PLAN_OUT="${ENV}.tfplan"

if [[ "$DESTROY" == true ]]; then
  PLAN_OPTIONS="-destroy"
else
  PLAN_OPTIONS=""
fi

infisical run --env="$ENV" --projectId=$INFISICAL_PROJECT_ID -- terraform plan \
  $PLAN_OPTIONS \
  -var-file="envs/$ENV/$ENV.tfvars" \
  -out="$PLAN_OUT"

echo -e "\n✅ Terraform plan created: $PLAN_OUT"
echo -n "❓ Do you want to apply this plan? (y/n): "
read -r CONFIRM

if [[ "$CONFIRM" == "y" ]]; then
  echo "🚀 Applying Terraform plan..."
  terraform apply "$PLAN_OUT"
else
  echo "❌ Apply aborted."
fi
