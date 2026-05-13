variable "project_id" {
  type        = string
  description = "OVH Public Cloud Project ID (UUID-like identifier from the OVH Manager)"
}

variable "region" {
  type        = string
  description = "OVH region (e.g., \"WAW\")"
}

variable "ovh_application_key" {
  type        = string
  description = "OVH API application key"
  sensitive   = true
}

variable "ovh_application_secret" {
  type        = string
  description = "OVH API application secret"
  sensitive   = true
}

variable "ovh_consumer_key" {
  type        = string
  description = "OVH API consumer key"
  sensitive   = true
}
