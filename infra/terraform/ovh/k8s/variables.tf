variable "environment" {
  type        = string
  description = "Environment name, e.g. dev, prd"
}

variable "aks_eso_infisical_client_id" {
  type        = string
  description = "Infisical identity client ID for accessing secrets in the cluster"
  sensitive   = true
}

variable "aks_eso_infisical_client_secret" {
  type        = string
  description = "Infisical identity client secret for accessing secrets in the cluster"
  sensitive   = true
}

variable "infisical_project_slug" {
  type        = string
  description = "Infisical project slug for secret management"
}
