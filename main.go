package main

import (
	"bytes"
	"fmt"
	"github.com/mumoshu/variant/cmd"
	"os"
)

func main() {
	// See https://help.github.com/en/articles/virtual-environments-for-github-actions#environment-variables
	// for the list of supported envvars that can be used to determine it is running on GitHub Actions (or not)
	if os.Getenv("GITHUB_ACTION") != "" {
		stdout := os.Stdout

		stdoutR, stdoutW, _ := os.Pipe()
		os.Stdout = stdoutW

		stderr := os.Stderr

		stderrR, stderrW, _ := os.Pipe()
		os.Stderr = stderrW

		stdoutCaptureStop := make(chan struct{})
		stderrCaptureStop := make(chan struct{})

		var allBuf bytes.Buffer

		// Concurrently read stdout to not block writes from cmd.MustRun()
		var stdoutBuf bytes.Buffer
		go func() {
			line := linesScanner(stdoutR, stderr)
			for line.Scan() {
				str := line.Text()
				//stdout.WriteString(fmt.Sprintf("str=%q, cnt=%d\n", str, charCount(str, stderr)))
				if charCount(str, stderr) > 0 {
					str += "\n"
				}
				if _, err := stdoutBuf.WriteString(str); err != nil {
					panic(err)
				}
				if _, err := allBuf.WriteString(str); err != nil {
					panic(err)
				}
				stdout.WriteString(str)
			}
			close(stdoutCaptureStop)
		}()

		// Concurrently read stderr to not block writes from cmd.MustRun()
		var stderrBuf bytes.Buffer
		go func() {
			line := linesScanner(stderrR, stderr)
			for line.Scan() {
				str := line.Text()
				//stdout.WriteString(fmt.Sprintf("str=%q, cnt=%d\n", str, charCount(str, stderr)))
				if charCount(str, stderr) > 0 {
					str += "\n"
				}
				if _, err := stderrBuf.WriteString(str); err != nil {
					panic(err)
				}
				if _, err := allBuf.WriteString(str); err != nil {
					panic(err)
				}
				stderr.WriteString(str)
			}
			close(stderrCaptureStop)
		}()

		runOpts, runErr := cmd.RunE()

		// Restore stdout/err before closing the pipes. Otherwise writes to os.Stdout/err(pipes) fail because they are closed
		os.Stdout = stdout
		os.Stderr = stderr

		stdoutW.Close()
		stderrW.Close()

		<-stdoutCaptureStop
		<-stderrCaptureStop

		stdoutCapture := stdoutBuf.String()
		allCapture := allBuf.String()

		send := func() error {
			name := os.Getenv("VARIANT_NAME")
			if name == "" {
				name = "variant"
			}

			c := os.Getenv("VARIANT_RUN")

			status := cmd.GetStatus(runErr, runOpts)
			return sendGitHubComment(name, c, fmt.Sprintf("%d", status), stdoutCapture, allCapture)
		}

		if runErr != nil {
			if os.Getenv("VARIANT_GITHUB_COMMENT") != "" || os.Getenv("VARIANT_GITHUB_COMMENT_ON_FAILURE") != "" {
				if err := send(); err != nil {
					cmd.HandleError(err, runOpts)
				}
			}
			cmd.HandleError(runErr, runOpts)
		} else {
			if os.Getenv("VARIANT_GITHUB_COMMENT") != "" || os.Getenv("VARIANT_GITHUB_COMMENT_ON_SUCCESS") != "" {
				if err := send(); err != nil {
					cmd.HandleError(err, runOpts)
				}
			}
		}
	} else {
		cmd.MustRun()
	}
}
