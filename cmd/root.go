package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

var (
	k = *koanf.New(".")

	createDraftPR bool
	mainBranch    string
)

var rootCmd = &cobra.Command{
	Use:          "fj",
	Short:        "A generator for Cobra based Applications",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initConfig()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		changeIDs, err := getStackChangeIDs()
		if err != nil {
			return fmt.Errorf("failed to get stack change IDs, err: %v", err)
		}

		prStack := []int{}
		descriptions := []string{}

		lastBranch := ""
		for _, changeID := range changeIDs {
			desc, err := getDescription(changeID)
			if err != nil {
				slog.Error("Failed to get description", "change", changeID, "err", err)
				os.Exit(1)
			}

			branch, err := getBranch(changeID)
			if err != nil {
				slog.Error("Failed to get branch", "change", changeID, "err", err)
				os.Exit(1)
			}

			if branch == "" {
				nextPRNum, err := getNextAvailablePRNumber()
				if err != nil {
					slog.Error("Failed to getNextAvailablePRNumber", "err", err)
					os.Exit(1)
				}

				branch, err = createBranch(changeID, nextPRNum)
				if err != nil {
					slog.Error("Failed to create branch", "err", err)
					os.Exit(1)
				}
			}

			if err := exec.Command("jj", "git", "push", "-r", changeID).Run(); err != nil {
				slog.Error("Failed to push branch to remote", "err", err)
				os.Exit(1)
			}
			slog.Debug("Branch pushed to remote")

			prNum, err := getPRNumber(branch)
			if err != nil {
				slog.Error("Failed to check PR exisitence", "err", err)
				os.Exit(1)
			}

			if prNum == -1 {
				slog.Debug("No PR created for branch yet, creating", "branch", branch)

				var baseBranch string
				if lastBranch == "" {
					baseBranch = mainBranch
				} else {
					baseBranch = lastBranch
				}

				args := []string{"pr", "create", "-H", branch, "-B", baseBranch, "--fill-first"}
				if createDraftPR {
					args = append(args, "--draft")
				}
				out, err := run(exec.Command("gh", args...))
				if err != nil {
					slog.Error("Failed to create PR", "output", out, "err", err)
					os.Exit(1)
				}
				slog.Debug("PR created", "output", out)
				prNum, _ = getPRNumber(branch)
			}
			prStack = append(prStack, prNum)
			descriptions = append(descriptions, desc)

			lastBranch = branch
		}

		for i, prNum := range prStack {
			desc := descriptions[i]

			var prInfo string
			if len(prStack) > 1 {
				prInfo = "\n---"
				for j := len(prStack) - 1; j >= 0; j-- {
					if i == j {
						prInfo += fmt.Sprintf("\n* **->** #%d", prStack[j])
					} else {
						prInfo += fmt.Sprintf("\n* #%d", prStack[j])
					}
				}
			} else {
				prInfo = ""
			}

			if out, err := run(exec.Command("gh", "pr", "edit", fmt.Sprint(prNum), "-b", desc+"\n"+prInfo)); err == nil {
				fmt.Println("Successfully updated PR:", out)
			}
		}

		return nil
	},
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {
	// Load default values using the confmap provider.
	// We provide a flat map with the "." delimiter.
	// A nested map can be loaded by setting the delimiter to an empty string "".
	k.Load(confmap.Provider(map[string]interface{}{
		"mainBranch":   "main",
		"branchPrefix": "username/pr-",
		"draft":        true,
	}, "."), nil)

	projectRoot, err := run(exec.Command("git", "rev-parse", "--show-toplevel"))
	if err != nil {
		slog.Error("Not in a git repo", "err", err)
		os.Exit(1)
	}
	configPath := filepath.Join(projectRoot, ".fj.toml")

	// Load TOML config on top of the default values.
	if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
		b, err := k.Marshal(toml.Parser())
		if err != nil {
			slog.Error("Failed to marshal default config", "err", err)
			os.Exit(1)
		}
		if err := os.WriteFile(configPath, b, 0o644); err != nil {
			slog.Error("Failed to write default config", "configPath", configPath, "err", err)
			os.Exit(1)
		}
		slog.Info("Initialized default config, please re-run the command", "configPath", configPath)
		os.Exit(0)
	}

	mainBranch = k.MustString("mainBranch")
	if k.Bool("draft") {
		createDraftPR = true
	}
}

func getStackChangeIDs() ([]string, error) {
	return getChangeIDs(fmt.Sprintf("%s..@-", mainBranch))
}

func getDescription(changeID string) (string, error) {
	return run(exec.Command(
		"jj", "log", "--no-graph",
		"-T", `description`,
		"-r", changeID,
	))
}

func getBranch(changeID string) (string, error) {
	out, err := run(exec.Command(
		"jj", "branch", "list",
		"-r", changeID,
	))
	if err != nil {
		return out, err
	}
	return strings.Split(out, ":")[0], nil
}

func getNextAvailablePRNumber() (int, error) {
	cmd := "gh pr list -L 1 --state all --json number | jq '.[0].number'"
	out, err := run(exec.Command("bash", "-c", cmd))
	if err != nil {
		return -1, fmt.Errorf("output: %s, err: %v", out, err)
	}
	if out == "null" {
		// No PRs yet
		return 1, nil
	}
	curPRNum, err := strconv.Atoi(out)
	if err != nil {
		return -1, err
	}

	return curPRNum + 1, nil
}

func createBranch(changeID string, nextPRNum int) (string, error) {
	branchName := fmt.Sprintf("%s%d", k.MustString("branchPrefix"), nextPRNum)

	out, err := run(exec.Command("jj", "branch", "create", "-r", changeID, branchName))
	if err != nil {
		return "", fmt.Errorf("output: %s, err: %v", out, err)
	}

	return branchName, nil
}

func getPRNumber(branch string) (int, error) {
	cmd := fmt.Sprintf("gh pr list -L 1 --state all --json number --head %s | jq '.[0].number'", branch)
	out, err := run(exec.Command("bash", "-c", cmd))
	if err != nil {
		return -1, err
	}

	number, err := strconv.Atoi(out)
	if err != nil {
		return -1, nil
	}

	return number, nil
}
