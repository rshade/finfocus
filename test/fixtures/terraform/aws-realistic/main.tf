variable "endpoint" {
  description = "Base URL of the local AWS emulator (moto) every service endpoint points at."
  type        = string
}

provider "aws" {
  region     = "us-east-1"
  access_key = "test"
  secret_key = "test"

  skip_credentials_validation = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true
  skip_region_validation      = true
  s3_use_path_style           = true

  endpoints {
    dynamodb = var.endpoint
    ec2      = var.endpoint
    iam      = var.endpoint
    rds      = var.endpoint
    s3       = var.endpoint
    sts      = var.endpoint
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*"]
  }
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = "finfocus-golden"
  }
}

resource "aws_subnet" "a" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-east-1a"
}

resource "aws_security_group" "web" {
  name   = "finfocus-golden-web"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_instance" "web" {
  ami                    = data.aws_ami.amazon_linux.id
  instance_type          = "t3.micro"
  availability_zone      = "us-east-1a"
  subnet_id              = aws_subnet.a.id
  vpc_security_group_ids = [aws_security_group.web.id]

  tags = {
    Name = "web"
  }
}

module "app" {
  source = "./modules/compute"

  ami            = data.aws_ami.amazon_linux.id
  subnet_id      = aws_subnet.a.id
  instance_type  = "m5.large"
  instance_count = 2
}

resource "aws_instance" "worker" {
  for_each = {
    small = "t3.small"
    large = "c5.xlarge"
  }

  ami               = data.aws_ami.amazon_linux.id
  instance_type     = each.value
  availability_zone = "us-east-1a"
  subnet_id         = aws_subnet.a.id

  tags = {
    Name = "worker-${each.key}"
  }
}

resource "aws_ebs_volume" "data" {
  availability_zone = "us-east-1a"
  size              = 100
  type              = "gp3"
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.web.id
}

resource "aws_s3_bucket" "assets" {
  bucket = "finfocus-golden-assets"
}

resource "aws_iam_role" "app" {
  name = "finfocus-golden-app"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_dynamodb_table" "sessions" {
  name         = "finfocus-golden-sessions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "id"

  attribute {
    name = "id"
    type = "S"
  }
}

resource "aws_db_instance" "main" {
  identifier          = "finfocus-golden-db"
  engine              = "postgres"
  engine_version      = "16.3"
  instance_class      = "db.t3.micro"
  allocated_storage   = 20
  storage_type        = "gp3"
  username            = "finfocus"
  password            = "golden-fixture-not-a-secret"
  skip_final_snapshot = true
}
