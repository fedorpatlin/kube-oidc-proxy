// Copyright Jetstack Ltd. See LICENSE for details.

package authorizer

import (
	"net/http"

	"github.com/jetstack/kube-oidc-proxy/pkg/noimpersonatedrequest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func (a *OPAAuthorizer) WithRequest(handler http.Handler) http.Handler {
	scheme := runtime.NewScheme()
	// Если авторизатор включен, то встраиваем его в обработку запроса
	// Запрос на API-сервер пойдет от имени SA пода, действующего с правами админа
	handler = noimpersonatedrequest.WithPodSA(handler, noimpersonatedrequest.ReadInClusterToken)
	handler = genericapifilters.WithAuthorization(handler, a, serializer.NewCodecFactory(scheme).WithoutConversion())

	// Без проинициализированной фабрики на авторизацию не приходят resourceAttributes, только nonResourceAttributes
	rif := request.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
	return genericapifilters.WithRequestInfo(handler, &rif)
}
