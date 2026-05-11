locals {
  # AKS AAD Server App ID is a universal constant for Azure-managed AKS clusters
  # This ID is the same across all Azure environments and regions
  aks_aad_server_app_id = "6dae42f8-4368-4678-94ff-3960e28e3630"
}

provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}

provider "kubernetes" {
  host                   = data.terraform_remote_state.aks.outputs.aks_host
  cluster_ca_certificate = base64decode(data.terraform_remote_state.aks.outputs.aks_cluster_ca_certificate)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "kubelogin"
    args = [
      "get-token",
      "--environment", "AzurePublicCloud",
      "--server-id", local.aks_aad_server_app_id,
      "--client-id", data.terraform_remote_state.aks.outputs.k8s_terraform_sp_client_id,
      "--client-secret", data.terraform_remote_state.aks.outputs.k8s_terraform_sp_client_secret,
      "--tenant-id", data.azurerm_subscription.current.tenant_id,
      "--login", "spn"
    ]
  }
}

provider "helm" {
  kubernetes = {
    host                   = data.terraform_remote_state.aks.outputs.aks_host
    cluster_ca_certificate = base64decode(data.terraform_remote_state.aks.outputs.aks_cluster_ca_certificate)

    exec = {
      api_version = "client.authentication.k8s.io/v1beta1"
      command     = "kubelogin"
      args = [
        "get-token",
        "--environment", "AzurePublicCloud",
        "--server-id", local.aks_aad_server_app_id,
        "--client-id", data.terraform_remote_state.aks.outputs.k8s_terraform_sp_client_id,
        "--client-secret", data.terraform_remote_state.aks.outputs.k8s_terraform_sp_client_secret,
        "--tenant-id", data.azurerm_subscription.current.tenant_id,
        "--login", "spn"
      ]
    }
  }
}
