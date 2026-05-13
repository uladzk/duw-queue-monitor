locals {
  cluster_name = "mks-duw-${var.env}-waw"
}

resource "ovh_cloud_project_kube" "mks" {
  service_name = var.project_id
  name         = local.cluster_name
  region       = var.region
  version      = var.kubernetes_version
}

resource "ovh_cloud_project_kube_nodepool" "default" {
  service_name  = var.project_id
  kube_id       = ovh_cloud_project_kube.mks.id
  name          = "default-pool"
  flavor_name   = var.node_flavor
  desired_nodes = var.node_count
  min_nodes     = var.node_count
  max_nodes     = var.node_count
  autoscale     = false
}
