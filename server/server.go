package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/1071496910/mysh/lib/etcd"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1071496910/mysh/auth"
	"github.com/1071496910/mysh/cons"
	"github.com/1071496910/mysh/proto"
	"github.com/1071496910/mysh/recorder"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

//UidLogout(context.Context, *UidLogoutRequest) (*UidLogoutResponse, error)

var (
	proxyReqCount        int32
	proxyPauseMap        = make(map[string]bool)
	proxyMtx             sync.Mutex
	uidEndpointsCacheMtx sync.Mutex
	uidEndpointsCache    = make(map[string]string)
	E_SUSPENSIVE         = errors.New("Uid is suspensive")
)

func RunSearchService(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterSearchServiceServer(s, NewSearchServer(port))
	proto.RegisterServerControllerServer(s, NewSearchController(port))
	reflection.Register(s)
	if err := etcd.Register(cons.ServerRegistryPrefix, lis.Addr().String()); err != nil {
		log.Println("Registry searchService ok...")
		return err
	}
	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil

}

type SearchController struct {
	port int
}

func NewSearchController(port int) *SearchController {
	return &SearchController{
		port: port,
	}
}

func (sc *SearchController) UidLogout(ctx context.Context, req *proto.UidLogoutRequest) (*proto.UidLogoutResponse, error) {
	log.Println("sc uidlogout", req)
	if err := auth.RemoveTokenCache(req.Uid); err != nil {
		return nil, err
	}
	log.Println("DEBUG after remove token cache")

	if err := recorder.DefaultRecorderManager().Stop(req.Uid); err != nil {
		return nil, err
	}
	log.Println("sc uidlogout finish", req)
	return &proto.UidLogoutResponse{ResponseCode: 200}, nil
}

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
	if err := etcd.Register(cons.ServerRegistryPrefix, lis.Addr().String()); err != nil {
		return err
	}
	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}

