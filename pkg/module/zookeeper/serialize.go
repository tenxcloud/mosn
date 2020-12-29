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
	"encoding/binary"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-zookeeper/zk"
)

func TheRest(content []byte, begin int) Untouched {
	return &originalContentView{
		content: content,
		begin:   begin,
		end:     Undefined,
	}
}

func OriginalContentView(content []byte, begin, end int) Untouched {
	return &originalContentView{
		content: content,
		begin:   begin,
		end:     end,
	}
}

func RequestHeader(content []byte) Untouched {
	return &originalContentView{
		content: content,
		begin:   Uint32Size,
		end:     3 * Uint32Size,
	}
}

func ResponseHeader(content []byte) Untouched {
	return &originalContentView{
		content: content,
		begin:   Uint32Size,
		end:     5 * Uint32Size,
	}
}

type originalContentView struct {
	content    []byte
	begin, end int
}

func (ocv originalContentView) Encode(buf []byte) (n int, _ error) {
	var content []byte
	if ocv.end <= 0 {
		content = ocv.content[ocv.begin:]
	} else {
		content = ocv.content[ocv.begin:ocv.end]
	}
	n = copy(buf, content)
	return
}

type DeltaLength interface {
	GetDeltaLength() int
}

func NewDeltaLength(length int) DeltaLength {
	return &deltaLength{
		length: length,
	}
}

type deltaLength struct {
	length int
}

func (dl deltaLength) GetDeltaLength() int {
	return dl.length
}

func (deltaLength) Encode(_ []byte) (int, error) {
	return 0, nil
}

type Untouched encoder

type encoder interface {
	Encode(buf []byte) (int, error)
}

func encodePacket(buf []byte, st interface{}) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(runtime.Error); ok && strings.HasPrefix(e.Error(), "runtime error: slice bounds out of range") {
				err = zk.ErrShortBuffer
			} else {
				panic(r)
			}
		}
	}()

	v := reflect.ValueOf(st)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0, zk.ErrPtrExpected
	}
	return encodePacketValue(buf, v)
}

func encodePacketValue(buf []byte, v reflect.Value) (int, error) {
	rv := v
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	n := 0
	switch v.Kind() {
	default:
		return n, zk.ErrUnhandledFieldType
	case reflect.Struct:
		if en, ok := rv.Interface().(encoder); ok {
			return en.Encode(buf)
		} else if en, ok := v.Interface().(encoder); ok {
			return en.Encode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := encodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		if v.Bool() {
			buf[n] = 1
		} else {
			buf[n] = 0
		}
		n++
	case reflect.Int32:
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(v.Int()))
		n += 4
	case reflect.Int64:
		binary.BigEndian.PutUint64(buf[n:n+8], uint64(v.Int()))
		n += 8
	case reflect.String:
		str := v.String()
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(str)))
		copy(buf[n+4:n+4+len(str)], []byte(str))
		n += 4 + len(str)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := v.Len()
			startN := n
			n += 4
			for i := 0; i < count; i++ {
				n2, err := encodePacketValue(buf[n:], v.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
			binary.BigEndian.PutUint32(buf[startN:startN+4], uint32(count))
		case reflect.Uint8:
			if v.IsNil() {
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(0xffffffff))
				n += 4
			} else {
				bytes := v.Bytes()
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(bytes)))
				copy(buf[n+4:n+4+len(bytes)], bytes)
				n += 4 + len(bytes)
			}
		}
	}
	return n, nil
}
