package vmimage

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/sirupsen/logrus"

	"github.com/openshift/openshift-azure/pkg/util/arm"
	"github.com/openshift/openshift-azure/pkg/util/azureclient"
	"github.com/openshift/openshift-azure/pkg/util/template"
	"github.com/openshift/openshift-azure/pkg/util/tls"
)

//go:generate ../../hack/build-archive.sh
//go:generate go get github.com/go-bindata/go-bindata/go-bindata
//go:generate go-bindata -nometadata -pkg $GOPACKAGE -prefix data data
//go:generate gofmt -s -l -w bindata.go

// Builder is the VM image configuration struct
type Builder struct {
	GitCommit                string
	Log                      *logrus.Entry
	Deployments              azureclient.DeploymentsClient
	Groups                   azureclient.GroupsClient
	SubscriptionID           string
	Location                 string
	BuildResourceGroup       string
	DeleteBuildResourceGroup bool
	DomainNameLabel          string
	Image                    string
	ImageResourceGroup       string
	ImageStorageAccount      string
	ImageContainer           string
	SSHKey                   *rsa.PrivateKey
	ClientKey                *rsa.PrivateKey
	ClientCert               *x509.Certificate
}

func (builder *Builder) generateTemplate() (map[string]interface{}, error) {
	script, err := template.Template("script.sh", string(MustAsset("script.sh")), nil, map[string]interface{}{
		"Archive":      MustAsset("archive.tgz"),
		"Builder":      builder,
		"ClientID":     os.Getenv("AZURE_CLIENT_ID"),
		"ClientSecret": os.Getenv("AZURE_CLIENT_SECRET"),
		"TenantID":     os.Getenv("AZURE_TENANT_ID"),
	})
	if err != nil {
		return nil, err
	}

	cse, err := cse(builder.Location, script)
	if err != nil {
		return nil, err
	}

	sshPublicKey, err := tls.SSHPublicKeyAsString(&builder.SSHKey.PublicKey)
	if err != nil {
		return nil, err
	}

	t := arm.Template{
		Schema:         "https://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
		ContentVersion: "1.0.0.0",
		Resources: []interface{}{
			vnet(builder.Location),
			ip(builder.BuildResourceGroup, builder.Location, builder.DomainNameLabel),
			nsg(builder.Location),
			nic(builder.SubscriptionID, builder.BuildResourceGroup, builder.Location),
			vm(builder.SubscriptionID, builder.BuildResourceGroup, builder.Location, sshPublicKey),
			cse,
		},
	}

	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	var template map[string]interface{}
	err = json.Unmarshal(b, &template)
	if err != nil {
		return nil, err
	}

	err = arm.FixupAPIVersions(template, versionMap)
	if err != nil {
		return nil, err
	}

	arm.FixupDepends(builder.SubscriptionID, builder.BuildResourceGroup, template)

	return template, nil
}

// Run is the main entry point
func (builder *Builder) Run(ctx context.Context) error {
	template, err := builder.generateTemplate()
	if err != nil {
		return err
	}

	defer func() {
		if builder.DeleteBuildResourceGroup {
			builder.Log.Infof("deleteBuildResourceGroup set, deleting build resource group")
			builder.Groups.Delete(ctx, builder.BuildResourceGroup)
		}
	}()

	builder.Log.Infof("creating resource group %s", builder.BuildResourceGroup)
	_, err = builder.Groups.CreateOrUpdate(ctx, builder.BuildResourceGroup, resources.Group{
		Location: to.StringPtr(builder.Location),
		Tags: map[string]*string{
			"now": to.StringPtr(fmt.Sprintf("%d", time.Now().Unix())),
			"ttl": to.StringPtr("6h"),
		},
	})
	if err != nil {
		return err
	}

	builder.Log.Infof("deploying template, ssh to VM if needed via:")
	builder.Log.Infof("  ssh -i id_rsa cloud-user@%s.%s.cloudapp.azure.com", builder.DomainNameLabel, builder.Location)
	future, err := builder.Deployments.CreateOrUpdate(ctx, builder.BuildResourceGroup, "azuredeploy", resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Template: template,
			Mode:     resources.Incremental,
		},
	})
	if err != nil {
		return err
	}

	go builder.ssh()

	cli := builder.Deployments.Client()
	cli.PollingDuration = time.Hour

	builder.Log.Infof("waiting for deployment")
	err = future.WaitForCompletionRef(ctx, cli)
	if err != nil {
		return err
	}

	return nil
}
