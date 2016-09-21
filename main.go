package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"syscall"
	"text/template"

	log "github.com/Sirupsen/logrus"
	bunyan "github.com/mumoshu/logrus-bunyan-formatter"

	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"./cmd"
	"./env"
	"./file"
)

func init() {
	log.SetOutput(os.Stdout)

	verbose := false
	for _, e := range os.Environ() {
		if strings.Contains(e, "VERBOSE=") {
			verbose = true
			break
		}
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func ParseEnviron() map[string]string {
	mergedEnv := map[string]string{}

	for _, pair := range os.Environ() {
		splits := strings.SplitN(pair, "=", 2)
		key, value := splits[0], splits[1]
		mergedEnv[key] = value
	}

	return mergedEnv
}

type Parameter struct {
	Name     string `yaml:"name,omitempty"`
	Value    string `yaml:"value,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`
}

type Input struct {
	Name        string               `yaml:"name,omitempty"`
	Parameters  map[string]Parameter `yaml:"parameters,omitempty"`
	Description string               `yaml:"description,omitempty"`
	Candidates  []string             `yaml:"candidates,omitempty"`
	Complete    string               `yaml:"complete,omitempty"`
}

type Variable struct {
	FlowKey     FlowKey
	FullName    string
	Name        string
	Parameters  map[string]Parameter
	Description string
	Candidates  []string
	Complete    string
}

func (v *Variable) ShortName() string {
	return strings.SplitN(v.FullName, ".", 2)[1]
}

type FlowConfig struct {
	Name        string        `yaml:"name,omitempty"`
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
	Steps       []Step        `yaml:"steps,omitempty"`
	Autoenv     bool          `yaml:"autoenv,omitempty"`
	Autodir     bool          `yaml:"autodir,omitempty"`
	Interactive bool          `yaml:"interactive,omitempty"`
}

type FlowConfigV1 struct {
	Name        string        `yaml:"name,omitempty"`
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
	StepConfigs []*StepConfig `yaml:"steps,omitempty"`
	Autoenv     bool          `yaml:"autoenv,omitempty"`
	Autodir     bool          `yaml:"autodir,omitempty"`
	Interactive bool          `yaml:"interactive,omitempty"`
}

type FlowConfigV2 struct {
	Description string                 `yaml:"description,omitempty"`
	Inputs      []*Input               `yaml:"inputs,omitempty"`
	FlowConfigs map[string]*FlowConfig `yaml:"flows,omitempty"`
	Script      string                 `yaml:"script,omitempty"`
	StepConfigs []*StepConfig          `yaml:"steps,omitempty"`
	Autoenv     bool                   `yaml:"autoenv,omitempty"`
	Autodir     bool                   `yaml:"autodir,omitempty"`
	Interactive bool                   `yaml:"interactive,omitempty"`
}

type StepConfig struct {
	Script interface{} `yaml:"script,omitempty"`
	Flow   interface{} `yaml:"flow,omitempty"`
}

type Step interface {
	GetName() string
	Run(project *Project, flow *Flow, parent ...FlowDef) (StepStringOutput, error)
}

type StepStringOutput struct {
	String string
}

type ScriptStep struct {
	Name string
	Code string
}

type FlowStep struct {
	Name          string
	FlowKeyString string
}

func (t *FlowConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	v3 := map[string]interface{}{}
	v3err := unmarshal(&v3)

	log.Debugf("Unmarshalling: %v", v3)

	log.Debugf("Trying to parse v1 format")

	v1 := FlowConfigV1{
		Autoenv:     false,
		Autodir:     false,
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		StepConfigs: []*StepConfig{},
	}

	err := unmarshal(&v1)

	if v1.Name == "" && len(v1.FlowConfigs) == 0 {
		e := fmt.Errorf("Not v1 format: Both `name` and `flows` are empty")
		log.Debugf("%s", e)
		err = e
	}

	if err == nil {
		t.Name = v1.Name
		t.Description = v1.Description
		t.Inputs = v1.Inputs
		t.FlowConfigs = v1.FlowConfigs
		t.Script = v1.Script
		t.Autoenv = v1.Autoenv
		t.Autodir = v1.Autodir
		t.Interactive = v1.Interactive
		steps, err := readStepsFromStepConfigs(v1.Script, v1.StepConfigs)
		if err != nil {
			return errors.Annotatef(err, "Error while reading v1 config")
		}
		t.Steps = steps
	}

	var v2 *FlowConfigV2

	if err != nil {
		log.Debugf("Trying to parse v2 format")
		v2 = &FlowConfigV2{
			Autoenv:     false,
			Autodir:     false,
			Interactive: false,
			Inputs:      []*Input{},
			FlowConfigs: map[string]*FlowConfig{},
			StepConfigs: []*StepConfig{},
		}

		err = unmarshal(&v2)

		if len(v2.FlowConfigs) == 0 && v2.Script == "" && len(v2.StepConfigs) == 0 {
			e := fmt.Errorf("Not v2 format: `flows`, `script`, `steps` are missing.")
			log.Debugf("%s", e)
			err = e
		}

		if err == nil {
			t.Description = v2.Description
			t.Inputs = v2.Inputs
			t.FlowConfigs = TransformV2FlowConfigMapToArray(v2.FlowConfigs)
			steps, err := readStepsFromStepConfigs(v2.Script, v2.StepConfigs)
			if err != nil {
				return errors.Annotatef(err, "Error while reading v2 config")
			}
			t.Steps = steps
			t.Script = v2.Script
			t.Autoenv = v2.Autoenv
			t.Autodir = v2.Autodir
			t.Interactive = v2.Interactive
		}

	}

	if err != nil {
		log.Debugf("Trying to parse v3 format")

		if v3err != nil {
			return errors.Annotate(v3err, "Failed to unmarshal as a map.")
		}

		if v3["flows"] != nil {
			rawFlows, ok := v3["flows"].(map[interface{}]interface{})
			if !ok {
				return fmt.Errorf("Not a map[interface{}]interface{}: v3[\"flows\"]'s type: %s, value: %s", reflect.TypeOf(v3["flows"]), v3["flows"])
			}
			flows, err := CastKeysToStrings(rawFlows)
			if err != nil {
				return errors.Annotate(err, "Failed to unmarshal as a map[string]interface{}")
			}
			t.Autoenv = false
			t.Autodir = false
			t.Inputs = []*Input{}

			t.FlowConfigs = TransformV3FlowConfigMapToArray(flows)

			return nil
		}
	}

	return errors.Trace(err)
}

func (t *FlowConfig) CopyTo(other *FlowConfig) {
	other.Description = t.Description
	other.Inputs = t.Inputs
	other.FlowConfigs = t.FlowConfigs
	other.Steps = t.Steps
	other.Script = t.Script
	other.Autoenv = t.Autoenv
	other.Autodir = t.Autodir
	other.Interactive = t.Interactive
}

func TransformV2FlowConfigMapToArray(v2 map[string]*FlowConfig) []*FlowConfig {
	result := []*FlowConfig{}
	for name, t2 := range v2 {
		t := &FlowConfig{}

		t.Name = name
		t2.CopyTo(t)

		result = append(result, t)
	}
	return result
}

func readStepsFromStepConfigs(script string, stepConfigs []*StepConfig) ([]Step, error) {
	result := []Step{}

	if script != "" {
		if len(stepConfigs) > 0 {
			return nil, fmt.Errorf("both script and steps exist.")
		}
		result = []Step{ScriptStep{
			Name: "script",
			Code: script,
		}}
	} else {

		for i, stepConfig := range stepConfigs {
			var step Step

			name := fmt.Sprintf("step-%d", i+1)

			if flowKey, isStr := stepConfig.Flow.(string); isStr && flowKey != "" {
				step = FlowStep{
					Name:          name,
					FlowKeyString: flowKey,
				}
			} else if code, isStr := stepConfig.Script.(string); isStr && code != "" {
				step = ScriptStep{
					Name: name,
					Code: code,
				}
			} else {
				return nil, fmt.Errorf("Error reading step[%d]: field named `flow` or `script` doesn't exist")
			}

			result = append(result, step)
		}
	}

	return result, nil
}

func CastKeysToStrings(m map[interface{}]interface{}) (map[string]interface{}, error) {
	r := map[string]interface{}{}
	for k, v := range m {
		str, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("Unexpected type %s for key %s", reflect.TypeOf(k), k)
		}
		r[str] = v
	}
	return r, nil
}

func TransformV3FlowConfigMapToArray(v3 map[string]interface{}) []*FlowConfig {
	result := []*FlowConfig{}
	for k, v := range v3 {
		t := &FlowConfig{
			Autoenv:     false,
			Autodir:     false,
			Inputs:      []*Input{},
			FlowConfigs: []*FlowConfig{},
		}

		log.Debugf("Arrived %s: %v", k, v)
		log.Debugf("Type of value: %v", reflect.TypeOf(v))

		t.Name = k

		var err error

		i2i, ok := v.(map[interface{}]interface{})

		if !ok {
			panic(fmt.Errorf("Not a map[interface{}]interface{}: %s", v))
		}

		s2i, err := CastKeysToStrings(i2i)

		if err != nil {
			panic(errors.Annotate(err, "Unexpected structure"))
		}

		leaf := s2i["script"] != nil

		if !leaf {
			t.FlowConfigs = TransformV3FlowConfigMapToArray(s2i)
		} else {
			log.Debugf("Not a nested map")
			err = mapstructure.Decode(s2i, t)
			if err != nil {
				panic(errors.Trace(err))
			}
			log.Debugf("Loaded %v", t)
		}

		result = append(result, t)
	}
	return result
}

func (p *Project) AllVariables(flowDef *FlowDef) []*Variable {
	return p.CollectVariablesRecursively(flowDef.Key, "")
}

func (p *Project) CollectVariablesRecursively(currentFlowKey FlowKey, path string) []*Variable {
	result := []*Variable{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentFlowKey.String())})

	currentFlowDef, err := p.FindFlowDef(currentFlowKey)

	if err != nil {
		allFlowDefs := []string{}
		for _, t := range p.FlowDefs {
			allFlowDefs = append(allFlowDefs, t.Key.String())
		}
		ctx.Debugf("is not a FlowDef in: %v", allFlowDefs)
		return []*Variable{}
	}

	for _, input := range currentFlowDef.Inputs {
		childKey := p.CreateFlowKeyFromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := p.CollectVariablesRecursively(childKey, fmt.Sprintf("%s.", currentFlowKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &Variable{
			FlowKey:     currentFlowKey,
			FullName:    fmt.Sprintf("%s.%s", currentFlowKey.String(), input.Name),
			Name:        input.Name,
			Parameters:  input.Parameters,
			Description: input.Description,
			Candidates:  input.Candidates,
			Complete:    input.Complete,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "flow": variable.FlowKey.String()}).Debugf("has var %s. short=%s", variable.Name, variable.ShortName())

		result = append(result, variable)
	}

	return result
}

func newDefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
	}
}

func newDefaultFlowConfig() *FlowConfig {
	return &FlowConfig{
		Inputs:      []*Input{},
		FlowConfigs: []*FlowConfig{},
		Autoenv:     false,
		Steps:       []Step{},
	}
}

type ProjectConfig struct {
	Name        string
	Description string        `yaml:"description,omitempty"`
	Inputs      []*Input      `yaml:"inputs,omitempty"`
	FlowConfigs []*FlowConfig `yaml:"flows,omitempty"`
	Script      string        `yaml:"script,omitempty"`
}

type FlowDef struct {
	Key         FlowKey
	ProjectName string
	Steps       []Step
	Inputs      []*Input
	Variables   []*Variable
	Autoenv     bool
	Autodir     bool
	Interactive bool
	FlowConfig  *FlowConfig
	Command     *cobra.Command
}

type Flow struct {
	Key         FlowKey
	ProjectName string
	Steps       []Step
	Vars        map[string]interface{}
	Autoenv     bool
	Autodir     bool
	Interactive bool
	FlowDef     *FlowDef
}

type FlowKey struct {
	Components []string
}

type Project struct {
	Name                string
	CommandRelativePath string
	FlowDefs            map[string]*FlowDef
	CachedFlowOutputs   map[string]interface{}
	Verbose             bool
	Output              string
	Env                 string
}

