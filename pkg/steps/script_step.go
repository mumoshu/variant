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

	"archive/tar"
	"compress/gzip"
	"github.com/mumoshu/variant/pkg/api/step"
	"io"
	"path/filepath"
	"runtime"
)

type ScriptStepLoader struct{}

func (l ScriptStepLoader) LoadStep(def step.StepDef, context step.LoadingContext) (step.Step, error) {
	script, isStr := def.Script()

	var runConf *runnerConfig
	{
		runner, ok := def.Get("runner").(map[string]interface{})
		log.Debugf("runner: %+v", runner)
		log.Debugf("def: %+v", def)
		if ok {
			args := []string{}
			switch a := runner["args"].(type) {
			case []interface{}:
				for _, arg := range a {
					args = append(args, arg.(string))
				}
			}
			artifacts := []Artifact{}
			switch rawArts := runner["artifacts"].(type) {
			case []interface{}:
				for _, rawArt := range rawArts {
					switch art := rawArt.(type) {
					case map[interface{}]interface{}:
						a := Artifact{
							Name: art["name"].(string),
							Path: art["path"].(string),
							Via:  art["via"].(string),
						}
						artifacts = append(artifacts, a)
					default:
						panic(fmt.Errorf("unexpected type of artifact"))
					}
				}
			case nil:

			default:
				panic(fmt.Errorf("unexpected type of artifacts"))
			}
			runConf = &runnerConfig{
				Image:     runner["image"].(string),
				Args:      args,
				Artifacts: artifacts,
			}
			if entrypoint, ok := runner["entrypoint"].(string); ok {
				runConf.Entrypoint = &entrypoint
			}
			if command, ok := runner["command"].(string); ok {
				runConf.Command = command
			}
			if envfile, ok := runner["envfile"].(string); ok {
				runConf.Envfile = envfile
			}
			if volumes, ok := runner["volumes"].([]interface{}); ok {
				vols := make([]string, len(volumes))
				for i, v := range volumes {
					vols[i] = os.ExpandEnv(v.(string))
				}
				runConf.Volumes = vols
			}
		} else {
			log.Debugf("runner wasn't expected type of map: %+v", runner)
		}

	}

	log.Debugf("step config: %v", def)

	if isStr && script != "" {
		step := ScriptStep{
			Name:   def.GetName(),
			Code:   script,
			silent: def.Silent(),
		}
		if runConf != nil {
			step.runnerConfig = *runConf
		}
		return step, nil
	}

	return nil, fmt.Errorf("no script step found. script=%v, isStr=%v, config=%v", def.Get("script"), isStr, def)
}

func NewScriptStepLoader() ScriptStepLoader {
	return ScriptStepLoader{}
}

type ScriptStep struct {
	Name         string
	Code         string
	silent       bool
	runnerConfig runnerConfig
}

type Artifact struct {
	Name string
	Path string
	Via  string
}

type runnerConfig struct {
	Image      string
	Command    string
	Entrypoint *string
	Artifacts  []Artifact
	Args       []string
	Envfile    string
	Volumes    []string
}

