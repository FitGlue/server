terraform {
  backend "gcs" {
    bucket = "fitglue-server-dev-terraform-state"
    prefix = "terraform/state"
  }
}
