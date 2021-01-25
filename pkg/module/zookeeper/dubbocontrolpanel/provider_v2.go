/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dubbocontrolpanel

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func handleProviderV2Create(upstream zookeeper.Upstream, request *zookeeper.Context) {
	request.OriginalPath = request.Path
	registerURL, v2Query, ok := analyzeProviderV2Create(request)
	if !ok {
		downstream, response := upstream.DirectForward(request)
		downstream.DirectReply(response)
		return
	}
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.handleProviderV2Create, this is v2 create, uri: %s, v2Query: %+v", registerURL, v2Query)
	// Prepare zk client
	metadataClient, err := zookeeper.MetadataClient()
	if err != nil {
		return
	}
	defer metadataClient.Close()
	serviceClient, err := zookeeper.ServiceClient()
	if err != nil {
		return
	}
	defer serviceClient.Close()
	// Real create metadata by zookeeper client
	metainfo, err := manualPublishMetadata(v2Query, metadataClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Create, manualPublishMetadata failed: %s", err)
		return
	}
	hostNet, err := net.ResolveTCPAddr("tcp", registerURL.Host)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Create, ResolveTCPAddr failed: %s", err)
		return
	}
	// Real create mapping by zookeeper client
	err = manualPublishMapping(v2Query, metadataClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Create, manualPublishMapping failed: %s", err)
		return
	}
	// Just prepare instance data
	serviceInstance, err := manualPublishInstance(v2Query, hostNet, metainfo.Revision, serviceClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Create, manualPublishInstance failed: %s", err)
		return
	}
	// Real create instance by proxy to upstream
	request.Path = fmt.Sprintf("/services/%s/%s", v2Query.Get("application"), hostNet.String())
	request.Value = serviceInstance
	downstream, response := upstream.ModifyAndSend(request, &zookeeper.CreateRequest{
		XidAndOpCode: zookeeper.RequestHeader(request.RawPayload),
		TheRest:      zookeeper.TheRest(request.RawPayload, request.DataEnd),
	})
	// Real reply to downsteam
	response.Path = request.OriginalPath
	downstream.ModifyAndReply(response, &zookeeper.CreateResponse{
		XidZxidAndErrCode: zookeeper.ResponseHeader(response.RawPayload),
	})
}

func analyzeProviderV2Create(request *zookeeper.Context) (registerURL *url.URL, v2Query url.Values, ok bool) {
	ok = false
	var urlEncoded string
	request.MustGetParam("url", &urlEncoded)
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.analyzeProviderV2Create, url: %s", urlEncoded)

	registerURLString, _ := url.QueryUnescape(urlEncoded)
	registerURL, err := url.Parse(registerURLString)
	if err != nil {
		log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.analyzeProviderV2Create, url.Parse failed: %s", err)
		return
	}
	v2Query, err = url.ParseQuery(registerURL.RawQuery)
	if err != nil {
		log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.analyzeProviderV2Create, url.ParseQuery failed: %s", err)
		return
	}
	// Check if request from v3, just skip
	if v2Query.Get("MIGRATION_MULTI_REGSITRY") == "true" {
		log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.analyzeProviderV2Create, skip for v3")
		return
	}
	ok = true
	return
}

func manualPublishMetadata(v2Query url.Values, metadataClient *zookeeper.Client) (metainfo *dubbo.MetadataInfo, err error) {
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualPublishMetadata, v2Query: %+v", v2Query)
	metainfo = &dubbo.MetadataInfo{
		Application: v2Query.Get("application"),
	}
	// Check old revision
	path := fmt.Sprintf("/dubbo/metadata/%s", metainfo.Application)
	revisions, _, err := metadataClient.Conn.Children(path)
	if err != nil {
		// Get old revision
		for _, item := range revisions {
			metainfoOld := &dubbo.MetadataInfo{}
			path = fmt.Sprintf("/dubbo/metadata/%s/%s", metainfo.Application, item)
			data, _, err := metadataClient.Conn.Get(path)
			if err != nil {
				log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualPublishMetadata, Get fail: %s", err)
				continue
			}
			err = json.Unmarshal(data, &metainfoOld)
			if err != nil {
				log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualPublishMetadata, Unmarshal failed: %s", err)
				continue
			}
			if metainfoOld.Application == metainfo.Application {
				for key, value := range metainfoOld.Services {
					metainfo.Services[key] = value
				}
			}
			// TODO delete old revision?
			break
		}
	}
	serviceKey := strings.Builder{}
	if v2Query.Get("group") != "" {
		serviceKey.WriteString(v2Query.Get("group"))
		serviceKey.WriteString("/")
	}
	serviceKey.WriteString(v2Query.Get("interface"))
	if v2Query.Get("version") != "" {
		serviceKey.WriteString(":")
		serviceKey.WriteString(v2Query.Get("version"))
	}
	serviceKey.WriteString(":dubbo")
	serviceNew := dubbo.ServiceInfo{
		Name:     v2Query.Get("interface"),
		Group:    v2Query.Get("group"),
		Version:  v2Query.Get("version"),
		Protocol: "dubbo",
		Path:     v2Query.Get("interface"),
		Params: map[string]string{
			"dubbo":      v2Query.Get("dubbo"),
			"release":    v2Query.Get("release"),
			"timeout":    v2Query.Get("timeout"),
			"deprecated": v2Query.Get("deprecated"),
			"group":      v2Query.Get("group"),
			"version":    v2Query.Get("version"),
		},
	}
	if metainfo.Services == nil {
		metainfo.Services = make(map[string]dubbo.ServiceInfo, 1)
	}
	metainfo.Services[serviceKey.String()] = serviceNew
	// Generate new revision data
	services := make([]dubbo.ServiceInfo, 0, len(metainfo.Services))
	for _, service := range metainfo.Services {
		services = append(services, service)
	}
	metainfo.Revision, err = dubbo.CalculateRevision(metainfo.Application, services)
	if err != nil {
		log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualPublishMetadata, CalculateRevision failed: %s", err)
		metainfo.Revision = "unknown"
	}
	// Create parent
	path = fmt.Sprintf("/dubbo/metadata/%s/%s", metainfo.Application, metainfo.Revision)
	data, err := json.Marshal(metainfo)
	if err != nil {
		return
	}
	_, err = metadataClient.CreateWithRecursive(path, data, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		return
	}
	// Update cached map
	dubbo.UpdateApplicationsByInterface(v2Query.Get("interface"), []string{metainfo.Application})
	dubbo.UpdateClustersByProvider(metainfo.Application, metainfo.Revision, services)
	return
}

