// Copyright Jetstack Ltd. See LICENSE for details.

package noimpersonatedrequest

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
	"k8s.io/klog"
)

var defaultInClusterSALocation = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func WithPodSA(next http.Handler, saToken func() []byte) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if context.NoImpersonation(req) {
			req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", string(saToken()))}
		}
		next.ServeHTTP(rw, req)
	})
}

func ReadInClusterToken() []byte {
	saToken, err := os.ReadFile(defaultInClusterSALocation)
	if err != nil {
		klog.Errorf("error reading serviceaccount token %s", err.Error())
		return []byte("")
	}
	return saToken
}