func (ss *SearchServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {

	if req.Uid != "" && req.SearchString == "" {
		log.Println("server.go ss search", req.Uid, req.CliAddr, req.Token)
		if !auth.CheckLoginState(req.Uid, req.Token, req.CliAddr) {
			log.Println("DEBUG in search auth check failure")
			return &proto.SearchResponse{
				Response:     nil,
				ResponseCode: 403,
			}, nil
		}
	}

	return &proto.SearchResponse{
		Response:     recorder.DefaultRecorderManager().Find(req.Uid, req.SearchString),
		ResponseCode: 200,
	}, nil
}

func (ss *SearchServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {

	log.Println("server.go ss upload", req.Uid, req.CliAddr, req.Token, req.Record)
	if !auth.CheckLoginState(req.Uid, req.Token, req.CliAddr) {
		log.Println("DEBUG in upload auth check failure")
		return &proto.UploadResponse{
			ErrorMsg:     "Invalid token",
			ResponseCode: 403,
		}, nil
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

		log.Println("Login success:", req.Uid)
		return resp, nil
	}
	log.Println("Login failt:", req.Uid)
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

type DashController struct {
	pcClientCache *PCCliManager
	scClientCache *SCCliManager
}

func NewDashController() *DashController {
	return &DashController{
		pcClientCache: &PCCliManager{cliMap: make(map[string]proto.ProxyControllerClient)},
		scClientCache: &SCCliManager{cliMap: make(map[string]proto.ServerControllerClient)},
	}
}

func (dc *DashController) Migrate(uid string, oep string, nep string) error {
	uidSuspend(uid)
	defer uidResume(uid)
	proxyNodes, err := etcd.ListKeyByPrefix(cons.ProxyRegistryPrefix)
	if err != nil {
		log.Println(err)
		return err
	}
	proxyEps := []string{}
	for _, pn := range proxyNodes {
		if ep, err := filepath.Rel(cons.ProxyRegistryPrefix, pn); err == nil {
			proxyEps = append(proxyEps, ep)
		} else {
			log.Println(err)
			return err
		}
	}

	for _, pe := range proxyEps {
		log.Println("DEBUG dash delete endpoint cache ", pe, uid)
		_, err := dc.pcClientCache.Pause(context.Background(), pe, &proto.PauseRequest{Uid: uid, Type: cons.OP_PAUSE})
		if err != nil {
			log.Println(err)
			return err
		}
		defer func(pe string) {
			log.Println("resume", pe)
			dc.pcClientCache.Pause(context.Background(), pe, &proto.PauseRequest{Uid: uid, Type: cons.OP_RESUME})
		}(pe)
	}
	log.Println("DEBUG after pause proxy")

	if _, err := dc.scClientCache.UidLogout(context.Background(), oep, &proto.UidLogoutRequest{Uid: uid}); err != nil {
		log.Println(err)
		return err
	}
	log.Println("DEBUG after logout")
	return updateEpInfo(uid, oep, nep)

}

func RunDashService(port int) error {
	log.Println("Running dash server")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	s := grpc.NewServer()

	proto.RegisterDashServiceServer(s, NewDashServer(port))

	log.Println("ProxyServer registry ok... ")
	reflection.Register(s)
	if err := etcd.Register(cons.ProxyRegistryPrefix, lis.Addr().String()); err != nil {
		return err
	}
	log.Println("Registry etcd ok ...")
	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
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

func addEpInfo(uid string, endpoint string) error {

	return etcd.AtomicMultiKVOp(cons.DashEpLock, &etcd.KV{
		Op:    etcd.OP_PUT,
		Key:   fmt.Sprintf(cons.DashEpQueryFormat, uid),
		Value: endpoint,
	}, &etcd.KV{
		Op:    etcd.OP_PUT,
		Key:   fmt.Sprintf(cons.DashUidsQueryFormat, endpoint, uid),
		Value: "",
	})
}

func delEpInfo(uid string, endpoint string) error {

	return etcd.AtomicMultiKVOp(cons.DashEpLock, &etcd.KV{
		Op:    etcd.OP_DEL,
		Key:   fmt.Sprintf(cons.DashEpQueryFormat, uid),
		Value: endpoint,
	}, &etcd.KV{
		Op:    etcd.OP_DEL,
		Key:   fmt.Sprintf(cons.DashUidsQueryFormat, endpoint, uid),
		Value: "",
	})
}

func updateEpInfo(uid string, oldEndpoint string, newEndpoint string) error {

	return etcd.AtomicMultiKVOp(cons.DashEpLock, &etcd.KV{
		Op:    etcd.OP_PUT,
		Key:   fmt.Sprintf(cons.DashEpQueryFormat, uid),
		Value: newEndpoint,
	}, &etcd.KV{
		Op:    etcd.OP_DEL,
		Key:   fmt.Sprintf(cons.DashUidsQueryFormat, oldEndpoint, uid),
		Value: "",
	}, &etcd.KV{
		Op:    etcd.OP_PUT,
		Key:   fmt.Sprintf(cons.DashUidsQueryFormat, newEndpoint, uid),
		Value: "",
	})
}

func allocEndpoint(uid string) (string, error) {
	if endpoints, err := etcd.ListKeyByPrefix(cons.ServerRegistryPrefix); err != nil {
		return "", err
	} else {
		if len(endpoints) == 0 {
			return "", errors.New("no valid endpoints")
		}
		log.Println("DEBUG: get endpoints ", endpoints)
		endpoint, err := filepath.Rel("/mysh/server/", endpoints[0])
		if err != nil {
			return "", err
		}

		//if err := etcd.PutKV(fmt.Sprintf(cons.DashEpQueryFormat, uid), endpoint); err != nil {
		if err := addEpInfo(uid, endpoint); err != nil {
			return "", err
		}
		return endpoint, nil
	}

}

func (d *DashServer) UidEndpoint(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryResponse, error) {
	//vbytes, err := etcd.GetKV(fmt.Sprintf("endpoint/%v", r.Req))
	//vbytes, err := etcd.GetKV(filepath.Join(cons.DashDataPrefix, "endpoints/", r.Req))
	waitUid(r.Req)
	/*if uidSuspensive(r.Req) {
		return nil, E_SUSPENSIVE
	}*/
	vbytes, err := etcd.GetKV(fmt.Sprintf(cons.DashEpQueryFormat, r.Req))
	if err != nil {
		if err == etcd.ETCD_ERROR_EMPTY_VALUE {
			//alloc endpoint
			if endpoint, err := allocEndpoint(r.Req); err != nil {
				return nil, err
			} else {
				return &proto.CommonQueryResponse{Resp: endpoint}, nil
			}
		}
		return nil, err
	}

	return &proto.CommonQueryResponse{
		Resp: string(vbytes),
	}, nil
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

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
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
	log.Print("DEBUG: endpoint ", endpoint)
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
	log.Println("server.go ecm upload", in.Record, in.Token, in.CliAddr, in.Uid)
	return cli.Upload(ctx, in, opts...)

}

//PC manager
type PCCliManager struct {
	cliMap map[string]proto.ProxyControllerClient
	mtx    sync.Mutex
}

func newPCServerCli(endpoint string) proto.ProxyControllerClient {

	creds, err := credentials.NewClientTLSFromFile(cons.UserCrt, "www.myshell.top")
	if err != nil {
		panic(err)
	}

	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		panic(err)
	}

	//conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return proto.NewProxyControllerClient(conn)
}

func (ecm *PCCliManager) ensureCliExists(endpoint string) {
	if _, ok := ecm.cliMap[endpoint]; !ok {

		ecm.mtx.Lock()
		defer ecm.mtx.Unlock()
		if _, ok = ecm.cliMap[endpoint]; !ok {
			ecm.cliMap[endpoint] = newPCServerCli(endpoint)
		}
	}
}

func (ecm *PCCliManager) Pause(ctx context.Context, endpoint string, in *proto.PauseRequest, opts ...grpc.CallOption) (*proto.PauseResponse, error) {
	//1. 先判断客户端是否存在
	//2. 获取或者创建客户端
	//3. 通过客户端请求

	log.Print("DEBUG: proxy controller endpoint ", endpoint)
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]

	return cli.Pause(ctx, in, opts...)
}

//

//SC manager
type SCCliManager struct {
	cliMap map[string]proto.ServerControllerClient
	mtx    sync.Mutex
}

func newSCServerCli(endpoint string) proto.ServerControllerClient {

	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return proto.NewServerControllerClient(conn)
}

func (ecm *SCCliManager) ensureCliExists(endpoint string) {
	if _, ok := ecm.cliMap[endpoint]; !ok {

		ecm.mtx.Lock()
		defer ecm.mtx.Unlock()
		if _, ok = ecm.cliMap[endpoint]; !ok {
			ecm.cliMap[endpoint] = newSCServerCli(endpoint)
		}
	}
}

func (ecm *SCCliManager) UidLogout(ctx context.Context, endpoint string, in *proto.UidLogoutRequest, opts ...grpc.CallOption) (*proto.UidLogoutResponse, error) {
	//1. 先判断客户端是否存在
	//2. 获取或者创建客户端
	//3. 通过客户端请求

	log.Print("DEBUG: server controller endpoint ", endpoint)
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]

	return cli.UidLogout(ctx, in, opts...)
}

//

func uidSuspensive(uid string) bool {
	proxyMtx.Lock()
	defer proxyMtx.Unlock()
	if pause, ok := proxyPauseMap[uid]; !ok || !pause {
		return false
	}
	return true
}

func waitUid(uid string) {
	for uidSuspensive(uid) {
		time.Sleep(cons.WaitUidInterval)
	}
}

func uidSuspend(uid string) {
	proxyMtx.Lock()
	defer proxyMtx.Unlock()
	proxyPauseMap[uid] = true
}

func uidResume(uid string) {
	proxyMtx.Lock()
	defer proxyMtx.Unlock()

	delete(proxyPauseMap, uid)
}

type ProxyController struct {
	port int
}

func NewProxyController(port int) *ProxyController {
	return &ProxyController{
		port: port,
	}
}

func RunProxyService(port int) error {
	log.Println("Running proxy server")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}
	creds, err := credentials.NewServerTLSFromFile(cons.Crt, cons.Key)
	if err != nil {
		return fmt.Errorf("could not load TLS keys: %s", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))

	//s := grpc.NewServer()
	proto.RegisterProxyControllerServer(s, NewProxyController(port))
	proto.RegisterSearchServiceServer(s, NewProxyServer(port))

	log.Println("ProxyServer registry ok... ")
	reflection.Register(s)
	if err := etcd.Register(cons.ProxyRegistryPrefix, lis.Addr().String()); err != nil {
		return err
	}
	log.Println("Registry etcd ok ...")
	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}

