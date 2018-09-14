package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/1071496910/mysh/auth"
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/lib/etcd"
	"github.com/1071496910/mysh/proto"
	"github.com/1071496910/mysh/recorder"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

type SearchServer struct {
	port int
}

func NewSearchServer(port int) *SearchServer {
	return &SearchServer{
		port: port,
	}

}

func (ss *SearchServer) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", ss.port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile(cons.Crt, cons.Key)
	if err != nil {
		return fmt.Errorf("could not load TLS keys: %s", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	proto.RegisterSearchServiceServer(s, ss)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func (ss *SearchServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		if !auth.CheckLoginState(req.Uid, req.Token, p.Addr.String()) {
			return &proto.SearchResponse{
				Response:     nil,
				ResponseCode: 403,
			}, nil
		}
	}

	return &proto.SearchResponse{
		Response: recorder.DefaultRecorderManager().Find(req.Uid, req.SearchString),
	}, nil
}

func (ss *SearchServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	if p, ok := peer.FromContext(ctx); ok {
		if !auth.CheckLoginState(req.Uid, req.Token, p.Addr.String()) {
			return &proto.UploadResponse{
				ErrorMsg:     "Invalid token",
				ResponseCode: 403,
			}, nil
		} else {
			defer auth.RemoveTokenCache(req.Uid, p.Addr.String())
		}
	}

	if err := recorder.DefaultRecorderManager().Add(req.Uid, req.Record); err != nil {
		log.Printf("Add record uid[%v] command[%v] error: %v\n", req.Uid, req.Record, err)
		return &proto.UploadResponse{
			ErrorMsg:     fmt.Sprint(err),
			ResponseCode: 503,
		}, err

	}
	log.Printf("Add record uid[%v] command[%v] success!\n", req.Uid, req.Record)
	return &proto.UploadResponse{
		ResponseCode: 200,
	}, nil
}

func (ss *SearchServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	resp := &proto.LoginResponse{
		ResponseCode: 403,
	}
	if p, ok := peer.FromContext(ctx); ok {
		if token, ok := auth.Login(req.Uid, req.Password, p.Addr.String()); ok {
			resp.ResponseCode = 200
			resp.Token = token

			auth.UpdateTokenCache(req.Uid, token, p.Addr.String())

			return resp, nil

		}
		return resp, fmt.Errorf("Incorrect username or password.")
	}
	return resp, fmt.Errorf("Can't get ip from peer")

}

func (ss *SearchServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	resp := &proto.LogoutResponse{
		ResponseCode: 503,
	}

	if p, ok := peer.FromContext(ctx); ok {
		if err := auth.RemoveTokenCache(req.Uid, p.Addr.String()); err == nil {
			resp.ResponseCode = 200
			return resp, nil

		} else {
			return resp, err
		}
	}
	return resp, fmt.Errorf("Can't get ip from peer")

}

type CertServer struct{}

func (c *CertServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if cert, err := auth.GetCert(); err == nil {
		w.Write([]byte(cert))
		return
	}
	w.WriteHeader(503)
	w.Write([]byte("load crt error"))
}

func NewCertServer() *CertServer {
	return &CertServer{}
}

type DashServer struct {
	port int
}

func NewDashServer(port int) *DashServer {
	return &DashServer{
		port: port,
	}

}

func (ss *DashServer) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", ss.port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile(cons.Crt, cons.Key)
	if err != nil {
		return fmt.Errorf("could not load TLS keys: %s", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	proto.RegisterDashServiceServer(s, ss)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func (d *DashServer) ClientUid(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryResponse, error) {
	vbytes, err := etcd.GetKV(fmt.Sprintf("uid.%v", r.Req))
	return &proto.CommonQueryResponse{
		Resp: string(vbytes),
	}, err
}
func (d *DashServer) UidState(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryResponse, error) {
	vbytes, err := etcd.GetKV(fmt.Sprintf("state.%v", r.Req))
	return &proto.CommonQueryResponse{
		Resp: string(vbytes),
	}, err
}
func (d *DashServer) UidEndpoint(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryResponse, error) {
	vbytes, err := etcd.GetKV(fmt.Sprintf("endpoint.%v", r.Req))
	return &proto.CommonQueryResponse{
		Resp: string(vbytes),
	}, err
}
func (d *DashServer) UidClients(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryListResponse, error) {
	ss := []string{}
	vbytes, err := etcd.GetKV(fmt.Sprintf("endpoint.%v", r.Req))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(vbytes, ss)
	if err != nil {
		return nil, err
	}

	return &proto.CommonQueryListResponse{
		Resp: ss,
	}, nil
}

//type CertServer struct {
//	port int
//}
//
//func NewCertServer(port int) *CertServer {
//	return &CertServer{
//		port: port,
//	}
//
//}
//
//func (cs *CertServer) Cert(ctx context.Context, req *proto.CertRequest) (*proto.CertEntry, error) {
//	if cert, err := auth.GetCert(); err == nil {
//		return &proto.CertEntry{
//			Content: cert,
//		}, nil
//	} else {
//
//		return nil, err
//	}
//
//}
//
//func (cs *CertServer) Run() error {
//	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", cs.port))
//	if err != nil {
//		return fmt.Errorf("init network error: %v", err)
//	}
//
//	s := grpc.NewServer()
//	proto.RegisterCertServiceServer(s, cs)
//	reflection.Register(s)
//	if err := s.Serve(lis); err != nil {
//		return err
//	}
//	return nil
//}
