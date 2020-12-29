package dubbocontrolpanel

import (
	"net/url"
	"testing"

	"mosn.io/pkg/log"
)

func TestV2Register(t *testing.T) {
	// urlEncoded := "dubbo%3A%2F%2F192.168.1.98%3A20880%2Forg.apache.dubbo.demo.DemoService%3Fanyhost%3Dtrue%26application%3Ddubbo-demo-api-provider%26default%3Dtrue%26deprecated%3Dfalse%26dubbo%3D2.0.2%26dynamic%3Dtrue%26generic%3Dfalse%26interface%3Dorg.apache.dubbo.demo.DemoService%26methods%3DsayHello%2CsayHelloAsync%26pid%3D159845%26release%3D2.7.9-SNAPSHOT%26revision%3D2.7.9-SNAPSHOT%26side%3Dprovider%26timestamp%3D1610004944049"
	// urlEncoded := "dubbo%3A%2F%2F172.31.1.43%3A20880%2Forg.apache.dubbo.demo.GreetingService%3FMIGRATION_MULTI_REGSITRY%3Dtrue%26anyhost%3Dtrue%26application%3Ddemo-service-provider%26default%3Dtrue%26deprecated%3Dfalse%26dubbo%3D2.0.2%26dynamic%3Dtrue%26generic%3Dfalse%26group%3Dgreeting%26interface%3Dorg.apache.dubbo.demo.GreetingService%26mapping-type%3Dmetadata%26mapping.type%3Dmetadata%26metadata-type%3Dremote%26methods%3Dhello%26pid%3D1%26release%3D2.7.9-SNAPSHOT%26revision%3D2.7.9-SNAPSHOT%26side%3Dprovider%26timeout%3D5000%26timestamp%3D1609915095293%26version%3D1.0.0"
	urlEncoded := "dubbo%3A%2F%2F192.168.1.98%3A20880%2Forg.apache.dubbo.demo.GreetingService%3Fanyhost%3Dtrue%26application%3Ddubbo-demo-v2-provider%26default%3Dtrue%26deprecated%3Dfalse%26dubbo%3D2.0.2%26dynamic%3Dtrue%26generic%3Dfalse%26group%3Dgreeting%26interface%3Dorg.apache.dubbo.demo.GreetingService%26methods%3Dhello%26pid%3D145017%26release%3D2.7.9-SNAPSHOT%26revision%3D2.7.9-SNAPSHOT%26side%3Dprovider%26timeout%3D5000%26timestamp%3D1610450399662%26version%3D1.0.0"
	urlRaw, _ := url.QueryUnescape(urlEncoded)
	log.DefaultLogger.SetLogLevel(log.TRACE)

	u, _ := url.Parse(urlRaw)
	t.Logf("%+v", u)
	q, _ := url.ParseQuery(u.RawQuery)
	t.Logf("%+v", q)
	// e := manualPublishMetadata(q)
	// t.Logf("%+v", e)

	url2 := "dubbo%3A%2F%2F192.168.1.98%3A20888%2Forg.apache.dubbo.demo.GreetingService%3Fanyhost%3Dtrue%26application%3Ddubbo-demo-v2-provider%26default%3Dtrue%26deprecated%3Dfalse%26dubbo%3D2.0.2%26dynamic%3Dtrue%26generic%3Dfalse%26interface%3Dorg.apache.dubbo.demo.GreetingService%26methods%3Dhello%26pid%3D159845%26release%3D2.7.9-SNAPSHOT%26revision%3D2.7.9-SNAPSHOT%26side%3Dprovider%26timestamp%3D1610446163478"
	urlRaw, _ = url.QueryUnescape(url2)
	log.DefaultLogger.SetLogLevel(log.TRACE)

	u, _ = url.Parse(urlRaw)
	t.Logf("%+v", u)
	q, _ = url.ParseQuery(u.RawQuery)
	t.Logf("%+v", q)
}
