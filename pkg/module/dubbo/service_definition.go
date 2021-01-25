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

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/definition/model/TypeDefinition.java
type TypeDefinition struct {
	ID              string                    `json:"id"`
	Type            string                    `json:"type"`
	Items           []TypeDefinition          `json:"items"`
	Enums           []string                  `json:"enum"`
	Reference       string                    `json:"$ref"`
	Properties      map[string]TypeDefinition `json:"properties"`
	TypeBuilderName string                    `json:"typeBuilderName"`
}

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/definition/model/MethodDefinition.java
type MethodDefinition struct {
	Name           string           `json:"name"`
	ParameterTypes []string         `json:"parameterTypes"`
	ReturnType     string           `json:"returnType"`
	Parameters     []TypeDefinition `json:"parameters"`
}

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/definition/model/ServiceDefinition.java
type ServiceDefinition struct {
	CanonicalName string             `json:"canonicalName"`
	CodeSource    string             `json:"codeSource"`
	Methods       []MethodDefinition `json:"methods"`
	Types         []TypeDefinition   `json:"types"`
}

// dubbo-metadata/dubbo-metadata-api/src/main/java/org/apache/dubbo/metadata/definition/model/FullServiceDefinition.java
type FullServiceDefinition struct {
	// Parameters struct{
	// 	MappingType string `json:"mapping-type"`
	// 	Side string `json:"side"`
	// 	Release string `json:"release"`
	// 	Methods string `json:"methods"`
	// 	Deprecated string `json:"deprecated"`
	// 	QosPort string `json:"qos.port"`
	// 	Dubbo string `json:"dubbo"`
	// 	Interface string `json:"interface"`
	// 	Version string `json:"version"`
	// 	Generic string `json:"generic"`
	// 	Revision string `json:"revision"`
	// 	MappingType2 string `json:"mapping.type"`
	// 	MetadataType string `json:"metadata-type"`
	// 	Application string `json:"application"`
	// 	Dynamic string `json:"dynamic"`
	// 	Group string `json:"group"`
	// 	AnyHost string `json:"anyhost"`
	// } `json:"parameters"`
	ServiceDefinition `json:",inline"`
	Parameters        map[string]string `json:"parameters"`
}
