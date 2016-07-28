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
	prefixed "github.com/x-cray/logrus-prefixed-formatter"

	"github.com/davecgh/go-spew/spew"
	"github.com/juju/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func init() {
	log.SetFormatter(new(prefixed.TextFormatter))

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

	return errors.Trace(err)
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
	Targets     []*Target `yaml:"targets,omitempty"`
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
				return nil, errors.Errorf("The value for the key %s was neither a `map[string]interface{}` nor a `string`: %v", k, v)
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

	invocation := struct {
		Stdout chan bool
		Stderr chan bool
	}{
		Stdout: make(chan bool),
		Stderr: make(chan bool),
	}

	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating StdoutPipe for Cmd", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(cmdReader)
	var output string
	go func() {
		defer func() {
			invocation.Stdout <- true
		}()
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
		defer func() {
			invocation.Stderr <- true
		}()
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
	err = cmd.Wait()

	<-invocation.Stdout
	<-invocation.Stderr

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

func (p Project) RunTask(taskKey TaskKey, options map[string]string, args []string) (string, error) {
	t, err := p.FindTask(taskKey)

	if err != nil {
		return "", errors.Annotate(err, "RunTaskError")
	}

	vars := map[string](interface{}){}
	vars["mysql"] = map[string]string{"host": "mysql2"}

	log.Debugf("Project: %s", spew.Sdump(p))
	log.Debugf("TaskKey: %s", spew.Sdump(taskKey))
	log.Debugf("TaskDef: %s", spew.Sdump(t))

	inputs, err := p.AggregateInputsFor(taskKey, args)

	if err != nil {
		return "", errors.Annotatef(err, "Task `%s` failed", taskKey.String())
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

	output, error := task.Run()

	if error != nil {
		error = errors.Annotatef(error, "Task `%s` failed", taskKey.String())
	}

	return output, error
}

func (p Project) AggregateInputsFor(taskKey TaskKey, args []string) (map[string]interface{}, error) {
	//	task := p.FindTask(taskKey)
	aggregated := map[string]interface{}{}
	if err := p.CollectInputsFor(taskKey, aggregated, args); err != nil {
		return nil, errors.Annotatef(err, "AggregateInputsFor(%s) failed", taskKey.String())
	}
	if err := p.AggregateInputsForParent(taskKey, aggregated); err != nil {
		return nil, errors.Annotatef(err, "AggregateInputsFor(%s) failed", taskKey.String())
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
				output, err = p.RunTask(p.CreateTaskKeyFromVariable(input), map[string]string{}, []string{})
				if err != nil {
					log.Errorf("`%s` required by `%s` is not provided. You have to provided it via a command line option or a positional argument!", k, taskKey.String())
					return errors.Annotatef(err, "Task `%s` failed. No command line options or positional arguments provided for the input `%s`.", k, k)
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

func (t Task) Run() (string, error) {
	var buff bytes.Buffer
	if err := t.Template.Execute(&buff, t.Vars); err != nil {
		return "", errors.Annotatef(err, "Template execution failed.\n\nScript:\n%s\n\nVars:\n%v", t.TaskDef.Target.Script, t.Vars)
	}

	script := buff.String()

	output, err := t.RunScript(script)

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

	options := map[string]string{}

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
			if _, err := p.RunTask(taskKey, options, args); err != nil {
				log.Errorf("%s failed.", taskKey.String())
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
				cmd.PersistentFlags().StringP(name, string(input.Name[0]), "", description)
				viper.BindPFlag(name, cmd.PersistentFlags().Lookup(flagName))
			}
		}
	}
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
		//return nil, errors.Errorf("failed to parse cluster: %v", err)
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

	p.GenerateAllFlags()

	rootCmd.PersistentFlags().BoolVarP(&(p.Verbose), "verbose", "v", false, "verbose output")

	//	_, err := p.GenerateCommands(c.Targets, rootCmd, []string{})

	if err != nil {
		log.Fatalf("error: %v", err)
	}

	rootCmd.Execute()
}
