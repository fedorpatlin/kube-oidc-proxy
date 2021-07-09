// Copyright Jetstack Ltd. See LICENSE for details.
package audit

import (
	"fmt"
	"net/http"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/klog"

	"github.com/jetstack/kube-oidc-proxy/cmd/app/options"
	"github.com/jetstack/kube-oidc-proxy/pkg/authorizer"
	"github.com/jetstack/kube-oidc-proxy/pkg/noimpersonatedrequest"
)

type Audit struct {
	opts         *options.AuditOptions
	serverConfig *server.CompletedConfig
	authzOpts    *options.AuthorizerOptions
}

// New creates a new Audit struct to handle auditing for proxy requests. This
// is mostly a wrapper for the apiserver auditing handlers to combine them with
// the proxy.
func New(opts *options.AuditOptions, authzOpts *options.AuthorizerOptions, externalAddress string, secureServingInfo *server.SecureServingInfo) (*Audit, error) {
	serverConfig := &server.Config{
		ExternalAddress: externalAddress,
		SecureServing:   secureServingInfo,

		// Default to treating watch as a long-running operation.
		// Generic API servers have no inherent long-running subresources.
		// This is so watch requests are handled correctly in the audit log.
		LongRunningFunc: genericfilters.BasicLongRunningRequestCheck(
			sets.NewString("watch"), sets.NewString()),
	}

	// We do not support dynamic auditing, so leave nil
	if err := opts.ApplyTo(serverConfig, nil, nil, nil, nil); err != nil {
		return nil, err
	}

	completed := serverConfig.Complete(nil)

	return &Audit{
		opts:         opts,
		authzOpts:    authzOpts,
		serverConfig: &completed,
	}, nil
}

// Run will run the audit backend if configured.
func (a *Audit) Run(stopCh <-chan struct{}) error {
	if a.serverConfig.AuditBackend != nil {
		if err := a.serverConfig.AuditBackend.Run(stopCh); err != nil {
			return fmt.Errorf("failed to run the audit backend: %s", err)
		}
	}

	return nil
}

// Shutdown will shutdown the audit backend if configured.
func (a *Audit) Shutdown() error {
	if a.serverConfig.AuditBackend != nil {
		a.serverConfig.AuditBackend.Shutdown()
	}

	return nil
}

// WithRequest will wrap the given handler to inject the request information
// into the context which is then used by the wrapped audit handler.
func (a *Audit) WithRequest(handler http.Handler) http.Handler {
	scheme := runtime.NewScheme()
	handler = genericapifilters.WithAudit(handler, a.serverConfig.AuditBackend, a.serverConfig.AuditPolicyChecker, a.serverConfig.LongRunningFunc)
	if len(a.authzOpts.AuthorizerUri) > 0 {
		handler = noimpersonatedrequest.WithPodSA(handler, func() []byte {
			saToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
			if err != nil {
				klog.Errorf("error reading serviceaccount token %s", err.Error())
				return []byte("")
			}
			return saToken
		})
		handler = genericapifilters.WithAuthorization(handler, authorizer.NewOPAAuthorizer(a.authzOpts), serializer.NewCodecFactory(scheme).WithoutConversion())
	}
	rif := request.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
	return genericapifilters.WithRequestInfo(handler, &rif)
}

// WithUnauthorized will wrap the given handler to inject the request
// information into the context which is then used by the wrapped audit
// handler.
func (a *Audit) WithUnauthorized(handler http.Handler) http.Handler {
	handler = genericapifilters.WithFailedAuthenticationAudit(handler, a.serverConfig.AuditBackend, a.serverConfig.AuditPolicyChecker)
	return genericapifilters.WithRequestInfo(handler, a.serverConfig.RequestInfoResolver)
}