type T struct {
	A string
	B struct {
		RenamedC int   `yaml:"c"`
		D        []int `yaml:",flow"`
	}
}

func (t Flow) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Vars, toEnvName)
}

func (t Flow) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
	logger := log.WithFields(log.Fields{"path": path})
	result := map[string]string{}
	for k, v := range env {
		if nestedEnv, ok := v.(map[string]interface{}); ok {
			nestedResult, err := t.GenerateAutoenvRecursively(fmt.Sprintf("%s.", k), nestedEnv, toEnvName)
			if err != nil {
				logger.Errorf("Error while recursiong: %v", err)
			}
			for k, v := range nestedResult {
				result[k] = v
			}
		} else if nestedEnv, ok := v.(map[string]string); ok {
			for k2, v := range nestedEnv {
				result[toEnvName(fmt.Sprintf("%s%s.%s", path, k, k2))] = v
			}
		} else if ary, ok := v.([]string); ok {
			for i, v := range ary {
				result[toEnvName(fmt.Sprintf("%s%s.%d", path, k, i))] = v
			}
		} else {
			if stringV, ok := v.(string); ok {
				result[toEnvName(fmt.Sprintf("%s%s", path, k))] = stringV
			} else {
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v", k, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

func (t ScriptStep) RunCommand(command string, depended bool, parentFlow *Flow) (string, error) {
	c := "sh"
	args := []string{"-c", command}
	log.Debugf("running command: %s", command)
	log.Debugf("shelling out: %v", append([]string{c}, args...))

	l := log.WithFields(log.Fields{"command": command})

	cmd := exec.Command(c, args...)

	mergedEnv := map[string]string{}

	for _, pair := range os.Environ() {
		splits := strings.SplitN(pair, "=", 2)
		key, value := splits[0], splits[1]
		mergedEnv[key] = value
	}

	if parentFlow.Autoenv {
		l.Debugf("Autoenv is enabled")
		autoEnv, err := parentFlow.GenerateAutoenv()
		if err != nil {
			log.Errorf("Failed to generate autoenv: %v", err)
		}
		for name, value := range autoEnv {
			mergedEnv[name] = value
		}

		cmdEnv := []string{}
		for name, value := range mergedEnv {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", name, value))
		}

		cmd.Env = cmdEnv

	} else {
		l.Debugf("Autoenv is disabled")
	}

	if parentFlow.Autodir {
		l.Debugf("Autodir is enabled")
		parentKey, err := parentFlow.Key.Parent()
		if parentKey != nil {
			l.Debugf("full: %s", parentKey.String())
			shortKey := parentKey.ShortString()
			l.Debugf("short: %s", shortKey)
			path := strings.Replace(shortKey, ".", "/", -1)
			l.Debugf("Dir: %s", path)
			if err != nil {
				l.Debugf("%s does not have parent", parentFlow.Key.String())
			} else {
				if _, err := os.Stat(path); err == nil {
					cmd.Dir = path
				}
			}
		}
	} else {
		l.Debugf("Autodir is disabled")
	}

	output := ""

	if parentFlow.Interactive {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Start the command
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
			os.Exit(1)
		}
	} else {
		// Pipes

		cmdReader, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
			os.Exit(1)
		}

		errReader, err := cmd.StderrPipe()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating StderrPipe for Cmd", err)
			os.Exit(1)
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
			os.Exit(1)
		}

		// Receive stdout and stderr

		channels := struct {
			Stdout chan string
			Stderr chan string
		}{
			Stdout: make(chan string),
			Stderr: make(chan string),
		}

		scanner := bufio.NewScanner(cmdReader)
		go func() {
			defer func() {
				close(channels.Stdout)
			}()
			for scanner.Scan() {
				text := scanner.Text()
				channels.Stdout <- text
				if output != "" {
					output += "\n"
				}
				output += text
			}
		}()

		errScanner := bufio.NewScanner(errReader)
		go func() {
			defer func() {
				close(channels.Stderr)
			}()
			for errScanner.Scan() {
				text := errScanner.Text()
				channels.Stderr <- text
			}
		}()

		stdoutEnds := false
		stderrEnds := false

		stdoutlog := log.WithFields(log.Fields{"prefix": "stdout"})

		// Coordinating stdout/stderr in this single place to not screw up message ordering
		for {
			select {
			case text, ok := <-channels.Stdout:
				if ok {
					if depended {
						stdoutlog.Debug(text)
					} else {
						stdoutlog.Info(text)
					}
				} else {
					stdoutEnds = true
				}
			case text, ok := <-channels.Stderr:
				if ok {
					l.WithFields(log.Fields{"stream": "stderr"}).Errorf("%s", text)
				} else {
					stderrEnds = true
				}
			}
			if stdoutEnds && stderrEnds {
				break
			}
		}
	}

	var waitStatus syscall.WaitStatus
	err := cmd.Wait()

	if err != nil {
		l.Fatalf("cmd.Wait: %v", err)
		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			print([]byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
	} else {
		// Command was successful
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		l.Debugf("exit status: %d", waitStatus.ExitStatus())
	}

	return strings.Trim(output, "\n "), nil
}

type MessageOnlyFormatter struct {
}

func (f *MessageOnlyFormatter) Format(entry *log.Entry) ([]byte, error) {
	return append([]byte(entry.Message), '\n'), nil
}

func (p Project) Reconfigure() {
	if p.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	commandName := path.Base(os.Args[0])
	if p.Output == "bunyan" {
		log.SetFormatter(&bunyan.Formatter{Name: commandName})
	} else if p.Output == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	} else if p.Output == "text" {
		log.SetFormatter(&log.TextFormatter{})
	} else if p.Output == "message" {
		log.SetFormatter(&MessageOnlyFormatter{})
	} else {
		log.Fatalf("Unexpected output format specified: %s", p.Output)
	}

}

