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
	"regexp"

	jsoniter "github.com/json-iterator/go"
	"mosn.io/pkg/log"
)

var (
	dubboExposePort   = 20888
	dubboOriginalPort = 20880

	MetadataEndpointKey = "dubbo.endpoints"
	MetadataRevisionKey = "dubbo.metadata.revision"

	ParameterApplicationKey = "application"

	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

func SetDubboExposePort(port int) {
	dubboExposePort = port
}

func GetDubboExposePort() int {
	return dubboExposePort
}

func SetDubboOriginalPort(port int) {
	dubboOriginalPort = port
}

func GetDubboOriginalPort() int {
	return dubboOriginalPort
}

const (
	portPathPattern    = `^(\S+):%d$`
	portReplacePattern = "$1:%d"
)

func RestoreInstancePathPort(path string) string {
	portPattern := fmt.Sprintf(portReplacePattern, dubboExposePort)
	pattern := regexp.MustCompile(fmt.Sprintf(portPathPattern, dubboOriginalPort))
	return pattern.ReplaceAllString(path, portPattern)
}

func ModifyInstanceInfo(path string, serviceInstance *ServiceInstance) (
	modifiedPath string, modifiedServiceInstance *ServiceInstance) {

	portPattern := fmt.Sprintf(portReplacePattern, dubboExposePort)
	pattern, err := regexp.Compile(fmt.Sprintf(portPathPattern, serviceInstance.Port))
	if err != nil {
		log.DefaultLogger.Errorf("dubbo.instance.ModifyInstanceInfo, compile path port replace pattern failed, %s", err)
	} else {
		modifiedPath = pattern.ReplaceAllString(path, portPattern)
	}

	modifiedServiceInstance = serviceInstance
	if pattern != nil {
		modifiedServiceInstance.ID = pattern.ReplaceAllString(modifiedServiceInstance.ID, portPattern)
	}
	modifiedServiceInstance.Port = dubboExposePort
	if metadata := modifiedServiceInstance.Payload.Metadata; metadata != nil {
		if rawEndpoints, exist := metadata[MetadataEndpointKey]; exist {
			var endpoints []*Endpoint
			if err := json.UnmarshalFromString(rawEndpoints, &endpoints); err != nil {
				log.DefaultLogger.Errorf("dubbo.instance.ModifyInstanceInfo, unmarshal endpoints failed, endpoints: %s, %s", rawEndpoints, err)
				return
			}
			for _, endpoint := range endpoints {
				if endpoint.Protocol == "dubbo" {
					endpoint.Port = dubboExposePort
				}
			}
			if modified, err := json.MarshalToString(endpoints); err != nil {
				log.DefaultLogger.Errorf("dubbo.instance.ModifyInstanceInfo, marshal modified endpoints failed, %s", err)
				return
			} else {
				metadata[MetadataEndpointKey] = modified
			}
		}
	}
	return
}
