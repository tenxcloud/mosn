package zookeeper

import (
	"testing"

	"mosn.io/pkg/log"

	"github.com/go-zookeeper/zk"
)

func TestCreateWithRecursive(t *testing.T) {
	path := "/test/a/b/c/d"
	data := "efg"
	log.DefaultLogger.SetLogLevel(log.TRACE)

	zkclient, err := NewClientWithHosts([]string{"192.168.1.98:32181"})
	if err != nil {
		panic(err)
	}
	defer zkclient.Close()

	p, err := zkclient.CreateWithRecursive(path, []byte(data), 0, zk.WorldACL(zk.PermAll))
	t.Logf("%+v", p)
	t.Logf("%+v", err)
}
