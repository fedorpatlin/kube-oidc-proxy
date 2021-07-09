// Copyright Jetstack Ltd. See LICENSE for details.
package proxy

import "errors"

// import (
// 	"bytes"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"time"

// 	v1 "k8s.io/api/authorization/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/klog"

// 	k8sJson "k8s.io/apimachinery/pkg/runtime/serializer/json"
// 	authuser "k8s.io/apiserver/pkg/authentication/user"
// 	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
// )

var errAccessDenied = errors.New("access denied")

// type opaResponse struct {
// 	Result v1.SubjectAccessReview
// }

// func (p *Proxy) withAuthz(rw http.ResponseWriter, r *http.Request) bool {
// 	user, ok := genericapirequest.UserFrom(r.Context())
// 	if !ok {
// 		p.handleError(rw, r, errNoImpersonationConfig)
// 		return false
// 	}
// 	// add allAuthenticated role
// 	allAuthFound := false
// 	groups := user.GetGroups()
// 	for _, elem := range groups {
// 		if elem == authuser.AllAuthenticated {
// 			allAuthFound = true
// 			break
// 		}
// 	}
// 	if !allAuthFound {
// 		groups = append(groups, authuser.AllAuthenticated)
// 	}
// 	ri, ok := genericapirequest.RequestInfoFrom(r.Context())
// 	if !ok {
// 		rif := genericapirequest.RequestInfoFactory{
// 			// APIPrefixes:          sets.NewString("api", "apis"),
// 			// GrouplessAPIPrefixes: sets.NewString("api"),
// 		}
// 		var err error
// 		ri, err = rif.NewRequestInfo(r)
// 		if err != nil {
// 			p.handleError(rw, r, fmt.Errorf("cannot get requestInfo from request"))
// 			return false
// 		}
// 	}
// 	sar := v1.SubjectAccessReview{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "SubjectAccessReview",
// 			APIVersion: "authorization.k8s.io/v1",
// 		},
// 		Spec: v1.SubjectAccessReviewSpec{
// 			ResourceAttributes: &v1.ResourceAttributes{
// 				Namespace:   ri.Namespace,
// 				Verb:        ri.Verb,
// 				Group:       ri.APIGroup,
// 				Version:     ri.APIVersion,
// 				Resource:    ri.Resource,
// 				Subresource: ri.Subresource,
// 				Name:        ri.Name},
// 			NonResourceAttributes: &v1.NonResourceAttributes{
// 				Path: ri.Path,
// 				Verb: ri.Verb,
// 			},
// 			User:   user.GetName(),
// 			Groups: groups,
// 			Extra: func(userExtra map[string][]string) map[string]v1.ExtraValue {
// 				extraValue := map[string]v1.ExtraValue{}
// 				for k, v := range userExtra {
// 					extraValue[k] = v
// 				}
// 				return extraValue
// 			}(user.GetExtra()),
// 		},
// 	}
// 	sarSerializer := k8sJson.NewSerializerWithOptions(k8sJson.DefaultMetaFactory, nil, nil, k8sJson.SerializerOptions{
// 		Yaml:   false,
// 		Pretty: false,
// 		Strict: true,
// 	})
// 	var buf []byte
// 	serialisedSAR := bytes.NewBuffer(buf)
// 	sarSerializer.Encode(&sar, serialisedSAR)
// 	opaClient := http.Client{
// 		Timeout: 5 * time.Second,
// 	}
// 	jsonPayload := fmt.Sprintf("{\"input\": %s}", serialisedSAR.Bytes())
// 	serialisedSAR = nil
// 	// request authorizer
// 	// whauthz := webhook.WebhookAuthorizer{}
// 	authzResponse, err := opaClient.Post(p.config.AuthorizerAddress, "application/json", bytes.NewReader([]byte(jsonPayload)))
// 	if err != nil {
// 		klog.Errorf("Authorization server is not responding: %s", err.Error())
// 		p.handleError(rw, r, err)
// 		return false
// 	}
// 	defer authzResponse.Body.Close()
// 	if authzResponse.StatusCode == 200 {
// 		bodyBytes := make([]byte, 2048)
// 		bytesRead, err := authzResponse.Body.Read(bodyBytes)
// 		if err != nil && err != io.EOF {
// 			klog.Errorf("Error reading Authz response body: %s", err.Error())
// 			p.handleError(rw, r, err)
// 			return false
// 		}
// 		bodyBytes = bodyBytes[:bytesRead]
// 		var resp opaResponse
// 		err = json.Unmarshal(bodyBytes, &resp)
// 		if err != nil {
// 			klog.Errorf("Error reading authzResponse: %s", err.Error())
// 			klog.Error(string(bodyBytes))
// 			p.handleError(rw, r, err)
// 			return false
// 		}
// 		if resp.Result.Status.Denied || !resp.Result.Status.Allowed {
// 			klog.Errorf("Access denied %s", resp.Result.Status.Reason)
// 			p.handleError(rw, r, errAccessDenied)
// 			return false
// 		}
// 		return true
// 	}
// 	return false
// }

// func (p *Proxy) withAuthorizeRequest(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
// 		// var ok bool
// 		if len(p.config.AuthorizerAddress) != 0 {
// 			p.withAuthz(rw, r)
// 		}
// 		// if ok {
// 		next.ServeHTTP(rw, r)
// 		// }
// 	})
// }
