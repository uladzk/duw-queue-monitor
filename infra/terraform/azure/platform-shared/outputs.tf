output "acr_id" {
  value = azurerm_container_registry.acr.id

  description = "ID of the Azure Container Registry (ACR) for pulling images"
}

output "acr_login_server" {
  value = azurerm_container_registry.acr.login_server

  description = "FQDN of the Azure Container Registry (ACR) for pulling images"
}

output "aks_admins_group_object_id" {
  value = azuread_group.aks_admins_group.object_id

  description = "Azure AD group ID for AKS admins"
}

output "gha_client_id" {
  value = azuread_application.gha.client_id

  description = "Client ID for the GitHub Actions publisher workflow"
  sensitive   = true
}
