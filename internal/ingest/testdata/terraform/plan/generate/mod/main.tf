variable "note" { type = string }
resource "local_file" "mod" {
  content  = var.note
  filename = "${path.module}/mod.txt"
}
