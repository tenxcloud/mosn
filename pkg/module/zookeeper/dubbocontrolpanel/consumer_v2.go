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
	"net/url"
	"strconv"

	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func handleConsumerV2Create(upstream zookeeper.Upstream, request *zookeeper.Context) {
	var interfaceName string
	request.MustGetParam("interface", &interfaceName)
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.handleConsumerV2Create, interface: %s", interfaceName)

	// TODO
	downstream, response := upstream.DirectForward(request)
	downstream.DirectReply(response)
}

func handleConsumerV2List(upstream zookeeper.Upstream, request *zookeeper.Context) {
	var interfaceName string
	request.MustGetParam("interface", &interfaceName)
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.handleConsumerV2List, interface: %s", interfaceName)
	log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.handleConsumerV2List")

	// Real send to upstream, useless data just for continuous xid
	request.Path = "/services"
	downstream, response := upstream.ModifyAndSend(request, &zookeeper.GetChildren2Request{
		XidAndOpCode: zookeeper.RequestHeader(request.RawPayload),
		TheRest:      zookeeper.TheRest(request.RawPayload, request.PathEnd),
	})

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

	applications, err := manualSubscribeMapping(interfaceName, metadataClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleConsumerV2List, manualSubscribeMapping fail: %s", err)
		return
	}

	// Get by new connection with zookeeper client
	serviceInstances, err := manualSubscribeInstance(applications, serviceClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleConsumerV2List, manualSubscribeInstance fail: %s", err)
		return
	}

	// Get by new connection with zookeeper client
	metadataInfoMap, err := manualSubscribeMetadata(applications, metadataClient)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.handleConsumerV2List, manualSubscribeRevision fail: %s", err)
		return
	}

	results := make([]string, 0, len(serviceInstances))
	for _, serviceInstance := range serviceInstances {
		revision := serviceInstance.Payload.Metadata["dubbo.metadata.revision"]
		metadataInfo, ok := metadataInfoMap[revision]
		if !ok {
			log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.handleConsumerV2List, revision not found")
			continue
		}
		var serviceInfo *dubbo.ServiceInfo
		for _, v := range metadataInfo.Services {
			if v.Name == interfaceName {
				serviceInfo = &v
				break
			}
		}
		if serviceInfo == nil {
			log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.handleConsumerV2List, serviceInfo not found, skip")
			continue
		}
		query := &url.Values{}
		query.Set("application", metadataInfo.Application)
		query.Set("deprecated", serviceInfo.Params["deprecated"])
		query.Set("dubbo", serviceInfo.Params["dubbo"])
		query.Set("interface", interfaceName)
		query.Set("release", serviceInfo.Params["release"])
		query.Set("side", "provider")
		query.Set("group", serviceInfo.Group)
		query.Set("version", serviceInfo.Version)
		query.Set("timestamp", strconv.FormatInt(serviceInstance.RegistrationTimeUTC, 10))
		// TODO
		// query.Set("anyhost", "true")
		// query.Set("default", "true")
		// query.Set("dynamic", "true")
		// query.Set("generic", "false")
		// query.Set("pid", "159845")
		// query.Set("revision", "2.7.9-SNAPSHOT")
		// query.Set("methods", "hello")

		addr := &url.URL{
			Scheme:   "dubbo",
			Host:     fmt.Sprintf("%s:%d", serviceInstance.Address, serviceInstance.Port),
			Path:     fmt.Sprintf("/%s", interfaceName),
			RawQuery: query.Encode(),
		}
		results = append(results, url.QueryEscape(addr.String()))
	}
	log.DefaultLogger.Debugf("zookeeper.dubbocontrolpanel.handleConsumerV2List, results: %+v", results)

	response.Children = results
	downstream.ModifyAndReply(response, &zookeeper.GetChildren2Response{
		XidZxidAndErrCode: zookeeper.ResponseHeader(response.RawPayload),
		Stat:              zookeeper.TheRest(response.RawPayload, response.ChildrenEnd),
	})
}

func manualSubscribeMapping(interfaceName string, metadataClient *zookeeper.Client) (applications []string, err error) {
	path := fmt.Sprintf("/dubbo/mapping/%s", interfaceName)
	applications, _, err = metadataClient.Conn.Children(path)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.manualSubscribeMapping, Children fail: %s", err)
		return
	}
	dubbo.UpdateApplicationsByInterface(interfaceName, applications)
	return
}

func manualSubscribeMetadata(applications []string, metadataClient *zookeeper.Client) (MetadataInfoMap map[string]*dubbo.MetadataInfo, err error) {
	MetadataInfoMap = make(map[string]*dubbo.MetadataInfo, len(applications))
	for _, application := range applications {
		path := fmt.Sprintf("/dubbo/metadata/%s", application)
		revisions, _, err := metadataClient.Conn.Children(path)
		if err != nil {
			log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualSubscribeMetadata, Children fail: %s", err)
			continue
		}
		for _, revision := range revisions {
			path := fmt.Sprintf("/dubbo/metadata/%s/%s", application, revision)
			data, _, err := metadataClient.Conn.Get(path)
			if err != nil {
				log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualSubscribeMetadata, Get fail: %s", err)
				continue
			}
			metainfo := &dubbo.MetadataInfo{}
			err = json.Unmarshal(data, &metainfo)
			if err != nil {
				log.DefaultLogger.Warnf("zookeeper.dubbocontrolpanel.manualSubscribeMetadata, Unmarshal failed: %s", err)
				continue
			}
			MetadataInfoMap[metainfo.Revision] = metainfo
			services := make([]dubbo.ServiceInfo, 0, len(metainfo.Services))
			for _, service := range metainfo.Services {
				services = append(services, service)
			}
			dubbo.UpdateClustersByProvider(application, revision, services)
		}
	}
	return
}

func manualSubscribeInstance(applications []string, serviceClient *zookeeper.Client) (serviceInstances []*dubbo.ServiceInstance, err error) {
	// Check exist with zookeeper client
	serviceInstances = make([]*dubbo.ServiceInstance, 0, len(applications))
	for _, application := range applications {
		path := fmt.Sprintf("/services/%s", application)
		instances, _, err := serviceClient.Conn.Children(path)
		if err != nil {
			log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualSubscribeInstance, Children fail: %s", err)
			continue
		}
		dubbo.UpdateEndpointsByApplication(application, instances)
		for _, instance := range instances {
			log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualSubscribeInstance, instance: %s", instance)
			path := fmt.Sprintf("/services/%s/%s", application, instance)
			data, _, err := serviceClient.Conn.Get(path)
			if err != nil {
				log.DefaultLogger.Tracef("zookeeper.dubbocontrolpanel.manualSubscribeInstance, Get fail: %s", err)
				continue
			}
			serviceInstance := &dubbo.ServiceInstance{}
			err = json.Unmarshal(data, serviceInstance)
			if err != nil {
				log.DefaultLogger.Errorf("zookeeper.dubbocontrolpanel.manualSubscribeInstance, unmarshal data failed, %s", err)
				continue
			}
			serviceInstances = append(serviceInstances, serviceInstance)
			// cache
			revision := serviceInstance.Payload.Metadata[dubbo.MetadataRevisionKey]
			endpoint := dubbo.GetEndpointByApplication(application, serviceInstance.Address, serviceInstance.Port)
			if endpoint != nil {
				endpoint.Revision = revision
			}
		}
	}
	return
}

type v2instance struct {
	addr  *url.URL
	query *url.Values
}

func (i *v2instance) String() string {
	i.addr.RawQuery = i.query.Encode()
	return url.QueryEscape(i.addr.String())
}
