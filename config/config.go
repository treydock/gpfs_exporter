package config

import (
	"fmt"
	"github.com/prometheus/common/log"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"sync"
)

type Targets struct {
	Targets []Target `yaml:"targets"`
}

type Target struct {
	sync.Mutex
	Name       string   `yaml:"name"`
	FSMounts   []string `yaml:"fs_mounts"`
	Collectors []string `yaml:"collectors"`
}

func (targets *Targets) LoadConfig(configFile string) error {
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Errorf("Error reading config file: %s", err)
		return err
	}
	if err := yaml.Unmarshal(yamlFile, targets); err != nil {
		log.Errorf("Error parsing config file: %s", err)
		return err
	}
	log.Infof("Loaded config file %s targets %d", configFile, len(targets.Targets))
	return nil
}

func (targets *Targets) GetTarget(target string) (Target, error) {
	for _, t := range targets.Targets {
		if t.Name == target {
			log.Debugf("GetTarget target=%s name=%s", target, t.Name)
			return t, nil
		}
	}
	if target == "default" {
		log.Debug("Using default target as one was not found")
		return Target{}, nil
	}
	return Target{}, fmt.Errorf("Target %s not found", target)
}
