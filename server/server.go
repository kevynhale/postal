package server

import (
	"net"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/pkg/capnslog"
	"github.com/jive/postal/api"
	"github.com/jive/postal/postal"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	plog = capnslog.NewPackageLogger("github.com/jive/postal", "server")
)

type PostalServer struct {
	etcd *clientv3.Client
}

func NewServer(etcd *clientv3.Client) *PostalServer {
	return &PostalServer{
		etcd: etcd,
	}
}

func (srv *PostalServer) Register(s *grpc.Server) {
	plog.Info("registering postal grpc server")
	api.RegisterPostalServer(s, srv)
}

func (srv *PostalServer) config() *postal.Config {
	return (&postal.Config{}).WithEtcdClient(srv.etcd)
}

// NetworkRange will return exactly 1 Network for a valid ID.
// If ID is empty then it will return a list of Network IDs.
func (srv *PostalServer) NetworkRange(ctx context.Context, req *api.NetworkRangeRequest) (*api.NetworkRangeResponse, error) {
	plog.Infof("rpc: NetworkRange(%s)", req.String())
	resp := &api.NetworkRangeResponse{}

	if len(req.ID) > 0 {
		nm, err := srv.config().Network(req.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.ID)
		}
		resp.Networks = []*api.Network{nm.APINetwork()}
		resp.Size_ = 1
		return resp, nil
	}

	networks, err := srv.config().Networks(req.Filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve network list")
	}

	resp.Size_ = int32(len(networks))
	resp.Networks = networks

	return resp, nil
}

func (srv *PostalServer) NetworkAdd(ctx context.Context, req *api.NetworkAddRequest) (*api.NetworkAddResponse, error) {
	plog.Infof("rpc: NetworkAdd(%s)", req)
	network, err := srv.config().NewNetwork(req.GetAnnotations(), req.Cidr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new network")
	}

	return &api.NetworkAddResponse{
		Network: network.APINetwork(),
	}, nil
}

func (srv *PostalServer) NetworkRemove(ctx context.Context, req *api.NetworkRemoveRequest) (*api.NetworkRemoveResponse, error) {
	return nil, errors.New("operation not supported")
}

func (srv *PostalServer) PoolRange(ctx context.Context, req *api.PoolRangeRequest) (*api.PoolRangeResponse, error) {
	plog.Infof("rpc: PoolRange(%s)", req)
	if req.ID == nil || req.ID.NetworkID == "" {
		pools, err := srv.config().Pools(req.Filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch pools")
		}

		return &api.PoolRangeResponse{
			Pools: pools,
			Size_: int32(len(pools)),
		}, nil
	}

	nm, err := srv.config().Network(req.ID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.ID.NetworkID)
	}

	if len(req.ID.ID) > 0 {
		pm, pErr := nm.Pool(req.ID.ID)
		if pErr != nil {
			return nil, errors.Wrapf(pErr, "failed to retrieve pool in network (%s) for id (%s)", req.ID.NetworkID, req.ID.ID)
		}
		return &api.PoolRangeResponse{
			Pools: []*api.Pool{pm.APIPool()},
			Size_: int32(1),
		}, nil
	}

	pools, err := nm.Pools(req.Filters)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pools for network id (%s)", req.ID.NetworkID)
	}

	resp := &api.PoolRangeResponse{
		Pools: pools,
		Size_: int32(len(pools)),
	}

	return resp, nil
}

func (srv *PostalServer) PoolAdd(ctx context.Context, req *api.PoolAddRequest) (*api.PoolAddResponse, error) {
	plog.Infof("rpc: PoolAdd(%s)", req)
	if len(req.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.NetworkID)
	}

	pm, err := nm.NewPool(req.Annotations, req.Maximum, req.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new pool")
	}

	return &api.PoolAddResponse{
		Pool: pm.APIPool(),
	}, nil
}

func (srv *PostalServer) PoolRemove(ctx context.Context, req *api.PoolRemoveRequest) (*api.PoolRemoveResponse, error) {
	return nil, errors.New("operation not supported")
}

func (srv *PostalServer) PoolSetMax(ctx context.Context, req *api.PoolSetMaxRequest) (*api.PoolSetMaxResponse, error) {
	plog.Infof("rpc: PoolSetMax(%s)", req)
	if req.PoolID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.PoolID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.PoolID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.PoolID.NetworkID)
	}

	pm, err := nm.Pool(req.PoolID.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
	}

	err = pm.SetMaxSize(req.Maximum)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set pool max")
	}

	return &api.PoolSetMaxResponse{}, nil
}

