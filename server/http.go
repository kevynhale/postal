package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"github.com/jive/postal/api"
)

func initHTTPServer(s *PostalServer) http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/v1/networks", s.httpHandleNetworks).Methods(http.MethodGet)
	r.HandleFunc("/v1/networks/{network}", s.httpHandleNetworks).Methods(http.MethodGet)

	r.HandleFunc("/v1/networks/{network}/pools", s.httpHandlePools).Methods(http.MethodGet)
	r.HandleFunc("/v1/networks/{network}/pools/{pool}", s.httpHandlePools).Methods(http.MethodGet)

	r.HandleFunc("/v1/networks/{network}/bindings", s.httpHandleBindings).Methods(http.MethodGet)
	r.HandleFunc("/v1/networks/{network}/{addr}", s.httpHandleBindings).Methods(http.MethodGet)
	r.HandleFunc("/v1/networks/{network}/pools/{pool}/bindings", s.httpHandleBindings).Methods(http.MethodGet)

	r.HandleFunc("/v1/networks/{network}/pools/{pool}/_allocate", s.httpHandleAllocate).Methods(http.MethodPost)
	r.HandleFunc("/v1/networks/{network}/pools/{pool}/{addr}/_allocate", s.httpHandleAllocate).Methods(http.MethodPost)

	r.HandleFunc("/v1/networks/{network}/pools/{pool}/_bind", s.httpHandleBind).Methods(http.MethodPost)
	r.HandleFunc("/v1/networks/{network}/pools/{pool}/{addr}/_bind", s.httpHandleBind).Methods(http.MethodPost)

	r.HandleFunc("/v1/networks/{network}/pools/{pool}/bindings/{binding}", s.httpHandleRelease).Methods(http.MethodDelete)
	r.HandleFunc("/v1/networks/{network}/{addr}", s.httpHandleRelease).Methods(http.MethodDelete)

	s.r = r
	return r
}

func (s *PostalServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		rw.Header().Set("Access-Control-Allow-Origin", origin)
		rw.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		rw.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}
	// Stop here if its Preflighted OPTIONS request
	if req.Method == "OPTIONS" {
		return
	}

	s.r.ServeHTTP(rw, req)
}

func (s *PostalServer) httpHandleNetworks(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	switch req.Method {
	case http.MethodGet:
		rangeReq := &api.NetworkRangeRequest{
			ID:      vars["network"],
			Filters: map[string]string{},
		}
		for _, filter := range req.URL.Query()["f"] {
			if s := strings.Split(filter, "="); len(s) == 2 {
				rangeReq.Filters[s[0]] = s[1]
			}
		}

		resp, err := s.NetworkRange(context.Background(), rangeReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
	case http.MethodDelete:
	default:

	}

}

func (s *PostalServer) httpHandlePools(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	switch req.Method {
	case http.MethodGet:
		rangeReq := &api.PoolRangeRequest{
			ID: &api.Pool_PoolID{
				NetworkID: vars["network"],
				ID:        vars["pool"],
			},
			Filters: map[string]string{},
		}
		for _, filter := range req.URL.Query()["f"] {
			if s := strings.Split(filter, "="); len(s) == 2 {
				rangeReq.Filters[s[0]] = s[1]
			}
		}

		resp, err := s.PoolRange(context.Background(), rangeReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
	case http.MethodDelete:
	default:

	}
}

func (s *PostalServer) httpHandleBindings(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	switch req.Method {
	case http.MethodGet:
		rangeReq := &api.BindingRangeRequest{
			NetworkID: vars["network"],
			Filters:   map[string]string{},
		}
		if addr, ok := vars["addr"]; ok {
			rangeReq.Filters["_address"] = addr
		}
		if pool, ok := vars["pool"]; ok {
			rangeReq.Filters["_pool"] = pool
		}
		for _, filter := range req.URL.Query()["f"] {
			if s := strings.Split(filter, "="); len(s) == 2 {
				rangeReq.Filters[s[0]] = s[1]
			}
		}

		resp, err := s.BindingRange(context.Background(), rangeReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case http.MethodPost:
	case http.MethodDelete:
	default:

	}
}

func (s *PostalServer) httpHandleAllocate(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	allocateReq := &api.AllocateAddressRequest{
		PoolID: &api.Pool_PoolID{
			NetworkID: vars["network"],
			ID:        vars["pool"],
		},
		Address: vars["addr"],
	}
	resp, err := s.AllocateAddress(context.Background(), allocateReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *PostalServer) httpHandleBind(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	bindReq := &api.BindAddressRequest{
		PoolID: &api.Pool_PoolID{
			NetworkID: vars["network"],
			ID:        vars["pool"],
		},
		Address: vars["addr"],
	}
	resp, err := s.BindAddress(context.Background(), bindReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *PostalServer) httpHandleRelease(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	releaseReq := &api.ReleaseAddressRequest{
		PoolID: &api.Pool_PoolID{
			NetworkID: vars["network"],
			ID:        vars["pool"],
		},
	}

	if binding, ok := vars["binding"]; ok {
		releaseReq.BindingID = binding
	} else {
		releaseReq.Address = vars["addr"]
	}

	resp, err := s.ReleaseAddress(context.Background(), releaseReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
