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

import "testing"

func TestRoute(t *testing.T) {
	if e := Register(OpCreate, "/services/demo-service-provider2/<string:host>:<int:port>/<regex:revision:[a-fA-F0-9]{32}>", func(u Upstream, c *Context) {
		var port int
		exist, err := c.GetParam("port", &port)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if !exist {
			t.Fatalf("not exist")
		}
	}); e != nil {
		t.Fatalf("e: %s", e)
	}
	ee, ms := resolveRoute(&Context{
		OpCode: OpCreate,
		Path:   "/services/demo-service-provider2/192.168.38.1:20888/D95B6D3E1EA42289CE11BAA3EBDF2CCB",
	})
	if ee == nil {
		t.Fatalf("not found")
	}
	ps := prepareParams(ee, ms)
	t.Logf("ps: %#v", ps)
	c := &Context{params: ps}
	var port int
	if exist, err := c.GetParam("port", &port); err != nil {
		t.Fatalf("getparam: %s", err)
	} else if !exist {
		t.Fatalf("getparam, not exist")
	}
}
