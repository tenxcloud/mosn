package dubbocontrolpanel

import (
	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
	"mosn.io/pkg/log"
)

func handleProviderCreateMetadata(_ zookeeper.Upstream, request *zookeeper.Context) {
	var application string
	var interfaceName string
	request.MustGetParam("application", &application)
	request.MustGetParam("interface", &interfaceName)
	var serviceDefinition dubbo.FullServiceDefinition
	if err := json.Unmarshal(request.Data, &serviceDefinition); err != nil {
		log.DefaultLogger.Errorf("zookeeper.filters.metadata.Invoke, unmarshal service definition failed, %s", err)
		return
	}
	dubbo.UpdateApplicationsByInterface(interfaceName, []string{application})
	request.Value = &serviceDefinition
	if applicationParameter, exist := serviceDefinition.Parameters[dubbo.ParameterApplicationKey]; exist && applicationParameter != "" {
		application = applicationParameter
	}
	dubbo.SetApplicationName(application)
	if log.DefaultLogger.GetLogLevel() >= log.TRACE {
		log.DefaultLogger.Tracef("zookeeper.filters.metadata.Invoke, application name: %s", application)
	}
}

func handleProviderCreateMetadataRevision(_ zookeeper.Upstream, request *zookeeper.Context) {
	var metainfo dubbo.MetadataInfo
	if err := json.Unmarshal(request.Data, &metainfo); err != nil {
		log.DefaultLogger.Errorf("handleProviderCreateMetadataRevision, unmarshal metainfo failed, data: %s, %s", string(request.Data), err)
		return
	}
	services := make([]dubbo.ServiceInfo, 0, len(metainfo.Services))
	for _, service := range metainfo.Services {
		services = append(services, service)
	}
	dubbo.UpdateClustersByProvider(metainfo.Application, metainfo.Revision, services)
}
