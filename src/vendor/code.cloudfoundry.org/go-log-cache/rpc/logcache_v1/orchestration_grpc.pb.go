// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package logcache_v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// OrchestrationClient is the client API for Orchestration service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OrchestrationClient interface {
	AddRange(ctx context.Context, in *AddRangeRequest, opts ...grpc.CallOption) (*AddRangeResponse, error)
	RemoveRange(ctx context.Context, in *RemoveRangeRequest, opts ...grpc.CallOption) (*RemoveRangeResponse, error)
	ListRanges(ctx context.Context, in *ListRangesRequest, opts ...grpc.CallOption) (*ListRangesResponse, error)
	SetRanges(ctx context.Context, in *SetRangesRequest, opts ...grpc.CallOption) (*SetRangesResponse, error)
}

type orchestrationClient struct {
	cc grpc.ClientConnInterface
}

func NewOrchestrationClient(cc grpc.ClientConnInterface) OrchestrationClient {
	return &orchestrationClient{cc}
}

func (c *orchestrationClient) AddRange(ctx context.Context, in *AddRangeRequest, opts ...grpc.CallOption) (*AddRangeResponse, error) {
	out := new(AddRangeResponse)
	err := c.cc.Invoke(ctx, "/logcache.v1.Orchestration/AddRange", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestrationClient) RemoveRange(ctx context.Context, in *RemoveRangeRequest, opts ...grpc.CallOption) (*RemoveRangeResponse, error) {
	out := new(RemoveRangeResponse)
	err := c.cc.Invoke(ctx, "/logcache.v1.Orchestration/RemoveRange", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestrationClient) ListRanges(ctx context.Context, in *ListRangesRequest, opts ...grpc.CallOption) (*ListRangesResponse, error) {
	out := new(ListRangesResponse)
	err := c.cc.Invoke(ctx, "/logcache.v1.Orchestration/ListRanges", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestrationClient) SetRanges(ctx context.Context, in *SetRangesRequest, opts ...grpc.CallOption) (*SetRangesResponse, error) {
	out := new(SetRangesResponse)
	err := c.cc.Invoke(ctx, "/logcache.v1.Orchestration/SetRanges", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OrchestrationServer is the server API for Orchestration service.
// All implementations must embed UnimplementedOrchestrationServer
// for forward compatibility
type OrchestrationServer interface {
	AddRange(context.Context, *AddRangeRequest) (*AddRangeResponse, error)
	RemoveRange(context.Context, *RemoveRangeRequest) (*RemoveRangeResponse, error)
	ListRanges(context.Context, *ListRangesRequest) (*ListRangesResponse, error)
	SetRanges(context.Context, *SetRangesRequest) (*SetRangesResponse, error)
	mustEmbedUnimplementedOrchestrationServer()
}

// UnimplementedOrchestrationServer must be embedded to have forward compatible implementations.
type UnimplementedOrchestrationServer struct {
}

func (UnimplementedOrchestrationServer) AddRange(context.Context, *AddRangeRequest) (*AddRangeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddRange not implemented")
}
func (UnimplementedOrchestrationServer) RemoveRange(context.Context, *RemoveRangeRequest) (*RemoveRangeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveRange not implemented")
}
func (UnimplementedOrchestrationServer) ListRanges(context.Context, *ListRangesRequest) (*ListRangesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListRanges not implemented")
}
func (UnimplementedOrchestrationServer) SetRanges(context.Context, *SetRangesRequest) (*SetRangesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetRanges not implemented")
}
func (UnimplementedOrchestrationServer) mustEmbedUnimplementedOrchestrationServer() {}

// UnsafeOrchestrationServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OrchestrationServer will
// result in compilation errors.
type UnsafeOrchestrationServer interface {
	mustEmbedUnimplementedOrchestrationServer()
}

func RegisterOrchestrationServer(s grpc.ServiceRegistrar, srv OrchestrationServer) {
	s.RegisterService(&Orchestration_ServiceDesc, srv)
}

func _Orchestration_AddRange_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddRangeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestrationServer).AddRange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logcache.v1.Orchestration/AddRange",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestrationServer).AddRange(ctx, req.(*AddRangeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orchestration_RemoveRange_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveRangeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestrationServer).RemoveRange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logcache.v1.Orchestration/RemoveRange",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestrationServer).RemoveRange(ctx, req.(*RemoveRangeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orchestration_ListRanges_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRangesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestrationServer).ListRanges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logcache.v1.Orchestration/ListRanges",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestrationServer).ListRanges(ctx, req.(*ListRangesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Orchestration_SetRanges_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetRangesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestrationServer).SetRanges(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logcache.v1.Orchestration/SetRanges",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestrationServer).SetRanges(ctx, req.(*SetRangesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Orchestration_ServiceDesc is the grpc.ServiceDesc for Orchestration service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Orchestration_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "logcache.v1.Orchestration",
	HandlerType: (*OrchestrationServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AddRange",
			Handler:    _Orchestration_AddRange_Handler,
		},
		{
			MethodName: "RemoveRange",
			Handler:    _Orchestration_RemoveRange_Handler,
		},
		{
			MethodName: "ListRanges",
			Handler:    _Orchestration_ListRanges_Handler,
		},
		{
			MethodName: "SetRanges",
			Handler:    _Orchestration_SetRanges_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/orchestration.proto",
}