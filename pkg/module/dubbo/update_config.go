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
	"net"

	configv2 "mosn.io/mosn/pkg/config/v2"
	dubboprotocol "mosn.io/mosn/pkg/protocol/xprotocol/dubbo"
	"mosn.io/mosn/pkg/router"
	"mosn.io/mosn/pkg/server"
	"mosn.io/mosn/pkg/upstream/cluster"
	"mosn.io/pkg/log"
)

const (
	// Need such listener name, for decoding external headers
	listenerName      = dubboprotocol.IngressDubbo
	routerName        = "dubbo"
	metadataInterface = "org.apache.dubbo.metadata.MetadataService"
)

func UpdateAllConfig() (err error) {
	err = updateListener()
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.UpdateAllConfig, updateListener failed: %s", err)
		return
	}
	log.DefaultLogger.Infof("dubbo.UpdateAllConfig, updateListener success")
	globalState := GetGlobalState()
	err = updateRouters(globalState)
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.UpdateAllConfig, updateRouters failed: %s", err)
		return
	}
	log.DefaultLogger.Infof("dubbo.UpdateAllConfig, updateRouters success")
	err = updateClusters(globalState)
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.UpdateAllConfig, updateClusters failed: %s", err)
		return
	}
	log.DefaultLogger.Infof("dubbo.UpdateAllConfig, updateClusters success")
	return
}

func updateListener() (err error) {
	listenerAddr := &net.TCPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: GetDubboExposePort(),
	}
	l := &configv2.Listener{
		Addr: listenerAddr,
		ListenerConfig: configv2.ListenerConfig{
			Name:       listenerName,
			AddrConfig: listenerAddr.String(),
			BindToPort: true,
			FilterChains: []configv2.FilterChain{
				{
					FilterChainConfig: configv2.FilterChainConfig{
						Filters: []configv2.Filter{
							{
								Type: configv2.DEFAULT_NETWORK_FILTER,
								Config: map[string]interface{}{
									"downstream_protocol": "X",
									"upstream_protocol":   "X",
									"router_config_name":  routerName,
									"extend_config": map[string]interface{}{
										"sub_protocol": "dubbo",
									},
								},
							},
						},
					},
				},
			},
			StreamFilters: []configv2.Filter{
				{
					Type: configv2.DubboStream,
				},
			},
		},
	}
	err = server.GetListenerAdapterInstance().AddOrUpdateListener("", l)
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.updateListener, TriggerClusterAndHostsAddOrUpdate failed: %s", err)
		return
	}
	log.DefaultLogger.Infof("dubbo.updateListener, AddOrUpdateListener success")
	return
}

func updateRouters(snapshot map[string]*GlobalStateSnapshotEntry) (err error) {
	// Temporary map for collecting virtualhosts by domain
	vhmap := make(map[string]*configv2.VirtualHost)
	for clusterKey, entry := range snapshot {
		if entry.Interface == metadataInterface {
			log.DefaultLogger.Debugf("dubbo.updateListener, ignore %s", metadataInterface)
			continue
		}
		r := configv2.Router{
			RouterConfig: configv2.RouterConfig{
				Match: configv2.RouterMatch{
					Path:    "/",
					Headers: []configv2.HeaderMatcher{},
				},
				Route: configv2.RouteAction{
					RouterActionConfig: configv2.RouterActionConfig{
						ClusterName: clusterKey,
					},
				},
			},
		}
		if entry.Version != "" {
			r.RouterConfig.Match.Headers = append(r.RouterConfig.Match.Headers, configv2.HeaderMatcher{
				Name:  dubboprotocol.VersionNameHeader,
				Value: entry.Version,
			})
		}
		if entry.Group != "" {
			r.RouterConfig.Match.Headers = append(r.RouterConfig.Match.Headers, configv2.HeaderMatcher{
				Name:  dubboprotocol.GroupNameHeader,
				Value: entry.Group,
			})
		}
		vh, ok := vhmap[entry.Interface]
		if !ok {
			vh = &configv2.VirtualHost{
				Name:    clusterKey,
				Domains: []string{entry.Interface},
				Routers: []configv2.Router{r},
			}
			vhmap[entry.Interface] = vh
		} else {
			vh.Routers = append(vh.Routers, r)
		}
	}
	// Generate full router config
	vhs := make([]*configv2.VirtualHost, 0, len(vhmap))
	for _, value := range vhmap {
		vhs = append(vhs, value)
	}
	routerConfiguration := &configv2.RouterConfiguration{
		RouterConfigurationConfig: configv2.RouterConfigurationConfig{
			RouterConfigName: routerName,
		},
		VirtualHosts: vhs,
	}
	// Real update
	err = router.GetRoutersMangerInstance().AddOrUpdateRouters(routerConfiguration)
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.updateRouters, AddOrUpdateRouters failed: %s", err)
		return
	}
	log.DefaultLogger.Infof("dubbo.updateRouters, AddOrUpdateRouters success")
	return
}

func updateClusters(snapshot map[string]*GlobalStateSnapshotEntry) (err error) {
	for clusterKey, entry := range snapshot {
		if entry.Interface == metadataInterface {
			log.DefaultLogger.Debugf("dubbo.updateListener, ignore %s", metadataInterface)
			continue
		}
		hosts := make([]configv2.Host, 0, len(entry.Endpoints))
		for _, ep := range entry.Endpoints {
			h := configv2.Host{
				HostConfig: configv2.HostConfig{
					Address: fmt.Sprintf("%s:%d", ep.IP, ep.Port),
				},
			}
			hosts = append(hosts, h)
		}
		clusterConfiguration := configv2.Cluster{
			Name:        clusterKey,
			ClusterType: configv2.SIMPLE_CLUSTER,
			LbType:      configv2.LB_RANDOM,
			Hosts:       hosts,
		}
		err = cluster.GetClusterMngAdapterInstance().TriggerClusterAndHostsAddOrUpdate(clusterConfiguration, clusterConfiguration.Hosts)
		if err != nil {
			log.DefaultLogger.Errorf("dubbo.updateCluster, TriggerClusterAndHostsAddOrUpdate failed: %s", err)
			return
		}
		log.DefaultLogger.Infof("dubbo.updateCluster, TriggerClusterAndHostsAddOrUpdate success")
	}
	return
}
