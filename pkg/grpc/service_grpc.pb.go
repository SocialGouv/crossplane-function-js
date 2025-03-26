// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: proto/xfuncjs/service.proto

package grpc

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

const (
	XFuncJSService_RunFunction_FullMethodName = "/xfuncjs.XFuncJSService/RunFunction"
)

// XFuncJSServiceClient is the client API for XFuncJSService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type XFuncJSServiceClient interface {
	// RunFunction executes a JavaScript/TypeScript function with the provided input
	RunFunction(ctx context.Context, in *RunFunctionRequest, opts ...grpc.CallOption) (*RunFunctionResponse, error)
}

type xfuncjsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewXFuncJSServiceClient(cc grpc.ClientConnInterface) XFuncJSServiceClient {
	return &xfuncjsServiceClient{cc}
}

func (c *xfuncjsServiceClient) RunFunction(ctx context.Context, in *RunFunctionRequest, opts ...grpc.CallOption) (*RunFunctionResponse, error) {
	out := new(RunFunctionResponse)
	err := c.cc.Invoke(ctx, XFuncJSService_RunFunction_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// XFuncJSServiceServer is the server API for XFuncJSService service.
// All implementations must embed UnimplementedXFuncJSServiceServer
// for forward compatibility
type XFuncJSServiceServer interface {
	// RunFunction executes a JavaScript/TypeScript function with the provided input
	RunFunction(context.Context, *RunFunctionRequest) (*RunFunctionResponse, error)
	mustEmbedUnimplementedXFuncJSServiceServer()
}

// UnimplementedXFuncJSServiceServer must be embedded to have forward compatible implementations.
type UnimplementedXFuncJSServiceServer struct {
}

func (UnimplementedXFuncJSServiceServer) RunFunction(context.Context, *RunFunctionRequest) (*RunFunctionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunFunction not implemented")
}
func (UnimplementedXFuncJSServiceServer) mustEmbedUnimplementedXFuncJSServiceServer() {}

// UnsafeXFuncJSServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to XFuncJSServiceServer will
// result in compilation errors.
type UnsafeXFuncJSServiceServer interface {
	mustEmbedUnimplementedXFuncJSServiceServer()
}

func RegisterXFuncJSServiceServer(s grpc.ServiceRegistrar, srv XFuncJSServiceServer) {
	s.RegisterService(&XFuncJSService_ServiceDesc, srv)
}

func _XFuncJSService_RunFunction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunFunctionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(XFuncJSServiceServer).RunFunction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: XFuncJSService_RunFunction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(XFuncJSServiceServer).RunFunction(ctx, req.(*RunFunctionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// XFuncJSService_ServiceDesc is the grpc.ServiceDesc for XFuncJSService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var XFuncJSService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "xfuncjs.XFuncJSService",
	HandlerType: (*XFuncJSServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RunFunction",
			Handler:    _XFuncJSService_RunFunction_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/xfuncjs/service.proto",
}
