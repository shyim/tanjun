target "sysctl" {
  context = "./sysctl"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["ghcr.io/shyim/tanjun/sysctl:v1"]
}

target "tcp-proxy" {
  context = "./tcp-proxy"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["ghcr.io/shyim/tanjun/tcp-proxy:v1"]
}

target "kv-store" {
  context = "."
  dockerfile = "kv-store/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["ghcr.io/shyim/tanjun/kv-store:v1"]
}

target "scheduler" {
  context = "."
  dockerfile = "scheduler/Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = ["ghcr.io/shyim/tanjun/scheduler:v1"]
}