package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func run(c *exec.Cmd) (string, error) {
	out, err := c.CombinedOutput()
	ret := strings.TrimRight(string(out), "\n")
	return ret, err
}

func runStdout(c *exec.Cmd) error {
	c.Stdout = os.Stdout
	return c.Run()
}

func getChangeIDs(revset string) ([]string, error) {
	out, err := run(exec.Command(
		"jj", "log", "--no-graph", "--reversed",
		"-r", revset,
		"-T", `change_id ++ "\n"`,
	))
	if err != nil {
		return []string{}, fmt.Errorf("jj out: %s\nerr: %s", out, err)
	}

	if len(out) == 0 {
		return []string{}, nil
	}

	changeIDs := strings.Split(out, "\n")
	return changeIDs, nil
}
