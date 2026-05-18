resource "ovh_cloud_project_alerting" "monthly_budget" {
  service_name      = var.project_id
  email             = var.alert_email
  monthly_threshold = var.monthly_threshold_eur
  delay             = 86400
}
