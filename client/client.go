package main

import (
	"flag"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	api "github.com/zshi-redhat/kube-app-netutil/apis/v1alpha"
)

const (
	endpoint = "/etc/netutil/net.sock"
)

func main() {
	flag.Parse()
	glog.Infof("starting netutil client")

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	defer conn.Close()

	c := api.NewNetUtilClient(conn)
	pid := os.Getpid()
	glog.Infof("os.Getpid() returns: %v", pid)
	response, err := c.GetCPUInfo(context.Background(), &api.CPURequest{Pid: strconv.Itoa(pid)})
	if err != nil {
		glog.Errorf("Error calling GetCPUInfo: %v", err)
	}
	glog.Infof("GetCPUInfo Response from NetUtilServer: %s", response.Cpuset)
	return
}
