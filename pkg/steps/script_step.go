package steps

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/juju/errors"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/mumoshu/variant/pkg/api/step"
)

type ScriptStepLoader struct{}

func (l ScriptStepLoader) LoadStep(stepConfig step.StepDef, context step.LoadingContext) (step.Step, error) {
	code, isStr := stepConfig.Get("script").(string)

	log.Debugf("step config: %v", stepConfig)

	if isStr && code != "" {
		return ScriptStep{
			Name:   stepConfig.GetName(),
			Code:   code,
			silent: stepConfig.Silent(),
		}, nil
	}

	return nil, fmt.Errorf("no script step found. script=%v, isStr=%v, config=%v", stepConfig.Get("script"), isStr, stepConfig)
}

func NewScriptStepLoader() ScriptStepLoader {
	return ScriptStepLoader{}
}

type ScriptStep struct {
	Name   string
	Code   string
	silent bool
}

func (s ScriptStep) Silent() bool {
	return s.silent
}

func (s ScriptStep) GetName() string {
	return s.Name
}

func (s ScriptStep) Run(context step.ExecutionContext) (step.StepStringOutput, error) {
	depended := len(context.Caller()) > 0

	script, err := context.Render(s.Code, s.GetName())
	if err != nil {
		log.WithFields(log.Fields{"source": s.Code, "vars": context.Vars}).Errorf("script step failed templating")
		return step.StepStringOutput{String: "scripterror"}, errors.Annotatef(err, "script step failed templating")
	}

	output, err := s.RunScript(script, depended, context)

	return step.StepStringOutput{String: output}, err
}

func (t ScriptStep) RunScript(script string, depended bool, context step.ExecutionContext) (string, error) {
	//commands := strings.Split(script, "\n")
	commands := []string{script}
	var lastOutput string
	for _, command := range commands {
		if command != "" {
			output, err := t.RunCommand(command, depended, context)
			if err != nil {
				return output, err
			}
			lastOutput = output
		}
	}
	return lastOutput, nil
}

func (t ScriptStep) RunCommand(command string, depended bool, context step.ExecutionContext) (string, error) {
	c := "sh"
	args := []string{"-c", command}

	ctx := log.WithFields(log.Fields{"cmd": append([]string{c}, args...)})

	ctx.Debug("script step started")

	cmd := exec.Command(c, args...)

	mergedEnv := map[string]string{}

	for _, pair := range os.Environ() {
		splits := strings.SplitN(pair, "=", 2)
		key, value := splits[0], splits[1]
		mergedEnv[key] = value
	}

	if context.Autoenv() {
		autoEnv, err := context.GenerateAutoenv()
		if err != nil {
			log.Errorf("script step failed to generate autoenv: %v", err)
		}
		for name, value := range autoEnv {
			mergedEnv[name] = value
		}

		cmdEnv := []string{}
		for name, value := range mergedEnv {
			cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", name, value))
		}

		cmd.Env = cmdEnv
	}

	if context.Autodir() {
		parentKey, err := context.Key().Parent()
		if parentKey != nil {
			shortKey := parentKey.ShortString()
			path := strings.Replace(shortKey, ".", "/", -1)
			if err != nil {
				log.Debugf("%s does not have parent", context.Key().ShortString())
			} else {
				if _, err := os.Stat(path); err == nil {
					cmd.Dir = path
				}
			}
		}
	}

	output := ""

	if context.Interactive() {
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

		stdoutlog := log.WithFields(log.Fields{"stream": "stdout"})
		stderrlog := log.WithFields(log.Fields{"stream": "stderr"})

		// Coordinating stdout/stderr in this single place to not screw up message ordering
		for {
			select {
			case text, ok := <-channels.Stdout:
				if ok {
					//if depended {
					//	stdoutlog.Debug(text)
					//} else {
					stdoutlog.Info(text)
					//}
				} else {
					stdoutEnds = true
				}
			case text, ok := <-channels.Stderr:
				if ok {
					stderrlog.Info(text)
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
		ctx.Errorf("script step failed: %v", err)
		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			print([]byte(fmt.Sprintf("%d", waitStatus.ExitStatus())))
		}
		return "scripterror", errors.Annotate(err, "script step failed")
	} else {
		// Command was successful
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		log.Debugf("script step finished command with status: %d", waitStatus.ExitStatus())
	}

	return strings.Trim(output, "\n "), nil
}
