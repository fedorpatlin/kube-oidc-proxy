// Copyright Jetstack Ltd. See LICENSE for details.

package options

import (
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

type AuthorizerOptions struct {
	AuthorizerUri          string
	ExtrasPath             string
	ExtrasAnnotationPrefix string
}

func NewAuthorizerOptions(cfs *cliflag.NamedFlagSets) *AuthorizerOptions {
	ao := AuthorizerOptions{
		AuthorizerUri:          "",
		ExtrasPath:             "",
		ExtrasAnnotationPrefix: "",
	}
	ao.AddFlags(cfs.FlagSet("Authorizer options"))
	return &ao
}

func (o *AuthorizerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.AuthorizerUri, "authorizer-url", "", "Authorizer Open policy agent URI")
	fs.StringVar(&o.ExtrasPath, "extras-url", "", "extra-data added to user.extras")
	fs.StringVar(&o.ExtrasAnnotationPrefix, "extras-prefix", "authorization.example.com/", "extra-data annotation prefix")
}
