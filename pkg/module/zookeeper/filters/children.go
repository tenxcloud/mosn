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

package filters

import (
	"encoding/binary"

	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func NewChildrenFilter() zookeeper.Filter {
	return new(children)
}

type children struct{}

func (children) HandleRequest(ctx *zookeeper.Context) {
}

func (children) HandleResponse(ctx *zookeeper.Context) {
	if ctx.Error != nil {
		return
	}
	var request *zookeeper.Context
	if request = ctx.Request; request == nil {
		return
	}
	if request.OpCode == zookeeper.Undefined || len(request.Path) <= 0 {
		return
	}
	switch request.OpCode {
	case zookeeper.OpGetChildren2:
		handleResponseChildren(ctx)
	default:
	}
}

func handleResponseChildren(ctx *zookeeper.Context) {
	handleChildrenInternal(ctx, 5*zookeeper.Uint32Size)
}

func handleChildrenInternal(ctx *zookeeper.Context, begin int) {
	buffer := ctx.RawPayload
	length := len(buffer)
	if length < begin {
		log.DefaultLogger.Errorf("zookeeper.filters.children.handleChildrenInternal, buffer too short")
		return
	}
	childrenCount := int(binary.BigEndian.Uint32(buffer[5*zookeeper.Uint32Size : 6*zookeeper.Uint32Size]))
	ctx.Children = make([]string, childrenCount)
	ctx.ChildrenBegin = begin + zookeeper.Uint32Size
	ctx.ChildrenEnd = begin + zookeeper.Uint32Size
	for i, nextBegin := 0, 6*zookeeper.Uint32Size; i < childrenCount; i++ {
		childLength := int(binary.BigEndian.Uint32(buffer[nextBegin:nextBegin + zookeeper.Uint32Size]))
		ctx.ChildrenEnd = nextBegin + zookeeper.Uint32Size + childLength
		path := string(buffer[nextBegin + zookeeper.Uint32Size:ctx.ChildrenEnd])
		ctx.Children[i] = path
		nextBegin = ctx.ChildrenEnd
	}
}
