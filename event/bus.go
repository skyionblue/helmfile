package event

import (
	"fmt"
	"github.com/roboll/helmfile/environment"
	"github.com/roboll/helmfile/helmexec"
	"github.com/roboll/helmfile/tmpl"
	"go.uber.org/zap"
	"os"
)

type Hook struct {
	Name    string   `yaml:"name"`
	Events  []string `yaml:"events"`
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type event struct {
	Name string
}

type Bus struct {
	Runner helmexec.Runner
	Hooks  []Hook

	BasePath      string
	StateFilePath string
	Namespace     string

	Env environment.Environment

	ReadFile func(string) ([]byte, error)
	Logger   *zap.SugaredLogger
}

func (bus *Bus) Trigger(evt string, context map[string]interface{}) (bool, error) {
	if bus.Runner == nil {
		bus.Runner = helmexec.ShellRunner{
			Dir: bus.BasePath,
		}
	}

	executed := false

	for _, hook := range bus.Hooks {
		contained := false
		for _, e := range hook.Events {
			contained = contained || e == evt
		}
		if !contained {
			continue
		}

		var err error

		name := hook.Name
		if name == "" {
			name = hook.Command
		}

		fmt.Fprintf(os.Stderr, "%s: basePath=%s\n", bus.StateFilePath, bus.BasePath)

		data := map[string]interface{}{
			"Environment": bus.Env,
			"Namespace":   bus.Namespace,
			"Event": event{
				Name: evt,
			},
		}
		for k, v := range context {
			data[k] = v
		}
		render := tmpl.NewTextRenderer(bus.ReadFile, bus.BasePath, data)

		bus.Logger.Debugf("hook[%s]: triggered by event \"%s\"\n", name, evt)

		command, err := render.RenderTemplateText(hook.Command)
		if err != nil {
			return false, fmt.Errorf("hook[%s]: %v", name, err)
		}

		args := make([]string, len(hook.Args))
		for i, raw := range hook.Args {
			args[i], err = render.RenderTemplateText(raw)
			if err != nil {
				return false, fmt.Errorf("hook[%s]: %v", name, err)
			}
		}

		bytes, err := bus.Runner.Execute(command, args, map[string]string{})
		bus.Logger.Debugf("hook[%s]: %s\n", name, string(bytes))

		if err != nil {
			return false, fmt.Errorf("hook[%s]: command `%s` failed: %v", name, command, err)
		}

		executed = true
	}

	return executed, nil
}
