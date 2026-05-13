output "tfstate_bucket" {
  value = ovh_cloud_project_storage.tfstate.name

  description = "Name of the OVH Object Storage bucket hosting Terraform state for all OVH modules"
}

output "tfstate_region" {
  value = ovh_cloud_project_storage.tfstate.region_name

  description = "Region of the OVH Object Storage bucket hosting Terraform state"
}

output "tfstate_access_key" {
  value = ovh_cloud_project_user_s3_credential.tfstate.access_key_id

  description = "Access key ID for the S3 backend (store in Infisical as AWS_ACCESS_KEY_ID)"
  sensitive   = true
}

output "tfstate_secret_key" {
  value = ovh_cloud_project_user_s3_credential.tfstate.secret_access_key

  description = "Secret access key for the S3 backend (store in Infisical as AWS_SECRET_ACCESS_KEY)"
  sensitive   = true
}
