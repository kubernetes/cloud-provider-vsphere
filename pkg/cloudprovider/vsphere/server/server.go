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

	"k8s.io/klog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
)

type NodeManagerInterface interface {
	ExportNodes(vcenter string, datacenter string, nodeList *[]*pb.Node) error
}

type GRPCServer interface {
	Start()
}

type server struct {
	binding string
	s       *grpc.Server
	nodeMgr NodeManagerInterface
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
