// Package terraform runs only the OpenTofu (tofu) binary and computes the
// history identity for a Terraform-compatible directory.
// Terraform core is never executed or required.
//
// JSON parsing of plan documents stays in internal/ingest. This package
// decides whether a working directory is a Terraform root, runs plan and
// show, and hashes the git origin into a history key.
package terraform
