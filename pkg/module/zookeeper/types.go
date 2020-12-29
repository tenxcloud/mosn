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
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sync"

	"github.com/go-zookeeper/zk"
	jsoniter "github.com/json-iterator/go"
	"mosn.io/mosn/pkg/log"
)

type ReadWriteReseter interface {
	io.ReadWriter
	Reset()
	WriteTo(w io.Writer) (n int64, err error)
	Bytes() []byte
	Len() int
}

type Manager interface {
	OnData(buffer ReadWriteReseter, isRequest bool) IOStateCondition
}

type IOStateCondition int

const (
	ZookeeperNeedMoreData      IOStateCondition = 1
	ZookeeperFinishedFiltering IOStateCondition = 2

	Uint32Size = 4
)

type OpCode int

const (
	OpNotify          OpCode = 0
	OpCreate          OpCode = 1
	OpDelete          OpCode = 2
	OpExists          OpCode = 3
	OpGetData         OpCode = 4
	OpSetData         OpCode = 5
	OpGetAcl          OpCode = 6
	OpSetAcl          OpCode = 7
	OpGetChildren     OpCode = 8
	OpSync            OpCode = 9
	OpPing            OpCode = 11
	OpGetChildren2    OpCode = 12
	OpCheck           OpCode = 13
	OpMulti           OpCode = 14
	OpReconfig        OpCode = 16
	OpCreateContainer OpCode = 19
	OpCreateTTL       OpCode = 21
	OpClose           OpCode = -11
	OpSetAuth         OpCode = 100
	OpSetWatches      OpCode = 101
	OpError           OpCode = -1
	OpWatcherEvent    OpCode = -2 // Not in protocol, used internally
)

var (
	opCodeNameMapping = map[OpCode]string{
		OpNotify:          "Notify",
		OpCreate:          "Create",
		OpDelete:          "Delete",
		OpExists:          "Exists",
		OpGetData:         "GetData",
		OpSetData:         "SetData",
		OpGetAcl:          "GetAcl",
		OpSetAcl:          "SetAcl",
		OpGetChildren:     "GetChildren",
		OpSync:            "Sync",
		OpPing:            "Ping",
		OpGetChildren2:    "GetChildren2",
		OpCheck:           "Check",
		OpMulti:           "Multi",
		OpReconfig:        "Reconfig",
		OpCreateContainer: "CreateContainer",
		OpCreateTTL:       "CreateTTL",
		OpClose:           "Close",
		OpSetAuth:         "SetAuth",
		OpSetWatches:      "SetWatches",
		OpError:           "Error",
		OpWatcherEvent:    "WatcherEvent",
	}

	ErrTypeMismatched = errors.New("type mismatched")
)

func (o OpCode) String() string {
	name, known := opCodeNameMapping[o]
	if known {
		return name
	}
	return "Unknown"
}

type pathSetter interface {
	SetPath(path string)
}

type dataSetter interface {
	SetData(data []byte)
}

type childrenSetter interface {
	SetChildren(children []string)
}

type serializer interface {
	Marshal(v interface{}) ([]byte, error)
}

type CreateRequest struct {
	XidAndOpCode Untouched
	Path         string
	Data         []byte
	TheRest      Untouched
}

func (cr *CreateRequest) SetPath(path string) {
	cr.Path = path
}

func (cr *CreateRequest) SetData(data []byte) {
	cr.Data = data
}

type CreateResponse struct {
	XidZxidAndErrCode Untouched
	Path              string
}

func (cr *CreateResponse) SetPath(path string) {
	cr.Path = path
}

type DeleteRequest struct {
	XidAndOpCode Untouched
	Path         string
	TheRest      Untouched
}

func (dr *DeleteRequest) SetPath(path string) {
	dr.Path = path
}

type GetDataResponse struct {
	XidZxidAndErrCode Untouched
	Data              []byte
	TheRest           Untouched
}

func (gdr *GetDataResponse) SetData(data []byte) {
	gdr.Data = data
}

type GetChildren2Request struct {
	XidAndOpCode Untouched
	Path         string
	TheRest      Untouched
}

func (r *GetChildren2Request) SetPath(path string) {
	r.Path = path
}

type GetChildren2Response struct {
	XidZxidAndErrCode Untouched
	Children          []string
	Stat              Untouched
}

func (c *GetChildren2Response) SetChildren(children []string) {
	c.Children = children
}

