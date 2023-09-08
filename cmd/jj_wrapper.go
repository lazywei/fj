package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

func getChangeIDs(revset string) ([]string, error) {
	out, err := Run(exec.Command(
		"jj", "log", "--no-graph", "--reversed",
		"-r", revset,
		"-T", `change_id ++ "\n"`,
	))
	if err != nil {
		return []string{}, fmt.Errorf("jj out: %s\nerr: %s", out, err)
	}

	changeIDs := strings.Split(out, "\n")
	return changeIDs, nil
}
