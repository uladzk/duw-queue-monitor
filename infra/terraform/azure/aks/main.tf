locals {
  location       = "Poland Central"
  location_short = "plc"
}

resource "azurerm_resource_group" "rg_aks" {
  name     = "rg-aks-${var.environment}"
  location = local.location
}

resource "azurerm_kubernetes_cluster" "aks" {
  name                = "aks-duw-${var.environment}-${local.location_short}"
  location            = azurerm_resource_group.rg_aks.location
  resource_group_name = azurerm_resource_group.rg_aks.name
  kubernetes_version  = var.aks_config.kubernetes_version
  sku_tier            = "Free"

  private_cluster_enabled = false # it's acceptable to allow access from all IPs. vNet integration is too complex and expensive. access is controlled by RBAC
  # for some reason the aks cluster is changed with empty api_server_access_profile. see issue: https://github.com/hashicorp/terraform-provider-azurerm/issues/20085
  api_server_access_profile {
    authorized_ip_ranges = [] # allow access from all IPs
  }
  role_based_access_control_enabled = true
  local_account_disabled            = true
  azure_active_directory_role_based_access_control {
    azure_rbac_enabled     = true
    tenant_id              = data.azurerm_subscription.current.tenant_id
    admin_group_object_ids = [data.terraform_remote_state.shared.outputs.aks_admins_group_object_id]
  }

  dns_prefix = "aksduw-${var.environment}"

  default_node_pool {
    name            = "default"
    node_count      = var.aks_config.default_node_count
    vm_size         = var.aks_config.default_vm_size
    os_disk_size_gb = var.aks_config.default_os_disk_size_gb
    os_disk_type    = "Ephemeral"
    os_sku          = "Ubuntu"

    upgrade_settings {
      max_surge                     = "100%"
      drain_timeout_in_minutes      = 5
      node_soak_duration_in_minutes = 1
    }
  }

  identity {
    type = "SystemAssigned"
  }

  tags = {
    environment = var.environment
  }
}

resource "azurerm_role_assignment" "aks_acr_pull" {
  scope                = data.terraform_remote_state.shared.outputs.acr_id
  principal_id         = azurerm_kubernetes_cluster.aks.kubelet_identity[0].object_id
  role_definition_name = "AcrPull"
}

# Azure AD Application for Terraform K8s provider authentication
resource "azuread_application" "k8s_terraform" {
  display_name = "terraform-k8s-provider-${var.environment}"
  owners       = [data.azurerm_client_config.current.object_id]
}

# Service Principal for the application
resource "azuread_service_principal" "k8s_terraform" {
  client_id = azuread_application.k8s_terraform.client_id
  owners    = [data.azurerm_client_config.current.object_id]
}

# Automatic password rotation trigger (365 days)
resource "time_rotating" "k8s_terraform_password_rotation" {
  rotation_days = 365
}

# Generate password for the service principal with automatic rotation
resource "azuread_service_principal_password" "k8s_terraform" {
  service_principal_id = azuread_service_principal.k8s_terraform.id

  rotate_when_changed = {
    rotation = time_rotating.k8s_terraform_password_rotation.id
  }

  # Created at: 20251110
  # Rotates automatically every 365 days via time_rotating resource
}

# Grant Azure RBAC permissions on AKS cluster
# This allows the service principal to manage Kubernetes resources
resource "azurerm_role_assignment" "k8s_terraform_aks_admin" {
  scope                = azurerm_kubernetes_cluster.aks.id
  role_definition_name = "Azure Kubernetes Service RBAC Cluster Admin"
  principal_id         = azuread_service_principal.k8s_terraform.object_id
}
