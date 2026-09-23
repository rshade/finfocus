variable "ami" {
  type = string
}

variable "subnet_id" {
  type = string
}

variable "instance_type" {
  type = string
}

variable "instance_count" {
  type = number
}

resource "aws_instance" "app" {
  count = var.instance_count

  ami               = var.ami
  instance_type     = var.instance_type
  availability_zone = "us-east-1a"
  subnet_id         = var.subnet_id

  tags = {
    Name = "app-${count.index}"
  }
}
