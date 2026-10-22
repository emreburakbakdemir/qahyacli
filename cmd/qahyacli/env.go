/*
	the client role, qahyacli env <subcommand>, used inside steps. it creates the API client and impelemnts
	five subcommands as thin wrappers: parse the arguments, call one client method, print the result
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"qahyacli/internal/jobapi"
	"qahyacli/internal/shell"
)

func cmdEnv(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: qahyacli env <list|get|set|unset|load> ...")
	}
	c, err := jobapi.NewClientFromEnv()
	if err != nil {
		return err
	}

	sub, args := args[0], args[1:]
	switch sub {
	case "list":
		return envList(c, args)
	case "get":
		return envGet(c, args)
	case "set":
		return envSet(c, args)
	case "unset":
		return envUnset(c, args)
	case "load":
		return envLoad(c, args)
	default:
		return fmt.Errorf("unknown env command %q", sub)
	}
}

func envList(c *jobapi.Client, args []string) error {
	if len(args) != 0 {
		return errors.New("usage: qahyacli env list")
	}
	entries, err := c.List()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVALUE\tSOURCE")
	for _, e := range entries {
		v := e.Value
		if e.Secret {
			v = "<secret " + e.Ref + ">"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Name, v, e.Source)
	}
	return tw.Flush()
}

// get prints just the value, for $(qahyacli env get NAME). It is the way to read
// a value set earlier in the *same* step, and to fetch one secret on demand.
func envGet(c *jobapi.Client, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: qahyacli env get <NAME>")
	}
	e, err := c.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Println(e.Value)
	return nil
}

func envSet(c *jobapi.Client, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: qahyacli env set <NAME=VALUE>...")
	}
	// one request per pair: if the second is rejected, the first stays set.
	// fine for now; a batch endpoint would make it all-or-nothing
	for _, a := range args {
		name, value, ok := strings.Cut(a, "=")
		if !ok {
			return fmt.Errorf("%q: expected NAME=VALUE", a)
		}
		if err := c.Set(name, value); err != nil {
			return err
		}
	}
	return nil
}

func envUnset(c *jobapi.Client, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: qahyacli env unset <NAME>...")
	}
	for _, name := range args {
		if err := c.Unset(name); err != nil {
			return err
		}
	}
	return nil
}

func envLoad(c *jobapi.Client, args []string) error {
	fs := flag.NewFlagSet("qahyacli env load", flag.ContinueOnError)
	profile := fs.String("profile", "", "profile to load on top of defaults")
	export := fs.Bool("export", false, "also print export lines for eval in the current shell")

	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return errors.New("usage: qahyacli env load <file> [--profile p] [--export]")
	}

	// the runner's working dir is not this step's, so a relative path would be
	// resolved against the wrong directory on the other side
	path, err := filepath.Abs(pos[0])
	if err != nil {
		return err
	}

	entries, err := c.Load(path, *profile, *export)
	if err != nil {
		return err
	}

	if *export {
		for _, e := range entries {
			fmt.Printf("export %s=%s\n", e.Name, shell.Quote(e.Value))
		}
	}
	// stderr, so it never ends up inside eval "$(...)"
	fmt.Fprintf(os.Stderr, "qahyacli: loaded %d variables from %s\n", len(entries), filepath.Base(path))
	return nil
}

// parseInterspersed lets flags come after positional args. The flag package
// stops at the first non-flag, so `load abc.config --profile x` would otherwise
// treat --profile as a second positional argument.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}
