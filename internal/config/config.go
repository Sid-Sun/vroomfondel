package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config contains all the necessary configurations.
type Config struct {
	Models     map[string]Model
	ModelNames []string
	OpenAIAPI  OpenAI
	OllamaAPI  Ollama
}

func errInvalidBackend(name string) error {
	return fmt.Errorf("invalid backend for model %q: must be openai or ollama", name)
}

// Load reads config from configFile if given, otherwise from the default
// locations: ~/.vroomfondel.yaml (primary) with fallback to
// ~/.config/vroomfondel/{vroomfondel,config}.yaml. It mirrors the
// openwebui-tg loading logic (viper + models map with a "default" entry).
func Load(configFile string) (Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigType("yaml")
		// Primary: ~/.vroomfondel.yaml (SetConfigName without extension
		// requires SetConfigType, set above).
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(home)
			v.AddConfigPath(filepath.Join(home, ".config", "vroomfondel"))
		}
		v.SetConfigName(".vroomfondel")
	}

	if err := v.ReadInConfig(); err != nil {
		// If the primary name missed, try the XDG-style names before giving up.
		if configFile == "" {
			loaded := false
			for _, name := range []string{"vroomfondel", "config"} {
				v.SetConfigName(name)
				if err2 := v.ReadInConfig(); err2 == nil {
					loaded = true
					break
				}
			}
			if !loaded {
				return Config{}, fmt.Errorf("read config: %w (expected ~/.vroomfondel.yaml, see example.yaml)", err)
			}
		} else {
			return Config{}, fmt.Errorf("read config %q: %w", configFile, err)
		}
	}

	v.SetDefault("models", []Model{defaultModel})

	var modelList []Model
	if err := v.UnmarshalKey("models", &modelList); err != nil {
		return Config{}, fmt.Errorf("parse models: %w", err)
	}
	if len(modelList) == 0 {
		return Config{}, fmt.Errorf("no models defined (a model named %q is required)", "default")
	}

	modelNames := make([]string, len(modelList))
	models := make(map[string]Model, len(modelList))
	for i, m := range modelList {
		nm, err := m.Normalize()
		if err != nil {
			return Config{}, err
		}
		modelList[i] = nm
		modelNames[i] = nm.Name
		if _, ok := models[nm.Name]; ok {
			return Config{}, fmt.Errorf("duplicate model name %q", nm.Name)
		}
		models[nm.Name] = nm
	}
	if _, ok := models["default"]; !ok {
		return Config{}, fmt.Errorf("default model not found")
	}

	return Config{
		OpenAIAPI: OpenAI{
			Endpoint: v.GetString("openai.endpoint"),
			APIKey:   v.GetString("openai.api_key"),
		},
		OllamaAPI: Ollama{
			Endpoint: v.GetString("ollama.endpoint"),
		},
		Models:     models,
		ModelNames: modelNames,
	}, nil
}