func (pc *ProxyController) Pause(ctx context.Context, req *proto.PauseRequest) (*proto.PauseResponse, error) {
	if req.Type == cons.OP_PAUSE {
		uidSuspend(req.Uid)
		//for proxyReqCount != 0 {
		for atomic.LoadInt32(&proxyReqCount) != 0 {
			time.Sleep(time.Millisecond * 10)
		}

		uidEndpointsCacheMtx.Lock()
		defer uidEndpointsCacheMtx.Unlock()
		log.Println("DEBUG proxy delete endpoint cache ", uidEndpointsCache, req.Uid)
		delete(uidEndpointsCache, req.Uid)

		return &proto.PauseResponse{ResponseCode: 200}, nil
	} else {
		uidResume(req.Uid)
		return &proto.PauseResponse{ResponseCode: 200}, nil
	}
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
	//conn2, err := grpc.Dial("127.0.0.1:8083", grpc.WithInsecure())
	//if err != nil {
	//	panic(err)
	//}
	//endpointCliManger.cliMap["127.0.0.1:8083"] = proto.NewSearchServiceClient(conn2)

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

	if err := etcd.Register(cons.ProxyRegistryPrefix, lis.Addr().String()); err != nil {
		return err
	}
	if err := s.Serve(lis); err != nil {
		return err
	}

	return nil
}

