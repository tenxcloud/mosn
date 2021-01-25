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
	"strings"
	"time"

	"mosn.io/pkg/log"

	"github.com/go-zookeeper/zk"
)

type Client struct {
	Conn *zk.Conn
}

var (
	serviceRegistryAddress  string
	metadataRegistryAddress string
)

func SetServiceRegistryAddress(address string) {
	serviceRegistryAddress = address
}

func SetMetadataRegistryAddress(address string) {
	metadataRegistryAddress = address
}

func MetadataClient() (client *Client, err error) {
	return NewClientWithHosts([]string{metadataRegistryAddress})
}

func ServiceClient() (client *Client, err error) {
	return NewClientWithHosts([]string{serviceRegistryAddress})
}

func NewClientWithHosts(hosts []string) (client *Client, err error) {
	conn, _, err := zk.Connect(hosts, time.Second*5)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.NewClientWithHosts, zk.Connect failed: %s", err)
		return
	}
	client = &Client{
		Conn: conn,
	}
	return
}

func (c *Client) Close() {
	c.Conn.Close()
}

func (c *Client) CreateParant(path string, flags int32, acl []zk.ACL) (string, error) {
	pathSplit := strings.Split(path, "/")
	pathCurrent := strings.Builder{}
	if len(pathSplit) <= 1 {
		return path, nil
	}
	for _, item := range pathSplit[:len(pathSplit)-1] {
		if item == "" {
			continue
		}
		pathCurrent.WriteString("/")
		pathCurrent.WriteString(item)
		exist, _, err := c.Conn.Exists(pathCurrent.String())
		if err != nil {
			return "", err
		}
		if !exist {
			log.DefaultLogger.Tracef("zookeeper.CreateWithRecursive, node not exist, creating %s", pathCurrent.String())
			c.Conn.Create(pathCurrent.String(), []byte{}, flags, acl)
		}
	}
	return pathCurrent.String(), nil
}

func (c *Client) CreateWithRecursive(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	_, err := c.CreateParant(path, flags, acl)
	if err != nil {
		return "", err
	}
	exist, stat, err := c.Conn.Exists(path)
	if err != nil {
		return "", err
	}
	if exist {
		log.DefaultLogger.Tracef("zookeeper.CreateWithRecursive, node exist, deleting %s", path)
		c.Conn.Delete(path, stat.Version)
	} else {
		log.DefaultLogger.Tracef("zookeeper.CreateWithRecursive, node not exist, creating %s", path)
	}
	return c.Conn.Create(path, data, flags, acl)
}
