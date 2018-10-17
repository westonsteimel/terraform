package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform/states"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/cli"
)

// StateListCommand is a Command implementation that lists the resources
// within a state file.
type StateListCommand struct {
	Meta
	StateMeta
}

func (c *StateListCommand) Run(args []string) int {
	args, err := c.Meta.process(args, true)
	if err != nil {
		return 1
	}

	cmdFlags := c.Meta.flagSet("state list")
	cmdFlags.StringVar(&c.Meta.statePath, "state", DefaultStateFilename, "path")
	lookupId := cmdFlags.String("id", "", "Restrict output to paths with a resource having the specified ID.")
	if err := cmdFlags.Parse(args); err != nil {
		return cli.RunResultHelp
	}
	args = cmdFlags.Args()

	// Load the backend
	b, backendDiags := c.Backend(nil)
	if backendDiags.HasErrors() {
		c.showDiagnostics(backendDiags)
		return 1
	}

	env := c.Workspace()
	// Get the state
	state, err := b.StateMgr(env)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to load state: %s", err))
		return 1
	}

	if err := state.RefreshState(); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to load state: %s", err))
		return 1
	}

	stateReal := state.State()
	if stateReal == nil {
		c.Ui.Error(fmt.Sprintf(errStateNotFound))
		return 1
	}

	filter := &terraform.StateFilter{State: stateReal}
	results, err := filter.Filter(args...)
	if err != nil {
		c.Ui.Error(fmt.Sprintf(errStateFilter, err))
		return cli.RunResultHelp
	}

	for _, result := range results {
		if is, ok := result.Value.(*states.ResourceInstance); ok {
			// If we search a specific ID, we need to unmarshal the
			// attributes and match the ID from the resource.
			if *lookupId != "" {
				id := ""
				if is.HasCurrent() {
					attrs := make(map[string]interface{})
					err = json.Unmarshal(is.Current.AttrsJSON, &attrs)
					if err != nil {
						c.Ui.Error(fmt.Sprintf("Failed to load attribute: %s", err))
						return 1
					}
					id, _ = attrs["id"].(string)
				}

				// Continue if the ID's don't match.
				if *lookupId != id {
					continue
				}
			}

			c.Ui.Output(result.Address)
		}
	}

	return 0
}

func (c *StateListCommand) Help() string {
	helpText := `
Usage: terraform state list [options] [pattern...]

  List resources in the Terraform state.

  This command lists resources in the Terraform state. The pattern argument
  can be used to filter the resources by resource or module. If no pattern
  is given, all resources are listed.

  The pattern argument is meant to provide very simple filtering. For
  advanced filtering, please use tools such as "grep". The output of this
  command is designed to be friendly for this usage.

  The pattern argument accepts any resource targeting syntax. Please
  refer to the documentation on resource targeting syntax for more
  information.

Options:

  -state=statefile    Path to a Terraform state file to use to look
                      up Terraform-managed resources. By default it will
                      use the state "terraform.tfstate" if it exists.

  -id=ID              Restricts the output to objects whose id is ID.

`
	return strings.TrimSpace(helpText)
}

func (c *StateListCommand) Synopsis() string {
	return "List resources in the state"
}

const errStateFilter = `Error filtering state: %[1]s

Please ensure that all your addresses are formatted properly.`

const errStateLoadingState = `Error loading the state: %[1]s

Please ensure that your Terraform state exists and that you've
configured it properly. You can use the "-state" flag to point
Terraform at another state file.`

const errStateNotFound = `No state file was found!

State management commands require a state file. Run this command
in a directory where Terraform has been run or use the -state flag
to point the command to a specific state location.`
