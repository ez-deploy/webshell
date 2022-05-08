package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	wsterminal "github.com/maoqide/kubeutil/pkg/terminal/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type SSHPodTicket struct {

	// id
	// Example: 1
	ID int64 `json:"id,omitempty" db:"id,type=INTEGER,primary,auto_increment"`

	// namespace name
	// Example: foobar
	NamespaceName string `json:"namespace_name,omitempty" db:"namespace_name,type=VARCHAR(255)"`

	// pod name
	// Example: foobar-9zqb2
	PodName string `json:"pod_name,omitempty" db:"pod_name,type=VARCHAR(255)"`

	// ticket
	// Example: AISBJFCOIZXUF==
	Ticket string `json:"ticket,omitempty" db:"ticket,type=VARCHAR(255)"`

	// user id
	// Example: 1
	UserID int64 `json:"user_id,omitempty" db:"user_id,type=INTEGER"`

	// create at, unix timestamp
	CreateAt int64 `json:"create_at,omitempty" db:"create_at,type=INTEGER"`
}

const ezDeployURL = "http://localhost:8888/api"

func getTicket(ticketValue string) (*SSHPodTicket, error) {
	verifyTicketURL := ezDeployURL + "/visit/pod/ticket/check?ticket_value=" + ticketValue

	fmt.Println(verifyTicketURL)
	req, err := http.NewRequest(http.MethodGet, verifyTicketURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("un expected status")
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("get data: ", string(data))

	ticketInfo := &SSHPodTicket{}
	if err := json.Unmarshal(data, ticketInfo); err != nil {
		return nil, err
	}

	return ticketInfo, nil
}

func servePodTerminal(w http.ResponseWriter, r *http.Request) {
	rawTicket := r.URL.Query().Get("ticket_value")
	fmt.Println(rawTicket)
	ticket, err := getTicket(rawTicket)
	if err != nil {
		fmt.Println("get ticket error", err)
		return
	}

	fmt.Println("get ticket ok")

	// get pty
	pty, err := wsterminal.NewTerminalSession(w, r, nil)
	if err != nil {
		log.Printf("get pty failed: %v\n", err)
		return
	}
	defer func() {
		log.Println("close session.")
		pty.Close()
	}()

	// init k8s client
	config, err := clientcmd.BuildConfigFromFlags("", "/home/wangsaiyu/.kube/config")
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(ticket.PodName).
		Namespace(ticket.NamespaceName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"bash"},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	url := req.URL()

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", url)
	if err != nil {
		panic(err)
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             pty,
		Stdout:            pty,
		Stderr:            pty,
		TerminalSizeQueue: pty,
		Tty:               true,
	})

	if err != nil {
		msg := fmt.Sprintf("Exec to pod error! err: %v", err)
		log.Println(msg)
		pty.Write([]byte(msg))
		pty.Done()
		panic(err)
	}
}

func main() {
	http.HandleFunc("/ws", servePodTerminal)
	log.Fatal(http.ListenAndServe("localhost:9999", nil))
}