func (ps *ProxyServer) uidEndpoint(uid string) (string, error) {
	uidEndpointsCacheMtx.Lock()
	defer uidEndpointsCacheMtx.Unlock()
	if ep, ok := uidEndpointsCache[uid]; ok {
		return ep, nil
	} else {

		resp, err := ps.dashCli.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
			Req: uid,
		})
		if err != nil {
			return "", err
		}
		uidEndpointsCache[uid] = resp.Resp
		return resp.Resp, nil
	}
}

type dealFunction func(p *peer.Peer, endpoint string)

func (ps *ProxyServer) doProxy(ctx context.Context, uid string, dealFunc dealFunction) error {
	waitUid(uid)
	endpoint, err := ps.uidEndpoint(uid)
	if err != nil {
		return err
	}

	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	if p, ok := peer.FromContext(ctx); ok {
		dealFunc(p, endpoint)
		return nil
	}

	return fmt.Errorf("Can't get ip from peer")
}

func (ps *ProxyServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {

	var resp *proto.SearchResponse
	var err error
	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) {

		req.CliAddr = p.Addr.String()
		log.Println("server.go ps search ", req.GetCliAddr(), req.GetToken(), req.GetSearchString(), req.GetUid())
		resp, err = ps.endpointCliManager.Search(context.Background(), endpoint, req)
		log.Println("server.go ps search", resp.GetResponse(), resp.GetResponseCode())

	}); e != nil {
		return nil, e
	}

	return resp, err
}

func (ps *ProxyServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	var resp *proto.UploadResponse
	var err error

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) {
		req.CliAddr = p.Addr.String()
		log.Println("server.go proxyserver upload() ", req.Uid, req.CliAddr, req.Token, req.Record)
		resp, err = ps.endpointCliManager.Upload(context.Background(), endpoint, req)
	}); e != nil {
		return nil, e
	}

	return resp, nil
}

func (ps *ProxyServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	var resp *proto.LoginResponse
	var err error

	log.Println("DEBUG: login:", req.Uid, req.Password)

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) {
		req.CliAddr = p.Addr.String()

		if resp, err = ps.endpointCliManager.Login(context.Background(), endpoint, req); err != nil {
			log.Println(err)
		}
	}); e != nil {
		return nil, e
	}

	return resp, nil

}

func (ps *ProxyServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	var resp *proto.LogoutResponse
	var err error

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) {
		req.CliAddr = p.Addr.String()
		resp, err = ps.endpointCliManager.Logout(context.Background(), endpoint, req)
	}); e != nil {
		return nil, e
	}

	return resp, nil

}

//old proxy server impl
/*func (ps *ProxyServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {
	//1. 获取uid对应的endpoint
	//2. 获取client的地址加入请求转给endpoint
	waitUid(req.Uid)
	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	endpoint, err := ps.uidEndpoint(req.Uid)
	if err != nil {
		return nil, err
	}

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
}*/

/*func (ps *ProxyServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	waitUid(req.Uid)
	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	endpoint, err := ps.uidEndpoint(req.Uid)
	if err != nil {
		return nil, err
	}

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Upload(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")
}*/

/*func (ps *ProxyServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	waitUid(req.Uid)
	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	endpoint, err := ps.uidEndpoint(req.Uid)
	if err != nil {
		return nil, err
	}

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Login(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")

}*/

/*func (ps *ProxyServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	waitUid(req.Uid)
	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	endpoint, err := ps.uidEndpoint(req.Uid)
	if err != nil {
		return nil, err
	}

	if p, ok := peer.FromContext(ctx); ok {
		req.CliAddr = p.Addr.String()
		sresp, err := ps.endpointCliManager.Logout(context.Background(), endpoint, req)
		if err != nil {
			return nil, err
		}

		return sresp, err

	}
	return nil, fmt.Errorf("Can't get ip from peer")
}*/
//old proxy server impl end

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
