package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"text/template"
)

var commentBody = "#### $VARIANT_NAME: `$VARIANT_RUN` completed.Status: ${SUCCESS}" +
	"{{ if .Summary }}\n```\n{{ .Summary }}```\n{{ end -}}" +
	"<details>\n" +
	"```\n" +
	"{{.Details}}" +
	"```\n" +
	"</details>\n"

func sendGitHubComment(summary, details string) error {
	data := map[string]string{
		"Summary": summary,
		"Details": details,
	}

	tpl := template.New("comment")
	tpl, err := tpl.Parse(commentBody)
	if err != nil {
		return err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}

	println("Trying to send a GitHub Issue/Pull request comment:")
	println(buf.String())

	if err := postGithubComment(buf.String()); err != nil {
		return err
	}

	return nil
}

func postGithubComment(commentBody string) error {
	u, err := getCommentsURL()
	if err != nil {
		return err
	}
	reqBody, err := json.Marshal(map[string]string{"body": commentBody})
	req, err := http.NewRequest("POST", u, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("token %s", os.Getenv("GITHUB_TOKEN")))
	req.Header.Add("Content-Type", "application/json")

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "finished posting a github comment: code=%d, response=%s\n", resp.StatusCode, string(contents))

	return nil
}

func getCommentsURL() (string, error) {
	// See https://help.github.com/en/articles/virtual-environments-for-github-actions#default-environment-variables
	// This is usually /github/workflow/event.json
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	event, err := ioutil.ReadFile(eventPath)
	if err != nil {
		return "", fmt.Errorf("get comments URL: %v", err)
	}
	evt := struct {
		PullRequest struct {
			CommentsURL string `json:"comments_url"`
		} `json:"pull_request"`
		Issue struct {
			CommentsURL string `json:"comments_url"`
		} `json:"issue"`
	}{}
	if err := json.Unmarshal(event, &evt); err != nil {
		return "", err
	}
	if evt.PullRequest.CommentsURL != "" {
		return evt.PullRequest.CommentsURL, nil
	}
	if evt.Issue.CommentsURL != "" {
		return evt.Issue.CommentsURL, nil
	}
	return "", fmt.Errorf("unable to detect issue comments URL in event.json: %s", string(event))
}

func linesScanner(reader io.Reader, log io.Writer) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	buf := make([]byte, 1024)
	scanner.Buffer(buf, bufio.MaxScanTokenSize)
	return scanner
}

func charCount(str string, log io.Writer) int {
	// Ignore ASCII color
	p := regexp.MustCompile("\033" + `\[[^m]+m`)
	str = p.ReplaceAllString(str, "")
	return len(str)
}
