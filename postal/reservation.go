/*
Copyright 2016 Jive Communications All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postal

import (
	"encoding/json"
	"net"
	"regexp"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/jive/postal/api"
	"github.com/pkg/errors"
)

func (nm *etcdNetworkManager) Reservations(filters map[string]string) ([]*api.Reservation, error) {
	resp, err := nm.etcd.KV.Get(context.Background(), networkListReservationsKey(nm.ID), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	noFilter := filters == nil || len(filters) == 0

	reservations := []*api.Reservation{}
	for idx := range resp.Kvs {
		reservation := &api.Reservation{}
		err = json.Unmarshal(resp.Kvs[idx].Value, reservation)
		if err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal reservation")
		}

		if noFilter {
			reservations = append(reservations, reservation)
		} else {
			var matched bool
			for field, filter := range filters {
				switch field {
				case "_cidr":
					matched, err = regexp.MatchString(filter, reservation.Cidr)
				case "_network":
					matched, err = regexp.MatchString(filter, reservation.NetworkID)
				default:
					if val, ok := reservation.Annotations[field]; ok {
						matched, err = regexp.MatchString(filter, val)
					} else {
						break
					}
				}
				if err != nil {
					return nil, errors.Wrapf(err, "failed to compile filter '%s'", filter)
				}

				if !matched {
					break
				}
			}

			if matched {
				reservations = append(reservations, reservation)
			}
		}
	}

	return reservations, nil
}

func (nm *etcdNetworkManager) AddReservation(cidr string, annotations map[string]string) (*api.Reservation, error) {
	reservation := &api.Reservation{
		NetworkID:   nm.ID,
		Cidr:        cidr,
		Annotations: annotations,
	}

	data, err := json.Marshal(reservation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal reservation")
	}

	resp, err := nm.etcd.KV.Txn(context.TODO()).If(
		clientv3.Compare(
			clientv3.Version(networkReservationKey(reservation.NetworkID, reservation.Cidr)),
			"=",
			0,
		),
	).Then(
		clientv3.OpPut(
			networkReservationKey(reservation.NetworkID, reservation.Cidr),
			string(data),
		),
	).Commit()

	if err != nil {
		return nil, errors.Wrap(err, "failed to commit reservation")
	}

	if resp.Succeeded {
		return reservation, nil
	}

	return nil, errors.New("reservation already exists")
}

func (nm *etcdNetworkManager) RemoveReservation(cidr string) error {
	resp, err := nm.etcd.KV.Txn(context.TODO()).If(
		clientv3.Compare(
			clientv3.Version(networkReservationKey(nm.ID, cidr)),
			">",
			0,
		),
	).Then(
		clientv3.OpDelete(
			networkReservationKey(nm.ID, cidr),
		),
	).Commit()

	if err != nil {
		return errors.Wrap(err, "failed to delete reservation")
	}

	if resp.Succeeded {

		return nil
	}

	return errors.New("reservation not found")
}

func (nm *etcdNetworkManager) IsReserved(ip net.IP) (bool, error) {
	reservations, err := nm.Reservations(nil)
	if err != nil {
		return false, errors.Wrap(err, "could not fetch reservatons")
	}

	for idx := range reservations {
		_, ipnet, _ := net.ParseCIDR(reservations[idx].Cidr)
		if ipnet != nil && ipnet.Contains(ip) {
			return true, nil
		}
	}

	return false, nil
}
