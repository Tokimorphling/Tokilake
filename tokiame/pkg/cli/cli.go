package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type RootCmdArgs struct {
	Namespace string
	Address   string
	// Password string // If you uncomment this later

	ApiAddress string
}

var rootCmdArgs = RootCmdArgs{} // Global instance to store flag values

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "Tokiame", // Replace with the actual name of your executable
	Short: "Cotrol your GPUs",
	Long: `This tool is a client for Tokilake.
It requires a namespace to identify the client and the remote address of the Tokilake service.
Both flags are mandatory for the tool to operate.`,
	RunE: func(cmd *cobra.Command, args []string) error {

		fmt.Printf("Configuration loaded: Namespace='%s', Address='%s'\n", rootCmdArgs.Namespace, rootCmdArgs.Address)
		fmt.Println("Proceeding with application logic using these arguments...")
		fmt.Printf("  API Address: '%s' (default if not specified)\n", rootCmdArgs.ApiAddress)
		return nil
	},
}

// init is called by Go when the package is initialized.
func init() {
	// Here we define our flags and bind them to the rootCmdArgs struct.
	// Flags are typically defined as persistent if they should be available to subcommands as well.
	// For a single root command, .Flags() is also fine.
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Namespace, "namespace", "n", "", "The client's namespace (required)")
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Address, "addr", "a", "", "The remote address of Tokilake (required)")
	rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.ApiAddress, "api-addr", "l", "localhost:7749", "The tokiame API address to listen on")
	// If you add Password back:
	// rootCmd.PersistentFlags().StringVarP(&rootCmdArgs.Password, "password", "p", "", "The client's password (optional)")

	// Mark flags as required. Cobra will print an error and the usage message if not provided.
	rootCmd.MarkPersistentFlagRequired("namespace")
	rootCmd.MarkPersistentFlagRequired("addr")

	// You can customize the usage template if needed, but Cobra's default is quite good.
	// The original usage message: "This tool requires a namespace and can optionally take a password."
	// is incorporated into the Long description of rootCmd.
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {

		os.Exit(1)
	}
}

func GetArgs() RootCmdArgs {
	return rootCmdArgs
}
