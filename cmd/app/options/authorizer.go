// Copyright Jetstack Ltd. See LICENSE for details.

package options

import (
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

type AuthorizerOptions struct {
	AuthorizerUri string
}

func NewAuthorizerOptions(cfs *cliflag.NamedFlagSets) *AuthorizerOptions {
	ao := AuthorizerOptions{AuthorizerUri: ""}
	ao.AddFlags(cfs.FlagSet("Authorizer options"))
	return &ao
}

func (o *AuthorizerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AuthorizerUri, "authorizer-uri", "", "Authorizer Open policy agent URI")
}
