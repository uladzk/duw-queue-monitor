data "terraform_remote_state" "mks" {
  backend = "s3"

  config = {
    bucket = "duw-tfstate-shared"
    key    = "mks.${var.environment}.tfstate"
    region = "waw"

    endpoints = {
      s3 = "https://s3.waw.io.cloud.ovh.net"
    }

    use_path_style              = true
    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
  }
}
