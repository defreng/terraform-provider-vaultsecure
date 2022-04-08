package vaultsecure

import (
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"vaultsecure": func() (tfprotov6.ProviderServer, error) {
		return tfsdk.NewProtocol6Server(New()), nil
	},
}
