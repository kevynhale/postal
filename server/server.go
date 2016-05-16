package server

import (
	"net"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/jive/postal/postal"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type PostalServer struct {
	etcd *clientv3.Client
}

func (srv *PostalServer) Register(s *grpc.Server) {
	api.RegisterPostalServer(s, srv)
}

func (srv *PostalServer) config() *postal.Config {
	return (&postal.Config{}).WithEtcdClient(srv.etcd)
}

// NetworkRange will return exactly 1 Network for a valid ID.
// If ID is empty then it will return a list of Network IDs.
func (srv *PostalServer) NetworkRange(ctx context.Context, req *api.NetworkRangeRequest) (*api.NetworkRangeResponse, error) {
	resp := &api.NetworkRangeResponse{}

	if len(req.ID) > 0 {
		nm, err := srv.config().Network(req.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.ID)
		}
		resp.Networks = []*api.Network{nm.APINetwork()}
		resp.Size_ = 1
		resp.Offset = 0
		return resp, nil
	}

	ids, err := srv.config().Networks()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve network list")
	}

	resp.Size_ = int32(len(ids))
	resp.Offset = 0
	resp.Networks = []*api.Network{}
	for idx := range ids {
		resp.Networks = append(resp.Networks, &api.Network{ID: ids[idx]})
	}

	return resp, nil
}

func (srv *PostalServer) NetworkAdd(ctx context.Context, req *api.NetworkAddRequest) (*api.NetworkAddResponse, error) {
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
	if req.ID == nil {
		return nil, errors.New("NetworkID must be valid")
	}

	if len(req.ID.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
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
			Pools:  []*api.Pool{pm.APIPool()},
			Size_:  int32(1),
			Offset: int32(0),
		}, nil
	}

	poolIds, err := nm.Pools()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve pools for network id (%s)", req.ID.NetworkID)
	}

	resp := &api.PoolRangeResponse{
		Pools:  []*api.Pool{},
		Size_:  int32(len(poolIds)),
		Offset: int32(0),
	}

	for idx := range poolIds {
		resp.Pools = append(resp.Pools, &api.Pool{
			ID: &api.Pool_PoolID{NetworkID: req.ID.NetworkID, ID: poolIds[idx]},
		})
	}

	return resp, nil
}

func (srv *PostalServer) PoolAdd(ctx context.Context, req *api.PoolAddRequest) (*api.PoolAddResponse, error) {
	if len(req.NetworkID) == 0 {
		return nil, errors.New("NetworkID must be valid")
	}

	nm, err := srv.config().Network(req.NetworkID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.NetworkID)
	}

	pm, err := nm.NewPool(req.Annotations, 0, int(req.Maximum), req.Type)
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

func (srv *PostalServer) PoolSetMax(ctx context.Context, req *api.PoolSetMinMaxRequest) (*api.PoolSetMinMaxResponse, error) {
	return nil, errors.New("operation not supported")
}

func (srv *PostalServer) LookupBinding(ctx context.Context, req *api.LookupBindingRequest) (*api.LookupBindingResponse, error) {
	if req.GetById() != nil {
		if req.GetById().PoolID == nil || len(req.GetById().ID) == 0 {
			return nil, errors.New("Network, Pool and Binding IDs must all be valid")
		}

		nm, err := srv.config().Network(req.GetById().PoolID.NetworkID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.GetById().PoolID.NetworkID)
		}

		pm, err := nm.Pool(req.GetById().PoolID.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve pool")
		}

		binding, err := pm.Binding(req.GetById().ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve binding")
		}

		return &api.LookupBindingResponse{
			Binding: binding,
		}, nil

	}

	if req.GetByAddress() != nil {
		if len(req.GetByAddress().NetworkID) == 0 {
			return nil, errors.New("NetworkID must be valid")
		}

		nm, err := srv.config().Network(req.GetByAddress().NetworkID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve network for id (%s)", req.GetByAddress().NetworkID)
		}

		binding, err := nm.Binding(net.ParseIP(req.GetByAddress().Address))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve binding")
		}

		return &api.LookupBindingResponse{
			Binding: binding,
		}, nil
	}

	return nil, errors.New("lookup did not match byAddress or byID methods")
}

func (srv *PostalServer) AllocateAddress(ctx context.Context, req *api.AllocateAddressRequest) (*api.AllocateAddressResponse, error) {
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

func (srv *PostalServer) BindAddress(ctx context.Context, req *api.BindAddressRequest) (*api.BindAddressResponse, error) {
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

	binding, err := pm.Binding(req.BindingID)
	if err != nil {
		return nil, errors.Wrap(err, "binding lookup failed")
	}

	err = pm.Release(binding, req.Hard)
	if err != nil {
		return nil, errors.Wrap(err, "release binding failed")
	}

	return &api.ReleaseAddressResponse{}, nil
}
