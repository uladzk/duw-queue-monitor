output "cluster_id" {
  value       = ovh_cloud_project_kube.mks.id
  description = "OVH MKS cluster ID"
}

output "cluster_endpoint" {
  value       = ovh_cloud_project_kube.mks.url
  description = "Kubernetes API server URL"
}

output "kubeconfig" {
  value       = ovh_cloud_project_kube.mks.kubeconfig
  description = "Raw kubeconfig YAML for kubectl access; consumed by ovh/k8s layer via terraform_remote_state"
  sensitive   = true
}
