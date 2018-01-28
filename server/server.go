package server

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/rancher/cluster-api/api/pod"
	"github.com/rancher/cluster-api/api/setup"
	"github.com/rancher/cluster-api/store"
	"github.com/rancher/norman-rbac"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, cluster *config.ClusterContext) (http.Handler, error) {
	if err := setup.Schemas(ctx, cluster, cluster.Schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.AccessControl = rbac.NewAccessControl(cluster.RBAC)
	server.URLParser = func(schemas *types.Schemas, url *url.URL) (parse.ParsedURL, error) {
		return URLParser(cluster.ClusterName, schemas, url)
	}
	server.StoreWrapper = store.ProjectSetter(server.StoreWrapper)

	if err := server.AddSchemas(cluster.Schemas); err != nil {
		return nil, err
	}

	logHandler := pod.Handler{
		Config: &cluster.RESTConfig,
		H:      pod.HandleLogWS,
	}
	execHandler := pod.Handler{
		Config: &cluster.RESTConfig,
		H:      pod.HandleExecWS,
	}

	r := mux.NewRouter()
	r.PathPrefix("/").Handler(server)
	r.Queries("action", "logs").Handler(logHandler)
	r.Queries("action", "exec").Handler(execHandler)
	return r, nil
}
