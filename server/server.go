package server

import (
	"encoding/json"
	"fmt"
	"github.com/1071496910/event-controller/controller"
	"github.com/1071496910/mysh/lib/etcd"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1071496910/consistent-hashing"
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
	uidEndpointsCacheMtx sync.RWMutex
	//uidEndpointsCache    = make(map[string]string)
	uidEndpointsCache = &stalingCache{cache: make(map[string]*scNode)}
	//E_SUSPENSIVE         = errors.New("Uid is suspensive")
	hr            hashring.HashRing
	scClientCache = &SCCliManager{cliMap: make(map[string]proto.ServerControllerClient)}
)

func init() {
	var err error
	if hr, err = hashring.NewHashRing(cons.HashRingSlotNum, cons.HashRingVNodeNum); err != nil {
		panic(err)
	}
	//!!fortest
	//hr.AddNode("[::]:8083")
}

type scNode struct {
	val      string
	deadline time.Time
}

type stalingCache struct {
	cache map[string]*scNode
	mtx   sync.Mutex
}

func (sc *stalingCache) Put(key string, val string) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()

	node := &scNode{
		val:      val,
		deadline: time.Now().Add(time.Second * 30),
	}

	sc.cache[key] = node
}

func (sc *stalingCache) Get(key string) (string, bool) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	if node, ok := sc.cache[key]; ok {
		if time.Now().Before(node.deadline) {
			log.Println("DEBUG: in stalingcache", time.Now(), "before", node.deadline)
			return node.val, true
		}
		delete(sc.cache, key)
	}
	return "", false
}

func (sc *stalingCache) Del(key string) {
	sc.mtx.Lock()
	defer sc.mtx.Unlock()
	delete(sc.cache, key)
}

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
		return err
	}
	log.Println("Registry searchService ok...")
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

func (sc *SearchController) Check(context.Context, *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{
		Status: proto.HealthCheckResponse_SERVING,
	}, nil

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
	ec            *controller.Controller
}

func NewDashController() (*DashController, error) {
	sucker, err := controller.NewEtcdSucker(cons.EtcdEndpoints, cons.ServerRegistryPrefix, controller.EtcdSuckerWithRecursive())
	if err != nil {
		return nil, err
	}

	dashController := &DashController{
		pcClientCache: &PCCliManager{cliMap: make(map[string]proto.ProxyControllerClient)},
	}

	dashController.ec = controller.NewController(sucker,
		func(i interface{}) {

			kv := i.(*controller.EtcdSuckerEventObj)
			if node, err := filepath.Rel(cons.ServerRegistryPrefix, string(kv.Key)); err != nil {
				log.Println("parse node info err ", err)
				return
			} else {
				log.Println("add node ", node, "to hash", hr.AddNode(node))
			}
		},

		func(i interface{}) {
			kv := i.(*controller.EtcdSuckerEventObj)
			fmt.Printf("In update func[key: %v, val: %v]\n", string(kv.Key), string(kv.Val))
		},
		func(i interface{}) {
			kv := i.(*controller.EtcdSuckerEventObj)
			if node, err := filepath.Rel(cons.ServerRegistryPrefix, string(kv.Key)); err != nil {
				log.Println("parse node info err ", err)
				return
			} else {
				/*selfID := GenSelfID()
				if ok, err := etcd.IsLeader(cons.DashLeaderKey, selfID); !ok || err != nil {
					return
				}

				err = etcd.DElKeysByPrefix(fmt.Sprintf(cons.DashUidsQueryFormat, node))
				if err != nil {
					log.Println("del node uid inof err", err)
					return
				}*/

				log.Println("del node ", node, "from hash", hr.DelNode(node))
			}
		}, cons.ControllerEventQueueLen)

	return dashController, nil

}

func (dc *DashController) Run() error {
	return dc.ec.Run()
}

func (dc *DashServer) cleanUidCache(uid string) error {
	if uid == "" {
		return fmt.Errorf("uid can't be empty")
	}
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
		_, err := dc.pcClientCache.CleanUidCache(context.Background(), pe, &proto.CleanRequest{Uid: uid})
		if err != nil {
			log.Println(err)
			return err
		}
	}
	log.Println("DEBUG clean proxy uid cache")
	return nil
}

/*func RunDashService(port int) error {
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
}*/

type DashServer struct {
	pcClientCache *PCCliManager
	addr          string
}

func NewDashServer(addr string) *DashServer {
	return &DashServer{
		addr:          addr,
		pcClientCache: &PCCliManager{cliMap: make(map[string]proto.ProxyControllerClient)},
	}

}

