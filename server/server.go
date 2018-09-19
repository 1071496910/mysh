package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

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

	s := grpc.NewServer()
	proto.RegisterSearchServiceServer(s, ss)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func (ss *SearchServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {

	if !auth.CheckLoginState(req.Uid, req.Token, req.CliAddr) {
		return &proto.SearchResponse{
			Response:     nil,
			ResponseCode: 403,
		}, nil
	}

	return &proto.SearchResponse{
		Response: recorder.DefaultRecorderManager().Find(req.Uid, req.SearchString),
	}, nil
}

func (ss *SearchServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	if !auth.CheckLoginState(req.Uid, req.Token, req.CliAddr) {
		return &proto.UploadResponse{
			ErrorMsg:     "Invalid token",
			ResponseCode: 403,
		}, nil
	} else {
		defer auth.RemoveTokenCache(req.Uid, req.CliAddr)
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
	fmt.Println(req.Uid, req.Password, req.CliAddr)
	if token, ok := auth.Login(req.Uid, req.Password, req.CliAddr); ok {
		resp.ResponseCode = 200
		resp.Token = token

		auth.UpdateTokenCache(req.Uid, token, req.CliAddr)

		return resp, nil

	}
	return resp, fmt.Errorf("Incorrect username or password.")

}

func (ss *SearchServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	resp := &proto.LogoutResponse{
		ResponseCode: 503,
	}

	if err := auth.RemoveTokenCache(req.Uid, req.CliAddr); err == nil {
		resp.ResponseCode = 200
		return resp, nil

	}
	return resp, nil

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

	s := grpc.NewServer()
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
	//vbytes, err := etcd.GetKV(fmt.Sprintf("endpoint.%v", r.Req))
	return &proto.CommonQueryResponse{
		Resp: "127.0.0.1:8083",
		//Resp: string(vbytes),
	}, nil
	//}, err
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

type EndpointCliManager struct {
	cliMap map[string]proto.SearchServiceClient
	mtx    sync.Mutex
}

func newSearchServerCli(endpoint string) proto.SearchServiceClient {

	conn, err := grpc.Dial(endpoint)
	if err != nil {
		panic(err)
	}
	return proto.NewSearchServiceClient(conn)
}

func (ecm *EndpointCliManager) ensureCliExists(endpoint string) {
	if _, ok := ecm.cliMap[endpoint]; !ok {

		ecm.mtx.Lock()
		defer ecm.mtx.Unlock()
		if _, ok = ecm.cliMap[endpoint]; !ok {
			ecm.cliMap[endpoint] = newSearchServerCli(endpoint)
		}
	}
}

func (ecm *EndpointCliManager) Login(ctx context.Context, endpoint string, in *proto.LoginRequest, opts ...grpc.CallOption) (*proto.LoginResponse, error) {
	//1. 先判断客户端是否存在
	//2. 获取或者创建客户端
	//3. 通过客户端请求
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]
	return cli.Login(ctx, in, opts...)
}

func (ecm *EndpointCliManager) Logout(ctx context.Context, endpoint string, in *proto.LogoutRequest, opts ...grpc.CallOption) (*proto.LogoutResponse, error) {
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]
	return cli.Logout(ctx, in, opts...)

}
func (ecm *EndpointCliManager) Search(ctx context.Context, endpoint string, in *proto.SearchRequest, opts ...grpc.CallOption) (*proto.SearchResponse, error) {

	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]
	return cli.Search(ctx, in, opts...)

}
func (ecm *EndpointCliManager) Upload(ctx context.Context, endpoint string, in *proto.UploadRequest, opts ...grpc.CallOption) (*proto.UploadResponse, error) {
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]
	return cli.Upload(ctx, in, opts...)

}

type ProxyServer struct {
	port               int
	dashCli            proto.DashServiceClient
	endpointCliManager *EndpointCliManager
}

func NewProxyServer(port int) *ProxyServer {
	conn, err := grpc.Dial("www.myshell.top:"+strconv.Itoa(cons.DashPort), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	endpointCliManger := &EndpointCliManager{
		cliMap: make(map[string]proto.SearchServiceClient),
	}
	conn2, err := grpc.Dial("127.0.0.1:8083", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	endpointCliManger.cliMap["127.0.0.1:8083"] = proto.NewSearchServiceClient(conn2)
	return &ProxyServer{
		port:               port,
		dashCli:            proto.NewDashServiceClient(conn),
		endpointCliManager: endpointCliManger,
	}

}

func (ps *ProxyServer) Run() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", ps.port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	creds, err := credentials.NewServerTLSFromFile(cons.Crt, cons.Key)
	if err != nil {
		return fmt.Errorf("could not load TLS keys: %s", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	proto.RegisterSearchServiceServer(s, ps)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		return err
	}
	return nil
}

func (ps *ProxyServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	//1. 获取uid对应的endpoint
	//2. 获取client的地址加入请求转给endpoint
	resp, err := ps.dashCli.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
		Req: req.Uid,
	})
	if err != nil {
		return nil, err
	}
	endpoint := resp.Resp

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		fmt.Println("DEBUG", req.GetCliAddr(), req.GetToken(), req.GetSearchString(), req.GetUid())
		sresp, err := ps.endpointCliManager.Search(context.Background(), endpoint, req)
		fmt.Println("DEBUG", sresp.GetResponse(), sresp.GetResponseCode())

		if err != nil {
			return nil, err
		}

		return sresp, err

	}

	return nil, fmt.Errorf("Can't get ip from peer")
}

func (ps *ProxyServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	resp, err := ps.dashCli.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
		Req: req.Uid,
	})
	if err != nil {
		return nil, err
	}
	endpoint := resp.Resp

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Upload(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")
}

func (ps *ProxyServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	resp, err := ps.dashCli.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
		Req: req.Uid,
	})
	if err != nil {
		return nil, err
	}
	endpoint := resp.Resp

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Login(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")

}

func (ps *ProxyServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	resp, err := ps.dashCli.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
		Req: req.Uid,
	})
	if err != nil {
		return nil, err
	}
	endpoint := resp.Resp

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Logout(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")
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
