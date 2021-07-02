// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/request"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func (p *Proxy) withAuthz(rw http.ResponseWriter, r *http.Request) {
	rif := request.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
	ri, err := rif.NewRequestInfo(r)
	if err != nil {
		fmt.Printf("error: %s", err.Error())
	}
	user, ok := genericapirequest.UserFrom(r.Context())
	if !ok {
		p.handleError(rw, r, errNoName)
		return
	}

	sar := v1.SubjectAccessReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SubjectAccessReview",
			APIVersion: "authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "impersonationaccessreview",
		},
		Spec: v1.SubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace:   ri.Namespace,
				Verb:        ri.Verb,
				Group:       ri.APIGroup,
				Version:     ri.APIVersion,
				Resource:    ri.Resource,
				Subresource: ri.Subresource,
				Name:        ri.Name},
			NonResourceAttributes: &v1.NonResourceAttributes{
				Path: ri.Path,
				Verb: ri.Verb,
			},
			User:   user.GetName(),
			Groups: user.GetGroups(),
			Extra: func(userExtra map[string][]string) map[string]v1.ExtraValue {
				extraValue := map[string]v1.ExtraValue{}
				for k, v := range userExtra {
					extraValue[k] = v
				}
				return extraValue
			}(user.GetExtra()),
		},
	}
	sarSerializer := k8sJson.NewSerializerWithOptions(k8sJson.DefaultMetaFactory, nil, nil, k8sJson.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: true,
	})
	var buf []byte
	jsonPayload := bytes.NewBuffer(buf)
	sarSerializer.Encode(&sar, jsonPayload)
	opaClient := http.Client{
		Timeout: 5 * time.Second,
	}
	authzResponse, err := opaClient.Post(p.config.AuthorizerAddress, "application/json", jsonPayload)
	if err != nil {
		klog.Errorf("Authorization server is not responding: %s", err.Error())
		p.handleError(rw, r, errUnauthorized)
		return
	}
	defer authzResponse.Body.Close()
	if authzResponse.StatusCode == 200 {
		bodyBytes := make([]byte, 2048)
		_, err := io.ReadFull(authzResponse.Body, bodyBytes)
		if err != nil {
			klog.Errorf("Error reading Authz response body: %s", err.Error())
			p.handleError(rw, r, errUnauthorized)
			return
		}
		sarSerializer.Decode(bodyBytes, &schema.GroupVersionKind{Kind: "SubjectAccessReview"}, &sar)
		if sar.Status.Denied || !sar.Status.Allowed {
			if len(sar.Status.Reason) > 0 {
				klog.Errorf("Access denied: %s", sar.Status.Reason)
			} else {
				klog.Error("Access denied")
			}
			p.handleError(rw, r, errUnauthorized)
			return
		}
	}
}

func (p *Proxy) withAuthorizeRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if len(p.config.AuthorizerAddress) != 0 {
			p.withAuthz(rw, r)
		}
		next.ServeHTTP(rw, r)
	})
}
