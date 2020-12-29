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

	"github.com/go-zookeeper/zk"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func NewPathFilter() zookeeper.Filter {
	return new(path)
}

type path struct {
}

func (p path) HandleRequest(ctx *zookeeper.Context) {
	if ctx.OpCode == zookeeper.Undefined {
		return
	}
	switch ctx.OpCode {
	case zookeeper.OpGetChildren, zookeeper.OpGetAcl, zookeeper.OpSync:
	case zookeeper.OpCreate, zookeeper.OpCreateTTL, zookeeper.OpDelete, zookeeper.OpSetData, zookeeper.OpSetAcl:
		handlePath(ctx)
	case zookeeper.OpGetChildren2, zookeeper.OpExists, zookeeper.OpGetData:
		handlePathAndWatch(ctx)
	default:
	}
}

const (
	pathEndIndex = 4 * zookeeper.Uint32Size
)

func handlePath(ctx *zookeeper.Context) {
	handlePathInternal(ctx)
}

func handlePathAndWatch(ctx *zookeeper.Context) {
	failed, index, buffer := handlePathInternal(ctx)
	if failed {
		return
	}
	if len(buffer) < index {
		log.DefaultLogger.Errorf("zookeeper.filters.path.handlePathAndWatch, buffer too short")
		return
	}
	ctx.Watch = buffer[index] != 0
}

func handlePathInternal(ctx *zookeeper.Context) (failed bool, pathEnd int, buffer []byte) {
	buffer = ctx.RawPayload
	length := len(buffer)
	if length < pathEndIndex {
		failed = true
		log.DefaultLogger.Errorf("zookeeper.filters.path.handlePath, buffer too short")
		return
	}
	pathLenght := int(binary.BigEndian.Uint32(buffer[3*zookeeper.Uint32Size : pathEndIndex]))
	if pathEndIndex+pathLenght > length {
		failed = true
		log.DefaultLogger.Errorf("zookeeper.filters.path.handlePath, buffer too short for path length")
		return
	}
	ctx.PathBegin = pathEndIndex
	pathEnd = pathEndIndex + pathLenght
	ctx.PathEnd = pathEnd
	ctx.Path = string(buffer[ctx.PathBegin:ctx.PathEnd])
	ctx.OriginalPath = ctx.Path
	return
}

func (path) HandleResponse(ctx *zookeeper.Context) {
	if ctx.Xid != -1 {
		return
	}
	// so it's watch event
	content := ctx.RawPayload
	eventType := zk.EventType(binary.BigEndian.Uint32(content[5*zookeeper.Uint32Size : 6*zookeeper.Uint32Size]))
	state := zk.State(binary.BigEndian.Uint32(content[6*zookeeper.Uint32Size : 7*zookeeper.Uint32Size]))
	pathBegin := 8 * zookeeper.Uint32Size
	pathLength := int(binary.BigEndian.Uint32(content[7*zookeeper.Uint32Size : pathBegin]))
	pathEnd := pathBegin + pathLength
	var path string
	if length := len(content); length < pathEnd {
		log.DefaultLogger.Errorf("zookeeper.filters.path.HandleResponse, buffer too short")
		return
	} else if length == pathEnd {
		path = string(content[pathBegin:])
	} else {
		path = string(content[pathBegin:pathEnd])
	}
	ctx.PathBegin = pathBegin
	ctx.PathEnd = pathEnd
	ctx.Path = path
	log.DefaultLogger.Debugf("zookeeper.filters.path.HandleResponse, watch event, path: %s, event: %s, state: %s", path, eventType, state)
}
