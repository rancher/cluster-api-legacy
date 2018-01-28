package pod

import (
	"net/http"

	"github.com/rancher/norman/types"
	"k8s.io/client-go/rest"
)

type Handler struct {
	Config *rest.Config
	H      func(*rest.Config, http.ResponseWriter, *http.Request) (int, error)
}

func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if status, err := fn.H(fn.Config, w, r); err != nil {
		switch status {
		case http.StatusNotFound:
			http.Error(w, "404 not found", http.StatusNotFound)
		case http.StatusInternalServerError:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Actions["logs"] = apiContext.URLBuilder.Action("logs", resource)
	resource.Actions["exec"] = apiContext.URLBuilder.Action("exec", resource)
}

func ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	switch actionName {
	case "logs":
		handleLogAction(request.Response, request.Request)
	case "exec":
		handleExecAction(request.Response, request.Request)
	}

	return nil
}
