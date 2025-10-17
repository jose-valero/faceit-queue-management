resource "aws_cloudwatch_log_group" "ecs-cluster" {
  name              = "/aws/ecs/${var.name}-${var.env}"
  retention_in_days = 30
}

resource "aws_cloudwatch_log_group" "postgres" {
  name              = "/aws/ecs/postgres"
  retention_in_days = 30
}

resource "aws_ecs_cluster" "main" {
  name = "ECS-faceit-infra"

  configuration {
    execute_command_configuration {
      logging = "OVERRIDE"
      log_configuration {
        cloud_watch_log_group_name = aws_cloudwatch_log_group.ecs-cluster.name
      }
    }
  }

  tags = {
    Environment = var.env
    Project     = var.name
  }
}

resource "aws_ecs_service" "faceitbotapp" {
  name                               = "faceit-bot"
  cluster                            = aws_ecs_cluster.main.id
  task_definition                    = aws_ecs_task_definition.app.arn
  desired_count                      = 1
  capacity_provider_strategy {
    capacity_provider = aws_ecs_capacity_provider.main.name
    weight            = 100
    base              = 1
  }
  enable_execute_command             = true
  deployment_maximum_percent         = 100
  deployment_minimum_healthy_percent = 1

}

resource "aws_ecs_service" "postgress" {
  name                               = "PostgresSQL"
  cluster                            = aws_ecs_cluster.main.id
  task_definition                    = aws_ecs_task_definition.postgres.arn
  desired_count                      = 1
  capacity_provider_strategy {
    capacity_provider = aws_ecs_capacity_provider.main.name
    weight            = 100
    base              = 1
  }
  enable_execute_command             = true
  deployment_maximum_percent         = 100
  deployment_minimum_healthy_percent = 1

}



resource "aws_ecs_task_definition" "app" {
  family                   = "${terraform.workspace}-${var.cluster_name}-app"
  network_mode             = "bridge"
  requires_compatibilities = ["EC2"]
  task_role_arn            = aws_iam_role.ecs_instance_role.arn
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  cpu                      = "512"
  memory                   = "128"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  container_definitions = jsonencode([
    {
      name      = "app"
      image     = "${aws_ecr_repository.app.repository_url}:latest"
      cpu       = 512
      memory    = 128
      essential = true
      portMappings = [
        {
          containerPort = 8080
          hostPort      = 8080
          protocol      = "tcp"
        }
      ]
      environment = [
        {
          name  = "DISCORD_BOT_TOKEN"
          value = var.discord_bot_token
        },
        {
          name  = "DISCORD_GUILD_ID"
          value = var.discord_guild_id
        },
        {
          name  = "DISCORD_APP_ID"
          value = var.discord_app_id
        },
        {
          name  = "VOICE_CATEGORY_ID"
          value = var.voice_category_id
        },
        {
          name  = "AFK_CHANNEL_ID"
          value = var.afk_channel_id
        },
        {
          name  = "ADMIN_ROLE_IDS"
          value = var.admin_role_ids
        },
        {
          name  = "FACEIT_API_KEY"
          value = var.faceit_api_key
        },
        {
          name  = "FACEIT_HUB_ID"
          value = var.faceit_hub_id
        },
        {
          name  = "WEBHOOK_HEADER_VALUE"
          value = var.webhook_header_value
        },
        {
          name  = "WEBHOOK_HEADER_NAME"
          value = var.webhook_header_name
        },
        {
          name  = "FACEIT_OAUTH_CLIENT_ID"
          value = var.faceit_oauth_client_id
        },
        {
          name  = "FACEIT_OAUTH_CLIENT_SECRET"
          value = var.faceit_oauth_client_secret
        },
        {
          name  = "FACEIT_OAUTH_REDIRECT_URL"
          value = var.faceit_oauth_redirect_url
        },
        {
          name  = "DB_HOST"
          value = "3.86.143.98"
        },
        {
          name  = "DB_PORT"
          value = "5432"
        },
        {
          name  = "DB_USER"
          value = var.postgres_user
        },
        {
          name  = "DB_PASSWORD"
          value = var.postgres_password
        },
        {
          name  = "DB_NAME"
          value = "faceit_db"
        },
        {
          name  = "DATABASE_URL"
          value = "postgres://${var.postgres_user}:${var.postgres_password}@3.86.143.98:5432/faceit_db"
        },
        {
          name  = "HTTP_ADDR"
          value = var.http_addr
        }
      ]
    }
  ])
}

resource "aws_ecs_task_definition" "postgres" {
  family                   = "${terraform.workspace}-${var.cluster_name}-postgres"
  network_mode             = "bridge"
  requires_compatibilities = ["EC2"]
  task_role_arn            = aws_iam_role.ecs_instance_role.arn
  execution_role_arn       = aws_iam_role.ecs_task_execution_role.arn
  cpu                      = "1024"
  memory                   = "256"
  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "ARM64"
  }
  volume {
    name      = "db-data"
    host_path = "/opt/postgres-data"
  }

  container_definitions = jsonencode([
    {
      name      = "postgres"
      image     = "${aws_ecr_repository.postgres.repository_url}:17.6-alpine"
      cpu       = 1024
      memory    = 256
      essential = true
      portMappings = [
        {
          containerPort = 5432
          hostPort      = 5432
          protocol      = "tcp"
        }
      ]
      mountPoints = [
        {
          sourceVolume  = "db-data"
          containerPath = "/var/lib/postgresql/data"
        }
      ]
      environment = [
        {
          name  = "POSTGRES_DB"
          value = "faceit_db"
        },
        {
          name  = "POSTGRES_USER"
          value = var.postgres_user
        },
        {
          name  = "POSTGRES_PASSWORD"
          value = var.postgres_password
        },
        {
          name  = "PGDATA"
          value = "/var/lib/postgresql/data/pgdata"
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-region"        = "us-east-1"
          "awslogs-group"         = aws_cloudwatch_log_group.postgres.name
          "awslogs-stream-prefix" = "postgres"
        }
      }
    }
  ])
}

