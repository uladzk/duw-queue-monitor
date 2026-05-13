locals {
  mks_kubeconfig   = yamldecode(data.terraform_remote_state.mks.outputs.kubeconfig)
  mks_cluster_info = local.mks_kubeconfig.clusters[0].cluster
  mks_user_info    = local.mks_kubeconfig.users[0].user
  mks_cluster_host = local.mks_cluster_info.server
  mks_cluster_ca   = base64decode(local.mks_cluster_info["certificate-authority-data"])
  mks_client_cert  = base64decode(local.mks_user_info["client-certificate-data"])
  mks_client_key   = base64decode(local.mks_user_info["client-key-data"])
}

provider "kubernetes" {
  host                   = local.mks_cluster_host
  cluster_ca_certificate = local.mks_cluster_ca
  client_certificate     = local.mks_client_cert
  client_key             = local.mks_client_key
}

provider "helm" {
  kubernetes = {
    host                   = local.mks_cluster_host
    cluster_ca_certificate = local.mks_cluster_ca
    client_certificate     = local.mks_client_cert
    client_key             = local.mks_client_key
  }
}

terraform {
  required_version = ">= 1.12.2"
  required_providers {
    kubernetes = { source = "hashicorp/kubernetes", version = "2.37.1" }
    helm       = { source = "hashicorp/helm", version = "3.0.2" }
  }
}
