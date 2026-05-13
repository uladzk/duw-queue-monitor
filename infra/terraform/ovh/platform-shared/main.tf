locals {
  tfstate_bucket = "duw-tfstate-shared"
}

resource "ovh_cloud_project_user" "tfstate" {
  service_name = var.project_id
  description  = "Terraform state backend S3 access"
  role_names   = ["objectstore_operator"]
}

resource "ovh_cloud_project_user_s3_credential" "tfstate" {
  service_name = var.project_id
  user_id      = ovh_cloud_project_user.tfstate.id
}

resource "ovh_cloud_project_storage" "tfstate" {
  service_name = var.project_id
  region_name  = upper(var.region)
  name         = local.tfstate_bucket
}
