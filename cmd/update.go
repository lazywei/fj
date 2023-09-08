package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "up",
	Short: "jj git fetch + rebase -d master/main + drop empty commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := Run(exec.Command("jj", "git", "fetch"))
		if err != nil {
			return fmt.Errorf("failed to fetch from git, out: %s, err: %v", out, err)
		}

		out, err = Run(exec.Command("jj", "rebase", "-d", mainBranch))
		if err != nil {
			return fmt.Errorf("failed to rebase to %s, out: %s, err: %v",
				mainBranch, out, err)
		}

		emptyChangeIDs, err := getChangeIDs(fmt.Sprintf("(%s..@) & empty()", mainBranch))
		if err != nil {
			return fmt.Errorf("failed to get empty change IDs, err: %v", err)
		}
		// c := exec.Command(
		// 	"jj", "log", "-T",
		// 	`change_id.short() ++ if(empty, " -> (empty)") ++ "\n" ++ if(empty, description, description.first_line())`,
		// 	// `separate(" ", `+
		// 	// 	`, change_id.short(),`+
		// 	// 	`if(empty, "\n" ++ description, description.first_line()))`,
		// 	"-r", fmt.Sprintf("%s-..@", mainBranch),
		// )
		c := exec.Command("jj", "log", "-r", fmt.Sprintf("%s-..@", mainBranch))
		c.Stdout = os.Stdout
		c.Run()

		// 		currentStatus, err := Run(exec.Command(
		// 			"jj", "log", "-T",
		// 			`change_id.short() ++ if(empty, " -> (empty)") ++ "\n" ++ if(empty, description, description.first_line())`,
		// 			// `separate(" ", `+
		// 			// 	`, change_id.short(),`+
		// 			// 	`if(empty, "\n" ++ description, description.first_line()))`,
		// 			"-r", fmt.Sprintf("%s-..@", mainBranch),
		// 		))
		// 		if err != nil {
		// 			return fmt.Errorf("failed to get current status, jj out: %s\nerr: %v", currentStatus, err)
		// 		}
		// fmt.Println(currentStatus)

		for _, changeID := range emptyChangeIDs {
			fmt.Printf("Abandoning change '%s'? (y/n) ", changeID[:5])

			reader := bufio.NewReader(os.Stdin)
			text, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read from stdin, err: %v", err)
			}
			text = strings.ToLower(strings.Trim(text, "\n"))

			if text == "y" || text == "yes" {
				Run(exec.Command("jj", "abandon", "-r", changeID))
			} else {
				fmt.Println("Abort")
				return nil
			}
		}

		return nil
	},
}
