package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	var (
		old string
		new string
	)

	command := &cobra.Command{
		Use:     "rpcdiff",
		Short:   "use rpcdiff to compare two openrpc schemas",
		Version: "0.0.0",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			diff, err := NewDiffFiles(old, new)
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println(diff.String())
		},
	}

	flags := command.Flags()
	flags.SortFlags = false

	flags.StringVarP(&old, "old", "o", "", "path to old schema file")
	cobra.MarkFlagRequired(flags, "old")

	flags.StringVarP(&new, "new", "n", "", "path to new schema file")
	cobra.MarkFlagRequired(flags, "new")

	command.Execute()
}
