# Stockyard on Docker / any cloud
resource "docker_container" "stockyard" {
  name  = "stockyard"
  image = "stockyard/stockyard:latest"

  ports {
    internal = 4000
    external = 4000
  }

  env = [
    "OPENAI_API_KEY=${var.openai_key}",
  ]

  volumes {
    host_path      = "/opt/stockyard/data"
    container_path = "/data"
  }
}

variable "openai_key" {
  type      = string
  sensitive = true
}
