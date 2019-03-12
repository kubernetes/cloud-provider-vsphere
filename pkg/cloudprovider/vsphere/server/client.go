package server

import (
	"context"
	"time"

	"k8s.io/klog"

	"google.golang.org/grpc"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

// NewVSphereCloudProviderClient creates CloudProviderVsphereClient
func NewVSphereCloudProviderClient(ctx context.Context) (pb.CloudProviderVsphereClient, error) {
	var conn *grpc.ClientConn
	var err error
	for i := 0; i < RetryAttempts; i++ {
		conn, err = grpc.Dial(vcfg.DefaultAPIBinding, grpc.WithInsecure())
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		klog.Errorf("did not connect: %v", err)
		return nil, err
	}

	c := pb.NewCloudProviderVsphereClient(conn)

	return c, nil
}
