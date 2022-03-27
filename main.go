package main

import (
	"context"
	"github.com/defreng/terraform-provider-vaultsecure/vaultsecure"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func main() {
	err := tfsdk.Serve(context.Background(), vaultsecure.New, tfsdk.ServeOpts{Name: "vaultsecure"})
	if err != nil {
		return
	}
}
