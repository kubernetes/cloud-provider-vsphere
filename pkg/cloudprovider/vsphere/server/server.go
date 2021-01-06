/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:generate protoc -I ../proto/ ../proto/cloudprovidervsphere.proto --go_out=plugins=grpc:../proto

package server

import (
	"log"
	"net"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	klog "k8s.io/klog/v2"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
)

const (
	// APIVersion gives the API version :)
	APIVersion = "0.0.1"

	// RetryAttempts is the number of times to retry a failed connection
	// attempt.
	RetryAttempts int = 3
)

// NodeManagerInterface describes types that can export a list of Kubernetes
// nodes into the supplied slice address.
type NodeManagerInterface interface {
	GetNode(UUID string, node *pb.Node) error
	ExportNodes(vcenter string, datacenter string, nodeList *[]*pb.Node) error
}

// GRPCServer describes an object that can start a gRPC server.
type GRPCServer interface {
	Start()
}

type server struct {
	binding string
	s       *grpc.Server
	nodeMgr NodeManagerInterface
}

// NewServer generates a new gRPC Server
func NewServer(binding string, nodeMgr NodeManagerInterface) GRPCServer {
	s := grpc.NewServer()
	myServer := &server{
		binding: binding,
		s:       s,
		nodeMgr: nodeMgr,
	}
	pb.RegisterCloudProviderVsphereServer(s, myServer)
	reflection.Register(s)
	return myServer
}

// GetNode implements CloudProviderVsphere interface
func (s *server) GetNode(ctx context.Context, request *pb.GetNodeRequest) (*pb.GetNodeReply, error) {
	reply := &pb.GetNodeReply{
		Node: &pb.Node{},
	}
	err := s.nodeMgr.GetNode(request.Uuid, reply.Node)
	if err != nil {
		reply.Error = err.Error()
	}
	return reply, nil
}

// ListNodes implements CloudProviderVsphere interface
func (s *server) ListNodes(ctx context.Context, request *pb.ListNodesRequest) (*pb.ListNodesReply, error) {
	reply := &pb.ListNodesReply{
		Nodes: make([]*pb.Node, 0),
	}
	//Do not allow specifying the Datacenter without specifying the vCenter
	if request.Vcenter == "" && request.Datacenter != "" {
		request.Datacenter = ""
	}
	err := s.nodeMgr.ExportNodes(request.Vcenter, request.Datacenter, &reply.Nodes)
	if err != nil {
		reply.Error = err.Error()
	}
	return reply, nil
}

// GetVersion implements obtaining the version of the API server
func (s *server) GetVersion(ctx context.Context, request *pb.VersionRequest) (*pb.VersionReply, error) {
	return &pb.VersionReply{
		Version: APIVersion,
	}, nil
}

// Start the server
func (s *server) Start() {
	go func() {
		lis, err := net.Listen("tcp", s.binding)
		if err != nil {
			klog.Fatalf("Server Listen() failed: %s", err)

		}

		err = s.s.Serve(lis)
		if err != nil {
			log.Printf("Server Serve() failed: %s", err)
		}
	}()

	//Wait until the server is up and running
	for i := 0; i < RetryAttempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), (5 * time.Second))
		defer cancel()

		c, err := NewVSphereCloudProviderClient(ctx)
		if err != nil {
			klog.Warningf("could not greet: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		r, err := c.GetVersion(ctx, &pb.VersionRequest{})
		if err != nil {
			klog.Warningf("could not getversion: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		klog.Infof("APIVersion: %s", r.GetVersion())
		break
	}
}

// Stop the server
func (s *server) Stop() {
	s.s.Stop()
}
