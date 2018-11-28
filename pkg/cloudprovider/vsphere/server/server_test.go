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

package server

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

const (
	exampleUUIDForGoTest = "422e4956-ad22-1139-6d72-59cc8f26bc90"
)

type fakeNodeMgr struct{}

func (nm *fakeNodeMgr) ExportNodes(vcenter string, datacenter string, nodeList *[]*pb.Node) error {
	pbNode := &pb.Node{
		Vcenter:    "127.0.0.1",
		Datacenter: "dc",
		Name:       "MyNode",
		Dnsnames:   make([]string, 0),
		Addresses:  make([]string, 0),
		Uuid:       exampleUUIDForGoTest,
	}
	pbNode.Addresses = append(pbNode.Addresses, "10.0.0.1")
	pbNode.Dnsnames = append(pbNode.Dnsnames, "fqdn")

	*nodeList = append(*nodeList, pbNode)

	return nil
}

func TestGRPCServerClient(t *testing.T) {
	//server
	s := grpc.NewServer()
	myServer := &server{
		binding: vcfg.DefaultAPIBinding,
		s:       s,
		nodeMgr: &fakeNodeMgr{},
	}
	pb.RegisterCloudProviderVsphereServer(s, myServer)
	reflection.Register(s)

	myServer.Start()

	//client
	var conn *grpc.ClientConn
	var err error
	for i := 0; i < 3; i++ {
		conn, err = grpc.Dial(vcfg.DefaultAPIBinding, grpc.WithInsecure())
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("did not connect: %v", err)
	}

	defer conn.Close()
	c := pb.NewCloudProviderVsphereClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.ListNodes(ctx, &pb.ListNodesRequest{})
	if err != nil {
		t.Fatalf("could not greet: %v", err)
	}

	found := false
	for _, node := range r.Nodes {
		if node.Uuid == exampleUUIDForGoTest {
			found = true
		}
	}

	if !found {
		t.Errorf("VM was not found!")
	}
}
