/*
	the reader for the job file, the YAML that lists the steps qahyacli run should execute
*/
package runner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Job is a list of shell snippets. Steps run in the job file's directory, so
// relative paths inside them mean the same thing wherever `qahyacli run` is called.
type Job struct {
	Dir   string
	Steps []Step
}

type Step struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
}

func LoadJob(path string) (*Job, error) {
	abs, err := filepath.Abs(path)
	if err != nil { return nil, err }
	
	data, err := os.ReadFile(abs)
	if err != nil { return nil, fmt.Errorf("read job: %w", err) }

	var raw struct {
		Steps	[]Step	`yaml:"steps"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil { return nil, fmt.Errorf("parse %s: %w", path, err) }
	if len(raw.Steps) == 0 { return nil, fmt.Errorf("%s: no steps", path) }

	for i := range raw.Steps {
		if raw.Steps[i].Run == "" {
			return  nil, fmt.Errorf("%s: step %d has no run",path, i+1)
		}
		if raw.Steps[i].Name == "" {
			raw.Steps[i].Name = fmt.Sprintf("step-%d", i + 1)
		}
	}
	return &Job{Dir: filepath.Dir(abs), Steps: raw.Steps}, nil
}
