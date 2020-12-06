package cmd

import (
	"os"
	"text/tabwriter"

	"github.com/k14s/starlark-go/starlark"
	"github.com/wonderix/shalm/pkg/shalm"

	"github.com/spf13/cobra"
)

var listOptions = &shalm.RepoListOptions{}
var listK8sArgs = &shalm.K8sConfigs{}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list shalm charts",
	Long:  ``,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		k8s, err := k8s(listK8sArgs.Merge())
		if err != nil {
			exit(err)
		}
		exit(list(k8s, listOptions))
	},
}

func list(k8s shalm.K8s, listOptions *shalm.RepoListOptions) error {
	repo, err := repo()
	if err != nil {
		return err
	}
	thread := &starlark.Thread{Name: "main", Load: rootExecuteOptions.load}
	charts, err := repo.List(thread, k8s, listOptions)
	if err != nil {
		return err
	}
	writer := tabwriter.NewWriter(os.Stdout, 3, 4, 1, ' ', 0)
	defer writer.Flush()
	writer.Write([]byte("GENUS\tNAMESPACE\tVERSION\n"))
	for _, c := range charts {
		writer.Write([]byte(c.GetGenus() + "\t" + c.GetNamespace() + "\t" + c.GetVersion().String() + "\n"))
	}
	return nil
}

func init() {
	listOptions.AddFlags(listCmd.Flags())
	listK8sArgs.AddFlags(listCmd.Flags())
}
