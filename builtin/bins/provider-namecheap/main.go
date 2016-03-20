package main

import (
	"github.com/hashicorp/terraform/builtin/providers/namecheap"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: namecheap.Provider,
	})
}
