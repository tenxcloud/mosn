package dubbocontrolpanel

import (
	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func handleConsumerCreateMetadata(upstream zookeeper.Upstream, request *zookeeper.Context) {
	var application string
	var interfaceName string
	request.MustGetParam("application", &application)
	request.MustGetParam("interface", &interfaceName)
	var parameters map[string]string
	if err := json.Unmarshal(request.Data, &parameters); err != nil {
		log.DefaultLogger.Errorf("zookeeper.filters.metadata.Invoke, unmarshal consumer parameters failed, %s", err)
		return
	}
	request.Value = parameters
	if parameters != nil {
		application = parameters[dubbo.ParameterApplicationKey]
	}
	dubbo.SetApplicationName(application)
	if log.DefaultLogger.GetLogLevel() >= log.TRACE {
		log.DefaultLogger.Tracef("zookeeper.filters.metadata.Invoke, application name: %s", application)
	}
}

func handleConsumerGetProviderMetadataRevision(upstream zookeeper.Upstream, request *zookeeper.Context) {
	_, response := upstream.DirectForward(request)
	var metainfo dubbo.MetadataInfo
	if err := json.Unmarshal(response.Data, &metainfo); err != nil {
		log.DefaultLogger.Errorf("handleConsumerGetProviderMetadataRevision, unmarshal metainfo failed, data: %s, %s", string(request.Data), err)
		return
	}
	services := make([]dubbo.ServiceInfo, 0, len(metainfo.Services))
	for _, service := range metainfo.Services {
		services = append(services, service)
	}
	dubbo.UpdateClustersByProvider(metainfo.Application, metainfo.Revision, services)
}
