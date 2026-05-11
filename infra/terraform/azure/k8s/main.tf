locals {
  infisical_region = "eu"
}

resource "kubernetes_namespace" "eso" {
  metadata {
    name = "external-secrets"
  }
}

resource "helm_release" "eso" {
  name       = "external-secrets"
  repository = "https://charts.external-secrets.io"
  chart      = "external-secrets"
  version    = "v0.18.2"
  namespace  = kubernetes_namespace.eso.metadata[0].name

  depends_on = [kubernetes_namespace.eso]
}

// TODO: helm release should be applied first to create required CRDs
resource "kubernetes_secret" "infisical_universal_identity" {
  metadata {
    name = "infisical-universal-auth-credentials"
  }
  type = "Opaque"

  data = {
    clientId     = var.aks_eso_infisical_client_id
    clientSecret = var.aks_eso_infisical_client_secret
  }

  depends_on = [helm_release.eso]
}

resource "kubernetes_manifest" "eso_infisical_secret_store" {
  manifest = yamldecode(templatefile("${path.module}/resources/eso-infisical-secret-store.yml", {
    infisical_universal_auth_credentials_secret_name = kubernetes_secret.infisical_universal_identity.metadata[0].name
    infisical_project_slug                           = var.infisical_project_slug
    infisical_environment_slug                       = var.environment
    infisical_region                                 = local.infisical_region
  }))

  depends_on = [kubernetes_secret.infisical_universal_identity]
}

resource "kubernetes_namespace" "cnpg" {
  metadata {
    name = "cnpg-system"
  }
}

resource "helm_release" "cnpg" {
  name       = "cnpg"
  repository = "https://cloudnative-pg.github.io/charts"
  chart      = "cloudnative-pg"
  version    = "0.23.0"
  namespace  = kubernetes_namespace.cnpg.metadata[0].name

  depends_on = [kubernetes_namespace.cnpg]
}