func (ss *DashServer) Run() error {
	lis, err := net.Listen("tcp", ss.addr)
	if err != nil {
		return fmt.Errorf("init network error: %v", err)
	}

	selfID := ss.addr
	interceptor := grpc.UnaryInterceptor(
		func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (resp interface{}, err error) {
			var isleader bool
			if isleader, err = etcd.IsLeader(cons.DashLeaderKey, selfID); !isleader || err != nil {
				err = cons.ErrDashLeaderUnavailable
				log.Println("in start dashserver :", err)
				return
			}
			return handler(ctx, req)

		})

	s := grpc.NewServer(interceptor)

	proto.RegisterDashServiceServer(s, ss)
	reflection.Register(s)
	go func() {
		go etcd.Elect(cons.DashLeaderKey, selfID, 3)
	}()
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
	log.Println("in add epinfo")
	defer log.Println("return add epinfo")
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

/*func getEpInfo(uid string) (string, error) {
	if byteRet, err := etcd.GetKV(fmt.Sprintf(cons.DashEpQueryFormat, uid)); err ==nil{
		return "", err
	} else {
		return string(byteRet), err
	}
}*/

/*func delEpInfo(uid string, endpoint string) error {

	return etcd.AtomicMultiKVOp(cons.DashEpLock, &etcd.KV{
		Op:    etcd.OP_DEL,
		Key:   fmt.Sprintf(cons.DashEpQueryFormat, uid),
		Value: endpoint,
	}, &etcd.KV{
		Op:    etcd.OP_DEL,
		Key:   fmt.Sprintf(cons.DashUidsQueryFormat, endpoint, uid),
		Value: "",
	})
}*/

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

	log.Println("in dash 0 ")
	if hashedEndpoint, err := hr.GetNode(uid); err != nil {
		log.Println("in dash server allocep get node ")
		return "", err
	} else {
		if err := addEpInfo(uid, hashedEndpoint); err != nil {
			log.Println("in dash server addEpinfo err", err)
			return "", err
		}
		log.Println("DEBUG: in allocEndpoint get ep", hashedEndpoint)
		return hashedEndpoint, nil
	}

	/*
			if curEndpoint == "" {
				if hashedEndpoint, err := hr.GetNode(uid); err != nil {
					return "", err
				} else {
					if err := addEpInfo(uid, hashedEndpoint); err != nil {
						log.Println("in dash server addEpinfo err", err)
						return "", err
					}
					return hashedEndpoint, nil
				}
				//return hr.GetNode(uid)
			}
			if hashedEndpoint, err := hr.GetNode(uid); err != nil {
				return curEndpoint, nil
			} else {
				if curEndpoint != hashedEndpoint {
					//迁移数据
					if _, err := scClientCache.UidLogout(context.Background(), curEndpoint, &proto.UidLogoutRequest{Uid: uid}); err != nil {
						log.Println(err)
						return "", err
					}
					log.Println("DEBUG after logout")
					//return updateEpInfo(uid, oep, nep)
					return hashedEndpoint, updateEpInfo(uid, curEndpoint, hashedEndpoint)
				}
				return curEndpoint, nil
			}
		}
		//return "", nil

		/*if endpoints, err := etcd.ListKeyByPrefix(cons.ServerRegistryPrefix); err != nil {
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
		}*/

}

func (d *DashServer) UidEndpoint(ctx context.Context, r *proto.CommonQueryRequest) (*proto.CommonQueryResponse, error) {

	waitUid(r.Req)

	vbytes, err := etcd.GetKV(fmt.Sprintf(cons.DashEpQueryFormat, r.Req))
	if err != nil {

		if err == etcd.ETCD_ERROR_EMPTY_VALUE {
			//alloc endpoint
			log.Println("in dashserver before allocEndpoint ")
			if endpoint, err := allocEndpoint(r.Req); err != nil {
				log.Println("in dashserver allocEndpoint err", err)
				return nil, err
			} else {
				log.Println("in dashserver allocEndpoint ", endpoint)
				return &proto.CommonQueryResponse{Resp: endpoint}, nil
			}
		}
		return nil, err
	}

	hashedEp, err := hr.GetNode(r.Req)
	if err != nil {
		return nil, err
	}
	curEndpoint := string(vbytes)
	if hashedEp != curEndpoint {

		resp, err := scClientCache.Check(context.Background(), hashedEp, &proto.HealthCheckRequest{})
		if err != nil || resp.Status != proto.HealthCheckResponse_SERVING {
			log.Println("DEBUG: uid endpoint check endpoint", hashedEp, "not ok")
			return &proto.CommonQueryResponse{
				Resp: curEndpoint}, nil
		}

		uidSuspend(r.Req)
		defer uidResume(r.Req)

		uids, err := etcd.ListKeyByPrefix(fmt.Sprintf(cons.DashUidsQueryFormat, curEndpoint))
		if err != nil {
			log.Println("get uid of node err", err)
			return nil, err
		}
		for _, uid := range uids {
			err := d.cleanUidCache(uid)
			if err != nil {
				log.Println("del node uid inof err", err)
				return nil, err
			}

		}

		if _, err := scClientCache.UidLogout(context.Background(), curEndpoint, &proto.UidLogoutRequest{Uid: r.Req}); err != nil {
			log.Println("DEBUG: dash uid endpoint, logout err:", err)

			//return nil, err
		}
		log.Println("DEBUG after logout")
		//return updateEpInfo(uid, oep, nep)
		return &proto.CommonQueryResponse{
			Resp: hashedEp}, updateEpInfo(r.Req, curEndpoint, hashedEp)
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
		log.Println(err)
	}

	conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Println(err)
	}

	/*conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}*/
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

