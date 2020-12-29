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

package configmanager

import (
	"encoding/json"
	"net"
	"regexp"

	jsoniter "github.com/json-iterator/go"
	"mosn.io/mosn/pkg/config/v2"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/module/zookeeper"
)

const (
	ZookeeperClusterName = "cluster"
	MetadataRegistryName = "metadata_cluster"
)

func ZookeeperConfigLoad(raw string) *v2.MOSNConfig {
	serviceRegistry, metadataRegistry := parseRawZookeeperAddress(raw)
	zookeeper.SetServiceRegistryAddress(serviceRegistry)
	zookeeper.SetMetadataRegistryAddress(metadataRegistry)
	log.StartLogger.Infof("[config] [zookeeper load] Generating config by ZookeeperConfigLoad")
	config := &v2.MOSNConfig{
		Servers: []v2.ServerConfig{
			{
				DefaultLogPath:  "stdout",
				DefaultLogLevel: "DEBUG",
				Listeners: []v2.Listener{
					{
						Addr: &net.TCPAddr{
							IP:   net.ParseIP("0.0.0.0"),
							Port: 2181,
						},
						ListenerConfig: v2.ListenerConfig{
							Name:       "litener",
							AddrConfig: "0.0.0.0:2181",
							BindToPort: true,
							FilterChains: []v2.FilterChain{
								{
									FilterChainConfig: v2.FilterChainConfig{
										Filters: []v2.Filter{
											{
												Type: "zookeeper",
											},
											{
												Type: v2.TCP_PROXY,
												Config: map[string]interface{}{
													"cluster": ZookeeperClusterName,
													"routes": map[string]interface{}{
														"cluster": ZookeeperClusterName,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		ClusterManager: v2.ClusterManagerConfig{
			Clusters: []v2.Cluster{
				{
					Name:                 ZookeeperClusterName,
					ClusterType:          v2.SIMPLE_CLUSTER,
					LbType:               v2.LB_RANDOM,
					MaxRequestPerConn:    1024,
					ConnBufferLimitBytes: 32768,
					Hosts: []v2.Host{
						{
							HostConfig: v2.HostConfig{
								Address: serviceRegistry,
							},
						},
					},
				},
			},
		},
		RawAdmin: json.RawMessage(`{"address": {"socket_address": {"address": "127.0.0.1","port_value": 34901}}}`),
	}
	if serviceRegistry == metadataRegistry {
		return config
	}
	config.Servers[0].Listeners = append(config.Servers[0].Listeners, v2.Listener{
		Addr: &net.TCPAddr{
			IP:   net.ParseIP("0.0.0.0"),
			Port: 2182,
		},
		ListenerConfig: v2.ListenerConfig{
			Name:       "metadata",
			AddrConfig: "0.0.0.0:2182",
			BindToPort: true,
			FilterChains: []v2.FilterChain{
				{
					FilterChainConfig: v2.FilterChainConfig{
						Filters: []v2.Filter{
							{
								Type: "zookeeper",
							},
							{
								Type: v2.TCP_PROXY,
								Config: map[string]interface{}{
									"cluster": MetadataRegistryName,
									"routes": map[string]interface{}{
										"cluster": MetadataRegistryName,
									},
								},
							},
						},
					},
				},
			},
		},
	})
	config.ClusterManager.Clusters = append(config.ClusterManager.Clusters, v2.Cluster{
		Name:                 MetadataRegistryName,
		ClusterType:          v2.SIMPLE_CLUSTER,
		LbType:               v2.LB_RANDOM,
		MaxRequestPerConn:    1024,
		ConnBufferLimitBytes: 32768,
		Hosts: []v2.Host{
			{
				HostConfig: v2.HostConfig{
					Address: metadataRegistry,
				},
			},
		},
	})
	return config
}

var (
	hostPortPattern = regexp.MustCompile("^\\S+:\\d+$")
)

func parseRawZookeeperAddress(raw string) (serviceRegistry, metadataRegistry string) {
	if hostPortPattern.MatchString(raw) {
		// only one zk server specified
		serviceRegistry = raw
		metadataRegistry = raw
		return
	}
	var registry struct {
		ServiceRegistry  string `json:"serviceRegistry"`
		MetadataRegistry string `json:"metadataRegistry"`
	}
	if err := jsoniter.ConfigCompatibleWithStandardLibrary.UnmarshalFromString(raw, &registry); err != nil {
		log.DefaultLogger.Fatalf("parse zookeeper server failed, raw: %s, %s", raw, err)
	}
	serviceRegistry = registry.ServiceRegistry
	metadataRegistry = registry.MetadataRegistry
	return
}
