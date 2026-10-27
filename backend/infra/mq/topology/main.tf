terraform {
  required_version = ">= 1.9.0"
  required_providers {
    rabbitmq = {
      source  = "cyrilgdn/rabbitmq"
      version = "~> 1.8"
    }
  }
}

locals {
  src = yamldecode(file("${path.module}/topology.yaml"))

  declared_queues      = [for q in local.src.queues : q if !try(q.args_json, false)]
  declared_json_queues = [for q in local.src.queues : q if try(q.args_json, false)]

  retry_queues = [for q in local.src.queues : q if can(q.retry)]

  own_dlq_queues = [
    for q in local.src.queues : q
    if can(q.dlq) && !can(q.dlq.queue)
  ]

  queue_arguments = {
    for q in local.src.queues : q.name => merge(
      try(q.arguments, {}),
      can(q.dlq) ? merge(
        { "x-dead-letter-exchange" = q.dlq.via },
        can(q.dlq.routing_key) ? { "x-dead-letter-routing-key" = q.dlq.routing_key } : {}
      ) : {},
      can(q.retry) ? { "x-dead-letter-exchange" = "${q.name}.dlx" } : {}
    )
  }

  all_exchanges = concat(
    [for e in local.src.exchanges : { name = e.name, type = e.type }],
    [for q in local.retry_queues : { name = "${q.name}.retry", type = "fanout" }],
    [for q in local.retry_queues : { name = "${q.name}.dlx", type = "fanout" }],
  )

  all_queues = concat(
    [for q in local.declared_queues : { name = q.name, arguments = local.queue_arguments[q.name] }],
    [for q in local.retry_queues : { name = "${q.name}.dlq", arguments = {} }],
    [for q in local.own_dlq_queues : { name = "${q.name}.dlq", arguments = {} }],
  )

  all_bindings = concat(
    flatten([
      for q in local.src.queues : [
        for b in try(q.bindings, []) : {
          source      = b.exchange
          destination = q.name
          routing_key = b.key
        }
      ]
    ]),
    [for q in local.retry_queues : {
      source      = "${q.name}.retry"
      destination = q.name
      routing_key = ""
    }],
    [for q in local.retry_queues : {
      source      = "${q.name}.dlx"
      destination = "${q.name}.dlq"
      routing_key = ""
    }],
    [for q in local.own_dlq_queues : {
      source      = q.dlq.via
      destination = "${q.name}.dlq"
      routing_key = "${q.name}.dlq"
    }],
  )
}

resource "rabbitmq_exchange" "this" {
  for_each = { for e in local.all_exchanges : e.name => e }

  name  = each.value.name
  vhost = var.vhost

  settings {
    type        = each.value.type
    durable     = true
    auto_delete = false
  }
}

resource "rabbitmq_queue" "this" {
  for_each = { for q in local.all_queues : q.name => q }

  name  = each.value.name
  vhost = var.vhost

  settings {
    durable     = true
    auto_delete = false
    arguments = (
      var.queue_type == "classic"
      ? each.value.arguments
      : merge(each.value.arguments, { "x-queue-type" = var.queue_type })
    )
  }
}

resource "rabbitmq_queue" "json_args" {
  for_each = { for q in local.declared_json_queues : q.name => q }

  name  = each.value.name
  vhost = var.vhost

  settings {
    durable     = true
    auto_delete = false
    arguments_json = (
      var.queue_type == "classic"
      ? jsonencode(local.queue_arguments[each.key])
      : jsonencode(merge(local.queue_arguments[each.key], { "x-queue-type" = var.queue_type }))
    )
  }
}

resource "rabbitmq_binding" "this" {
  for_each = {
    for b in local.all_bindings :
    "${b.source}|queue|${b.destination}|${b.routing_key}" => b
  }

  source           = each.value.source
  vhost            = var.vhost
  destination      = each.value.destination
  destination_type = "queue"
  routing_key      = each.value.routing_key

  depends_on = [rabbitmq_exchange.this, rabbitmq_queue.this, rabbitmq_queue.json_args]
}
