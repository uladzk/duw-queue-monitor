provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}
provider "azuread" {}

terraform {
  required_version = ">= 1.12.2"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "~> 4.33.0" }
    azuread = { source = "hashicorp/azuread", version = "~> 3.4.0" }
  }
}
