package pulumi

import (
	"errors"
	"os"
	"strings"
)

// StackContext is what finfocus can know about a stack without the Pulumi CLI.
type StackContext struct {
	ProjectDir  string
	ProjectName string
	BackendURL  string
	Stack       string
	Environment StackEnvironment
}

// LoadStackContext reads the project file, the selected stack, and that
// stack's environment imports. It does not run the Pulumi CLI.
// stackFlag wins over the workspace file when it is not empty.
func LoadStackContext(projectDir, stackFlag string) (StackContext, error) {
	if strings.TrimSpace(projectDir) == "" {
		return StackContext{}, errProjectDirRequired
	}
	project, err := readProjectFile(projectDir)
	if err != nil {
		return StackContext{}, err
	}
	stack := strings.TrimSpace(stackFlag)
	if stack == "" {
		selected, selErr := SelectedStack(projectDir)
		if selErr != nil {
			return StackContext{}, selErr
		}
		stack = selected
	}
	env, err := loadEnvironment(projectDir, stack)
	if err != nil {
		return StackContext{}, err
	}
	return StackContext{
		ProjectDir:  projectDir,
		ProjectName: project.Name,
		BackendURL:  project.Backend.URL,
		Stack:       stack,
		Environment: env,
	}, nil
}

func loadEnvironment(projectDir, stack string) (StackEnvironment, error) {
	if stack == "" {
		return StackEnvironment{Form: StackEnvAbsent}, nil
	}
	data, err := ReadStackSettingsFile(projectDir, stack)
	if err != nil {
		return absentWhenMissing(err)
	}
	return ParseStackEnvironment(data)
}

func absentWhenMissing(err error) (StackEnvironment, error) {
	if errors.Is(err, os.ErrNotExist) {
		return StackEnvironment{Form: StackEnvAbsent}, nil
	}
	return StackEnvironment{}, err
}
