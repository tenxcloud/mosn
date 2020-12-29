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
	jsoniter "github.com/json-iterator/go"
	"mosn.io/mosn/pkg/module/zookeeper"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
)

func init() {
	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/services/<string:application>/<string:host>:<int:port>",
		handleProviderInstanceRegister)

	zookeeper.MustRegister(
		zookeeper.OpGetData,
		"/services/<string:application>/<string:host>:<int:port>",
		handleConsumerInstanceResolve)

	zookeeper.MustRegister(
		zookeeper.OpDelete,
		"/services/<string:application>/<string:host>:<int:port>",
		handleProviderInstanceDeregister)

	zookeeper.MustRegister(
		zookeeper.OpGetChildren2,
		"/dubbo/mapping/<regex:interface:\\S+\\.\\S+>",
		handleConsumerListProviders)

	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/dubbo/metadata/<string:interface>/<string:version>/<string:group>/provider/<string:application>",
		handleProviderCreateMetadata)

	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/dubbo/metadata/<string:interface>/<string:version>/<string:group>/consumer/<string:application>",
		handleConsumerCreateMetadata)

	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/dubbo/metadata/<string:application>/<regex:revision:[a-fA-F0-9]{32}>",
		handleProviderCreateMetadataRevision)

	zookeeper.MustRegister(
		zookeeper.OpGetData,
		"/dubbo/metadata/<string:application>/<regex:revision:[a-fA-F0-9]{32}>",
		handleConsumerGetProviderMetadataRevision)

	zookeeper.MustRegister(
		zookeeper.OpGetChildren2,
		"/services/<string:application>",
		handleConsumerListProviderApplications)

	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/dubbo/<string:interface>/providers/<regex:url:dubbo%3A%2F%2F\\S+>",
		handleProviderV2Create)

	zookeeper.MustRegister(
		zookeeper.OpDelete,
		"/dubbo/<string:interface>/providers/<regex:url:dubbo%3A%2F%2F\\S+>",
		handleProviderV2Delete)

	zookeeper.MustRegister(
		zookeeper.OpCreate,
		"/dubbo/<string:interface>/consumers/<regex:url:consumer%3A%2F%2F\\S+>",
		handleConsumerV2Create)

	zookeeper.MustRegister(
		zookeeper.OpGetChildren2,
		"/dubbo/<regex:interface:\\S+\\.\\S+>/providers",
		handleConsumerV2List)
}
