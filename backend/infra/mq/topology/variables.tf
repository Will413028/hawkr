variable "queue_type" {
  description = "注入每條 durable queue 的 x-queue-type；classic = 完全不注入該 arg（見 main.tf 註解）"
  type        = string
  validation {
    condition     = contains(["classic", "quorum"], var.queue_type)
    error_message = "queue_type must be \"classic\" or \"quorum\"."
  }
}

variable "vhost" {
  type    = string
  default = "/"
}
