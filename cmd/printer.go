package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jive/postal/api"
)

type printer interface {
	NetworkAdd(*api.NetworkAddResponse)
	PoolAdd(*api.PoolAddResponse)

	NetworkRange(*api.NetworkRangeResponse)
	PoolRange(*api.PoolRangeResponse)
	BindingRange(*api.BindingRangeResponse)

	AllocateAddress(*api.AllocateAddressResponse)
	BulkAllocateAddress(*api.BulkAllocateAddressResponse)
	BindAddress(*api.BindAddressResponse)
	ReleaseAddress(*api.ReleaseAddressResponse)
	PoolSetMax(*api.PoolSetMaxResponse)
}

func NewPrinter(printerType string) printer {
	switch printerType {
	case "simple":
		return &simplePrinter{}
	}
	return nil
}

type simplePrinter struct{}

func (s *simplePrinter) NetworkAdd(resp *api.NetworkAddResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintf(
		w,
		"id:%s\tcidr:%s\tannotations:%s\n",
		resp.Network.ID, resp.Network.Cidr,
		strings.Join(flattenAnnotations(resp.Network.Annotations), ","))
	w.Flush()
}

func (s *simplePrinter) PoolAdd(resp *api.PoolAddResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintf(
		w,
		"network_id:%s\tpool_id:%s\tmax:%d\ttype:%s\tannotations:%s\n",
		resp.Pool.ID.NetworkID, resp.Pool.ID.ID,
		resp.Pool.MaximumAddresses, resp.Pool.Type.String(),
		strings.Join(flattenAnnotations(resp.Pool.Annotations), ","))
	w.Flush()
}

func (s *simplePrinter) NetworkRange(resp *api.NetworkRangeResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "network_id\tcidr\tannotations")
	for _, n := range resp.Networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n", n.ID, n.Cidr, strings.Join(flattenAnnotations(n.Annotations), ","))
	}
	w.Flush()
}

func (s *simplePrinter) PoolRange(resp *api.PoolRangeResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "network_id\tpool_id\tmax\ttype\tannotations")
	for _, p := range resp.Pools {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			p.ID.NetworkID, p.ID.ID,
			p.MaximumAddresses, p.Type.String(),
			strings.Join(flattenAnnotations(p.Annotations), ","))
	}
	w.Flush()
}

func (s *simplePrinter) BindingRange(resp *api.BindingRangeResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "network_id\tpool_id\tbinding_id\taddress\tAalocated\tbound\treleased\tannotations")
	for _, b := range resp.Bindings {
		s.binding(w, b)
	}
	w.Flush()
}

func (s *simplePrinter) PoolSetMax(resp *api.PoolSetMaxResponse) {

}

func (s *simplePrinter) AllocateAddress(resp *api.AllocateAddressResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "network_id\tpool_id\tbinding_id\taddress\tAalocated\tbound\treleased\tannotations")
	s.binding(w, resp.Binding)
}

func (s *simplePrinter) BulkAllocateAddress(resp *api.BulkAllocateAddressResponse) {
	if len(resp.Errors) > 0 {
		fmt.Printf("%d of %v addresses successfully allocated to pool.\n", len(resp.Bindings), len(resp.Bindings)+len(resp.Errors))
		fmt.Println("The following addresses failed to allocate:")
		for ip, berr := range resp.Errors {
			fmt.Printf("---> %s: %s\n", ip, berr.Message)
		}
	} else {
		fmt.Printf("All %v addresses successfully allocated to pool.\n", len(resp.Bindings))
	}
}

func (s *simplePrinter) BindAddress(resp *api.BindAddressResponse) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "network_id\tpool_id\tbinding_id\taddress\tAalocated\tbound\treleased\tannotations")
	s.binding(w, resp.Binding)
}

func (s *simplePrinter) ReleaseAddress(resp *api.ReleaseAddressResponse) {}

func (s *simplePrinter) binding(w *tabwriter.Writer, b *api.Binding) {
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		b.PoolID.NetworkID, b.PoolID.ID, b.ID, b.Address,
		s.formatTime(time.Unix(0, b.AllocateTime)),
		s.formatTime(time.Unix(0, b.BindTime)),
		s.formatTime(time.Unix(0, b.ReleaseTime)),
		strings.Join(flattenAnnotations(b.Annotations), ","))
}

func (s *simplePrinter) formatTime(t time.Time) string {
	if t.Unix() == 0 {
		return ""
	}

	if human {
		return humanize.Time(t)
	}

	return t.String()
}
