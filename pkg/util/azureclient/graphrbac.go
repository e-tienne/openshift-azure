package azureclient

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest"
	"github.com/sirupsen/logrus"
)

// RBACApplicationsClient is a minimal interface for azure ApplicationsClient
type RBACApplicationsClient interface {
	Create(ctx context.Context, parameters graphrbac.ApplicationCreateParameters) (result graphrbac.Application, err error)
	Delete(ctx context.Context, applicationObjectID string) (result autorest.Response, err error)
	List(ctx context.Context, filter string) (result graphrbac.ApplicationListResultPage, err error)
	Patch(ctx context.Context, applicationObjectID string, parameters graphrbac.ApplicationUpdateParameters) (result autorest.Response, err error)
}

type rbacApplicationsClient struct {
	graphrbac.ApplicationsClient
}

var _ RBACApplicationsClient = &rbacApplicationsClient{}

// NewRBACApplicationsClient creates a new ApplicationsClient
func NewRBACApplicationsClient(ctx context.Context, log *logrus.Entry, tenantID string, authorizer autorest.Authorizer) RBACApplicationsClient {
	client := graphrbac.NewApplicationsClient(tenantID)
	setupClient(ctx, log, "graphrbac.ApplicationsClient", &client.Client, authorizer)

	return &rbacApplicationsClient{
		ApplicationsClient: client,
	}
}

// RBACGroupsClient is a minimal interface for azure GroupsClient
type RBACGroupsClient interface {
	RBACGroupsClientAddons
}

type rbacGroupsClient struct {
	graphrbac.GroupsClient
}

var _ RBACGroupsClient = &rbacGroupsClient{}

// NewRBACApplicationsClient creates a new ApplicationsClient
func NewRBACGroupsClient(ctx context.Context, log *logrus.Entry, tenantID string, authorizer autorest.Authorizer) RBACGroupsClient {
	client := graphrbac.NewGroupsClient(tenantID)
	setupClient(ctx, log, "graphrbac.GroupsClient", &client.Client, authorizer)

	return &rbacGroupsClient{
		GroupsClient: client,
	}
}

type ServicePrincipalsClient interface {
	Create(ctx context.Context, parameters graphrbac.ServicePrincipalCreateParameters) (result graphrbac.ServicePrincipal, err error)
	List(ctx context.Context, filter string) (graphrbac.ServicePrincipalListResultPage, error)
}

type servicePrincipalsClient struct {
	graphrbac.ServicePrincipalsClient
}

var _ ServicePrincipalsClient = &servicePrincipalsClient{}

// NewServicePrincipalsClient create a client to query ServicePrincipal information
func NewServicePrincipalsClient(ctx context.Context, log *logrus.Entry, tenantID string, authorizer autorest.Authorizer) ServicePrincipalsClient {
	client := graphrbac.NewServicePrincipalsClient(tenantID)
	setupClient(ctx, log, "graphrbac.ServicePrincipalsClient", &client.Client, authorizer)

	return &servicePrincipalsClient{
		ServicePrincipalsClient: client,
	}
}
