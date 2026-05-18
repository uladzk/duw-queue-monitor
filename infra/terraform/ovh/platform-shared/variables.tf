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

variable "alert_email" {
  type        = string
  description = "Email address that receives OVH Public Cloud project cost alerts"
}

variable "monthly_threshold_eur" {
  type        = number
  description = "Monthly spend threshold (EUR) above which OVH sends a cost alert"
  default     = 15
}
