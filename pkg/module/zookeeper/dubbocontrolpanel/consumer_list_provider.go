package dubbocontrolpanel

import (
	"mosn.io/mosn/pkg/module/dubbo"
	"mosn.io/mosn/pkg/module/zookeeper"
)

func handleConsumerListProviders(upstream zookeeper.Upstream, request *zookeeper.Context) {
	var interfaceName string
	request.MustGetParam("interface", &interfaceName)
	_, response := upstream.DirectForward(request)
	if response.Error != nil {
		return
	}
	children := response.Children
	response.Value = children
	dubbo.UpdateApplicationsByInterface(interfaceName, children)
}

func handleConsumerListProviderApplications(upstream zookeeper.Upstream, request *zookeeper.Context) {
	_, response := upstream.DirectForward(request)
	children := response.Children
	request.Value = children
	var application string
	request.MustGetParam("application", &application)
	dubbo.UpdateEndpointsByApplication(application, children)
}
