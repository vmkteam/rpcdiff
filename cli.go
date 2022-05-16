package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	var (
		old  string
		new  string
		opts Options
	)

	command := &cobra.Command{
		Use:     "rpcdiff",
		Short:   "use rpcdiff to compare two openrpc schemas",
		Version: "0.0.0",
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		Run: func(cmd *cobra.Command, args []string) {
			diff, err := NewDiff(old, new, opts)
			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println(diff.String())
		},
	}

	flags := command.Flags()
	flags.SortFlags = false

	flags.StringVarP(&old, "old", "o", "", "path/url to old schema")
	cobra.MarkFlagRequired(flags, "old")

	flags.StringVarP(&new, "new", "n", "", "path/url to new schema")
	cobra.MarkFlagRequired(flags, "new")

	flags.BoolVar(&opts.ShowMeta, "compare-meta", false, "true to compare schema meta info")

	command.Execute()
}