func (p Project) CreateFlowKey(flowKeyStr string) FlowKey {
	c := strings.Split(flowKeyStr, ".")
	return FlowKey{Components: c}
}

func (p Project) CreateFlowKeyFromVariable(variable *Variable) FlowKey {
	return p.CreateFlowKeyFromInputName(variable.Name)
}

func (p Project) CreateFlowKeyFromInput(input *Input) FlowKey {
	return p.CreateFlowKeyFromInputName(input.Name)
}

func (p Project) CreateFlowKeyFromInputName(inputName string) FlowKey {
	c := strings.Split(p.Name+"."+inputName, ".")
	return FlowKey{Components: c}
}

func (t FlowKey) String() string {
	return strings.Join(t.Components, ".")
}

func (t FlowKey) ShortString() string {
	return strings.Join(t.Components[1:], ".")
}

func (t FlowKey) Parent() (*FlowKey, error) {
	if len(t.Components) > 1 {
		return &FlowKey{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return nil, errors.Errorf("FlowKey %v doesn't have a parent", t)
	}
}

func (p Project) RunFlowForKeyString(keyStr string, args []string, parent ...FlowDef) (string, error) {
	flowKey := p.CreateFlowKey(fmt.Sprintf("%s.%s", p.Name, keyStr))
	return p.RunFlowForKey(flowKey, args, parent...)
}

func (p Project) RunFlowForKey(flowKey FlowKey, args []string, parent ...FlowDef) (string, error) {
	provided := p.GetValueForConfigKey(flowKey.ShortString())

	if provided != "" {
		log.Debugf("Output for flow %s is already provided in configuration: %s", flowKey.ShortString(), provided)
		log.Info(provided)
		return provided, nil
	}

	flowDef, err := p.FindFlowDef(flowKey)

	if err != nil {
		return "", errors.Annotate(err, "RunFlowError")
	}

	vars := map[string](interface{}){}
	vars["args"] = args
	vars["env"] = p.Env
	vars["cmd"] = p.CommandRelativePath

	inputs, err := p.AggregateVariablesOfFlowForKey(flowKey, args, parent...)

	if err != nil {
		return "", errors.Annotatef(err, "Flow `%s` failed", flowKey.String())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	flow := &Flow{
		Key:         flowKey,
		ProjectName: flowDef.ProjectName,
		Steps:       flowDef.Steps,
		Vars:        vars,
		Autoenv:     flowDef.Autoenv,
		Autodir:     flowDef.Autodir,
		Interactive: flowDef.Interactive,
		FlowDef:     flowDef,
	}

	log.Debugf("Flow: %v", flow)

	output, error := flow.Run(&p, parent...)

	log.Debugf("Output: %s", output)

	if error != nil {
		error = errors.Annotatef(error, "Flow `%s` failed", flowKey.String())
	}

	return output, error
}

func (p Project) AggregateVariablesOfFlowForKey(flowKey FlowKey, args []string, parent ...FlowDef) (map[string]interface{}, error) {
	aggregated := map[string]interface{}{}
	if err := p.CollectVariablesOfFlowForKey(flowKey, aggregated, args, parent...); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.String())
	}
	if err := p.CollectVariablesOfParent(flowKey, aggregated); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", flowKey.String())
	}
	return aggregated, nil
}