func (c *GetChildren2Response) Encode(buf []byte) (n int, _ error) {
	log.DefaultLogger.Tracef("zookeeper.GetChildren2Response.Encode")
	// encode header
	n1, _ := c.XidZxidAndErrCode.Encode(buf)
	n += n1
	// encode children
	b := bytes.Buffer{}
	childCount := make([]byte, Uint32Size)
	binary.BigEndian.PutUint32(childCount, uint32(len(c.Children)))
	b.Write(childCount)
	for _, i := range c.Children {
		child := []byte(i)
		childLength := make([]byte, Uint32Size)
		binary.BigEndian.PutUint32(childLength, uint32(len(child)))
		b.Write(childLength)
		b.Write(child)
	}
	n2 := copy(buf[n:], b.Bytes())
	n += n2
	// encode stat
	n3, _ := c.Stat.Encode(buf[n:])
	n += n3
	return
}

type Context struct {
	Buffer                     ReadWriteReseter // the buffer object
	RawPayload                 []byte           // content of buffer buffer
	Xid                        int
	Zxid                       uint64
	OpCode                     OpCode
	Error                      error       // error responsed from zk, not errors occurred in golang
	Path                       string      // the path we want it to be
	OriginalPath               string      // the path it used to be
	Data                       []byte      // the data
	Value                      interface{} // the unmarshaled object from data
	Children                   []string    // the chilren
	PathBegin, PathEnd         int         // the index in the raw payload
	DataBegin, DataEnd         int         // the index in the raw payload
	ChildrenBegin, ChildrenEnd int         // the index in the raw payload
	Request                    *Context    // if this is a response, it's the corresponding request
	Watch                      bool

	modified bool
	payload  interface{} // the final payload layout if we modified the request or respone
	route    *upstream
	params   map[string]interface{}
	session  *sync.Map // the request, response mapping, you should never play with this
}

func (c Context) MustGetParam(name string, receiver interface{}) {
	if _, err := c.GetParam(name, receiver); err != nil {
		panic(err)
	}
}

func (c Context) GetParam(name string, receiver interface{}) (exist bool, err error) {
	if c.params == nil {
		return
	}
	var value interface{}
	if value, exist = c.params[name]; !exist {
		return
	}
	rv := reflect.Indirect(reflect.ValueOf(receiver))
	vv := reflect.ValueOf(value)
	if vv.Type().AssignableTo(rv.Type()) && rv.CanSet() {
		rv.Set(reflect.ValueOf(value))
		return
	}
	err = ErrTypeMismatched
	return
}

func modifyIfNeeded(ctx *Context) (ReadWriteReseter, error) {
	if ctx.payload == nil || !ctx.modified {
		return ctx.Buffer, nil
	}
	payload := ctx.payload
	setNewPath, needSetPath := payload.(pathSetter)
	var deltaLength int
	if needSetPath {
		setNewPath.SetPath(ctx.Path)
		deltaLength += len(ctx.Path) - (ctx.PathEnd - ctx.PathBegin)
	}
	setNewData, needSetData := payload.(dataSetter)
	if needSetData {
		serialize, ok := payload.(serializer)
		if !ok {
			serialize = jsoniter.ConfigCompatibleWithStandardLibrary
		}
		data, err := serialize.Marshal(ctx.Value)
		if err != nil {
			log.DefaultLogger.Errorf("zookeeper.modify, marshal data failed, %s", err)
			return nil, err
		}
		setNewData.SetData(data)
		deltaLength += len(data) - (ctx.DataEnd - ctx.DataBegin)
	}
	setNewChildren, needSetChildren := payload.(childrenSetter)
	if needSetChildren {
		setNewChildren.SetChildren(ctx.Children)
		deltaLength += Uint32Size
		for _, c := range ctx.Children {
			deltaLength += Uint32Size + len(c)
		}
		deltaLength -= (ctx.ChildrenEnd - ctx.ChildrenBegin)
	}
	deltaLengthGetter, hasDeltaLength := payload.(DeltaLength)
	if hasDeltaLength {
		deltaLength += deltaLengthGetter.GetDeltaLength()
	}
	bufferLength := len(ctx.RawPayload) + deltaLength
	buffer := make([]byte, bufferLength)
	if _, err := encodePacket(buffer[Uint32Size:], payload); err != nil {
		log.DefaultLogger.Errorf("zookeeper.modify, marshal payload failed, %s", err)
		return nil, err
	}
	binary.BigEndian.PutUint32(buffer[:Uint32Size], uint32(bufferLength-Uint32Size))
	return bytes.NewBuffer(buffer), nil
}

