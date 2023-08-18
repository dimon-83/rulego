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

package mqtt

import (
	"context"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/mqtt"
	"github.com/rulego/rulego/endpoint"
	"log"
	"net/textproto"
	"strconv"
)

//RequestMessage http请求消息
type RequestMessage struct {
	request paho.Message
	msg     *types.RuleMsg
}

func (r *RequestMessage) Body() []byte {
	return r.request.Payload()
}
func (r *RequestMessage) Headers() textproto.MIMEHeader {
	header := make(map[string][]string)
	header["topic"] = []string{r.request.Topic()}
	return header
}

func (r *RequestMessage) From() string {
	return r.request.Topic()
}

func (r *RequestMessage) GetParam(key string) string {
	return ""
}

func (r *RequestMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}

func (r *RequestMessage) GetMsg() *types.RuleMsg {
	if r.msg == nil {
		ruleMsg := types.NewMsg(0, r.From(), types.JSON, types.NewMetadata(), string(r.Body()))

		ruleMsg.Metadata.PutValue("topic", r.From())

		r.msg = &ruleMsg
	}
	return r.msg
}

func (r *RequestMessage) SetStatusCode(statusCode int) {
}

func (r *RequestMessage) SetBody(body []byte) {
}

func (r *RequestMessage) Request() paho.Message {
	return r.request
}

//ResponseMessage http响应消息
type ResponseMessage struct {
	request  paho.Message
	response paho.Client
	body     []byte
	msg      *types.RuleMsg
	headers  textproto.MIMEHeader
}

func (r *ResponseMessage) Body() []byte {
	return r.body
}

func (r *ResponseMessage) Headers() textproto.MIMEHeader {
	if r.headers == nil {
		r.headers = make(map[string][]string)
	}
	return r.headers
}

func (r *ResponseMessage) From() string {
	return r.request.Topic()
}

func (r *ResponseMessage) GetParam(key string) string {
	return ""
}

func (r *ResponseMessage) SetMsg(msg *types.RuleMsg) {
	r.msg = msg
}
func (r *ResponseMessage) GetMsg() *types.RuleMsg {
	return r.msg
}

func (r *ResponseMessage) SetStatusCode(statusCode int) {
}

func (r *ResponseMessage) SetBody(body []byte) {
	r.body = body

	topic := r.Headers().Get("topic")
	if topic != "" {
		qosStr := r.Headers().Get("qos")
		qos := byte(0)
		if qosStr != "" {
			qosInt, _ := strconv.Atoi(qosStr)
			qos = byte(qosInt)
		}
		r.response.Publish(topic, qos, false, r.body)
	}
}

func (r *ResponseMessage) Response() paho.Client {
	return r.response
}

//Mqtt MQTT 接收端端点
type Mqtt struct {
	Config mqtt.Config
	client *mqtt.Client
}

func (m *Mqtt) Start() error {
	if m.client == nil {
		if client, err := mqtt.NewClient(m.Config); err != nil {
			return err
		} else {
			m.client = client
			return nil
		}
	}
	return nil
}

func (m *Mqtt) Stop() {
	_ = m.Close()
}

func (m *Mqtt) Close() error {
	if nil != m.client {
		return m.client.Close()
	}
	return nil
}

//AddRouter 添加路由
func (m *Mqtt) AddRouter(routers ...*endpoint.Router) *Mqtt {
	if m.client == nil {
		if client, err := mqtt.NewClient(m.Config); err != nil {
			panic(err)
		} else {
			m.client = client
		}
	}

	for _, router := range routers {
		form := router.GetFrom()
		if form != nil {
			m.client.RegisterHandler(mqtt.Handler{
				Topic:  form.ToString(),
				Qos:    m.Config.QOS,
				Handle: m.handler(router),
			})
		}
	}
	return m
}

func (m *Mqtt) handler(router *endpoint.Router) func(c paho.Client, data paho.Message) {
	return func(c paho.Client, data paho.Message) {
		defer func() {
			//捕捉异常
			if e := recover(); e != nil {
				log.Printf("rest handler err :%v", e)
			}
		}()
		exchange := &endpoint.Exchange{
			In: &RequestMessage{
				request: data,
			},
			Out: &ResponseMessage{
				request:  data,
				response: c,
			}}

		processResult := true
		if fromFlow := router.GetFrom(); fromFlow != nil {
			processResult = fromFlow.ExecuteProcess(exchange)
		}
		//执行to端逻辑
		if router.GetFrom() != nil && router.GetFrom().GetTo() != nil && processResult {
			router.GetFrom().GetTo().Execute(context.TODO(), exchange)
		}
	}
}
