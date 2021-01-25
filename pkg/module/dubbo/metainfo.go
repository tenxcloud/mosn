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
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"sort"

	"mosn.io/mosn/pkg/log"
)

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/MetadataInfo.java
type MetadataInfo struct {
	Application string                 `json:"app"`
	Revision    string                 `json:"revision"`
	Services    map[string]ServiceInfo `json:"services"`
	// ExtendParams map[string]string `json:""`
	// and others fields
}

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/MetadataInfo.java
type ServiceInfo struct {
	Name     string            `json:"name"`
	Group    string            `json:"group"`
	Version  string            `json:"version"`
	Protocol string            `json:"protocol"`
	Path     string            `json:"path"`
	Params   map[string]string `json:"params"`
	Methods  []string          `json:"-"`
	// and other fields
}

func CalculateRevision(application string, serviceInfo []ServiceInfo) (revision string, err error) {
	buffer := new(bytes.Buffer)
	if _, err = buffer.WriteString(application); err != nil {
		return
	}
	for _, si := range serviceInfo {
		if err = si.toDescString(buffer); err != nil {
			return
		}
	}
	hasher := md5.New()
	log.DefaultLogger.Tracef("dubbo.metainfo.CalculateRevision, buffer: %s", buffer.String())
	if _, err = io.Copy(hasher, buffer); err != nil {
		return
	}
	hash := hasher.Sum(nil)
	revision = fmt.Sprintf("%X", hash)
	log.DefaultLogger.Tracef("dubbo.metainfo.CalculateRevision, revision: %s", revision)
	return
}

func (si ServiceInfo) toDescString(buffer io.StringWriter) (err error) {
	if len(si.Group) > 0 {
		if _, err = buffer.WriteString(si.Group); err != nil {
			return
		}
		if _, err = buffer.WriteString("/"); err != nil {
			return
		}
	}
	if _, err = buffer.WriteString(si.Path); err != nil {
		return
	}
	if len(si.Version) > 0 {
		if _, err = buffer.WriteString(":"); err != nil {
			return
		}
		if _, err = buffer.WriteString(si.Version); err != nil {
			return
		}
	}
	if len(si.Protocol) > 0 {
		if _, err = buffer.WriteString(":"); err != nil {
			return
		}
		if _, err = buffer.WriteString(si.Protocol); err != nil {
			return
		}
	}
	if si.Methods != nil {
		if err = getMethodSignaturesString(si.Methods, buffer); err != nil {
			return
		}
	}
	if si.Params != nil {
		if err = mapToString(si.Params, buffer); err != nil {
			return
		}
	}
	return
}

func methodToString(method string) string {
	// TODO: get params and return value
	// something like this public abstract java.lang.String org.apache.dubbo.metadata.MetadataService.getServiceDefinition(java.lang.String),
	return method
}

func getMethodSignaturesString(methods []string, writer io.StringWriter) (err error) {
	methodStrings := make([]string, len(methods))
	for i, method := range methods {
		methodStrings[i] = methodToString(method)
	}
	sort.Strings(methodStrings)
	if _, err = writer.WriteString("["); err != nil {
		return
	}
	first := true
	for _, method := range methodStrings {
		if !first {
			if _, err = writer.WriteString(", "); err != nil {
				return
			}
		}
		if _, err = writer.WriteString(method); err != nil {
			return
		}
		first = false
	}
	_, err = writer.WriteString("]")
	return
}

func mapToString(ms map[string]string, writer io.StringWriter) (err error) {
	sortedkeys := make([]string, 0, len(ms))
	for k := range ms {
		sortedkeys = append(sortedkeys, k)
	}
	sort.Strings(sortedkeys)
	if _, err = writer.WriteString("{"); err != nil {
		return
	}
	first := true
	for _, key := range sortedkeys {
		value := ms[key]
		if !first {
			if _, err = writer.WriteString(", "); err != nil {
				return
			}
		}
		if _, err = writer.WriteString(key); err != nil {
			return
		}
		if _, err = writer.WriteString("="); err != nil {
			return
		}
		if _, err = writer.WriteString(value); err != nil {
			return
		}
		first = false
	}
	_, err = writer.WriteString("}")
	return
}
