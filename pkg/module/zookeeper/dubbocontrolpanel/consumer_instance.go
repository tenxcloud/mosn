package dubbocontrolpanel

import (
	"fmt"

	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func handleConsumerInstanceResolve(upstream zookeeper.Upstream, request *zookeeper.Context) {
	var application string
	var host string
	var port int
	request.MustGetParam("application", &application)
	request.MustGetParam("host", &host)
	request.MustGetParam("port", &port)
	downstream, response := upstream.DirectForward(request)
	if response.Error != nil {
		return
	}
	var serviceInstance dubbo.ServiceInstance
	if err := json.Unmarshal(response.Data, &serviceInstance); err != nil {
		log.DefaultLogger.Errorf("zookeeper.filters.instance.handleGetDataResponse, unmarshal data failed, %s", err)
		return
	}
	revision := serviceInstance.Payload.Metadata[dubbo.MetadataRevisionKey]
	endpoint := dubbo.GetEndpointByApplication(application, host, port)
	if endpoint != nil {
		endpoint.Revision = revision
	}
	exposePort := dubbo.GetDubboExposePort()
	serviceInstance.Address = "127.0.0.1"
	serviceInstance.ID = fmt.Sprintf("127.0.0.1:%d", exposePort)
	serviceInstance.Port = exposePort
	var endpoints []dubbo.Endpoint
	json.UnmarshalFromString(serviceInstance.Payload.Metadata[dubbo.MetadataEndpointKey], &endpoints)
	for i := range endpoints {
		endpoints[i].Port = exposePort
	}
	metadataEndpoints, err := json.MarshalToString(endpoints)
	if err != nil {
		log.DefaultLogger.Errorf("zookeeper.filters.instance.handleGetDataResponse, marshal modified endpoints failed, %s", err)
		return
	}
	serviceInstance.Payload.Metadata[dubbo.MetadataEndpointKey] = metadataEndpoints
	response.Value = &serviceInstance
	content := response.RawPayload
	downstream.ModifyAndReply(response, &zookeeper.GetDataResponse{
		XidZxidAndErrCode: zookeeper.ResponseHeader(content),
		TheRest:           zookeeper.TheRest(content, response.DataEnd),
	})
}
