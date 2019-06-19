package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	api "github.com/zshi-redhat/kube-app-netutil/apis/v1alpha"
)

const (
	endpoint = "/etc/netutil/net.sock"
	cpusetPath = "/sys/fs/cgroup/cpuset/cpuset.cpus"
)

type NetUtilServer struct {
	grpcServer	*grpc.Server
}

func newNetutilServer() *NetUtilServer {
	return &NetUtilServer{
		grpcServer: grpc.NewServer(),
	}
}

func (ns *NetUtilServer) start() error {
	glog.Infof("starting netutil server at: %s\n", endpoint)
	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		glog.Errorf("Error creating netutil gRPC service: %v", err)
		return err
	}

	api.RegisterNetUtilServer(ns.grpcServer, ns)
	go ns.grpcServer.Serve(lis)

	// Wait for server to start
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		glog.Errorf("Error starting netutil server: %v", err)
		return err
	}
	glog.Infof("netutil server start serving")
	conn.Close()
	return nil
}

func (ns *NetUtilServer) stop() error {
	glog.Infof("stopping netutil server")
	if ns.grpcServer != nil {
		ns.grpcServer.Stop()
		ns.grpcServer = nil
	}
	err := os.Remove(endpoint)
	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("Error cleaning up socket file")
	}
	return nil
}

func (ns *NetUtilServer) GetCPUInfo(ctx context.Context, rqt *api.CPURequest) (*api.CPUResponse, error) {
	path := filepath.Join("/proc", rqt.Pid, "root", cpusetPath)
	glog.Infof("getting cpuset from path: %s", path)
	cpus, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("Error getting cpuset info: %v", err)
		return nil, err
	}
	return &api.CPUResponse{Cpuset: string(bytes.TrimSpace(cpus))}, nil
}

func main() {
	flag.Parse()
	ns := newNetutilServer()
	if ns == nil {
		glog.Errorf("Error initializing netutil manager")
		return
	}
	err := ns.start()
	if err != nil {
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-sigCh:
		glog.Infof("signal received, shutting down", sig)
		ns.stop()
		return
	}
}
