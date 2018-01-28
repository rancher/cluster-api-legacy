package pod

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	dockerterm "github.com/docker/docker/pkg/term"
	"github.com/gorilla/websocket"
	"github.com/rancher/norman/parse"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/kubectl/util/term"
)

const (
	CMDSEP = ","
)

type ExecOption struct {
	podName       string
	nameSpace     string
	containerName string
	stdin         bool
	stdout        bool
	stderr        bool
	tty           bool
	command       []string
}

type ExecInput struct {
	Namespace     string `json:"namespace"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
	Command       string `json:"command"`
	TTY           bool   `json:"tty"`
	STDIN         bool   `json:"stdin"`
	STDOUT        bool   `json:"stdout"`
	STDERR        bool   `json:"stderr"`
}

func HandleExecWS(clusterConfig *rest.Config, w http.ResponseWriter, r *http.Request) (int, error) {
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
		return http.StatusInternalServerError, err
	}
	options, err := getExecOptionsFromQuery(r)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	pod, err := coreClient.CoreV1().Pods(options.nameSpace).Get(options.podName, metav1.GetOptions{})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
		return http.StatusInternalServerError, fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	containerName := options.containerName
	if len(containerName) == 0 {
		containerName = pod.Spec.Containers[0].Name
	}
	upgrader := websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	ioWrapper := &WebsocketIOWrapper{
		Conn: conn,
	}

	steamOptions := StreamOptions{
		Namespace:     options.nameSpace,
		PodName:       options.podName,
		ContainerName: options.containerName,
		Stdin:         true,
		TTY:           options.tty,
		In:            ioWrapper,
		Out:           ioWrapper,
		Err:           ioWrapper,
	}
	t := steamOptions.setupTTY()
	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		steamOptions.Err = nil
	}

	fn := func() error {
		req := restClient.Post().
			Resource("pods").
			Name(options.podName).
			Namespace(options.nameSpace).
			SubResource("exec")

		execOption := &v1.PodExecOptions{
			Container: containerName,
			Command:   options.command,
			Stdin:     options.stdin,
			Stdout:    options.stdout,
			Stderr:    options.stderr,
			TTY:       options.tty,
		}
		req.VersionedParams(execOption, dynamic.VersionedParameterEncoderWithV1Fallback)
		executor, err := remotecommand.NewSPDYExecutor(clusterConfig, "POST", req.URL())
		if err != nil {
			conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(time.Second*30))
			return err
		}
		option := remotecommand.StreamOptions{
			Stdin:             ioWrapper,
			Stdout:            ioWrapper,
			Stderr:            ioWrapper,
			Tty:               t.Raw,
			TerminalSizeQueue: sizeQueue,
		}
		if option.Tty {
			option.Stderr = nil
		}
		return executor.Stream(option)
	}

	if err := t.Safe(fn); err != nil {
		conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(time.Second*30))
		return http.StatusInternalServerError, err
	}
	return 200, nil
}

func handleExecAction(w http.ResponseWriter, r *http.Request) {
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
	query.Set("tty", strconv.FormatBool(options["tty"].(bool)))
	query.Set("stdin", strconv.FormatBool(options["stdin"].(bool)))
	query.Set("stdout", strconv.FormatBool(options["stdout"].(bool)))
	query.Set("stderr", strconv.FormatBool(options["stderr"].(bool)))
	query.Set("command", options["command"].(string))
	webSocketURL.RawQuery = query.Encode()
	if err := json.NewEncoder(w).Encode(webSocketURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type WebsocketIOWrapper struct {
	Conn *websocket.Conn
}

func (w *WebsocketIOWrapper) Read(p []byte) (n int, err error) {
	_, data, err := w.Conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	return copy(p, data), nil
}

func (w *WebsocketIOWrapper) Write(p []byte) (n int, err error) {
	return len(p), w.Conn.WriteMessage(websocket.TextMessage, p)
}

type StreamOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         bool
	TTY           bool
	// minimize unnecessary output
	Quiet bool
	In    io.Reader
	Out   io.Writer
	Err   io.Writer
}

func (o *StreamOptions) setupTTY() term.TTY {
	t := term.TTY{
		Out: o.Out,
	}

	if !o.Stdin {
		// need to nil out o.In to make sure we don't create a stream for stdin
		o.In = nil
		o.TTY = false
		return t
	}

	t.In = o.In
	if !o.TTY {
		return t
	}

	// if we get to here, the user wants to attach stdin, wants a TTY, and o.In is a terminal, so we
	// can safely set t.Raw to true
	t.Raw = true

	stdin, stdout, _ := dockerterm.StdStreams()
	o.In = stdin
	t.In = stdin
	if o.Out != nil {
		o.Out = stdout
		t.Out = stdout
	}

	return t
}

func getExecOptionsFromQuery(r *http.Request) (ExecOption, error) {
	values := r.URL.Query()

	option := ExecOption{}
	for k, v := range values {
		if v == nil {
			return ExecOption{}, fmt.Errorf("%s can't be nil", k)
		}
		switch k {
		case "containerName":
			option.containerName = v[0]
		case "command":
			option.command = strings.Split(v[0], CMDSEP)
		case "namespace":
			option.nameSpace = v[0]
		case "podName":
			option.podName = v[0]
		case "tty":
			option.tty, _ = strconv.ParseBool(v[0])
		case "stdin":
			option.stdin, _ = strconv.ParseBool(v[0])
		case "stdout":
			option.stdout, _ = strconv.ParseBool(v[0])
		case "stderr":
			option.stderr, _ = strconv.ParseBool(v[0])
		}
	}
	return option, nil
}
