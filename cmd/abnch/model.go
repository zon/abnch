package main

import (
	"fmt"
	"sort"
)

// configModel is a model entry in the CLI config. The JSON tags shape the model
// a run passes to an agent.
type configModel struct {
	Provider string `yaml:"provider" json:"provider"`
	Name     string `yaml:"name" json:"name"`
	Variant  string `yaml:"variant" json:"variant"`
}

// modelView is a model as the get commands print it. It carries the stub the
// model is stored under.
type modelView struct {
	Stub     string `yaml:"stub"`
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	Variant  string `yaml:"variant,omitempty"`
}

// getModels writes the models in the CLI config under the working directory as
// YAML, in stub order. An empty config writes nothing.
func getModels(d deps) error {
	models, err := loadConfigModels(d.workDir)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return nil
	}

	stubs := make([]string, 0, len(models))
	for stub := range models {
		stubs = append(stubs, stub)
	}
	sort.Strings(stubs)

	views := make([]modelView, 0, len(stubs))
	for _, stub := range stubs {
		views = append(views, modelViewOf(stub, models[stub]))
	}
	return writeYAML(d.out, views)
}

// getModel writes the model the stub names as YAML. It fails when the model is
// missing.
func getModel(d deps, stub string) error {
	models, err := loadConfigModels(d.workDir)
	if err != nil {
		return err
	}
	model, ok := models[stub]
	if !ok {
		return fmt.Errorf("model %s not found", stub)
	}
	return writeYAML(d.out, modelViewOf(stub, model))
}

// modelViewOf returns the printable view of a model stored under stub.
func modelViewOf(stub string, model configModel) modelView {
	return modelView{Stub: stub, Provider: model.Provider, Name: model.Name, Variant: model.Variant}
}
