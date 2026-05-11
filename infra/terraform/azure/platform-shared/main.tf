locals {
  location              = "Poland Central"
  gh_repo               = "uladzk/duw-queue-monitor"
  gh_branch             = "main" // main only for now
  delete_retention_days = 30
}

resource "azurerm_resource_group" "rg_platform_shared" {
  name     = "rg-platform-shared"
  location = local.location
}

resource "azurerm_container_registry" "acr" {
  name                = "acrduwshared"
  resource_group_name = azurerm_resource_group.rg_platform_shared.name
  location            = azurerm_resource_group.rg_platform_shared.location
  sku                 = "Basic"
}

resource "azuread_group" "aks_admins_group" {
  display_name     = "ug-aks-admins"
  security_enabled = true
}

resource "azuread_application" "gha" {
  display_name = "gha-publisher-${replace(local.gh_repo, "/", "-")}"
}

resource "azuread_service_principal" "gha_sp" {
  client_id = azuread_application.gha.client_id
}

resource "azuread_application_federated_identity_credential" "gha_fic" {
  application_id = azuread_application.gha.id
  display_name   = "gha-publisher-${replace(local.gh_repo, "/", "-")}-${local.gh_branch}"

  issuer    = "https://token.actions.githubusercontent.com"
  subject   = "repo:${local.gh_repo}:ref:refs/heads/${local.gh_branch}"
  audiences = ["api://AzureADTokenExchange"]

  description = "GitHub Actions publisher for ${local.gh_repo} on branch ${local.gh_branch}"
}

resource "azurerm_role_assignment" "gha_sp_acr_push" {
  scope                = azurerm_container_registry.acr.id
  role_definition_name = "AcrPush"
  principal_id         = azuread_service_principal.gha_sp.object_id
}

resource "azurerm_resource_group" "rg_tfstate" {
  name     = "rg-tfstate-shared"
  location = local.location
}

resource "azurerm_storage_account" "sa_tfstate" {
  name                     = "saduwtfstateshared"
  resource_group_name      = azurerm_resource_group.rg_tfstate.name
  location                 = azurerm_resource_group.rg_tfstate.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  access_tier              = "Cool"

  blob_properties {
    versioning_enabled = true
    delete_retention_policy {
      days = local.delete_retention_days
    }

    container_delete_retention_policy {
      days = local.delete_retention_days
    }
  }
}

resource "azurerm_storage_container" "sc_tfstate" {
  name                  = "scduwtfstate"
  storage_account_id    = azurerm_storage_account.sa_tfstate.id
  container_access_type = "private"
}
