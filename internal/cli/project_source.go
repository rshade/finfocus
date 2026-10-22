package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/terraform"
)

// ProjectKind is the nearer project marker in a working directory.
type ProjectKind string

const (
	// ProjectKindPulumi means a Pulumi project file is the nearer marker.
	ProjectKindPulumi ProjectKind = "pulumi"
	// ProjectKindTerraform means a Terraform root is the nearer marker.
	ProjectKindTerraform ProjectKind = "terraform"
)

// ProjectSource is the project marker selected for a working directory.
type ProjectSource struct {
	Kind ProjectKind
	Dir  string
}

// ErrNoProjectMarker is returned when the working directory has neither a
// Pulumi project nor a Terraform root.
var ErrNoProjectMarker = errors.New("no Pulumi project or Terraform root found")

// DetectProjectSource chooses the nearer marker. A Pulumi file in the working
// directory wins over Terraform files there. Terraform files in the working
// directory win over a Pulumi file that exists only in a parent.
func DetectProjectSource(dir string) (ProjectSource, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ProjectSource{}, fmt.Errorf("resolving project directory: %w", err)
	}
	pulumiDir, pulumiErr := pulumi.FindProject(absDir)
	if pulumiErr != nil && !errors.Is(pulumiErr, pulumi.ErrNoProject) {
		return ProjectSource{}, fmt.Errorf("searching for a Pulumi project: %w", pulumiErr)
	}
	hasTerraform, tfErr := terraform.HasRoot(absDir)
	if tfErr != nil {
		return ProjectSource{}, tfErr
	}
	if pulumiErr == nil && pulumiDir == absDir {
		return ProjectSource{Kind: ProjectKindPulumi, Dir: pulumiDir}, nil
	}
	if hasTerraform {
		return ProjectSource{Kind: ProjectKindTerraform, Dir: absDir}, nil
	}
	if pulumiErr == nil {
		return ProjectSource{Kind: ProjectKindPulumi, Dir: pulumiDir}, nil
	}
	return ProjectSource{}, ErrNoProjectMarker
}
