// Copyright Jetstack Ltd. See LICENSE for details.

package noimpersonatedrequest

import (
	"fmt"
	"net/http"

	"github.com/jetstack/kube-oidc-proxy/pkg/proxy/context"
)

func WithPodSA(next http.Handler, saToken func() []byte) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if context.NoImpersonation(req) {
			req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", string(saToken()))}
		}
		next.ServeHTTP(rw, req)
	})
}
