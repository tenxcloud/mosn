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

package dubbo

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"mosn.io/mosn/pkg/log"
)

type GlobalStateSnapshotEntry struct {
	Interface string
	Group     string
	Version   string
	Endpoints []*GlobalStateEndpoint
}

type GlobalStateEndpoint struct {
	IP   string
	Port int
}

func GetGlobalState() (snapshot map[string]*GlobalStateSnapshotEntry) {
	snapshot = make(map[string]*GlobalStateSnapshotEntry)
	clusterIndex.Range(func(key, value interface{}) bool {
		clusterKey := key.(string)
		cluster := value.(*ServiceInfo)
		entry := &GlobalStateSnapshotEntry{
			Interface: cluster.Name,
			Group:     cluster.Group,
			Version:   cluster.Version,
		}
		snapshot[clusterKey] = entry
		rawProviders, exist := clusterToProviders.Load(clusterKey)
		providers := make([]*Provider, 0)
		if exist {
			providerSet := rawProviders.(*sync.Map)
			providerSet.Range(func(psk, psv interface{}) bool {
				provider := psv.(*Provider)
				providers = append(providers, provider)
				return true
			})
		}
		endpoints := make([]*GlobalStateEndpoint, 0)
		for _, provider := range providers {
			rawEndpoints, exist := applicationToEndpoints.Load(provider.Application)
			if !exist {
				continue
			}
			endpointSet := rawEndpoints.(*sync.Map)
			endpointSet.Range(func(_, ev interface{}) bool {
				endpoint := ev.(*ProviderEndpoint)
				if endpoint.Revision == provider.Revision {
					endpoints = append(endpoints, &GlobalStateEndpoint{IP: endpoint.IP, Port: endpoint.Port})
				} else if endpoint.IP == "127.0.0.1" && endpoint.Port == GetDubboOriginalPort() {
					endpoints = append(endpoints, &GlobalStateEndpoint{IP: endpoint.IP, Port: endpoint.Port})
				}
				return true
			})
		}
		entry.Endpoints = endpoints
		return true
	})
	return
}

var (
	interfaceToApplications = new(sync.Map)
	applicationToEndpoints  = new(sync.Map)
	clusterToProviders      = new(sync.Map)
	clusterIndex            = new(sync.Map)
)

type ProviderEndpoint struct {
	IP       string
	Port     int
	Revision string
}

type Provider struct {
	Application string
	Revision    string
}

func endpointKey(pe *ProviderEndpoint) string {
	return fmt.Sprintf("%s:%d", pe.IP, pe.Port)
}

func clusterKey(interfaceName, group, version string) (key string) {
	key = fmt.Sprintf("i:%s;g:%s;v:%s", interfaceName, group, version)
	return
}

func providerKey(application, revision string) string {
	return fmt.Sprintf("a:%s;r:%s", application, revision)
}

func GetEndpointByApplication(application, ip string, port int) (endpoint *ProviderEndpoint) {
	rawEPs, exist := applicationToEndpoints.Load(application)
	if !exist {
		return
	}
	eps, ok := rawEPs.(*sync.Map)
	if !ok {
		return
	}
	key := endpointKey(&ProviderEndpoint{IP: ip, Port: port})
	rawEP, ok := eps.Load(key)
	if !ok {
		return
	}
	endpoint = rawEP.(*ProviderEndpoint)
	return
}

func UpdateEndpointsByApplication(application string, rawEndpoints []string) {
	endpoints := new(sync.Map)
	for _, re := range rawEndpoints {
		if parts := strings.Split(re, ":"); len(parts) != 2 {
			log.DefaultLogger.Errorf("dubbo.globalState.UpdateEndpointsByApplication, parse endpoint host port failed, endpoint: %s", re)
			continue
		} else if port, err := strconv.Atoi(parts[1]); err != nil {
			log.DefaultLogger.Errorf("dubbo.globalState.UpdateEndpointsByApplication, parse endpoint host port failed, endpoint: %s, port: %s", re, parts[1])
			continue
		} else {
			endpoint := &ProviderEndpoint{
				IP:   parts[0],
				Port: port,
			}
			key := endpointKey(endpoint)
			endpoints.Store(key, endpoint)
		}
	}
	oldEPs, exist := applicationToEndpoints.Load(application)
	if exist {
		oldEndpoints := oldEPs.(*sync.Map)
		endpoints.Range(func(k, v interface{}) bool {
			key := k.(string)
			value := v.(*ProviderEndpoint)
			if oldEP, exist := oldEndpoints.Load(key); exist {
				oldEndpoint := oldEP.(*ProviderEndpoint)
				value.Revision = oldEndpoint.Revision
			}
			return true
		})
	}
	applicationToEndpoints.Store(application, endpoints)
	go func() {
		if err := UpdateAllConfig(); err != nil {
			log.DefaultLogger.Errorf("dubbo.globalState.UpdateEndpointsByApplication, update xds failed, %s", err)
		}
	}()
}

func UpdateClustersByProvider(application, revision string, serviceInfo []ServiceInfo) {
	needUpdateXds := false
	updateInterfacesByApplication(application, serviceInfo)
	for i, si := range serviceInfo {
		providers := new(sync.Map)
		pk := providerKey(application, revision)
		provider := &Provider{Application: application, Revision: revision}
		providers.Store(pk, provider)
		key := clusterKey(si.Name, si.Group, si.Version)
		old, exist := clusterToProviders.LoadOrStore(key, providers)
		if exist {
			ops := old.(*sync.Map)
			ops.Store(pk, provider)
		}
		clusterIndex.Store(key, &(serviceInfo[i]))
		needUpdateXds = true
	}
	if needUpdateXds {
		go func() {
			if err := UpdateAllConfig(); err != nil {
				log.DefaultLogger.Errorf("dubbo.globalState.UpdateClustersByProvider, update xds failed, %s", err)
			}
		}()
	}
}

func updateInterfacesByApplication(application string, serviceInfo []ServiceInfo) {
	interfaces := make([]string, 0, len(serviceInfo))
	for _, i := range serviceInfo {
		interfaces = append(interfaces, i.Name)
	}
	eps := new(sync.Map)
	if old, exist := applicationToEndpoints.LoadOrStore(application, eps); exist {
		eps = old.(*sync.Map)
	}
	for _, i := range interfaces {
		apps := new(sync.Map)
		apps.Store(application, eps)
		if old, exist := interfaceToApplications.LoadOrStore(i, apps); exist {
			apps = old.(*sync.Map)
		}
		apps.Store(application, eps)
	}
}

func UpdateApplicationsByInterface(interfaceName string, applications []string) {
	apps := new(sync.Map)
	for _, application := range applications {
		eps := new(sync.Map)
		old, exist := applicationToEndpoints.LoadOrStore(application, eps)
		if exist {
			eps = old.(*sync.Map)
		}
		apps.Store(application, eps)
	}
	interfaceToApplications.Store(interfaceName, apps)
}
