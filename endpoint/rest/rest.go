/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rest

import (
	"github.com/julienschmidt/httprouter"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/endpoint"
	"io/ioutil"
	"log"
	"net/http"
	"net/textproto"
)

const (
	ContentTypeKey  = "Content-Type"
	JsonContextType = "application/json"
)

//RequestMessage http请求消息
type RequestMessage struct {
	request *http.Request
	body    []byte
	//路径参数
	Params httprouter.Params
	msg    *types.RuleMsg
}

func (r *RequestMessage) Body() []byte {
	if r.body == nil {
		defer func() {
			if r.request.Body != nil {
				_ = r.request.Body.Close()
			}
		}()
		entry, _ := ioutil.ReadAll(r.request.Body)
		r.body = entry
	}
	return r.body
}
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	return textproto.MIMEHeader(r.request.Header)
}

func (r RequestMessage) From() string {
	return r.request.URL.String()
}

func (r *RequestMessage) GetParam(key string) string {
	return r.request.FormValue(key)
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), "")
		if contentType := r.Headers().Get(ContentTypeKey); contentType == JsonContextType {
			//如果Content-Type是application/json 类型，把body复制到msg.Data
			ruleMsg.Data = string(r.Body())
		}

		r.msg = &ruleMsg
	}
	return r.msg
}
func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
	r.body = body
}

func (r *RequestMessage) Request() *http.Request {
	return r.request
}

//ResponseMessage http响应消息
type ResponseMessage struct {
	request  *http.Request
	response http.ResponseWriter
	body     []byte
	to       string
	msg      *types.RuleMsg
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	return textproto.MIMEHeader(r.response.Header())
}

func (r *ResponseMessage) From() string {
	return r.request.URL.String()
}

func (r *ResponseMessage) GetParam(key string) string {
	return r.request.FormValue(key)
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
	r.response.WriteHeader(statusCode)
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body
	_, _ = r.response.Write(body)
}

func (r *ResponseMessage) Response() http.ResponseWriter {
	return r.response
}

//Config Rest 服务配置
type Config struct {
	Addr        string
	CertFile    string
	CertKeyFile string
}

//Rest 接收端端点
type Rest struct {
	endpoint.BaseEndpoint
	//配置
	Config Config
	//路由器
	router *httprouter.Router
}

func (rest *Rest) Start() error {
	var err error
	if rest.Config.CertKeyFile != "" && rest.Config.CertFile != "" {
		log.Printf("starting server with TLS on :%s", rest.Config.Addr)
		err = http.ListenAndServeTLS(rest.Config.Addr, rest.Config.CertFile, rest.Config.CertKeyFile, rest.router)
	} else {
		log.Printf("starting server on :%s", rest.Config.Addr)
		err = http.ListenAndServe(rest.Config.Addr, rest.router)
	}
	return err

}

// AddRouter 注册1个或者多个路由
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
func (rest *Rest) AddRouter(method string, routers ...*endpoint.Router) *Rest {
	if rest.router == nil {
		rest.router = httprouter.New()
	}

	for _, rt := range routers {
		rest.router.Handle(method, rt.FromToString(), rest.handler(rt))
	}
	return rest
}

func (rest *Rest) GET(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodGet, routers...)
	return rest
}

func (rest *Rest) HEAD(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodHead, routers...)
	return rest
}

func (rest *Rest) OPTIONS(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodOptions, routers...)
	return rest
}

func (rest *Rest) POST(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPost, routers...)
	return rest
}

func (rest *Rest) PUT(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPut, routers...)
	return rest
}

func (rest *Rest) PATCH(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodPatch, routers...)
	return rest
}

func (rest *Rest) DELETE(routers ...*endpoint.Router) *Rest {
	rest.AddRouter(http.MethodDelete, routers...)
	return rest
}

func (rest *Rest) Router() *httprouter.Router {
	return rest.router
}

func (rest *Rest) Stop() {

}

func (rest *Rest) handler(router *endpoint.Router) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				log.Printf("rest handler err :%v", e)
			}
		}()

		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: r,
				Params:  params,
			},
			Out: &ResponseMessage{
				request:  r,
				response: w,
			}}

		msg := exchange.In.GetMsg()
		//把路径参数放到msg元数据中
		for _, param := range params {
			msg.Metadata.PutValue(param.Key, param.Value)
		}
		rest.DoProcess(router, exchange)
	}
}
