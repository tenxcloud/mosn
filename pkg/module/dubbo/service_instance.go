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

// C:\Users\lizhen\.m2\repository\org\apache\curator\curator-x-discovery\4.0.1\curator-x-discovery-4.0.1.jar!\org\apache\curator\x\discovery\ServiceInstance.class
type ServiceInstance struct {
	Name                string                   `json:"name"`
	ID                  string                   `json:"id"`
	Address             string                   `json:"address"`
	Port                int                      `json:"port"`
	SslPort             *int                     `json:"sslPort"`
	Payload             ZookeeperInstancePayload `json:"payload"`
	RegistrationTimeUTC int64                    `json:"registrationTimeUTC"`
	ServiceType         string                   `json:"serviceType"`
	URISpec             *URISpec                 `json:"uriSpec"`
	Enabled             *bool                    `json:"enabled"`
}

type ZookeeperInstancePayload struct {
	Class             string `json:"@class"`
	ZookeeperInstance `json:",inline"`
}

// dubbo-registry/dubbo-registry-zookeeper/src/main/java/org/apache/dubbo/registry/zookeeper/ZookeeperInstance.java
type ZookeeperInstance struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
}

// C:\Users\lizhen\.m2\repository\org\apache\curator\curator-x-discovery\4.0.1\curator-x-discovery-4.0.1.jar!\org\apache\curator\x\discovery\UriSpec.class
type URISpecPart struct {
	Value    string `json:"value"`
	Variable bool   `json:"variable"`
}

type URISpec struct {
	Parts []URISpecPart `json:"parts"`
}

type URLParams struct {
	Dubbo struct {
		Version string `json:"version"`
		Dubbo   string `json:"dubbo"`
		Port    string `json:"port"`
	} `json:"dubbo"`
}

type Endpoint struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}
