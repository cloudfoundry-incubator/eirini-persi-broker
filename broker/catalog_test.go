package broker_test

import (
	"fmt"

	"github.com/SUSE/eirini-persi-broker/config"
	brokerconfig "github.com/SUSE/eirini-persi-broker/config"
	"github.com/pivotal-cf/brokerapi"
)

var (
	DefaultOrgID      = "6b07fc5a-a078-443c-9e27-18cbb687464a"
	DefaultSpaceID    = "ada81a23-dfa4-4bb0-9ba1-0ecf683c244f"
	DefaultAppID      = "90e4f8f1-ecd8-4f3d-8df6-af01da702968"
	DefaultPlanID     = "cbc1d629-00b8-4cf0-81cc-9b9f96635071"
	DefaultServiceID  = "747c021a-8a9f-41a0-adf0-27296246ac79"
	DefaultInstanceID = "d03166fd-8cf8-4bf6-982d-ba95187cb72a"
	DefaultBindingID  = "30695473-b320-4fe3-87f4-6c1673cfc98c"

	DefaultStorageClass  = "storageClass"
	DefaultPlanName      = "fooPlan"
	DefaultServiceName   = "barService"
	DefaultNamespace     = "baz"
	DefaultAnnotationKey = "eirini-broker-binding-" + DefaultBindingID
	DefaultMountLocation = "/testmount"
)

func DefaultProvisionDetails() brokerapi.ProvisionDetails {
	return brokerapi.ProvisionDetails{
		OrganizationGUID: DefaultOrgID,
		PlanID:           DefaultPlanID,
		ServiceID:        DefaultServiceID,
		SpaceGUID:        DefaultSpaceID,
	}
}

func DefaultPlanConfiguration() config.Plan {
	return config.Plan{

		ID:           DefaultPlanID,
		Name:         DefaultPlanName,
		StorageClass: DefaultStorageClass,
		Free:         true,
	}
}

func DefaultServiceConfiguration() config.ServiceConfiguration {
	return config.ServiceConfiguration{
		ServiceID:   DefaultServiceID,
		ServiceName: DefaultServiceName,
		Plans:       []brokerconfig.Plan{DefaultPlanConfiguration()},
	}
}

func DefaultDeprovisionDetails() brokerapi.DeprovisionDetails {
	return brokerapi.DeprovisionDetails{
		PlanID:    DefaultPlanID,
		ServiceID: DefaultServiceID,
	}
}

func DefaultBindDetails() brokerapi.BindDetails {
	parameters := []byte(fmt.Sprintf(`{"dir": "%s"}`, DefaultMountLocation))

	return brokerapi.BindDetails{
		PlanID:        DefaultPlanID,
		ServiceID:     DefaultServiceID,
		AppGUID:       DefaultAppID,
		RawParameters: parameters,
	}
}

func DefaultUnbindDetails() brokerapi.UnbindDetails {
	return brokerapi.UnbindDetails{
		PlanID:    DefaultPlanID,
		ServiceID: DefaultServiceID,
	}
}
