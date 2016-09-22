package steps

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"

	"../engine"
)

type ScriptStepLoader struct{}

func (l ScriptStepLoader) TryToLoad(stepConfig engine.StepConfig) engine.Step {
	if code, isStr := stepConfig.Script.(string); isStr && code != "" {
		return ScriptStep{
			Name: stepConfig.Name,
			Code: code,
		}
	}

	return nil
}

func NewScriptStepLoader() ScriptStepLoader {
	return ScriptStepLoader{}
}

type ScriptStep struct {
	Name string
	Code string
}

func (s ScriptStep) GetName() string {
	return s.Name
}

func (s ScriptStep) Run(project *engine.Project, flow *engine.Flow, caller ...engine.FlowDef) (engine.StepStringOutput, error) {
	depended := len(caller) > 0

	t := template.New(fmt.Sprintf("%s.definition.yaml: %s.%s.script", flow.ProjectName, s.GetName(), flow.Key.ShortString()))
	t.Option("missingkey=error")

	tmpl, err := t.Funcs(flow.CreateFuncMap()).Parse(s.Code)
	if err != nil {
		log.Errorf("Error: %v", err)
	}

	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, flow.Vars); err != nil {
		return engine.StepStringOutput{String: "scripterror"}, errors.Annotatef(err, "Template execution failed.\n\nScript:\n%s\n\nVars:\n%v", s.Code, flow.Vars)
	}

	script := buff.String()

	output, err := s.RunScript(script, depended, flow)

	return engine.StepStringOutput{String: output}, err
}

func (t ScriptStep) RunScript(script string, depended bool, flow *engine.Flow) (string, error) {
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

func (t ScriptStep) RunCommand(command string, depended bool, parentFlow *engine.Flow) (string, error) {
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
