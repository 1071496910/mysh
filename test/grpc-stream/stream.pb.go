// Code generated by protoc-gen-go. DO NOT EDIT.
// source: stream.proto

package stream

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type PauseRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PauseRequest) Reset()         { *m = PauseRequest{} }
func (m *PauseRequest) String() string { return proto.CompactTextString(m) }
func (*PauseRequest) ProtoMessage()    {}
func (*PauseRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_bb17ef3f514bfe54, []int{0}
}

func (m *PauseRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PauseRequest.Unmarshal(m, b)
}
func (m *PauseRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PauseRequest.Marshal(b, m, deterministic)
}
func (m *PauseRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PauseRequest.Merge(m, src)
}
func (m *PauseRequest) XXX_Size() int {
	return xxx_messageInfo_PauseRequest.Size(m)
}
func (m *PauseRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PauseRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PauseRequest proto.InternalMessageInfo

type PauseResponse struct {
	ResponseCode         int32    `protobuf:"varint,1,opt,name=responseCode,proto3" json:"responseCode,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PauseResponse) Reset()         { *m = PauseResponse{} }
func (m *PauseResponse) String() string { return proto.CompactTextString(m) }
func (*PauseResponse) ProtoMessage()    {}
func (*PauseResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_bb17ef3f514bfe54, []int{1}
}

func (m *PauseResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PauseResponse.Unmarshal(m, b)
}
func (m *PauseResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PauseResponse.Marshal(b, m, deterministic)
}
func (m *PauseResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PauseResponse.Merge(m, src)
}
func (m *PauseResponse) XXX_Size() int {
	return xxx_messageInfo_PauseResponse.Size(m)
}
func (m *PauseResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PauseResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PauseResponse proto.InternalMessageInfo

func (m *PauseResponse) GetResponseCode() int32 {
	if m != nil {
		return m.ResponseCode
	}
	return 0
}

func init() {
	proto.RegisterType((*PauseRequest)(nil), "PauseRequest")
	proto.RegisterType((*PauseResponse)(nil), "PauseResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ProxyControllerClient is the client API for ProxyController service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ProxyControllerClient interface {
	Pause(ctx context.Context, in *PauseRequest, opts ...grpc.CallOption) (ProxyController_PauseClient, error)
}

type proxyControllerClient struct {
	cc *grpc.ClientConn
}

func NewProxyControllerClient(cc *grpc.ClientConn) ProxyControllerClient {
	return &proxyControllerClient{cc}
}

func (c *proxyControllerClient) Pause(ctx context.Context, in *PauseRequest, opts ...grpc.CallOption) (ProxyController_PauseClient, error) {
	stream, err := c.cc.NewStream(ctx, &_ProxyController_serviceDesc.Streams[0], "/ProxyController/Pause", opts...)
	if err != nil {
		return nil, err
	}
	x := &proxyControllerPauseClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ProxyController_PauseClient interface {
	Recv() (*PauseResponse, error)
	grpc.ClientStream
}

type proxyControllerPauseClient struct {
	grpc.ClientStream
}

func (x *proxyControllerPauseClient) Recv() (*PauseResponse, error) {
	m := new(PauseResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ProxyControllerServer is the server API for ProxyController service.
type ProxyControllerServer interface {
	Pause(*PauseRequest, ProxyController_PauseServer) error
}

func RegisterProxyControllerServer(s *grpc.Server, srv ProxyControllerServer) {
	s.RegisterService(&_ProxyController_serviceDesc, srv)
}

func _ProxyController_Pause_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PauseRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ProxyControllerServer).Pause(m, &proxyControllerPauseServer{stream})
}

type ProxyController_PauseServer interface {
	Send(*PauseResponse) error
	grpc.ServerStream
}

type proxyControllerPauseServer struct {
	grpc.ServerStream
}

func (x *proxyControllerPauseServer) Send(m *PauseResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _ProxyController_serviceDesc = grpc.ServiceDesc{
	ServiceName: "ProxyController",
	HandlerType: (*ProxyControllerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Pause",
			Handler:       _ProxyController_Pause_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "stream.proto",
}

func init() { proto.RegisterFile("stream.proto", fileDescriptor_bb17ef3f514bfe54) }

var fileDescriptor_bb17ef3f514bfe54 = []byte{
	// 128 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x2e, 0x29, 0x4a,
	0x4d, 0xcc, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x57, 0xe2, 0xe3, 0xe2, 0x09, 0x48, 0x2c, 0x2d,
	0x4e, 0x0d, 0x4a, 0x2d, 0x2c, 0x4d, 0x2d, 0x2e, 0x51, 0x32, 0xe6, 0xe2, 0x85, 0xf2, 0x8b, 0x0b,
	0xf2, 0xf3, 0x8a, 0x53, 0x85, 0x94, 0xb8, 0x78, 0x8a, 0xa0, 0x6c, 0xe7, 0xfc, 0x94, 0x54, 0x09,
	0x46, 0x05, 0x46, 0x0d, 0xd6, 0x20, 0x14, 0x31, 0x23, 0x6b, 0x2e, 0xfe, 0x80, 0xa2, 0xfc, 0x8a,
	0x4a, 0xe7, 0xfc, 0xbc, 0x92, 0xa2, 0xfc, 0x9c, 0x9c, 0xd4, 0x22, 0x21, 0x0d, 0x2e, 0x56, 0xb0,
	0x39, 0x42, 0xbc, 0x7a, 0xc8, 0xe6, 0x4b, 0xf1, 0xe9, 0xa1, 0x18, 0x6f, 0xc0, 0x98, 0xc4, 0x06,
	0x76, 0x88, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0x1f, 0xc5, 0x20, 0x3d, 0x98, 0x00, 0x00, 0x00,
}