func (srv *PostalServer) BindingRange(ctx context.Context, req *api.BindingRangeRequest) (*api.BindingRangeResponse, error) {
	plog.Infof("rpc: BindingRange(%s)", req)
	if len(req.NetworkID) == 0 {
		return nil, errors.New("networkID is not set")
	}

	nm, err := srv.config().Network(req.NetworkID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get network")
	}

	bindings, err := nm.Bindings(req.Filters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get bindings")
	}

	return &api.BindingRangeResponse{
		Bindings: bindings,
		Size_:    int32(len(bindings)),
	}, nil
}

func (srv *PostalServer) AllocateAddress(ctx context.Context, req *api.AllocateAddressRequest) (*api.AllocateAddressResponse, error) {
	plog.Infof("rpc: AllocateAddress(%s)", req)
	if req.PoolID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.PoolID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.PoolID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.PoolID.NetworkID)
	}

	pm, err := nm.Pool(req.PoolID.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
	}

	binding, err := pm.Allocate(net.ParseIP(req.Address))
	if err != nil {
		return nil, errors.Wrap(err, "allocate failed")
	}

	return &api.AllocateAddressResponse{
		Binding: binding,
	}, nil
}

func (srv *PostalServer) BulkAllocateAddress(ctx context.Context, req *api.BulkAllocateAddressRequest) (*api.BulkAllocateAddressResponse, error) {
	plog.Infof("rpc: BulkAllocateAddress(%s)", req)
	if req.PoolID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.PoolID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	ip, ipnet, err := net.ParseCIDR(req.Cidr)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse cidr")
	}

	nm, err := srv.config().Network(req.PoolID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.PoolID.NetworkID)
	}

	pm, err := nm.Pool(req.PoolID.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
	}

	bindings := []*api.Binding{}
	errs := map[string]*api.Error{}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		binding, err := pm.Allocate(ip)
		if err != nil {
			errs[ip.String()] = &api.Error{Message: err.Error()}
		} else {
			bindings = append(bindings, binding)
		}
	}

	return &api.BulkAllocateAddressResponse{
		Bindings: bindings,
		Errors:   errs,
	}, nil
}

func (srv *PostalServer) BindAddress(ctx context.Context, req *api.BindAddressRequest) (*api.BindAddressResponse, error) {
	plog.Infof("rpc: BindAddress(%s)", req)
	if req.PoolID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.PoolID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.PoolID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.PoolID.NetworkID)
	}

	pm, err := nm.Pool(req.PoolID.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
	}

	var binding *api.Binding
	addr := net.ParseIP(req.Address)

	if addr == nil || addr.IsUnspecified() {
		binding, err = pm.BindAny(req.Annotations)
		if err != nil {
			return nil, errors.Wrap(err, "bind failed")
		}
	} else {
		binding, err = pm.Bind(req.Annotations, addr)
		if err != nil {
			return nil, errors.Wrap(err, "bind failed")
		}
	}

	return &api.BindAddressResponse{
		Binding: binding,
	}, nil
}

func (srv *PostalServer) ReleaseAddress(ctx context.Context, req *api.ReleaseAddressRequest) (*api.ReleaseAddressResponse, error) {
	plog.Infof("rpc: ReleaseAddress(%s)", req)
	if req.PoolID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.PoolID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.PoolID.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.PoolID.NetworkID)
	}

	var pm postal.PoolManager
	var binding *api.Binding

	if len(req.Address) > 0 {
		binding, err = nm.Binding(net.ParseIP(req.Address))
		if err != nil {
			if req.Hard {
				nm.ScrubAddress(net.ParseIP(req.Address))
			}
			return nil, errors.Wrapf(err, "failed to find binding for ip (%s)", req.Address)
		}
	} else {

		pm, err = nm.Pool(req.PoolID.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
		}

		binding, err = pm.Binding(req.BindingID)
		if err != nil {
			return nil, errors.Wrap(err, "binding lookup failed")
		}
	}

	if pm == nil {
		pm, err = nm.Pool(binding.PoolID.ID)
		if err != nil {
			if req.Hard && len(req.Address) > 0 {
				nm.ScrubAddress(net.ParseIP(req.Address))
			}
			return nil, errors.Wrapf(err, "failed to retrieve pool in network (%s) for id (%s)", req.PoolID.NetworkID, req.PoolID.ID)
		}
	}

	err = pm.Release(binding, req.Hard)
	if err != nil {
		return nil, errors.Wrap(err, "release binding failed")
	}

	return &api.ReleaseAddressResponse{}, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
