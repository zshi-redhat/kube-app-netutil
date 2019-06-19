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
	"google.golang.org/grpc"
)

const (
	endpoint = "/etc/netutil/net.sock"
	cpusetPath = "/sys/fs/cgroup/cpuset/cpuset.cpus"
)

type netutilManager struct {
	socketFile	string
	grpcServer	*grpc.Server
}

type cpuResource struct {
	cpuset	string
}

func newNetutilManager() *netutilManager {
	return &netutilManager{
		grpcServer: grpc.NewServer(),
	}
}

func (nn *netutilManager) start() error {
	glog.Infof("starting netutil server at: %s\n", endpoint)
	lis, err := net.Listen("unix", endpoint)
	if err != nil {
		glog.Errorf("Error creating netutil gRPC service: %v", err)
		return err
	}

	go nn.grpcServer.Serve(lis)

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

func (nn *netutilManager) stop() error {
	glog.Infof("stopping netutil server")
	if nn.grpcServer != nil {
		nn.grpcServer.Stop()
		nn.grpcServer = nil
	}
	err := os.Remove(endpoint)
	if err != nil && !os.IsNotExist(err) {
		glog.Errorf("Error cleaning up socket file")
	}
	return nil
}

func (nn *netutilManager) GetCPU(pid string) (*cpuResource, error) {
	path := filepath.Join("/proc", pid, "root", cpusetPath)
	cpus, err := ioutil.ReadFile(path)
	if err != nil {
		glog.Errorf("Error getting cpuset info: %v", err)
		return nil, err
	}
	return &cpuResource{cpuset: string(bytes.TrimSpace(cpus))}, nil
}

func main() {
	flag.Parse()
	nn := newNetutilManager()
	if nn == nil {
		glog.Errorf("Error initializing netutil manager")
		return
	}
	err := nn.start()
	if err != nil {
		return
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-sigCh:
		glog.Infof("signal received, shutting down", sig)
		nn.stop()
		return
	}
}