type AnyMap map[string]interface{}

func (p Project) CollectVariablesOfParent(flowKey FlowKey, aggregated AnyMap) error {
	parentKey, err := flowKey.Parent()
	if err != nil {
		log.Debug("%v", err)
	} else {
		if err := p.CollectVariablesOfFlowForKey(*parentKey, aggregated, []string{}); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.String())
		}
		if err := p.CollectVariablesOfParent(*parentKey, aggregated); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", flowKey.String())
		}
	}
	return nil
}

func (p Project) GetValueForConfigKey(k string) string {
	ctx := log.WithFields(log.Fields{"prefix": k})

	lastIndex := strings.LastIndex(k, ".")

	provided := ""

	if lastIndex != -1 {
		a := []rune(k)
		k1 := string(a[:lastIndex])
		k2 := string(a[lastIndex+1:])

		values := viper.GetStringMapString(k1)

		ctx.Debugf("viper.GetStringMap(k1=%s)=%v, k2=%s", k1, values, k2)

		if values != nil && values[k2] != "" {
			provided = values[k2]
			return provided
		}
	}

	provided = viper.GetString(k)
	ctx.Debugf("viper.GetString(\"%s\") #=> \"%s\"", k, provided)

	return provided
}

func (p Project) CollectVariablesOfFlowForKey(flowKey FlowKey, variables AnyMap, args []string, parent ...FlowDef) error {
	var initialFlowKey string
	if len(parent) > 0 {
		initialFlowKey = parent[0].Key.ShortString()
	} else {
		initialFlowKey = ""
	}

	if initialFlowKey != "" {
		log.Debugf("Collecting inputs for the flow `%v` via the flow `%s`", flowKey.ShortString(), initialFlowKey)
	} else {
		log.Debugf("Collecting inputs for the flow `%v`", flowKey.ShortString())
	}

	flowDef, err := p.FindFlowDef(flowKey)
	if err != nil {
		return errors.Trace(err)
	}
	for i, input := range flowDef.Variables {
		log.Debugf("Flow `%v` depends on the input `%s`", flowKey.ShortString(), input.ShortName())
		ctx := log.WithFields(log.Fields{"prefix": input.Name})

		var arg *string
		if len(args) >= i+1 {
			ctx.Debugf("positional argument provided: %s", args[i])
			arg = &args[i]
		}

		var provided string

		if initialFlowKey != "" {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", initialFlowKey, input.ShortName()))
		}

		if provided == "" && strings.LastIndex(input.ShortName(), flowKey.ShortString()) == -1 {
			provided = p.GetValueForConfigKey(fmt.Sprintf("%s.%s", flowKey.ShortString(), input.ShortName()))
		}

		if provided == "" {
			provided = p.GetValueForConfigKey(input.ShortName())
		}

		pathComponents := strings.Split(input.Name, ".")

		if arg != nil {
			SetValueAtPath(variables, pathComponents, *arg)
		} else if provided == "" {
			var output interface{}
			var err error
			if output, err = FetchCache(p.CachedFlowOutputs, pathComponents); output == nil {
				output, err = p.RunFlowForKey(p.CreateFlowKeyFromVariable(input), []string{}, *flowDef)
				if err != nil {
					return errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a flow for it`", input.ShortName())
				}
				SetValueAtPath(p.CachedFlowOutputs, pathComponents, output)
			}
			if err != nil {
				return errors.Trace(err)
			}
			SetValueAtPath(variables, pathComponents, output)
		} else {
			SetValueAtPath(variables, pathComponents, provided)
		}

	}
	return nil
}

func FetchCache(cache map[string]interface{}, keyComponents []string) (interface{}, error) {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	if len(rest) == 0 {
		return cache[k], nil
	} else {
		nested, ok := cache[k].(map[string]interface{})
		if ok {
			v, err := FetchCache(nested, rest)
			if err != nil {
				return nil, err
			}
			return v, nil
		} else if cache[k] != nil {
			return nil, errors.Errorf("%s is not a map[string]interface{}", k)
		} else {
			return nil, nil
		}
	}
}

func SetValueAtPath(cache map[string]interface{}, keyComponents []string, value interface{}) error {
	k, rest := keyComponents[0], keyComponents[1:]

	k = strings.Replace(k, "-", "_", -1)

	if len(rest) == 0 {
		cache[k] = value
	} else {
		_, ok := cache[k].(map[string]interface{})
		if !ok && cache[k] != nil {
			return errors.Errorf("%s is not an map[string]interface{}", k)
		}
		if cache[k] == nil {
			cache[k] = map[string]interface{}{}
		}
		err := SetValueAtPath(cache[k].(map[string]interface{}), rest, value)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (p *Project) FindFlowDef(flowKey FlowKey) (*FlowDef, error) {
	t := p.FlowDefs[flowKey.String()]

	if t == nil {
		return nil, errors.Errorf("No FlowDef exists for the flow key `%s`", flowKey.String())
	}

	return t, nil
}

func (p *Project) RegisterFlowDef(flowKey FlowKey, flowDef *FlowDef) {
	p.FlowDefs[flowKey.String()] = flowDef
}

func recursiveFetchFromMap(m map[string]interface{}, key string) (string, error) {
	sep := "."

	components := strings.Split(strings.Replace(key, "-", "_", -1), sep)
	log.Debugf("components=%v", components)
	head := components[0]
	rest := components[1:]
	value, exists := m[head]
	if !exists {
		return "", fmt.Errorf("No value for %s in %+v", head, m)
	}

	next, isMap := value.(map[string]interface{})
	result, isStr := value.(string)

	if !isStr {
		if !isMap {
			return "", fmt.Errorf("Not map or string: %s in %+v", head, m)
		}

		if len(rest) == 0 {
			return "", fmt.Errorf("%s in %+v is a map but no more key to recurse", head, m)
		}

		return recursiveFetchFromMap(next, strings.Join(rest, sep))
	}

	return result, nil
}

func (t *Flow) Run(project *Project, parent ...FlowDef) (string, error) {
	if len(parent) > 0 {
		log.Debugf("running flow `%s` via `%s`", t.Key.String(), parent[0].Key.String())
	} else {
		log.Infof("running flow: %s", t.Key.String())
	}

	var output StepStringOutput
	var err error

	for _, step := range t.Steps {
		output, err = step.Run(project, t, parent...)

		if err != nil {
			return "", errors.Annotate(err, "Flow#Run failed while running a script")
		}
	}

	if err != nil {
		err = errors.Annotate(err, "Flow#Run failed while running a script")
	}

	return output.String, err
}

func (s FlowStep) Run(project *Project, flow *Flow, parent ...FlowDef) (StepStringOutput, error) {
	output, err := project.RunFlowForKeyString(s.FlowKeyString, []string{}, parent...)
	return StepStringOutput{String: output}, err
}

func (s FlowStep) GetName() string {
	return s.Name
}

func (s ScriptStep) GetName() string {
	return s.Name
}

func (s ScriptStep) Run(project *Project, flow *Flow, parent ...FlowDef) (StepStringOutput, error) {
	depended := len(parent) > 0

	t := template.New(fmt.Sprintf("%s.definition.yaml: %s.%s.script", flow.ProjectName, s.GetName(), flow.Key.ShortString()))
	t.Option("missingkey=error")

	tmpl, err := t.Funcs(flow.CreateFuncMap()).Parse(s.Code)
	if err != nil {
		log.Errorf("Error: %v", err)
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, flow.Vars); err != nil {
		return StepStringOutput{String: "scripterror"}, errors.Annotatef(err, "Template execution failed.\n\nScript:\n%s\n\nVars:\n%v", s.Code, flow.Vars)
	}

	script := buff.String()

	output, err := s.RunScript(script, depended, flow)

	return StepStringOutput{String: output}, err
}

func (t ScriptStep) RunScript(script string, depended bool, flow *Flow) (string, error) {
	//commands := strings.Split(script, "\n")
	commands := []string{script}
	var lastOutput string
	for _, command := range commands {
		if command != "" {
			output, err := t.RunCommand(command, depended, flow)
			if err != nil {
				return output, err
			}
			lastOutput = output
		}
	}
	return lastOutput, nil
}

func (f Flow) CreateFuncMap() template.FuncMap {
	get := func(key string) (interface{}, error) {
		val, err := recursiveFetchFromMap(f.Vars, key)

		if err != nil {
			return nil, errors.Trace(err)
		}
		return val, nil
	}

	escapeDoubleQuotes := func(str string) (interface{}, error) {
		val := strings.Replace(str, "\"", "\\\"", -1)
		return val, nil
	}

	fns := template.FuncMap{
		"get":                get,
		"escapeDoubleQuotes": escapeDoubleQuotes,
	}

	return fns
}

func (p *Project) GenerateCommand(flowConfig *FlowConfig, rootCommand *cobra.Command, parentFlowKey []string) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range flowConfig.Inputs {
		if i != len(flowConfig.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(flowConfig.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{

		Use: fmt.Sprintf("%s %s", flowConfig.Name, positionalArgs),
	}
	if flowConfig.Description != "" {
		cmd.Short = flowConfig.Description
		cmd.Long = flowConfig.Description
	}

	flowKeyStr := strings.Join(append(parentFlowKey, flowConfig.Name), ".")
	flowKey := p.CreateFlowKey(flowKeyStr)
	flowDef := &FlowDef{
		Key:         flowKey,
		Inputs:      flowConfig.Inputs,
		ProjectName: p.Name,
		Steps:       flowConfig.Steps,
		Autoenv:     flowConfig.Autoenv,
		Autodir:     flowConfig.Autodir,
		Interactive: flowConfig.Interactive,
		FlowConfig:  flowConfig,
		Command:     cmd,
	}
	p.RegisterFlowDef(flowKey, flowDef)

	if len(flowConfig.Steps) > 0 {
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.Reconfigure()

			log.Debugf("Number of inputs: %v", len(flowConfig.Inputs))

			if _, err := p.RunFlowForKey(flowKey, args); err != nil {
				c := strings.Join(strings.Split(flowKey.String(), "."), " ")
				stack := strings.Split(errors.ErrorStack(err), "\n")
				for i := len(stack)/2 - 1; i >= 0; i-- {
					opp := len(stack) - 1 - i
					stack[i], stack[opp] = stack[opp], stack[i]
				}
				log.Errorf("Command `%s` failed\n\nCaused by:\n%s", c, strings.Join(stack, "\n"))
				log.Debugf("Stack:\n%v", errors.ErrorStack(errors.Trace(err)))
				os.Exit(1)
			}
		}
	}

	if rootCommand != nil {
		rootCommand.AddCommand(cmd)
	}

	log.WithFields(log.Fields{"prefix": flowKey.String()}).Debug("is a flow")

	p.GenerateCommands(flowConfig.FlowConfigs, cmd, append(parentFlowKey, flowConfig.Name))

	return cmd, nil
}

func (p *Project) GenerateCommands(flowConfigs []*FlowConfig, rootCommand *cobra.Command, parentFlowKey []string) (*cobra.Command, error) {
	for _, c := range flowConfigs {
		p.GenerateCommand(c, rootCommand, parentFlowKey)
	}

	return rootCommand, nil
}

func (p *Project) GenerateAllFlags() {
	for _, flowDef := range p.FlowDefs {
		flowDef.Variables = p.AllVariables(flowDef)
		for _, input := range flowDef.Variables {
			log.Debugf("Configuring flag and config key for flow %s's input: %s", flowDef.Key.String(), input.Name)

			flowConfig := flowDef.FlowConfig
			cmd := flowDef.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.FlowKey.String() == flowDef.Key.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			log.Debugf("short=%s, full=%s, name=%s, selected=%s", input.ShortName(), input.FullName, input.Name, name)

			flagName := strings.Replace(name, ".", "-", -1)

			var longerName string
			if input.FlowKey.ShortString() == flowDef.Key.ShortString() {
				longerName = input.ShortName()
			} else {
				longerName = fmt.Sprintf("%s.%s", flowDef.Key.ShortString(), input.ShortName())
			}

			if len(flowConfig.FlowConfigs) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				//log.Debugf("Binding flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
				log.Debugf("Binding flag --%s of the command %s to the input %s with the config key %s", flagName, flowDef.Key.ShortString(), input.Name, longerName)
				viper.BindPFlag(longerName, cmd.Flags().Lookup(flagName))
			} else {
				cmd.PersistentFlags().StringP(flagName, "" /*string(input.Name[0])*/, "" /*default*/, description)
				//log.Debugf("Binding persistent flag --%s to the config key %s", flagName, name)
				//viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
				log.Debugf("Binding persistent flag --%s to the config key %s", flagName, longerName)
				viper.BindPFlag(longerName, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
}

func ReadFlowConfigFromString(data string) (*FlowConfig, error) {
	err, t := ReadFlowConfigFromBytes([]byte(data))
	return err, t
}

func ReadFlowConfigFromBytes(data []byte) (*FlowConfig, error) {
	c := newDefaultFlowConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, errors.Annotatef(err, "yaml.Unmarshal failed: %v", err)
	}
	return c, nil
}

func ReadFlowConfigFromFile(path string) (*FlowConfig, error) {
	log.Debugf("Loading %s", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", path)
	}

	yamlBytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error while loading %s", path)
	}

	log.Debugf("%s", string(yamlBytes))

	t, err := ReadFlowConfigFromBytes(yamlBytes)

	if err != nil {
		return nil, errors.Annotatef(err, "Error while loading %s", path)
	}

	return t, nil
}

func main() {
	var commandName string
	var commandPath string
	var varfile string
	var args []string

	if len(os.Args) > 1 && (os.Args[0] != "var" || os.Args[0] != "/usr/bin/env") && file.Exists(os.Args[1]) {
		varfile = os.Args[1]
		args = os.Args[2:]
		commandName = path.Base(varfile)
		commandPath = varfile
	} else {
		commandName = path.Base(os.Args[0])
		commandPath = os.Args[0]
		varfile = fmt.Sprintf("%s.definition.yaml", commandName)
		args = os.Args[1:]
	}

	environ := ParseEnviron()

	if environ["VARFILE"] != "" {
		varfile = environ["VARFILE"]
	}

	var rootFlowConfig *FlowConfig

	varfileExists := file.Exists(varfile)

	if varfileExists {

		flowConfigFromFile, err := ReadFlowConfigFromFile(varfile)

		if err != nil {
			log.Errorf(errors.ErrorStack(err))
			panic(errors.Trace(err))
		}
		rootFlowConfig = flowConfigFromFile
	} else {
		rootFlowConfig = newDefaultFlowConfig()
	}

	var err error

	rootFlowConfig.Name = commandName

	var envFromFile string
	envFromFile, err = env.New(rootFlowConfig.Name).GetOrSet("dev")
	if err != nil {
		panic(errors.Trace(err))
	}

	p := &Project{
		Name:                rootFlowConfig.Name,
		CommandRelativePath: commandPath,
		FlowDefs:            map[string]*FlowDef{},
		CachedFlowOutputs:   map[string]interface{}{},
		Verbose:             false,
		Output:              "text",
		Env:                 envFromFile,
	}

	rootCmd, err := p.GenerateCommand(rootFlowConfig, nil, []string{})
	rootCmd.AddCommand(cmd.EnvCmd)
	rootCmd.AddCommand(cmd.VersionCmd(log.StandardLogger()))

	p.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&(p.Output), "output", "o", "text", "Output format. One of: json|text|bunyan")

	// see `func ExecuteC` in https://github.com/spf13/cobra/blob/master/command.go#L671-L677 for usage of ParseFlags()
	rootCmd.ParseFlags(args)

	// Workaround: We want to set log leve via command-line option before the rootCmd is run
	p.Reconfigure()

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// Deferred to respect output format specified via the --output flag
	if !varfileExists {
		log.Infof("%s does not exist", varfile)
	}

	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// See "How to merge two config files" https://github.com/spf13/viper/issues/181
	viper.SetConfigName(rootFlowConfig.Name)
	commonConfigFile := fmt.Sprintf("%s.yaml", rootFlowConfig.Name)
	if file.Exists(commonConfigFile) {
		log.Debugf("Loading common configuration from %s.yaml", rootFlowConfig.Name)
		if err := viper.MergeInConfig(); err != nil {
			panic(err)
		}
	} else {
		log.Infof("%s does not exist. Skipping", commonConfigFile)
	}

	env.SetAppName(commandName)
	log.Debugf("Loading current env from %s", env.GetPath())
	envName, err := env.Get()
	if err != nil {
		log.Debugf("No env set, no additional config to load")
	} else {
		envConfigFile := fmt.Sprintf("config/environments/%s", envName)
		viper.SetConfigName(envConfigFile)
		log.Debugf("Loading env specific configuration from %s.yaml", envConfigFile)
		if err := viper.MergeInConfig(); err != nil {
			log.Infof("%s.yaml does not exist. Skipping", envConfigFile)
		}
	}

	//Set the environment prefix as app name
	viper.SetEnvPrefix(strings.ToUpper(commandName))
	viper.AutomaticEnv()

	//Substitute the . and - to _,
	replacer := strings.NewReplacer(".", "_", "-", "_")
	viper.SetEnvKeyReplacer(replacer)

	//	var rootCmd = &cobra.Command{Use: c.Name}

	rootCmd.SetArgs(args)
	rootCmd.Execute()
}
