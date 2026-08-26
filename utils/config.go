package utils

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/projecteru2/agent/types"
)

type fieldWalker func(reflect.Value) error

// LoadConfig loads the agent config from the YAML file at configPath.
func LoadConfig(configPath string) (*types.Config, error) {
	config := &types.Config{}
	value := reflect.ValueOf(config).Elem()
	if err := applyDefaults(value); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // the config path comes from the operator's own flag
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}
	if err = checkRequired(value); err != nil {
		return nil, err
	}
	if err = checkIntervals(config); err != nil {
		return nil, err
	}
	return config, nil
}

func checkIntervals(config *types.Config) error {
	for name, value := range map[string]int64{
		"heartbeat_interval":        int64(config.HeartbeatInterval),
		"healthcheck.interval":      int64(config.HealthCheck.Interval),
		"healthcheck.timeout":       int64(config.HealthCheck.Timeout),
		"metrics.step":              config.Metrics.Step,
		"global_connection_timeout": int64(config.GlobalConnectionTimeout),
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	return nil
}

func applyDefaults(value reflect.Value) error {
	for i := range value.NumField() {
		field := value.Field(i)
		structField := value.Type().Field(i)
		if tag := structField.Tag.Get("default"); tag != "" && field.IsZero() {
			if err := yaml.Unmarshal([]byte(tag), field.Addr().Interface()); err != nil {
				return fmt.Errorf("bad default for %s: %w", structField.Name, err)
			}
		}
		if err := walkNested(field, applyDefaults); err != nil {
			return err
		}
	}
	return nil
}

func checkRequired(value reflect.Value) error {
	for i := range value.NumField() {
		field := value.Field(i)
		if structField := value.Type().Field(i); structField.Tag.Get("required") == "true" && field.IsZero() {
			return fmt.Errorf("%s is required, but blank", structField.Name)
		}
		if err := walkNested(field, checkRequired); err != nil {
			return err
		}
	}
	return nil
}

func walkNested(field reflect.Value, walk fieldWalker) error {
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.Struct:
		return walk(field)
	case reflect.Slice:
		for i := range field.Len() {
			if elem := reflect.Indirect(field.Index(i)); elem.Kind() == reflect.Struct {
				if err := walk(elem); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
