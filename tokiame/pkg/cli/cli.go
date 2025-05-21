package cli

import (
	"flag"
	"fmt"
	"os"
)

type Arg struct {
	Namespace string
	// Password  string
	Address string
}

func Parse() Arg {
	var arg Arg
	flag.StringVar(&arg.Namespace, "namespace", "", "The client's namespace (required)")
	flag.StringVar(&arg.Address, "addr", "", "The remote address of Tokilake (required)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  This tool requires a namespace and can optionally take a password.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if arg.Namespace == "" || arg.Address == "" {
		fmt.Fprintln(os.Stderr, "Error: -namespace and -address flags are both required.")
		flag.Usage()
		os.Exit(1)
	}
	return arg
}
