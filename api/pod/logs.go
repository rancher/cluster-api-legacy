package pod

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rancher/norman/parse"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api"
	kubeutil "k8s.io/kubernetes/pkg/kubectl/util"
)

const (
	websocketScheme = "ws://"
)

type LogInput struct {
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	Follow        bool   `json:"follow"`
	Timestamp     bool   `json:"timestamp"`
	Since         string `json:"since"`
}

func HandleLogWS(clusterConfig *rest.Config, w http.ResponseWriter, r *http.Request) (int, error) {
	gv := v1.SchemeGroupVersion
	contentConfig := rest.ContentConfig{}
	contentConfig.GroupVersion = &gv
	contentConfig.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	clusterConfig.APIPath = "/api"
	clusterConfig.ContentConfig = contentConfig
	coreClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	restClient, err := rest.RESTClientFor(clusterConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	options, err := getLogOptionsFromQuery(r)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	pod, err := coreClient.CoreV1().Pods(options.NameSpace).Get(options.PodName, metav1.GetOptions{})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	containerName := options.Container
	if len(containerName) == 0 {
		containerName = pod.Spec.Containers[0].Name
	}
	conn, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
	fmt.Println(conn)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	ioWrapper := &WebsocketIOWrapper{
		Conn: conn,
	}

	podLogOption := v1.PodLogOptions{
		Container:  containerName,
		Timestamps: options.Timestamps,
		Follow:     options.Follow,
		SinceTime:  options.SinceTime,
	}
	req := restClient.Get().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.NameSpace).
		SubResource("log").
		VersionedParams(&podLogOption, dynamic.VersionedParameterEncoderWithV1Fallback)

	reader, err := req.Stream()
	if err != nil {
		conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(time.Second*30))
		return http.StatusInternalServerError, err
	}
	defer reader.Close()
	_, err = io.Copy(ioWrapper, reader)
	if err != nil {
		conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(time.Second*30))
		return http.StatusInternalServerError, err
	}
	return 200, nil
}

func handleLogAction(w http.ResponseWriter, r *http.Request) {
	options, err := parse.ReadBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	webSocketURL := r.URL
	webSocketURL.Scheme = websocketScheme
	query := webSocketURL.Query()
	query.Set("podName", options["podName"].(string))
	query.Set("containerName", options["containerName"].(string))
	query.Set("namespace", options["namespace"].(string))
	query.Set("timestamp", strconv.FormatBool(options["timestamp"].(bool)))
	query.Set("follow", strconv.FormatBool(options["timestamp"].(bool)))
	query.Set("since", options["since"].(string))
	webSocketURL.RawQuery = query.Encode()
	if err := json.NewEncoder(w).Encode(webSocketURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type LogOptions struct {
	api.PodLogOptions
	PodName   string
	NameSpace string
}

func getLogOptionsFromQuery(r *http.Request) (LogOptions, error) {
	values := r.URL.Query()

	options := LogOptions{}
	for k, v := range values {
		if v == nil {
			return LogOptions{}, fmt.Errorf("%s can't be nil", k)
		}
		switch k {
		case "podName":
			options.PodName = v[0]
		case "namespace":
			options.NameSpace = v[0]
		case "containerName":
			options.Container = v[0]
		case "timestamp":
			options.Timestamps, _ = strconv.ParseBool(v[0])
		case "follow":
			options.Follow, _ = strconv.ParseBool(v[0])
		case "since":
			t, err := kubeutil.ParseRFC3339(v[0], metav1.Now)
			if err != nil {
				return options, err
			}
			options.SinceTime = &t
		}
	}
	return options, nil
}
