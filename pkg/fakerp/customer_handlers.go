package fakerp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"

	internalapi "github.com/openshift/openshift-azure/pkg/api"
	v20190430 "github.com/openshift/openshift-azure/pkg/api/2019-04-30"
	admin "github.com/openshift/openshift-azure/pkg/api/admin"
	"github.com/openshift/openshift-azure/pkg/fakerp/shared"
	"github.com/openshift/openshift-azure/pkg/util/azureclient"
)

func (s *Server) handleDelete(w http.ResponseWriter, req *http.Request) {
	cs := s.read()
	if cs == nil {
		s.badRequest(w, "Failed to read the internal config")
		return
	}

	authorizer, err := azureclient.GetAuthorizerFromContext(req.Context(), internalapi.ContextKeyClientAuthorizer)
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to determine request credentials: %v", err))
		return
	}
	// TODO: Determine subscription ID from the request path
	gc := resources.NewGroupsClient(os.Getenv("AZURE_SUBSCRIPTION_ID"))
	gc.Authorizer = authorizer

	am, err := newAADManager(req.Context(), s.log, cs)
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to delete service principals: %v", err))
		return
	}

	s.log.Info("deleting service principals")
	err = am.deleteApps(req.Context())
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to delete service principals: %v", err))
		return
	}

	// delete dns records
	// TODO: get resource group from request path
	dm, err := newDNSManager(req.Context(), s.log, os.Getenv("AZURE_SUBSCRIPTION_ID"), os.Getenv("DNS_RESOURCEGROUP"), os.Getenv("DNS_DOMAIN"))
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to delete dns records: %v", err))
		return
	}
	err = dm.deleteOCPDNS(req.Context(), cs)
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to delete dns records: %v", err))
		return
	}

	resourceGroup := filepath.Base(req.URL.Path)
	s.log.Infof("deleting resource group %s", resourceGroup)

	future, err := gc.Delete(req.Context(), resourceGroup)
	if err != nil {
		if autoRestErr, ok := err.(autorest.DetailedError); ok {
			if original, ok := autoRestErr.Original.(*azure.RequestError); ok {
				if original.StatusCode == http.StatusNotFound {
					return
				}
			}
		}
		s.badRequest(w, fmt.Sprintf("Failed to delete resource group: %v", err))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := future.WaitForCompletionRef(ctx, gc.Client); err != nil {
			s.badRequest(w, fmt.Sprintf("Failed to wait for resource group deletion: %v", err))
			return
		}
		resp, err := future.Result(gc)
		if err != nil {
			s.badRequest(w, fmt.Sprintf("Failed to get resource group deletion response: %v", err))
			return
		}
		// If the resource group deletion is successful, cleanup the object
		// from the memory so the next GET from the client waiting for this
		// long-running operation can exit successfully.
		if resp.StatusCode == http.StatusOK {
			s.log.Infof("deleted resource group %s", resourceGroup)
			s.write(nil)
		}
	}()
	s.writeState(internalapi.Deleting)
	// Update headers with Location so subsequent GET requests know the
	// location to query.
	headers := w.Header()
	headers.Add(autorest.HeaderLocation, fmt.Sprintf("http://%s%s", s.address, req.URL.Path))
	// And last but not least, we have accepted this DELETE request
	// and are processing it in the background.
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleGet(w http.ResponseWriter, req *http.Request) {
	s.reply(w, req)
}

func (s *Server) handlePut(w http.ResponseWriter, req *http.Request) {
	// read old config if it exists
	var oldCs *internalapi.OpenShiftManagedCluster
	var err error
	if !shared.IsUpdate() {
		s.writeState(internalapi.Creating)
	} else {
		s.log.Info("read old config")
		oldCs = s.read()
		if oldCs == nil {
			s.badRequest(w, "Failed to read old config: internal state does not exist")
			return
		}
		s.writeState(internalapi.Updating)
	}

	// TODO: Align with the production RP once it supports the admin API
	isAdminRequest := strings.HasPrefix(req.URL.Path, "/admin")

	// convert the external API manifest into the internal API representation
	s.log.Info("read request and convert to internal")
	var cs *internalapi.OpenShiftManagedCluster
	if isAdminRequest {
		var oc *admin.OpenShiftManagedCluster
		oc, err = s.readAdminRequest(req.Body)
		if err == nil {
			cs, err = admin.ToInternal(oc, oldCs)
		}
	} else {
		var oc *v20190430.OpenShiftManagedCluster
		oc, err = s.read20190430Request(req.Body)
		if err == nil {
			cs, err = v20190430.ToInternal(oc, oldCs)
		}
	}
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to convert to internal type: %v", err))
		return
	}
	s.write(cs)

	// apply the request
	cs, err = createOrUpdate(req.Context(), s.plugin, s.log, cs, oldCs, isAdminRequest, s.testConfig)
	if err != nil {
		s.writeState(internalapi.Failed)
		s.badRequest(w, fmt.Sprintf("Failed to apply request: %v", err))
		return
	}
	s.write(cs)
	s.writeState(internalapi.Succeeded)
	// TODO: Should return status.Accepted similar to how we handle DELETEs
	s.reply(w, req)
}

func (s *Server) reply(w http.ResponseWriter, req *http.Request) {
	cs := s.read()
	if cs == nil {
		// If the object is not found in memory then
		// it must have been deleted or never existed.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	state := s.readState()
	cs.Properties.ProvisioningState = state

	var res []byte
	var err error
	if strings.HasPrefix(req.URL.Path, "/admin") {
		oc := admin.FromInternal(cs)
		res, err = json.Marshal(oc)
	} else {
		oc := v20190430.FromInternal(cs)
		res, err = json.Marshal(oc)
	}
	if err != nil {
		s.badRequest(w, fmt.Sprintf("Failed to marshal response: %v", err))
		return
	}
	w.Write(res)
}
