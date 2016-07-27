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

	//	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var data = `
inputs:
- name: env
targets:
- name: web
  description: web関連のコマンド
  targets:
  - name: deploy
    description: webをデプロイする
    inputs:
    - name: mysql.host
      description: Webサーバの接続先となるMySQLホスト
    script: |
      echo deploy"(mysql_host={{.mysql.host}})"
      echo {{index .args 0 }}
      echo err message 1>&2
      echo {{.env}}
      MYSQL_HOST={{.mysql.host}} sh -c 'export | grep MYSQL_HOST'
- name: mysql
  targets:
  - name: host
    script: |
      echo mysql
`

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
}

type ProjectConfig struct {
	Name        string
	Description string    `yaml:"description,omitempty"`
	Inputs      []*Input  `yaml:"inputs,omitempty"`
	Targets     []*Target `yaml:"targets,omitempty"`
	Script      string    `yaml:"script,omitempty"`
}

type Task struct {
	Key      TaskKey
	Template *template.Template
	Inputs   []*Input
}

type TaskKey struct {
	Components []string
}

type Project struct {
	Name              string
	Tasks             map[string]*Task
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
	}
}

func runScript(script string) (string, error) {
	commands := strings.Split(script, "\n")
	var lastOutput string
	for _, command := range commands {
		if command != "" {
			output, err := runCommand(command)
			if err != nil {
				return output, err
			}
			lastOutput = output
		}
	}
	return lastOutput, nil
}

func runCommand(command string) (string, error) {
	c := "sh"
	args := []string{"-c", command}
	log.Debugf("running command: %s", command)
	log.Debugf("shelling out: %v", append([]string{c}, args...))
	cmd := exec.Command(c, args...)
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
			log.WithFields(log.Fields{"stream": "stdout", "command": command}).Printf("%s", text)
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
			log.WithFields(log.Fields{"stream": "stderr", "command": command}).Errorf("%s", text)
		}
	}()

	err = cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error starting Cmd", err)
		os.Exit(1)
	}

	var waitStatus syscall.WaitStatus
	if err := cmd.Wait(); err != nil {
		log.Fatalf("%v", err)
		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			print([]byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
	} else {
		// Command was successful
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		log.WithFields(log.Fields{"command": command}).Debugf("exit status: %d", waitStatus.ExitStatus())
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

	log.Errorf("p=%v, taskKey=%v, t=%v", p, taskKey, t)

	inputs, _ := p.AggregateInputsFor(taskKey)
	for k, v := range inputs {
		vars[k] = v
	}

	output, error := t.Run(vars)
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

func (p *Project) FindTask(taskKey TaskKey) *Task {
	return p.Tasks[taskKey.String()]
}

func (p *Project) RegisterTask(taskKey TaskKey, task *Task) {
	p.Tasks[taskKey.String()] = task
}

func (t Task) Run(vars map[string]interface{}) (string, error) {
	var buff bytes.Buffer
	if err := t.Template.Execute(&buff, vars); err != nil {
		log.Panicf("Error: %v", err)
	}

	script := buff.String()

	output, err := runScript(script)

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
	task := &Task{
		Key:    taskKey,
		Inputs: target.Inputs,
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
		Tasks:             map[string]*Task{},
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
