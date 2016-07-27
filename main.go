package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	log "github.com/Sirupsen/logrus"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

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

type Target struct {
	Name        string    `yaml:"name,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"targets,omitempty"`
	Script      string    `yaml:"script,omitempty"`
	Autoenv     bool      `yaml:"autoenv,omitempty"`
	Autodir     bool      `yaml:"autodir,omitempty"`
}

type Target_ struct {
	Name        string    `yaml:"name,omitempty"`
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"targets,omitempty"`
	Script      string    `yaml:"script,omitempty"`
	Autoenv     bool      `yaml:"autoenv,omitempty"`
	Autodir     bool      `yaml:"autodir,omitempty"`
}

func (t *Target) UnmarshalYAML(unmarshal func(interface{}) error) error {
	data := Target_{
		Autoenv: true,
		Autodir: true,
		Inputs:  []*Input{},
		Targets: []*Target{},
	}
	err := unmarshal(&data)

	t.Name = data.Name
	t.Description = data.Description
	t.Inputs = data.Inputs
	t.Targets = data.Targets
	t.Script = data.Script
	t.Autoenv = data.Autoenv
	t.Autodir = data.Autodir

	return err
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
	Targets     []*Target `yaml:"targets,omitempty"`
	Script      string    `yaml:"script,omitempty"`
}

type TaskDef struct {
	Key      TaskKey
	Template *template.Template
	Inputs   []*Input
	Autoenv  bool
	Autodir  bool
}

type Task struct {
	Key      TaskKey
	Template *template.Template
	Vars     map[string]interface{}
	Autoenv  bool
	Autodir  bool
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

func (t Task) RunScript(script string) (string, error) {
	commands := strings.Split(script, "\n")
	var lastOutput string
	for _, command := range commands {
		if command != "" {
			output, err := t.RunCommand(command)
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
				return nil, fmt.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v", k, v)
			}
		}
	}
	logger.Debugf("Generated autoenv: %v", result)
	return result, nil
}

func (t Task) RunCommand(command string) (string, error) {
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


	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	var output string
	go func() {
		for scanner.Scan() {
			text := scanner.Text()
			l.WithFields(log.Fields{"stream": "stdout"}).Printf("%s", text)
			output += text
		}
	}()

	errReader, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StderrPipe for Cmd", err)
		os.Exit(1)
	}
	errScanner := bufio.NewScanner(errReader)
	go func() {
		for errScanner.Scan() {
			text := errScanner.Text()
			l.WithFields(log.Fields{"stream": "stderr"}).Errorf("%s", text)
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
		os.Exit(1)
	}

	var waitStatus syscall.WaitStatus
	if err := cmd.Wait(); err != nil {
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

func (p Project) CreateTaskKeyFromInputName(inputName string) TaskKey {
	c := strings.Split(p.Name+"."+inputName, ".")
	return TaskKey{Components: c}
}

func (t TaskKey) String() string {
	return strings.Join(t.Components, ".")
}

func (t TaskKey) Parent() (*TaskKey, error) {
	if len(t.Components) > 1 {
		return &TaskKey{Components: t.Components[:len(t.Components)-1]}, nil
	} else {
		return nil, fmt.Errorf("TaskKey %v doesn't have a parent", t)
	}
}

func (p Project) RunTask(taskKey TaskKey, options map[string]string, args []string) (string, error) {
	t := p.FindTask(taskKey)

	vars := map[string](interface{}){}
	vars["mysql"] = map[string]string{"host": "mysql2"}
	vars["args"] = args

	log.Debugf("Project: %s", spew.Sdump(p))
	log.Debugf("TaskKey: %s", spew.Sdump(taskKey))
	log.Debugf("TaskDef: %s", spew.Sdump(t))

	inputs, _ := p.AggregateInputsFor(taskKey)
	for k, v := range inputs {
		vars[k] = v
	}

	task := &Task{
		Key:      taskKey,
		Template: t.Template,
		Vars:     vars,
		Autoenv:  t.Autoenv,
	}

	log.Debugf("Task: %v", task)

	output, error := task.Run()
	return output, error
}

func (p Project) AggregateInputsFor(taskKey TaskKey) (map[string]interface{}, error) {
	//	task := p.FindTask(taskKey)
	aggregated := map[string]interface{}{}
	p.CollectInputsFor(taskKey, aggregated)
	p.AggregateInputsForParent(taskKey, aggregated)
	return aggregated, nil
}

type AnyMap map[string]interface{}

func (p Project) AggregateInputsForParent(taskKey TaskKey, aggregated AnyMap) {
	parentKey, err := taskKey.Parent()
	if err != nil {
		log.Debug("%v", err)
	} else {
		p.CollectInputsFor(*parentKey, aggregated)
		p.AggregateInputsForParent(*parentKey, aggregated)
	}
}

func (p Project) CollectInputsFor(taskKey TaskKey, aggregated AnyMap) (AnyMap, error) {
	log.Debugf("Collecting inputs for the task `%v`", taskKey.String())
	task := p.FindTask(taskKey)
	for _, input := range task.Inputs {
		log.Debugf("Task `%v` depends on the input `%v`", taskKey.String(), input.Name)

		k := input.Name
		components := strings.Split(k, ".")

		provided := viper.GetString(k)

		log.Debugf("viper provided: %v for %v", provided, k)

		if provided == "" {
			var output interface{}
			var err error
			if output, err = FetchCache(p.CachedTaskOutputs, components); output == nil {
				output, _ = p.RunTask(p.CreateTaskKeyFromInputName(k), map[string]string{}, []string{})
				PopulateCache(p.CachedTaskOutputs, components, output)
			}
			if err != nil {
				return nil, err
			}
			PopulateCache(aggregated, components, output)
		} else {
			PopulateCache(aggregated, components, provided)
		}

	}
	return aggregated, nil
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
			return nil, fmt.Errorf("%s is not a map[string]interface{}", k)
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
			return fmt.Errorf("%s is not an map[string]interface{}", k)
		}
		if cache[k] == nil {
			cache[k] = map[string]interface{}{}
		}
		PopulateCache(cache[k].(map[string]interface{}), rest, value)
	}
	return nil
}

func (p *Project) FindTask(taskKey TaskKey) *TaskDef {
	return p.Tasks[taskKey.String()]
}

func (p *Project) RegisterTask(taskKey TaskKey, task *TaskDef) {
	p.Tasks[taskKey.String()] = task
}

func (t Task) Run() (string, error) {
	var buff bytes.Buffer
	if err := t.Template.Execute(&buff, t.Vars); err != nil {
		log.Panicf("Error: %v", err)
	}

	script := buff.String()

	output, err := t.RunScript(script)

	return output, err
}

func (p *Project) GenerateCommand(target *Target, rootCommand *cobra.Command, parentTaskKey []string) (*cobra.Command, error) {
	var cmd = &cobra.Command{
		Use: fmt.Sprintf("%s", target.Name),
	}
	if target.Description != "" {
		cmd.Short = target.Description
		cmd.Long = target.Description
	}

	options := map[string]string{}

	tk := strings.Join(append(parentTaskKey, target.Name), ".")
	taskKey := p.CreateTaskKey(tk)
	task := &TaskDef{
		Key:     taskKey,
		Inputs:  target.Inputs,
		Autoenv: target.Autoenv,
	}
	p.RegisterTask(taskKey, task)

	if target.Script != "" {
		tmpl, err := template.New(fmt.Sprintf("%s.yaml.template", os.Args[0])).Parse(target.Script)
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
				if len(target.Targets) == 0 {
					viper.BindPFlag(input.Name, cmd.Flags().Lookup(name))
					log.Debugf("Looked up %v: %v", name, cmd.Flags().Lookup(name))
				} else {
					viper.BindPFlag(input.Name, cmd.PersistentFlags().Lookup(name))
					log.Debugf("Looked up %v: %v", name, cmd.PersistentFlags().Lookup(name))
				}
			}

			err := viper.ReadInConfig() // Find and read the config file
			if err != nil {             // Handle errors reading the config file
				panic(fmt.Errorf("Fatal error config file: %s \n", err))
			}
			p.RunTask(taskKey, options, args)
		}
	}

	for _, input := range target.Inputs {
		var description string
		if input.Description != "" {
			description = input.Description
		} else {
			description = input.Name
		}
		name := strings.Replace(input.Name, ".", "-", -1)

		if len(target.Targets) == 0 {
			cmd.Flags().StringP(name, string(input.Name[0]), "", description)
			viper.BindPFlag(name, cmd.Flags().Lookup(name))
		} else {
			cmd.PersistentFlags().StringP(name, string(input.Name[0]), "", description)
			viper.BindPFlag(name, cmd.PersistentFlags().Lookup(name))
		}
	}

	if rootCommand != nil {
		rootCommand.AddCommand(cmd)
	}

	p.GenerateCommands(target.Targets, cmd, append(parentTaskKey, target.Name))

	return cmd, nil
}

func (p *Project) GenerateCommands(targets []*Target, rootCommand *cobra.Command, parentTaskKey []string) (*cobra.Command, error) {
	for _, target := range targets {
		p.GenerateCommand(target, rootCommand, parentTaskKey)
	}

	return rootCommand, nil
}

func main() {
	commandName := strings.Replace(os.Args[0], "./", "", -1)

	yamlBytes, err := ioutil.ReadFile(fmt.Sprintf("%s.definition.yaml", commandName))

	if err != nil {
		panic(err)
	}

	c := newDefaultTargetConfig()
	//	if err := yaml.Unmarshal([]byte(data), c); err != nil {
	if err := yaml.Unmarshal(yamlBytes, c); err != nil {
		//return nil, fmt.Errorf("failed to parse cluster: %v", err)
		log.Fatalf("failed to parse project: %v", err)
	}
	//spew.Printf("ProjectConfig: %#+v", c)
	//fmt.Printf("Target: %v", c)

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

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")

	//	_, err := p.GenerateCommands(c.Targets, rootCmd, []string{})

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	rootCmd.Execute()
}
