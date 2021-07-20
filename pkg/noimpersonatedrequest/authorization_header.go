// Copyright Jetstack Ltd. See LICENSE for details.

package noimpersonatedrequest

import (
	"fmt"
	"net/http"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

func WithPodSA(next http.Handler, saToken func() []byte) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if context.NoImpersonation(req) {
			delete(req.Header, "Authorization")
			req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", string(saToken()))}
		}
		next.ServeHTTP(rw, req)
	})
}

func RestConfigToken(restConfig *rest.Config) func() []byte {
	return func() []byte {
		return []byte(restConfig.BearerToken)
	}
}

func ReadInClusterToken() []byte {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error reading serviceaccount token %s", err.Error())
		return []byte("")
	}
	return []byte(restConfig.BearerToken)
}
