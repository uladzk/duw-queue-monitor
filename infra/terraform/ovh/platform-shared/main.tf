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

resource "ovh_cloud_project_user_s3_policy" "tfstate" {
  service_name = var.project_id
  user_id      = ovh_cloud_project_user.tfstate.id
  policy = jsonencode({
    Statement = [{
      Sid    = "FullAccessToTfstateBucket"
      Effect = "Allow"
      Action = [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket",
        "s3:GetBucketLocation",
        "s3:ListMultipartUploadParts",
        "s3:ListBucketMultipartUploads",
        "s3:AbortMultipartUpload",
      ]
      Resource = [
        "arn:aws:s3:::${ovh_cloud_project_storage.tfstate.name}",
        "arn:aws:s3:::${ovh_cloud_project_storage.tfstate.name}/*"
      ]
    }]
  })
}
