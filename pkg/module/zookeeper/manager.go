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

package zookeeper

import (
	"bytes"
	"encoding/binary"
	"sync"

	"mosn.io/mosn/pkg/log"
)

func NewManager(filters []Filter, session *sync.Map) Manager {
	return &manager{
		filters: filters,
		session: session,
	}
}

type manager struct {
	buffer               *bytes.Buffer
	currentPayloadLength int
	filters              []Filter
	session              *sync.Map
}

func (m *manager) OnData(buffer ReadWriteReseter, isRequest bool) IOStateCondition {
	condition := m.onDataInternal(buffer, isRequest)
	if condition == ZookeeperNeedMoreData {
		if d, ok := buffer.(drainer); ok {
			d.Drain(buffer.Len())
		}
	}
	return condition
}

func (m *manager) onDataInternal(ioReference ReadWriteReseter, isRequest bool) IOStateCondition {
	// there was incomplete data
	if buffer := m.buffer; buffer != nil {

		if _, err := ioReference.WriteTo(buffer); err != nil {
			// caching failed
			log.DefaultLogger.Errorf("zookeeper.manager.handleCachingFailure, caching message buffer failed, %s", err)
			m.handleCachingFailure(buffer, ioReference)
			return ZookeeperFinishedFiltering
		}

		if m.currentPayloadLength <= 0 && buffer.Len() > Uint32Size {
			m.currentPayloadLength = int(binary.BigEndian.Uint32(buffer.Bytes()[:4]))
		}

		if got, except := buffer.Len(), m.currentPayloadLength+Uint32Size; got == except {
			// data complete
			defer m.reset()
			m.invokeFiltersForOneMessage(buffer, ioReference, isRequest)

			return ZookeeperFinishedFiltering
		} else if got > except {
			// more than one message
			completed, incompleted, incompletedMessageLength := splitCompletedMessage(buffer, m.currentPayloadLength)
			m.invokeFiltersForMessages(completed, ioReference, isRequest)

			if incompleted != nil {
				m.buffer = incompleted
				m.currentPayloadLength = incompletedMessageLength
			} else {
				defer m.reset()
			}

			return ZookeeperFinishedFiltering
		}

		// data still incomplete
		log.DefaultLogger.Debugf("zookeeper.manager.OnData, cached message exist, data incomplete")
		return ZookeeperNeedMoreData
	}

	length := ioReference.Len()
	if length <= Uint32Size {
		m.buffer = new(bytes.Buffer)
		if _, err := ioReference.WriteTo(m.buffer); err != nil {
			// because caching data failed, return the original data
			defer m.reset()
			log.DefaultLogger.Errorf("zookeeper.manager.OnData, caching incompleted message failed, %s", err)
			return ZookeeperFinishedFiltering
		}
		log.DefaultLogger.Debugf("zookeeper.manager.OnData, no cached message, data incomplete")
		return ZookeeperNeedMoreData
	}

	currentPayloadLength := int(binary.BigEndian.Uint32(ioReference.Bytes()[:4]))
	if length == currentPayloadLength+Uint32Size {
		defer m.reset()
		m.invokeFiltersForOneMessage(ioReference, ioReference, isRequest)
		return ZookeeperFinishedFiltering
	}

	m.buffer = new(bytes.Buffer)
	if _, err := ioReference.WriteTo(m.buffer); err != nil {
		// because caching data failed, return the original data
		defer m.reset()
		log.DefaultLogger.Errorf("zookeeper.manager.OnData, caching incompleted message failed, %s", err)
		return ZookeeperFinishedFiltering
	}
	m.currentPayloadLength = currentPayloadLength
	log.DefaultLogger.Debugf("zookeeper.manager.OnData, data incomplete")
	return ZookeeperNeedMoreData
}

func (m *manager) reset() {
	m.buffer = nil
	m.currentPayloadLength = -1
}

func (m *manager) invokeFiltersForOneMessage(message, ioReference ReadWriteReseter, isRequest bool) bool {
	return m.invokeFiltersForMessages([]ReadWriteReseter{message}, ioReference, isRequest)
}

type drainer interface {
	Drain(offset int)
}

func (m *manager) invokeFiltersForMessages(
	messages []ReadWriteReseter, ioReference ReadWriteReseter, isRequest bool) (failed bool) {

	ctxs := make([]*Context, 0, len(messages))
	for _, message := range messages {
		ctx := NewContext(message, m.session)
		ctxs = append(ctxs, ctx)
		for _, filter := range m.filters {
			if isRequest {
				filter.HandleRequest(ctx)
			} else {
				filter.HandleResponse(ctx)
			}
		}
	}
	buffer, err := SerializeContexts(ctxs)
	if err != nil {
		failed = true
		log.DefaultLogger.Errorf("zookeeper.manager.invokeFilters, serialize filtered message failed, %s", err)
		return
	}
	if d, ok := ioReference.(drainer); ok {
		d.Drain(ioReference.Len())
	}
	ioReference.Reset()
	if _, err := buffer.WriteTo(ioReference); err != nil {
		failed = true
		log.DefaultLogger.Errorf("zookeeper.manager.invokeFilters, forward filtered message failed, %s", err)
		return
	}
	return
}

func (m *manager) handleCachingFailure(source, target ReadWriteReseter) (failed bool) {
	defer m.reset()
	if d, ok := target.(drainer); ok {
		d.Drain(target.Len())
	}
	target.Reset()
	if _, err := source.WriteTo(target); err != nil {
		failed = true
		log.DefaultLogger.Errorf("zookeeper.manager.handleCachingFailure, forward cached message buffer failed, %s", err)
	}
	return
}

func splitCompletedMessage(
	buffer ReadWriteReseter, currentPayloadLength int) (
	completed []ReadWriteReseter,
	incompleted *bytes.Buffer,
	incompletedMessageLength int) {

	completed = make([]ReadWriteReseter, 0)
	content := buffer.Bytes()
	length := len(content)
	begin := 0
	end := currentPayloadLength + Uint32Size
	for end <= length {
		if end == length {
			completed = append(completed, bytes.NewBuffer(content[begin:]))
			break
		}
		completed = append(completed, bytes.NewBuffer(content[begin:end]))
		left := length - end
		if left < Uint32Size {
			incompleted = bytes.NewBuffer(content[end:])
			break
		}
		currentPayloadLength = int(binary.BigEndian.Uint32(content[end : end+Uint32Size]))
		if left < currentPayloadLength+Uint32Size {
			incompletedMessageLength = currentPayloadLength
			incompleted = bytes.NewBuffer(content[end:])
			break
		}
		begin = end
		end += currentPayloadLength + Uint32Size
	}
	return
}