func (ecm *PCCliManager) CleanUidCache(ctx context.Context, endpoint string, in *proto.CleanRequest, opts ...grpc.CallOption) (*proto.CleanResponse, error) {

	log.Print("DEBUG: proxy controller endpoint ", endpoint)
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]

	return cli.CleanUidCache(ctx, in, opts...)
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

	log.Print("DEBUG: server controller endpoint ", endpoint)
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]

	return cli.UidLogout(ctx, in, opts...)
}

func (ecm *SCCliManager) Check(ctx context.Context, endpoint string, in *proto.HealthCheckRequest, opts ...grpc.CallOption) (*proto.HealthCheckResponse, error) {

	log.Print("DEBUG: in check server controller endpoint ", endpoint)
	ecm.ensureCliExists(endpoint)
	cli := ecm.cliMap[endpoint]

	return cli.Check(ctx, in, opts...)
}

//DS manager
type DSCliManager struct {
	cliMap map[string]proto.DashServiceClient
	mtx    sync.Mutex
}

func newDSServerCli(endpoint string) proto.DashServiceClient {

	dashPort := strings.Split(endpoint, ":")[1]
	conn, err := grpc.Dial("www.myshell.top:"+dashPort, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	return proto.NewDashServiceClient(conn)
}

func (dsm *DSCliManager) ensureCliExists(endpoint string) {
	if _, ok := dsm.cliMap[endpoint]; !ok {

		dsm.mtx.Lock()
		defer dsm.mtx.Unlock()
		if _, ok = dsm.cliMap[endpoint]; !ok {
			dsm.cliMap[endpoint] = newDSServerCli(endpoint)
		}
	}
}

func (dsm *DSCliManager) UidEndpoint(ctx context.Context, in *proto.CommonQueryRequest, opts ...grpc.CallOption) (*proto.CommonQueryResponse, error) {

	endpoint, err := etcd.GetElectorID(cons.DashLeaderKey)
	if err != nil {
		return nil, err
	}

	log.Print("DEBUG: server controller endpoint ", endpoint)
	dsm.ensureCliExists(endpoint)
	cli := dsm.cliMap[endpoint]

	return cli.UidEndpoint(ctx, in, opts...)
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

func (pc *ProxyController) CleanUidCache(ctx context.Context, req *proto.CleanRequest) (*proto.CleanResponse, error) {
	uidEndpointsCacheMtx.Lock()
	defer uidEndpointsCacheMtx.Unlock()
	for atomic.LoadInt32(&proxyReqCount) != 0 {
		log.Println("wait req ")
		time.Sleep(time.Millisecond * 10)
	}

	log.Println("DEBUG proxy delete endpoint cache ", uidEndpointsCache, req.Uid)
	uidEndpointsCache.Del(req.Uid)

	return &proto.CleanResponse{ResponseCode: 200}, nil
}

type ProxyServer struct {
	port           int
	dashCliManager *DSCliManager
	//dashCli            proto.DashServiceClient
	endpointCliManager *EndpointCliManager
}

func NewProxyServer(port int) *ProxyServer {
	/*dashPort := strings.Split(cons.DashAddr,":")[1]
	conn, err := grpc.Dial("www.myshell.top:"+dashPort, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}*/
	endpointCliManger := &EndpointCliManager{
		cliMap: make(map[string]proto.SearchServiceClient),
	}
	dashCliManager := &DSCliManager{
		cliMap: make(map[string]proto.DashServiceClient),
	}

	return &ProxyServer{
		port:               port,
		dashCliManager:     dashCliManager,
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
	uidEndpointsCacheMtx.RLock()
	ep, ok := uidEndpointsCache.Get(uid)
	uidEndpointsCacheMtx.RUnlock()
	if ok {
		log.Println("DEBUG: in proxyServer get endpoint ", ep)
		return ep, nil
	}

	uidEndpointsCacheMtx.Lock()
	defer uidEndpointsCacheMtx.Unlock()

	resp, err := ps.dashCliManager.UidEndpoint(context.Background(), &proto.CommonQueryRequest{
		Req: uid,
	})
	if err != nil {
		return "", err
	}
	//uidEndpointsCache[uid] = resp.Resp
	uidEndpointsCache.Put(uid, resp.Resp)
	return resp.Resp, nil

}

type dealFunction func(p *peer.Peer, endpoint string) error

func checkConnErr(err error) bool {
	return strings.Contains(err.Error(), "connection error")
}

func checkLeaderUnavailableErr(err error) bool {
	return strings.Contains(err.Error(), cons.ErrDashLeaderUnavailable.Error())
}

func (ps *ProxyServer) doProxy(ctx context.Context, uid string, dealFunc dealFunction) error {
	//waitUid(uid)

	endpoint, err := ps.uidEndpoint(uid)
	for err != nil {
		log.Println("in doproxy get uidEndpoint err:", err)
		if !checkLeaderUnavailableErr(err) {
			return err
		}
		time.Sleep(time.Second)
		endpoint, err = ps.uidEndpoint(uid)
	}
	atomic.AddInt32(&proxyReqCount, 1)
	defer atomic.AddInt32(&proxyReqCount, -1)

	if p, ok := peer.FromContext(ctx); ok {
		err := dealFunc(p, endpoint)
		if err != nil {
			log.Println("DEBUG: in do search err: ", err)
			if !checkConnErr(err) {
				return err
			}
			atomic.AddInt32(&proxyReqCount, -1)

			uidEndpointsCacheMtx.Lock()
			uidEndpointsCache.Del(uid)
			uidEndpointsCacheMtx.Unlock()

			time.Sleep(time.Second)
			ps.doProxy(ctx, uid, dealFunc)
			atomic.AddInt32(&proxyReqCount, 1)
		}
		return nil
	}

	return fmt.Errorf("Can't get ip from peer")
}

/*func (ss *ProxyServer) Check(context.Context, *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	return &proto.HealthCheckResponse{
		Status: proto.HealthCheckResponse_SERVING,
	}, nil
}*/

func (ps *ProxyServer) Search(ctx context.Context, req *proto.SearchRequest) (*proto.SearchResponse, error) {

	var resp *proto.SearchResponse
	var err error
	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) error {

		req.CliAddr = p.Addr.String()
		log.Println("server.go ps search ", req.GetCliAddr(), req.GetToken(), req.GetSearchString(), req.GetUid())
		resp, err = ps.endpointCliManager.Search(context.Background(), endpoint, req)
		log.Println("server.go ps search", resp.GetResponse(), resp.GetResponseCode())
		return err

	}); e != nil {
		return nil, e
	}

	return resp, nil
}

func (ps *ProxyServer) Upload(ctx context.Context, req *proto.UploadRequest) (*proto.UploadResponse, error) {
	var resp *proto.UploadResponse
	var err error

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) error {
		req.CliAddr = p.Addr.String()
		log.Println("server.go proxyserver upload() ", req.Uid, req.CliAddr, req.Token, req.Record)
		resp, err = ps.endpointCliManager.Upload(context.Background(), endpoint, req)
		return err
	}); e != nil {
		return nil, e
	}

	return resp, nil
}

func (ps *ProxyServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	var resp *proto.LoginResponse
	var err error

	log.Println("DEBUG: login:", req.Uid, req.Password)

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) error {
		req.CliAddr = p.Addr.String()

		resp, err = ps.endpointCliManager.Login(context.Background(), endpoint, req)
		if err != nil {
			log.Println(err)
		}
		return err
	}); e != nil {
		return nil, e
	}

	return resp, nil

}

func (ps *ProxyServer) Logout(ctx context.Context, req *proto.LogoutRequest) (*proto.LogoutResponse, error) {
	var resp *proto.LogoutResponse
	var err error

	if e := ps.doProxy(ctx, req.Uid, func(p *peer.Peer, endpoint string) error {
		req.CliAddr = p.Addr.String()
		resp, err = ps.endpointCliManager.Logout(context.Background(), endpoint, req)
		return err
	}); e != nil {
		return nil, e
	}

	return resp, nil

}