func (c runnerConfig) commandNameAndArgsToRunScript(script string, context step.ExecutionContext) (string, []string) {
	var cmd string
	if c.Command != "" {
		cmd = c.Command
	} else if c.Image == "" {
		cmd = "sh"
	}

	for _, a := range c.Artifacts {
		s3Prefix, err := context.Render(a.Via, "runner.via")
		if err != nil {
			panic(err)
		}
		name := a.Name
		setup := fmt.Sprintf(`echo downloading artifacts from %s/%s.tgz 1>&2
aws s3 cp %s/%s.tgz %s.tgz 1>&2
tar zxvf %s.tgz 1>&2
`, s3Prefix, name, s3Prefix, name, name, name)
		script = setup + script
	}

	var cmdArgs []string
	if c.Args != nil {
		cmdArgs = append([]string{}, c.Args...)
		cmdArgs = append(cmdArgs, script)
	} else {
		cmdArgs = []string{"-c", script}
	}

	if c.Image != "" {
		dockerArgs := []string{}
		for _, v := range c.Volumes {
			dockerArgs = append(dockerArgs, "-v", v)
		}
		if c.Envfile != "" {
			dockerArgs = append(dockerArgs, "--env-file", c.Envfile)
		} else if context.Autoenv() {
			autoEnv, err := context.GenerateAutoenv()
			if err != nil {
				log.Errorf("script step failed to generate autoenv: %v", err)
			}
			for name, value := range autoEnv {
				if value == "" {
					continue
				}
				dockerArgs = append(dockerArgs, "--env", fmt.Sprintf("%s=%s", name, value))
			}
		}
		if c.Entrypoint != nil {
			dockerArgs = append(dockerArgs, "--entrypoint", *c.Entrypoint)
		}
		var args []string
		args = append(args, dockerArgs...)
		args = append(args, "--entrypoint", c.Entrypoint)
		args = append(args, c.Image)
		args = append(args, cmd)
		args = append(args, cmdArgs...)

		return "docker", append([]string{"run", "--rm", "-i"}, args...)
	} else {
		return cmd, cmdArgs
	}
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

	output, err := s.runScriptWithArtifacts(script, depended, context)

	return step.StepStringOutput{String: output}, err
}

func (t ScriptStep) runScriptWithArtifacts(script string, depended bool, context step.ExecutionContext) (string, error) {
	for _, a := range t.runnerConfig.Artifacts {
		err := createTarFromGlob(fmt.Sprintf("%s.tgz", a.Name), a.Path)
		if err != nil {
			return "", err
		}
		via, err := context.Render(a.Via, "runner.via")
		if err != nil {
			return "", err
		}
		setup := fmt.Sprintf(`aws s3 cp %s.tgz %s/%s.tgz 1>&2`, a.Name, via, a.Name)
		name, args := runnerConfig{}.commandNameAndArgsToRunScript(setup, context)
		out, err := t.runCommand(name, args, depended, context)
		if err != nil {
			return out, err
		}
	}

	name, args := t.runnerConfig.commandNameAndArgsToRunScript(script, context)
	output, err := t.runCommand(name, args, depended, context)
	if err != nil {
		return output, err
	}
	return output, nil
}

func (t ScriptStep) runCommand(name string, args []string, depended bool, context step.ExecutionContext) (string, error) {
	ctx := log.WithFields(log.Fields{"cmd": append([]string{name}, args...)})

	ctx.Debug("script step started")

	cmd := exec.Command(name, args...)

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
				errOutPrefix := "variant.stderr: "
				if strings.HasPrefix(text, errOutPrefix) {
					channels.Stderr <- strings.SplitN(text, errOutPrefix, 2)[1]
				} else {
					channels.Stdout <- text
					if output != "" {
						output += "\n"
					}
					output += text
				}
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

func createTarFromGlob(filename string, pattern string) error {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	return createTarFromFiles(filename, paths)
}

func createTarFromFiles(filename string, paths []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	var fileWriter io.WriteCloser = file
	if strings.HasSuffix(filename, ".gz") || strings.HasSuffix(filename, ".tgz") {
		fileWriter = gzip.NewWriter(file)
		defer fileWriter.Close()
	}
	writer := tar.NewWriter(fileWriter)
	defer writer.Close()
	for _, p := range paths {
		if err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				if err := writeFileToTar(writer, path); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeFileToTar(writer *tar.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	header := &tar.Header{
		Name:    sanitizedName(filename),
		Mode:    int64(stat.Mode()),
		Uid:     os.Getuid(),
		Gid:     os.Getgid(),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}
	if err = writer.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func sanitizedName(filename string) string {
	if len(filename) > 1 && filename[1] == ':' &&
		runtime.GOOS == "windows" {
		filename = filename[2:]
	}
	filename = filepath.ToSlash(filename)
	filename = strings.TrimLeft(filename, "/.")
	return strings.Replace(filename, "../", "", -1)
}
