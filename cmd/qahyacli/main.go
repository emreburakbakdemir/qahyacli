// qahyacli is one binary with two roles:
//
//	qahyacli run <job.yaml>  host: owns the job env, serves the job API, spawns steps
//	qahyacli env ...         client: used inside steps, talks to the host
package main

import (
	"fmt"
	"os"
)

const usage = `usage:
  qahyacli run <job.yaml>                          run a job

  qahyacli env list                                show the job env (secret values hidden)
  qahyacli env get <NAME>                          print one value (secrets resolved)
  qahyacli env set <NAME=VALUE>...                 set values for later steps
  qahyacli env unset <NAME>...                     remove values for later steps
  qahyacli env load <file> [--profile p] [--export]
                                                   load a config for later steps;
                                                   --export also prints shell code:
                                                   eval "$(qahyacli env load ... --export)"
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "env":
		err = cmdEnv(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		err = fmt.Errorf("unknown command %q (try qahyacli help)", os.Args[1])
	}

	// non-zero exit on any error, so `set -e` in a step fails the step right there
	if err != nil {
		fmt.Fprintln(os.Stderr, "qahyacli:", err)
		os.Exit(1)
	}
}
