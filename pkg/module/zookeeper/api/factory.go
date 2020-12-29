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

package api

import (
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/mosn/pkg/module/zookeeper/filters"
)

var (
	DebugFilter    zookeeper.Filter = new(debugFilter)
	HeaderFilter   zookeeper.Filter = filters.NewHeaderFilter()
	PathFilter     zookeeper.Filter = filters.NewPathFilter()
	DataFilter     zookeeper.Filter = filters.NewDataFilter()
	ChildrenFilter zookeeper.Filter = filters.NewChildrenFilter()
	SessionFilter  zookeeper.Filter = zookeeper.NewSessionFilter()
	RouterFilter   zookeeper.Filter = zookeeper.NewRouteFilter()
)

func GetApplicationName() (name string) {
	name = applicationName
	return
}

func SetApplicationName(name string) {
	applicationName = name
}

var (
	applicationName string
	knownFilters    = make(map[string]zookeeper.Filter)
)

type debugFilter struct{}

func (debugFilter) HandleRequest(ctx *zookeeper.Context) {
	invoke(ctx)
}

func (debugFilter) HandleResponse(ctx *zookeeper.Context) {
	invoke(ctx)
}

func invoke(ctx *zookeeper.Context) {
	var method string
	if ctx.Request == nil {
		method = "request"
	} else {
		method = "response"
	}
	if ctx.OpCode != zookeeper.Undefined {
		log.DefaultLogger.Debugf("zookeeper.debugFilter.Invoke, %s, Xid: %d, OpCode: %s, Watch: %t", method, ctx.Xid, ctx.OpCode, ctx.Watch)
	}
	if method == "response" {
		if ctx.Error != nil {
			log.DefaultLogger.Debugf("zookeeper.debugFilter.Invoke, %s, Xid: %d, Zxid: %d, Watch: %t, Error: %s",
				method, ctx.Xid, ctx.Zxid, ctx.Watch, ctx.Error)
		} else {
			log.DefaultLogger.Debugf("zookeeper.debugFilter.Invoke, %s, Xid: %d, Zxid: %d, Watch: %t",
				method, ctx.Xid, ctx.Zxid, ctx.Watch)
		}
	}
	if len(ctx.Path) > 0 {
		log.DefaultLogger.Debugf("zookeeper.debugFilter.Invoke, Path: %s", ctx.Path)
	}
	if ctx.Value != nil {
		log.DefaultLogger.Debugf("zookeeper.debugFilter.Invoke, Value: %v", ctx.Value)
	}
	if log.DefaultLogger.GetLogLevel() >= log.TRACE {
		log.DefaultLogger.Tracef("zookeeper.debugFilter.Invoke: %s", string(ctx.Buffer.Bytes()[4:]))
	}
}
