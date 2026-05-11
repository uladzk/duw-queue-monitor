output "aks_host" {
  value       = azurerm_kubernetes_cluster.aks.kube_config.0.host
  description = "AKS API server endpoint"
  sensitive   = true
}

output "aks_cluster_ca_certificate" {
  value       = azurerm_kubernetes_cluster.aks.kube_config.0.cluster_ca_certificate
  description = "AKS cluster CA certificate (base64 encoded)"
  sensitive   = true
}

output "aks_cluster_name" {
  value       = azurerm_kubernetes_cluster.aks.name
  description = "AKS cluster name"
}

# Service principal outputs for K8s Terraform provider authentication
output "k8s_terraform_sp_client_id" {
  value       = azuread_application.k8s_terraform.client_id
  description = "Client ID of service principal for Terraform K8s provider"
  sensitive   = true
}

output "k8s_terraform_sp_client_secret" {
  value       = azuread_service_principal_password.k8s_terraform.value
  description = "Client secret of service principal for Terraform K8s provider"
  sensitive   = true
}
