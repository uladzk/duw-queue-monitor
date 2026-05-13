variable "project_id" {
  type        = string
  description = "OVH Public Cloud Project ID"
}

variable "env" {
  type        = string
  description = "Environment name (dev or prd) — used in the cluster name suffix"
}

variable "kubernetes_version" {
  type        = string
  description = "Kubernetes minor version (e.g., \"1.33\")"
}

variable "node_flavor" {
  type        = string
  description = "OVH node flavor name (e.g., \"d2-4\", \"b2-7\")"
}

variable "node_count" {
  type        = number
  description = "Fixed node count (autoscale is disabled — min/max/desired all equal this value)"
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
