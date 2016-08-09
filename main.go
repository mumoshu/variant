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
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func init() {
	log.SetFormatter(new(prefixed.TextFormatter))

	log.SetOutput(os.Stderr)

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
	TaskKey     TaskKey
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

type Target struct {
	Name        string    `yaml:"name,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"flows,omitempty"`
	Script      string    `yaml:"script,omitempty"`
	Autoenv     bool      `yaml:"autoenv,omitempty"`
	Autodir     bool      `yaml:"autodir,omitempty"`
}

type TargetV1 struct {
	Name        string    `yaml:"name,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"flows,omitempty"`
	Script      string    `yaml:"script,omitempty"`
	Autoenv     bool      `yaml:"autoenv,omitempty"`
	Autodir     bool      `yaml:"autodir,omitempty"`
}

type TargetV2 struct {
	Description string             `yaml:"description,omitempty"`
	Inputs      []*Input           `yaml:"inputs,omitempty"`
	Targets     map[string]*Target `yaml:"flows,omitempty"`
	Script      string             `yaml:"script,omitempty"`
	Autoenv     bool               `yaml:"autoenv,omitempty"`
	Autodir     bool               `yaml:"autodir,omitempty"`
}

func (t *Target) UnmarshalYAML(unmarshal func(interface{}) error) error {
	v3 := map[string]interface{}{}
	v3err := unmarshal(&v3)

	log.Debugf("Unmarshalling: %v", v3)

	log.Debugf("Trying to parse v1 format")

	v1 := TargetV1{
		Autoenv: true,
		Autodir: true,
		Inputs:  []*Input{},
		Targets: []*Target{},
	}

	err := unmarshal(&v1)

	if v1.Name == "" && len(v1.Targets) == 0 {
		e := fmt.Errorf("Not v1 format: Both Name and Targets are empty")
		log.Debugf("%s", e)
		err = e
	}

	if err == nil {
		t.Name = v1.Name
		t.Description = v1.Description
		t.Inputs = v1.Inputs
		t.Targets = v1.Targets
		t.Script = v1.Script
		t.Autoenv = v1.Autoenv
		t.Autodir = v1.Autodir
		return nil
	}

	var v2 *TargetV2

	if err != nil {
		log.Debugf("Trying to parse v2 format")
		v2 = &TargetV2{
			Autoenv: true,
			Autodir: true,
			Inputs:  []*Input{},
			Targets: map[string]*Target{},
		}

		err = unmarshal(&v2)

		if len(v2.Targets) == 0 && v2.Script == "" {
			e := fmt.Errorf("Not v2 format: Targets and Script are both missing.")
			log.Debugf("%s", e)
			err = e
		}

		if err == nil {
			t.Description = v2.Description
			t.Inputs = v2.Inputs
			t.Targets = ReadV2Targets(v2.Targets)
			t.Script = v2.Script
			t.Autoenv = v2.Autoenv
			t.Autodir = v2.Autodir
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
			t.Autoenv = true
			t.Autodir = true
			t.Inputs = []*Input{}

			t.Targets = ReadV3Targets(flows)

			return nil
		}
	}

	return errors.Trace(err)
}

func (t *Target) CopyTo(other *Target) {
	other.Description = t.Description
	other.Inputs = t.Inputs
	other.Targets = t.Targets
	other.Script = t.Script
	other.Autoenv = t.Autoenv
	other.Autodir = t.Autodir
}

func ReadV2Targets(v2 map[string]*Target) []*Target {
	result := []*Target{}
	for name, t2 := range v2 {
		t := &Target{}

		t.Name = name
		t2.CopyTo(t)

		result = append(result, t)
	}
	return result
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

func ReadV3Targets(v3 map[string]interface{}) []*Target {
	result := []*Target{}
	for k, v := range v3 {
		t := &Target{
			Autoenv: true,
			Autodir: true,
			Inputs:  []*Input{},
			Targets: []*Target{},
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
			t.Targets = ReadV3Targets(s2i)
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

func (p *Project) AllVariables(taskDef *TaskDef) []*Variable {
	return p.CollectVariablesRecursively(taskDef.Key, "")
}

func (p *Project) CollectVariablesRecursively(currentTaskKey TaskKey, path string) []*Variable {
	result := []*Variable{}

	ctx := log.WithFields(log.Fields{"prefix": fmt.Sprintf("%s", currentTaskKey.String())})

	currentTaskDef, err := p.FindTask(currentTaskKey)

	if err != nil {
		tasks := []string{}
		for _, t := range p.Tasks {
			tasks = append(tasks, t.Key.String())
		}
		ctx.Debugf("is not a task in: %v", tasks)
		return []*Variable{}
	}

	for _, input := range currentTaskDef.Inputs {
		childKey := p.CreateTaskKeyFromInput(input)

		ctx.Debugf("depends on %s", childKey.String())

		vars := p.CollectVariablesRecursively(childKey, fmt.Sprintf("%s.", currentTaskKey.String()))

		for _, v := range vars {
			result = append(result, v)
		}

		variable := &Variable{
			TaskKey:     currentTaskKey,
			FullName:    fmt.Sprintf("%s.%s", currentTaskKey.String(), input.Name),
			Name:        input.Name,
			Parameters:  input.Parameters,
			Description: input.Description,
			Candidates:  input.Candidates,
			Complete:    input.Complete,
		}

		ctx.WithFields(log.Fields{"full": variable.FullName, "task": variable.TaskKey.String()}).Debugf("has var %s", variable.Name)

		result = append(result, variable)
	}

	return result
}

func newDefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Inputs:  []*Input{},
		Targets: []*Target{},
	}
}

func newDefaultTargetConfig() *Target {
	return &Target{
		Inputs:  []*Input{},
		Targets: []*Target{},
		Autoenv: true,
	}
}

type ProjectConfig struct {
	Name        string
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"flows,omitempty"`
	Script      string    `yaml:"script,omitempty"`
}

type TaskDef struct {
	Key       TaskKey
	Template  *template.Template
	Inputs    []*Input
	Variables []*Variable
	Autoenv   bool
	Autodir   bool
	Target    *Target
	Command   *cobra.Command
}

type Task struct {
	Key      TaskKey
	Template *template.Template
	Vars     map[string]interface{}
	Autoenv  bool
	Autodir  bool
	TaskDef  *TaskDef
}

type TaskKey struct {
	Components []string
}

type Project struct {
	Name              string
	Tasks             map[string]*TaskDef
	CachedTaskOutputs map[string]interface{}
	Verbose           bool
}

type T struct {
	A string
	B struct {
		RenamedC int   `yaml:"c"`
		D        []int `yaml:",flow"`
	}
}

func (t Task) RunScript(script string, depended bool) (string, error) {
	commands := strings.Split(script, "\n")
	var lastOutput string
	for _, command := range commands {
		if command != "" {
			output, err := t.RunCommand(command, depended)
			if err != nil {
				return output, err
			}
			lastOutput = output
		}
	}
	return lastOutput, nil
}

func (t Task) GenerateAutoenv() (map[string]string, error) {
	replacer := strings.NewReplacer("-", "_", ".", "_")
	toEnvName := func(parName string) string {
		return strings.ToUpper(replacer.Replace(parName))
	}
	return t.GenerateAutoenvRecursively("", t.Vars, toEnvName)
}

func (t Task) GenerateAutoenvRecursively(path string, env map[string]interface{}, toEnvName func(string) string) (map[string]string, error) {
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

func (t Task) RunCommand(command string, depended bool) (string, error) {
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

	if t.Autoenv {
		l.Debugf("Autoenv is enabled")
		autoEnv, err := t.GenerateAutoenv()
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

	if t.Autodir {
		l.Debugf("Autodir is enabled")
		parentKey, err := t.Key.Parent()
		l.Debugf("full: %s", parentKey.String())
		shortKey := parentKey.ShortString()
		l.Debugf("short: %s", shortKey)
		path := strings.Replace(shortKey, ".", "/", -1)
		l.Debugf("Dir: %s", path)
		if err != nil {
			l.Debugf("%s does not have parent", t.Key.String())
		} else {
			if _, err := os.Stat(path); err == nil {
				cmd.Dir = path
			}
		}
	} else {
		l.Debugf("Autodir is disabled")
	}

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
	var output string
	go func() {
		defer func() {
			close(channels.Stdout)
		}()
		for scanner.Scan() {
			text := scanner.Text()
			channels.Stdout <- text
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
					fmt.Println(text)
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

	var waitStatus syscall.WaitStatus
	err = cmd.Wait()

	if err != nil {
		l.Fatalf("%v", err)
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

func (p Project) Reconfigure() {
	if p.Verbose {
		log.SetLevel(log.DebugLevel)
	}
}

func (p Project) CreateTaskKey(taskKeyStr string) TaskKey {
	c := strings.Split(taskKeyStr, ".")
	return TaskKey{Components: c}
}

func (p Project) CreateTaskKeyFromVariable(variable *Variable) TaskKey {
	return p.CreateTaskKeyFromInputName(variable.Name)
}

func (p Project) CreateTaskKeyFromInput(input *Input) TaskKey {
	return p.CreateTaskKeyFromInputName(input.Name)
}

func (p Project) CreateTaskKeyFromInputName(inputName string) TaskKey {
	c := strings.Split(p.Name+"."+inputName, ".")
	return TaskKey{Components: c}
}

func (t TaskKey) String() string {
	return strings.Join(t.Components, ".")
}

func (t TaskKey) ShortString() string {
	return strings.Join(t.Components[1:], ".")
}

func (t TaskKey) Parent() (*TaskKey, error) {
	if len(t.Components) > 1 {
		return &TaskKey{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return nil, errors.Errorf("TaskKey %v doesn't have a parent", t)
	}
}

func (p Project) RunTask(taskKey TaskKey, args []string, depended bool) (string, error) {
	t, err := p.FindTask(taskKey)

	if err != nil {
		return "", errors.Annotate(err, "RunTaskError")
	}

	vars := map[string](interface{}){}
	vars["args"] = args

	inputs, err := p.AggregateInputsFor(taskKey, args)

	if err != nil {
		return "", errors.Annotatef(err, "Flow `%s` failed", taskKey.String())
	}

	for k, v := range inputs {
		vars[k] = v
	}

	task := &Task{
		Key:      taskKey,
		Template: t.Template,
		Vars:     vars,
		Autoenv:  t.Autoenv,
		Autodir:  t.Autodir,
		TaskDef:  t,
	}

	log.Debugf("Task: %v", task)

	output, error := task.Run(depended)

	if error != nil {
		error = errors.Annotatef(error, "Flow `%s` failed", taskKey.String())
	}

	return output, error
}

func (p Project) AggregateInputsFor(taskKey TaskKey, args []string) (map[string]interface{}, error) {
	//	task := p.FindTask(taskKey)
	aggregated := map[string]interface{}{}
	if err := p.CollectInputsFor(taskKey, aggregated, args); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", taskKey.String())
	}
	if err := p.AggregateInputsForParent(taskKey, aggregated); err != nil {
		return nil, errors.Annotatef(err, "One or more inputs for flow %s failed", taskKey.String())
	}
	return aggregated, nil
}

type AnyMap map[string]interface{}

func (p Project) AggregateInputsForParent(taskKey TaskKey, aggregated AnyMap) error {
	parentKey, err := taskKey.Parent()
	if err != nil {
		log.Debug("%v", err)
	} else {
		if err := p.CollectInputsFor(*parentKey, aggregated, []string{}); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", taskKey.String())
		}
		if err := p.AggregateInputsForParent(*parentKey, aggregated); err != nil {
			return errors.Annotatef(err, "AggregateInputsForParent(%s) failed", taskKey.String())
		}
	}
	return nil
}

func (p Project) CollectInputsFor(taskKey TaskKey, aggregated AnyMap, args []string) error {
	log.Debugf("Collecting inputs for the task `%v`", taskKey.String())
	task, err := p.FindTask(taskKey)
	if err != nil {
		return errors.Trace(err)
	}
	for i, input := range task.Variables {
		log.Debugf("Task `%v` depends on the input `%s`", taskKey.String(), input.ShortName())
		ctx := log.WithFields(log.Fields{"prefix": input.Name})

		var arg *string
		if len(args) >= i+1 {
			ctx.Debugf("positional argument provided: %s", args[i])
			arg = &args[i]
		}

		k := input.ShortName()
		provided := viper.GetString(k)

		if provided != "" {
			ctx.Debugf("a command-line option or a configuration value provided: %s=%s", k, provided)
		} else {
			ctx.Debugf("no command-line option or config value provided: %s", k)

			k = input.Name
			provided = viper.GetString(k)

			if provided != "" {
				ctx.Debugf("a command-line option or a configuration value provided: %s=%s", k, provided)
			} else {
				ctx.Debugf("no command-line option or config value provided: %s", k)
			}
		}

		components := strings.Split(input.Name, ".")

		if arg != nil {
			PopulateCache(aggregated, components, *arg)
		} else if provided == "" {
			var output interface{}
			var err error
			if output, err = FetchCache(p.CachedTaskOutputs, components); output == nil {
				output, err = p.RunTask(p.CreateTaskKeyFromVariable(input), []string{}, true)
				if err != nil {
					return errors.Annotatef(err, "Missing value for input `%s`. Please provide a command line option or a positional argument or a flow for it`", k)
				}
				PopulateCache(p.CachedTaskOutputs, components, output)
			}
			if err != nil {
				return errors.Trace(err)
			}
			PopulateCache(aggregated, components, output)
		} else {
			PopulateCache(aggregated, components, provided)
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

func PopulateCache(cache map[string]interface{}, keyComponents []string, value interface{}) error {
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
		PopulateCache(cache[k].(map[string]interface{}), rest, value)
	}
	return nil
}

func (p *Project) FindTask(taskKey TaskKey) (*TaskDef, error) {
	t := p.Tasks[taskKey.String()]

	if t == nil {
		return nil, errors.Errorf("No TaskDef exists for the task key `%s`", taskKey.String())
	}

	return t, nil
}

func (p *Project) RegisterTask(taskKey TaskKey, task *TaskDef) {
	p.Tasks[taskKey.String()] = task
}

func (t Task) Run(depended bool) (string, error) {
	if depended {
		log.Debugf("running flow: %s", t.Key.String())
	} else {
		log.Infof("running flow: %s", t.Key.String())
	}

	var buff bytes.Buffer
	if err := t.Template.Execute(&buff, t.Vars); err != nil {
		return "", errors.Annotatef(err, "Template execution failed.\n\nScript:\n%s\n\nVars:\n%v", t.TaskDef.Target.Script, t.Vars)
	}

	script := buff.String()

	output, err := t.RunScript(script, depended)

	if err != nil {
		err = errors.Annotate(err, "Task#Run failed while running a script")
	}

	return output, err
}

func (p *Project) GenerateCommand(target *Target, rootCommand *cobra.Command, parentTaskKey []string) (*cobra.Command, error) {
	positionalArgs := ""
	for i, input := range target.Inputs {
		if i != len(target.Inputs)-1 {
			positionalArgs += fmt.Sprintf("[%s ", input.Name)
		} else {
			positionalArgs += fmt.Sprintf("[%s", input.Name)
		}
	}
	for i := 0; i < len(target.Inputs); i++ {
		positionalArgs += "]"
	}

	var cmd = &cobra.Command{

		Use: fmt.Sprintf("%s %s", target.Name, positionalArgs),
	}
	if target.Description != "" {
		cmd.Short = target.Description
		cmd.Long = target.Description
	}

	tk := strings.Join(append(parentTaskKey, target.Name), ".")
	taskKey := p.CreateTaskKey(tk)
	task := &TaskDef{
		Key:     taskKey,
		Inputs:  target.Inputs,
		Autoenv: target.Autoenv,
		Autodir: target.Autodir,
		Target:  target,
		Command: cmd,
	}
	p.RegisterTask(taskKey, task)

	if target.Script != "" {
		tmpl, err := template.New(fmt.Sprintf("%s.yaml: %s.script", p.Name, taskKey.String())).Parse(target.Script)
		if err != nil {
			log.Panicf("Error: %v", err)
		}
		task.Template = tmpl
		cmd.Run = func(cmd *cobra.Command, args []string) {
			p.Reconfigure()

			log.Debugf("Number of inputs: %v", len(target.Inputs))
			for _, input := range target.Inputs {
				name := strings.Replace(input.Name, ".", "-", -1)

				log.Debugf("BindPFlag(name=%v)", name)
				//				if len(target.Targets) == 0 {
				//					viper.BindPFlag(input.Name, cmd.Flags().Lookup(name))
				//					log.Debugf("Looked up %v: %v", name, cmd.Flags().Lookup(name))
				//				} else {
				//					viper.BindPFlag(input.Name, cmd.PersistentFlags().Lookup(name))
				//					log.Debugf("Looked up %v: %v", name, cmd.PersistentFlags().Lookup(name))
				//				}
			}

			err := viper.ReadInConfig() // Find and read the config file
			if err != nil {             // Handle errors reading the config file
				panic(errors.Errorf("Fatal error config file: %s \n", err))
			}
			if _, err := p.RunTask(taskKey, args, false); err != nil {
				c := strings.Join(strings.Split(taskKey.String(), "."), " ")
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

	log.WithFields(log.Fields{"prefix": taskKey.String()}).Debug("is a task")

	p.GenerateCommands(target.Targets, cmd, append(parentTaskKey, target.Name))

	// After all the commands and tasks are generated...

	//	for _, input := range target.Inputs {
	//		var description string
	//		if input.Description != "" {
	//			description = input.Description
	//		} else {
	//			description = input.Name
	//		}
	//		name := strings.Replace(input.Name, ".", "-", -1)
	//
	//		if len(target.Targets) == 0 {
	//			cmd.Flags().StringP(name, string(input.Name[0]), "", description)
	//			viper.BindPFlag(name, cmd.Flags().Lookup(name))
	//		} else {
	//			cmd.PersistentFlags().StringP(name, string(input.Name[0]), "", description)
	//			viper.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
	//		}
	//	}

	return cmd, nil
}

func (p *Project) GenerateCommands(targets []*Target, rootCommand *cobra.Command, parentTaskKey []string) (*cobra.Command, error) {
	for _, target := range targets {
		p.GenerateCommand(target, rootCommand, parentTaskKey)
	}

	return rootCommand, nil
}

func (p *Project) GenerateAllFlags() {
	for _, taskDef := range p.Tasks {
		taskDef.Variables = p.AllVariables(taskDef)
		for _, input := range taskDef.Variables {
			log.Debugf("%s -> %s", taskDef.Key.String(), input.Name)

			target := taskDef.Target
			cmd := taskDef.Command
			var description string
			if input.Description != "" {
				description = input.Description
			} else {
				description = input.Name
			}

			var name string
			if input.TaskKey.String() == taskDef.Key.String() {
				name = input.Name
			} else {
				name = input.ShortName()
			}

			flagName := strings.Replace(name, ".", "-", -1)

			if len(target.Targets) == 0 {
				cmd.Flags().StringP(flagName, "" /*string(input.Name[0])*/, "", description)
				viper.BindPFlag(name, cmd.Flags().Lookup(flagName))
			} else {
				cmd.PersistentFlags().StringP(flagName, "" /*string(input.Name[0])*/, "" /*default*/, description)
				viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
}

func ReadFromString(data string) (*Target, error) {
	err, t := ReadFromBytes([]byte(data))
	return err, t
}

func ReadFromBytes(data []byte) (*Target, error) {
	c := newDefaultTargetConfig()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, errors.Annotatef(err, "yaml.Unmarshal failed: %v", err)
	}
	return c, nil
}

func ReadFromFile(path string) (*Target, error) {
	log.Debugf("Loading %s", path)

	yamlBytes, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf("Error while loading %s", path)
	}

	t, err := ReadFromBytes(yamlBytes)

	if err != nil {
		return nil, errors.Annotatef(err, "Error while loading %s", path)
	}

	return t, nil
}

func main() {
	commandName := path.Base(os.Args[0])

	varfile := fmt.Sprintf("%s.definition.yaml", commandName)

	environ := ParseEnviron()

	if environ["VARFILE"] != "" {
		varfile = environ["VARFILE"]
	}

	c, err := ReadFromFile(varfile)

	if err != nil {
		log.Errorf(errors.ErrorStack(err))
		panic(errors.Trace(err))
	}

	c.Name = commandName

	viper.SetConfigType("yaml")
	viper.SetConfigName(c.Name)
	viper.AddConfigPath(".")

	//Set the environment prefix as app name
	viper.SetEnvPrefix(strings.ToUpper(commandName))
	viper.AutomaticEnv()

	//Substitute the . and - to _,
	replacer := strings.NewReplacer(".", "_", "-", "_")
	viper.SetEnvKeyReplacer(replacer)

	//	var rootCmd = &cobra.Command{Use: c.Name}

	p := &Project{
		Name:              c.Name,
		Tasks:             map[string]*TaskDef{},
		CachedTaskOutputs: map[string]interface{}{},
		Verbose:           false,
	}

	rootCmd, err := p.GenerateCommand(c, nil, []string{})

	p.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")

	//	_, err := p.GenerateCommands(c.Targets, rootCmd, []string{})

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	rootCmd.Execute()
}