func manualPublishMapping(v2Query url.Values, metadataClient *zookeeper.Client) (err error) {
	path := fmt.Sprintf("/dubbo/mapping/%s/%s", v2Query.Get("interface"), v2Query.Get("application"))
	data, err := json.Marshal(map[string]string{
		"timestamp": strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
	})
	_, err = metadataClient.CreateWithRecursive(path, data, 0, zk.WorldACL(zk.PermAll))
	return
}

func manualPublishInstance(v2Query url.Values, hostNet *net.TCPAddr, revision string, serviceClient *zookeeper.Client) (serviceInstance *dubbo.ServiceInstance, err error) {
	hostNet.Port = dubbo.GetDubboExposePort()
	path := fmt.Sprintf("/services/%s/%s", v2Query.Get("application"), hostNet.String())
	// Generate data
	endpoints, err := json.Marshal([]map[string]interface{}{
		{
			"port":     hostNet.Port,
			"protocol": "dubbo",
		},
	})
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.manualPublishInstance, json.Marshal failed: %s", err)
	}
	params, err := json.Marshal(map[string]interface{}{
		"dubbo": map[string]interface{}{
			"version": v2Query.Get("version"),
			"dubbo":   v2Query.Get("dubbo"),
			"release": v2Query.Get("release"),
			"port":    20881,
		},
	})
	serviceInstance = &dubbo.ServiceInstance{
		Name:    v2Query.Get("application"),
		ID:      hostNet.String(),
		Address: hostNet.IP.String(),
		Port:    hostNet.Port,
		SslPort: nil,
		Payload: dubbo.ZookeeperInstancePayload{
			Class: "org.apache.dubbo.registry.zookeeper.ZookeeperInstance",
			ZookeeperInstance: dubbo.ZookeeperInstance{
				ID:   "",
				Name: v2Query.Get("application"),
				Metadata: map[string]string{
					"dubbo.endpoints":                   string(endpoints),
					"dubbo.metadata-service.url-params": string(params),
					"dubbo.metadata.revision":           revision,
					"dubbo.metadata.storage-type":       "remote",
				},
			},
		},
		RegistrationTimeUTC: time.Now().UnixNano() / int64(time.Millisecond),
		ServiceType:         "DYNAMIC",
		URISpec:             nil,
		Enabled:             nil,
	}
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualPublishInstance, path: %+v", path)
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualPublishInstance, serviceInstance: %+v", serviceInstance)

	serviceClient.CreateParant(path, 0, zk.WorldACL(zk.PermAll))
	// Update cached map
	dubbo.UpdateEndpointsByApplication(v2Query.Get("application"), []string{fmt.Sprintf("127.0.0.1:%d", dubbo.GetDubboOriginalPort())})
	return
}

func handleProviderV2Delete(upstream zookeeper.Upstream, request *zookeeper.Context) {
	registerURL, v2Query, ok := analyzeProviderV2Create(request)
	if !ok {
		downstream, response := upstream.DirectForward(request)
		downstream.DirectReply(response)
		return
	}
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.handleProviderV2Delete, this is v2 delete, uri: %s, v2Query: %+v", registerURL, v2Query)

	hostNet, err := net.ResolveTCPAddr("tcp", registerURL.Host)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Delete, ResolveTCPAddr failed: %s", err)
		return
	}
	// Modify and send to upstream, not with zookeeper client
	err = manualDeleteInstance(v2Query, hostNet, upstream, request)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleProviderV2Delete, manualPublishInstance failed: %s", err)
		return
	}
}

func manualDeleteInstance(v2Query url.Values, hostNet *net.TCPAddr, upstream zookeeper.Upstream, request *zookeeper.Context) (err error) {
	hostNet.Port = dubbo.GetDubboExposePort()
	path := fmt.Sprintf("/services/%s/%s", v2Query.Get("application"), hostNet.String())
	// Real send to upstream
	request.Path = path
	downstream, response := upstream.ModifyAndSend(request, &zookeeper.DeleteRequest{
		XidAndOpCode: zookeeper.RequestHeader(request.RawPayload),
		TheRest:      zookeeper.TheRest(request.RawPayload, request.DataEnd),
	})

	// Response to downsteam
	response.Path = request.OriginalPath
	downstream.DirectReply(response)

	// Update cached map
	// TODO

	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualDeleteMetadata, deleted, path: %s", path)
	return
}
