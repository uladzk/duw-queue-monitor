data "azurerm_subscription" "current" {
}

// todo: remove hardcoded values
data "terraform_remote_state" "shared" {
  backend = "azurerm"

  config = {
    resource_group_name  = "rg-tfstate-shared"
    storage_account_name = "saduwtfstateshared"
    container_name       = "scduwtfstate"
    key                  = "shared.terraform.tfstate"
  }
}

data "terraform_remote_state" "aks" {
  backend = "azurerm"

  config = {
    resource_group_name  = "rg-tfstate-shared"
    storage_account_name = "saduwtfstateshared"
    container_name       = "scduwtfstate"
    key                  = "aks.${var.environment}.terraform.tfstate"
  }
}
