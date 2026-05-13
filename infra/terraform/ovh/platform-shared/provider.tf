provider "ovh" {
  endpoint           = "ovh-eu"
  application_key    = var.ovh_application_key
  application_secret = var.ovh_application_secret
  consumer_key       = var.ovh_consumer_key
}

terraform {
  required_version = ">= 1.12.2"
  required_providers {
    ovh = { source = "ovh/ovh", version = "2.13.1" }
  }
}
