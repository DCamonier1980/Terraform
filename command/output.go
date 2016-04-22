package command

import (
	"encoding/json"
	"flag"
	"fmt"
	"sort"
	"strings"
)

// OutputCommand is a Command implementation that reads an output
// from a Terraform state and prints it.
type OutputCommand struct {
	Meta
}

func (c *OutputCommand) Run(args []string) int {
	args = c.Meta.process(args, false)

	var module string
	var format string
	cmdFlags := flag.NewFlagSet("output", flag.ContinueOnError)
	cmdFlags.StringVar(&c.Meta.statePath, "state", DefaultStateFilename, "path")
	cmdFlags.StringVar(&module, "module", "", "module")
	cmdFlags.StringVar(&format, "format", "display", "format")
	cmdFlags.Usage = func() { c.Ui.Error(c.Help()) }

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	args = cmdFlags.Args()
	if len(args) > 1 {
		c.Ui.Error(
			"The output command expects exactly one argument with the name\n" +
				"of an output variable or no arguments to show all outputs.\n")
		cmdFlags.Usage()
		return 1
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	stateStore, err := c.Meta.State()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading state: %s", err))
		return 1
	}

	if module == "" {
		module = "root"
	} else {
		module = "root." + module
	}

	// Get the proper module we want to get outputs for
	modPath := strings.Split(module, ".")

	state := stateStore.State()
	mod := state.ModuleByPath(modPath)

	if mod == nil {
		c.Ui.Error(fmt.Sprintf(
			"The module %s could not be found. There is nothing to output.",
			module))
		return 1
	}

	if state.Empty() || len(mod.Outputs) == 0 {
		c.Ui.Error(fmt.Sprintf(
			"The state file has no outputs defined. Define an output\n" +
				"in your configuration with the `output` directive and re-run\n" +
				"`terraform apply` for it to become available."))
		return 1
	}

	if name == "" {
		ks := make([]string, 0, len(mod.Outputs))
		for k, _ := range mod.Outputs {
			ks = append(ks, k)
		}
		sort.Strings(ks)

		switch format {
		case "json":
			j := make(map[string]interface{})

			for _, k := range ks {
				v := mod.Outputs[k]
				j[k] = v
			}

			out, err := json.Marshal(j)

			if err != nil {
				c.Ui.Error(fmt.Sprintf("Error exporting outputs to json: %s", err))
				return 1
			}

			c.Ui.Output(string(out))
		case "display":
			for _, k := range ks {
				v := mod.Outputs[k]
				c.Ui.Output(fmt.Sprintf("%s = %s", k, v))
			}
		default:
			c.Ui.Error(fmt.Sprintf("Unknown output format: %s", format))
			return 1
		}

		return 0
	}

	v, ok := mod.Outputs[name]
	if !ok {
		c.Ui.Error(fmt.Sprintf(
			"The output variable requested could not be found in the state\n" +
				"file. If you recently added this to your configuration, be\n" +
				"sure to run `terraform apply`, since the state won't be updated\n" +
				"with new output variables until that command is run."))
		return 1
	}

	c.Ui.Output(v)
	return 0
}

func (c *OutputCommand) Help() string {
	helpText := `
Usage: terraform output [options] [NAME]

  Reads an output variable from a Terraform state file and prints
  the value.  If NAME is not specified, all outputs are printed.

Options:

  -state=path      Path to the state file to read. Defaults to
                   "terraform.tfstate".

  -no-color        If specified, output won't contain any color.

  -module=name     If specified, returns the outputs for a
                   specific module

	-format=name     If specified, returns the outputs in the format
									 specified. Only valid when all outputs are
									 rendered. Possible options [display, json].
									 Default "display".

`
	return strings.TrimSpace(helpText)
}

func (c *OutputCommand) Synopsis() string {
	return "Read an output from a state file"
}
