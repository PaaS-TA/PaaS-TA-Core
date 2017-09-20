package tps

import "github.com/tedsuo/rata"

const (
	LRPStatus     = "LRPStatus"
	LRPStats      = "LRPStats"
	BulkLRPStatus = "BulkLRPStatus"
)

var Routes = rata.Routes{
	{Path: "/v1/bulk_actual_lrp_status", Method: "GET", Name: BulkLRPStatus},
	{Path: "/v1/actual_lrps/:guid", Method: "GET", Name: LRPStatus},
	{Path: "/v1/actual_lrps/:guid/stats", Method: "GET", Name: LRPStats},
}
