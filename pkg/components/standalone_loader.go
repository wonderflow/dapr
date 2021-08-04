// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation and Dapr Contributors.
// Licensed under the MIT License.
// ------------------------------------------------------------

package components

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	components_v1alpha1 "github.com/dapr/dapr/pkg/apis/components/v1alpha1"
	config "github.com/dapr/dapr/pkg/config/modes"
)

const (
	yamlSeparator = "\n---"
	componentKind = "Component"
)

// StandaloneComponents loads components in a standalone mode environment.
type StandaloneComponents struct {
	config config.StandaloneConfig
}

// NewStandaloneComponents returns a new standalone loader.
func NewStandaloneComponents(configuration config.StandaloneConfig) *StandaloneComponents {
	return &StandaloneComponents{
		config: configuration,
	}
}

// LoadComponents loads dapr components from a given directory.
func (s *StandaloneComponents) LoadComponents() ([]components_v1alpha1.Component, error) {
	files, err := ioutil.ReadDir(s.config.ComponentsPath)
	if err != nil {
		return nil, err
	}

	list := []components_v1alpha1.Component{}

	for _, file := range files {
		if !file.IsDir() && s.isYaml(file.Name()) {
			components := s.loadComponentsFromFile(file.Name())
			if len(components) > 0 {
				list = append(list, components...)
			}
		}
	}

	return list, nil
}

func (s *StandaloneComponents) loadComponentsFromFile(filename string) []components_v1alpha1.Component {
	var errors []error

	components := []components_v1alpha1.Component{}
	path := filepath.Join(s.config.ComponentsPath, filename)

	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Warnf("daprd load components error when reading file %s : %s", path, err)
		return components
	}
	components, errors = s.decodeYaml(b)
	for _, err := range errors {
		log.Warnf("daprd load components error when parsing components yaml resource in %s : %s", path, err)
	}
	return components
}

// isYaml checks whether the file is yaml or not.
func (s *StandaloneComponents) isYaml(fileName string) bool {
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension == ".yaml" || extension == ".yml" {
		return true
	}
	return false
}

// decodeYaml decodes the yaml document.
func (s *StandaloneComponents) decodeYaml(b []byte) ([]components_v1alpha1.Component, []error) {
	list := []components_v1alpha1.Component{}
	errors := []error{}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Split(s.splitYamlDoc)

	for {
		var comp components_v1alpha1.Component
		comp.Spec = components_v1alpha1.ComponentSpec{}
		err := s.decode(scanner, &comp)
		if err == io.EOF {
			break
		}

		if err != nil {
			errors = append(errors, err)
			continue
		}

		if comp.Kind != componentKind {
			continue
		}

		list = append(list, comp)
	}

	return list, errors
}

// decode reads the YAML resource in document.
func (s *StandaloneComponents) decode(scanner *bufio.Scanner, c interface{}) error {
	if scanner.Scan() {
		return yaml.Unmarshal(scanner.Bytes(), &c)
	}

	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	return err
}

// splitYamlDoc - splits the yaml docs.
func (s *StandaloneComponents) splitYamlDoc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	sep := len([]byte(yamlSeparator))
	if i := bytes.Index(data, []byte(yamlSeparator)); i >= 0 {
		i += sep
		after := data[i:]

		if len(after) == 0 {
			if atEOF {
				return len(data), data[:len(data)-sep], nil
			}
			return 0, nil, nil
		}
		if j := bytes.IndexByte(after, '\n'); j >= 0 {
			return i + j + 1, data[0 : i-sep], nil
		}
		return 0, nil, nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}
