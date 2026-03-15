package main

import (
	"fmt"
	"os"
)

var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: encrypten <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: clean, diff, export-key, init, keygen, lock, smudge, status, unlock, version")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "clean":
		if err := cmdClean(); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten clean: %v\n", err)
			os.Exit(1)
		}
	case "diff":
		if err := cmdDiff(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten diff: %v\n", err)
			os.Exit(1)
		}
	case "export-key":
		if err := cmdExportKey(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten export-key: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if err := cmdInit(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten init: %v\n", err)
			os.Exit(1)
		}
	case "lock":
		if err := cmdLock(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten lock: %v\n", err)
			os.Exit(1)
		}
	case "keygen":
		if err := runKeygen(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten keygen: %v\n", err)
			os.Exit(1)
		}
	case "smudge":
		if err := cmdSmudge(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten smudge: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := cmdStatus(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten status: %v\n", err)
			os.Exit(1)
		}
	case "unlock":
		if err := cmdUnlock(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "encrypten unlock: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("encrypten %s\n", Version)
	default:
		fmt.Fprintf(os.Stderr, "encrypten: unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: encrypten <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: clean, diff, export-key, init, keygen, lock, smudge, status, unlock, version")
		os.Exit(1)
	}
}