func serializeContextInternal(ctx *Context, buffer *bytes.Buffer) (err error) {
	var content ReadWriteReseter
	if content, err = modifyIfNeeded(ctx); err != nil {
		return
	}
	_, err = content.WriteTo(buffer)
	return
}

func SerializeContexts(ctxs []*Context) (ReadWriteReseter, error) {
	buffer := new(bytes.Buffer)
	for _, ctx := range ctxs {
		if err := serializeContextInternal(ctx, buffer); err != nil {
			return nil, err
		}
	}
	return buffer, nil
}

const (
	Undefined = math.MinInt32
)

func NewContext(buffer ReadWriteReseter, session *sync.Map) *Context {
	return &Context{
		Buffer:     buffer,
		RawPayload: buffer.Bytes(),
		Xid:        Undefined,
		Zxid:       math.MaxUint64,
		OpCode:     Undefined,
		session:    session,
	}
}

type Filter interface {
	HandleRequest(ctx *Context)
	HandleResponse(ctx *Context)
}

func ParseErrorCode(code int) error {
	err, known := errCodeErrorMapping[code]
	if known {
		return err
	}
	return errors.New(fmt.Sprintf("unknown zookeeper error code, %d", code))
}

const (
	errOk = 0
	// System and server-side errors
	errSystemError          = -1
	errRuntimeInconsistency = -2
	errDataInconsistency    = -3
	errConnectionLoss       = -4
	errMarshallingError     = -5
	errUnimplemented        = -6
	errOperationTimeout     = -7
	errBadArguments         = -8
	errInvalidState         = -9
	// API errors
	errAPIError                = -100
	errNoNode                  = -101 // *
	errNoAuth                  = -102
	errBadVersion              = -103 // *
	errNoChildrenForEphemerals = -108
	errNodeExists              = -110 // *
	errNotEmpty                = -111
	errSessionExpired          = -112
	errInvalidCallback         = -113
	errInvalidAcl              = -114
	errAuthFailed              = -115
	errClosing                 = -116
	errNothing                 = -117
	errSessionMoved            = -118
	// Attempts to perform a reconfiguration operation when reconfiguration feature is disabled
	errZReconfigDisabled = -123
)

var (
	ErrInvalidCallback      = errors.New("InvalidCallback")
	ErrZReconfigDisabled    = errors.New("ZReconfigDisabled")
	ErrSystemError          = errors.New("SystemError")
	ErrRuntimeInconsistency = errors.New("RuntimeInconsistency")
	ErrDataInconsistency    = errors.New("DataInconsistency")
	ErrConnectionLoss       = errors.New("ConnectionLoss")
	ErrMarshallingError     = errors.New("MarshallingError")
	ErrUnimplemented        = errors.New("Unimplemented")
	ErrOperationTimeout     = errors.New("OperationTimeout")
	ErrBadArguments         = errors.New("BadArguments")
	ErrInvalidState         = errors.New("InvalidState")

	errCodeErrorMapping = map[int]error{
		errOk:                      nil,
		errAPIError:                zk.ErrAPIError,
		errNoNode:                  zk.ErrNoNode,
		errNoAuth:                  zk.ErrNoAuth,
		errBadVersion:              zk.ErrBadVersion,
		errNoChildrenForEphemerals: zk.ErrNoChildrenForEphemerals,
		errNodeExists:              zk.ErrNodeExists,
		errNotEmpty:                zk.ErrNotEmpty,
		errSessionExpired:          zk.ErrSessionExpired,
		errInvalidCallback:         ErrInvalidCallback,
		errInvalidAcl:              zk.ErrInvalidACL,
		errAuthFailed:              zk.ErrAuthFailed,
		errClosing:                 zk.ErrClosing,
		errNothing:                 zk.ErrNothing,
		errSessionMoved:            zk.ErrSessionMoved,
		errZReconfigDisabled:       ErrZReconfigDisabled,
		errSystemError:             ErrSystemError,
		errRuntimeInconsistency:    ErrRuntimeInconsistency,
		errDataInconsistency:       ErrDataInconsistency,
		errConnectionLoss:          ErrConnectionLoss,
		errMarshallingError:        ErrMarshallingError,
		errUnimplemented:           ErrUnimplemented,
		errOperationTimeout:        ErrOperationTimeout,
		errBadArguments:            ErrBadArguments,
		errInvalidState:            ErrInvalidState,
	}
)
