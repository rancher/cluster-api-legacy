package namespace

import (
	"context"
	"math/rand"
	"os"
	"os/exec"

	"strconv"
	"time"

	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	managementv3 "github.com/rancher/types/client/management/v3"
	projectv3 "github.com/rancher/types/client/project/v3"
)

const (
	base       = 32768
	end        = 61000
	tillerName = "tiller"
	helmName   = "helm"
	cacheRoot  = "helm-controller"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Actions["upgrade"] = apiContext.URLBuilder.Action("upgrade", resource)
	resource.Actions["rollback"] = apiContext.URLBuilder.Action("rollback", resource)
}

func ProjectMap(apiContext *types.APIContext) (map[string]string, error) {
	var namespaces []projectv3.Namespace
	if err := access.List(apiContext, &schema.Version, projectv3.NamespaceType, &types.QueryOptions{}, &namespaces); err != nil {
		return nil, err
	}

	result := map[string]string{}
	for _, namespace := range namespaces {
		result[namespace.Name] = namespace.ProjectID
	}

	return result, nil
}

func ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	store := apiContext.Schema.Store
	switch actionName {
	case "upgrade":
		cont, cancel := context.WithCancel(context.Background())
		defer cancel()
		addr := generateRandomPort()
		go startTiller(cont, addr, apiContext.ID)
		actionInput, err := parse.Body(apiContext.Request)
		if err != nil {
			return err
		}
		externalID := actionInput["externalId"]
		updateData := map[string]interface{}{}
		updateData["externalId"] = externalID
		data, err := store.Update(apiContext, apiContext.Schema, updateData, apiContext.ID)
		if err != nil {
			return err
		}
		var templateVersion managementv3.TemplateVersion
		if err := access.ByID(apiContext, &managementschema.Version, managementv3.TemplateVersionType, convert.ToString(externalID), &templateVersion); err != nil {
			return err
		}
		files := convertTemplates(templateVersion.Files)
		rootDir := filepath.Join(os.Getenv("HOME"), cacheRoot)
		tempDir, err := writeTempDir(rootDir, files)
		if err != nil {
			return err
		}
		if err := upgradeCharts(tempDir, addr, convert.ToString(data["name"])); err != nil {
			return err
		}
		_, err = store.Update(apiContext, apiContext.Schema, updateData, apiContext.ID)
		if err != nil {
			return err
		}
		return nil
	case "rollback":
		cont, cancel := context.WithCancel(context.Background())
		defer cancel()
		addr := generateRandomPort()
		go startTiller(cont, addr, apiContext.ID)
		actionInput, err := parse.Body(apiContext.Request)
		if err != nil {
			return err
		}
		revision := actionInput["revision"]
		if err := rollbackCharts(addr, apiContext.ID, convert.ToString(revision)); err != nil {
			return err
		}
		data := map[string]interface{}{
			"name": apiContext.ID,
		}
		_, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func writeTempDir(rootDir string, files map[string]string) (string, error) {
	for name, content := range files {
		fp := filepath.Join(rootDir, name)
		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return "", err
		}
		if err := ioutil.WriteFile(fp, []byte(content), 0755); err != nil {
			return "", err
		}
	}
	for name := range files {
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			return filepath.Join(rootDir, parts[0]), nil
		}
	}
	return "", nil
}

func convertTemplates(files []managementv3.File) map[string]string {
	templates := map[string]string{}
	for _, f := range files {
		templates[f.Name] = f.Contents
	}
	return templates
}

func upgradeCharts(rootDir, port, releaseName string) error {
	cmd := exec.Command(helmName, "upgrade", "--namespace", releaseName, releaseName, rootDir)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func rollbackCharts(port, releaseName, revision string) error {
	cmd := exec.Command(helmName, "rollback", releaseName, revision)
	cmd.Env = []string{fmt.Sprintf("%s=%s", "HELM_HOST", "127.0.0.1:"+port)}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func generateRandomPort() string {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	port := base + r1.Intn(end-base+1)
	return strconv.Itoa(port)
}

// startTiller start tiller server and return the listening address of the grpc address
func startTiller(context context.Context, port, namespace string) error {
	// todo: we need to pass impersonation kubeconfig
	cmd := exec.Command(tillerName, "--listen", ":"+port)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", "TILLER_NAMESPACE", namespace))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	select {
	case <-context.Done():
		return cmd.Process.Kill()
	}
}
