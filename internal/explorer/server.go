package explorer

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	httpSwagger "github.com/swaggo/http-swagger"

	// swagger configuration
	_ "github.com/threefoldtech/grid_proxy_server/docs"
	"github.com/threefoldtech/grid_proxy_server/internal/explorer/db"
	"github.com/threefoldtech/grid_proxy_server/internal/explorer/mw"
	"github.com/threefoldtech/grid_proxy_server/pkg/types"
	"github.com/threefoldtech/zos/pkg/rmb"
)

const (
	// SSDOverProvisionFactor factor by which the ssd are allowed to be overprovisioned
	SSDOverProvisionFactor = 2
)

// getV1Farms godoc
// @Summary Show farms on the grid
// @Description Get all farms on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set farms' count on headers based on filter"
// @Param free_ips query int false "Min number of free ips in the farm"
// @Param total_ips query int false "Min number of total ips in the farm"
// @Param pricing_policy_id query int false "Pricing policy id"
// @Param version query int false "farm version"
// @Param farm_id query int false "farm id"
// @Param twin_id query int false "twin id associated with the farm"
// @Param name query string false "farm name"
// @Param name_contains query string false "farm name contains"
// @Param certification_type query string false "certificate type DIY or Certified"
// @Param dedicated query bool false "farm is dedicated"
// @Param stellar_address query string false "farm stellar_address"
// @Success 200 {object} []types.Farm
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /farms [get]
func (a *App) getV1Farms(r *http.Request) (interface{}, mw.Response) {
	return a.listFarms(r)
}

// getV2Farms godoc
// @Summary Show farms on the grid
// @Description Get all farms on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set farms' count on headers based on filter"
// @Param free_ips query int false "Min number of free ips in the farm"
// @Param total_ips query int false "Min number of total ips in the farm"
// @Param pricing_policy_id query int false "Pricing policy id"
// @Param version query int false "farm version"
// @Param farm_id query int false "farm id"
// @Param twin_id query int false "twin id associated with the farm"
// @Param name query string false "farm name"
// @Param name_contains query string false "farm name contains"
// @Param certification_type query string false "certificate type DIY or Certified"
// @Param dedicated query bool false "farm is dedicated"
// @Param stellar_address query string false "farm stellar_address"
// @Success 200 {object} []types.Farm
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/farms [get]
func (a *App) getV2Farms(r *http.Request) (interface{}, mw.Response) {
	return a.listFarms(r)
}

func (a *App) listFarms(r *http.Request) (interface{}, mw.Response) {
	filter, limit, err := a.handleFarmRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	dbFarms, farmsCount, err := a.db.GetFarms(filter, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to query farm")
		return nil, mw.Error(err)
	}
	farms := make([]types.Farm, 0, len(dbFarms))
	for _, farm := range dbFarms {
		f, err := farmFromDBFarm(farm)
		if err != nil {
			log.Err(err).Msg("couldn't convert db farm to api farm")
		}
		farms = append(farms, f)
	}
	resp := mw.Ok()

	// return the number of pages and totalCount in the response headers
	if limit.RetCount {
		pages := math.Ceil(float64(farmsCount) / float64(limit.Size))
		resp = resp.WithHeader("count", fmt.Sprintf("%d", farmsCount)).
			WithHeader("size", fmt.Sprintf("%d", limit.Size)).
			WithHeader("pages", fmt.Sprintf("%d", int(pages)))
	}
	return farms, resp
}

// getV1Stats godoc
// @Summary Show stats about the grid
// @Description Get statistics about the grid
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Success 200 {object} []types.Counters
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /stats [get]
func (a *App) getV1Stats(r *http.Request) (interface{}, mw.Response) {
	return a.loadStats(r)
}

// getV2Stats godoc
// @Summary Show stats about the grid
// @Description Get statistics about the grid
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Success 200 {object} []types.Counters
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/stats [get]
func (a *App) getV2Stats(r *http.Request) (interface{}, mw.Response) {
	return a.loadStats(r)
}

func (a *App) loadStats(r *http.Request) (interface{}, mw.Response) {
	filter, err := a.handleStatsRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	counters, err := a.db.GetCounters(filter)
	if err != nil {
		return nil, mw.Error(err)
	}
	return counters, nil
}

// getV1Nodes godoc
// @Summary Show nodes on the grid
// @Description Get all nodes on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set nodes' count on headers based on filter"
// @Param free_mru query int false "Min free reservable mru in bytes"
// @Param free_hru query int false "Min free reservable hru in bytes"
// @Param free_sru query int false "Min free reservable sru in bytes"
// @Param free_ips query int false "Min number of free ips in the farm of the node"
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Param city query string false "Node city filter"
// @Param country query string false "Node country filter"
// @Param farm_name query string false "Get nodes for specific farm"
// @Param ipv4 query bool false "Set to true to filter nodes with ipv4"
// @Param ipv6 query bool false "Set to true to filter nodes with ipv6"
// @Param domain query bool false "Set to true to filter nodes with domain"
// @Param dedicated query bool false "Set to true to get the dedicated nodes only"
// @Param rentable query bool false "Set to true to filter the available nodes for renting"
// @Param rented query bool false "Set to true to filter rented nodes"
// @Param rented_by query int false "rented by twin id"
// @Param available_for query int false "available for twin id"
// @Param farm_ids query string false "List of farms separated by comma to fetch nodes from (e.g. '1,2,3')"
// @Success 200 {object} []types.Node
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /nodes [get]
func (a *App) getV1Nodes(r *http.Request) (interface{}, mw.Response) {
	return a.listNodes(r)
}

// getV2Nodes godoc
// @Summary Show nodes on the grid
// @Description Get all nodes on the grid, It has pagination. Nodes displayed with nested capacity object
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set nodes' count on headers based on filter"
// @Param free_mru query int false "Min free reservable mru in bytes"
// @Param free_hru query int false "Min free reservable hru in bytes"
// @Param free_sru query int false "Min free reservable sru in bytes"
// @Param free_ips query int false "Min number of free ips in the farm of the node"
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Param city query string false "Node city filter"
// @Param country query string false "Node country filter"
// @Param farm_name query string false "Get nodes for specific farm"
// @Param ipv4 query bool false "Set to true to filter nodes with ipv4"
// @Param ipv6 query bool false "Set to true to filter nodes with ipv6"
// @Param domain query bool false "Set to true to filter nodes with domain"
// @Param dedicated query bool false "Set to true to get the dedicated nodes only"
// @Param rentable query bool false "Set to true to filter the available nodes for renting"
// @Param rented query bool false "Set to true to filter rented nodes"
// @Param rented_by query int false "rented by twin id"
// @Param available_for query int false "available for twin id"
// @Param farm_ids query string false "List of farms separated by comma to fetch nodes from (e.g. '1,2,3')"
// @Success 200 {object} []types.NodeWithNestedCapacity
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/nodes [get]
func (a *App) getV2Nodes(r *http.Request) (interface{}, mw.Response) {
	return a.listNodesWithNestedCapacity(r)
}

// getV1Gateways godoc
// @Summary Show gateways on the grid
// @Description Get all gateways on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set nodes' count on headers based on filter"
// @Param free_mru query int false "Min free reservable mru in bytes"
// @Param free_hru query int false "Min free reservable hru in bytes"
// @Param free_sru query int false "Min free reservable sru in bytes"
// @Param free_ips query int false "Min number of free ips in the farm of the node"
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Param city query string false "Node city filter"
// @Param country query string false "Node country filter"
// @Param farm_name query string false "Get nodes for specific farm"
// @Param ipv4 query bool false "Set to true to filter nodes with ipv4"
// @Param ipv6 query bool false "Set to true to filter nodes with ipv6"
// @Param domain query bool false "Set to true to filter nodes with domain"
// @Param dedicated query bool false "Set to true to get the dedicated nodes only"
// @Param rentable query bool false "Set to true to filter the available nodes for renting"
// @Param rented query bool false "Set to true to filter rented nodes"
// @Param rented_by query int false "rented by twin id"
// @Param available_for query int false "available for twin id"
// @Param farm_ids query string false "List of farms separated by comma to fetch nodes from (e.g. '1,2,3')"
// @Success 200 {object} []types.Node
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /gateways [get]
func (a *App) getV1Gateways(r *http.Request) (interface{}, mw.Response) {
	return a.listNodes(r)
}

// getV2Gateways godoc
// @Summary Show gateways on the grid
// @Description Get all gateways on the grid, It has pagination. Nodes displayed with nested capacity object
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set nodes' count on headers based on filter"
// @Param free_mru query int false "Min free reservable mru in bytes"
// @Param free_hru query int false "Min free reservable hru in bytes"
// @Param free_sru query int false "Min free reservable sru in bytes"
// @Param free_ips query int false "Min number of free ips in the farm of the node"
// @Param status query string false "Node status filter, 'up': for only up nodes & 'down': for all up/down nodes."
// @Param city query string false "Node city filter"
// @Param country query string false "Node country filter"
// @Param farm_name query string false "Get nodes for specific farm"
// @Param ipv4 query bool false "Set to true to filter nodes with ipv4"
// @Param ipv6 query bool false "Set to true to filter nodes with ipv6"
// @Param domain query bool false "Set to true to filter nodes with domain"
// @Param dedicated query bool false "Set to true to get the dedicated nodes only"
// @Param rentable query bool false "Set to true to filter the available nodes for renting"
// @Param rented query bool false "Set to true to filter rented nodes"
// @Param rented_by query int false "rented by twin id"
// @Param available_for query int false "available for twin id"
// @Param farm_ids query string false "List of farms separated by comma to fetch nodes from (e.g. '1,2,3')"
// @Success 200 {object} []types.NodeWithNestedCapacity
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/gateways [get]
func (a *App) getV2Gateways(r *http.Request) (interface{}, mw.Response) {
	return a.listNodesWithNestedCapacity(r)
}

func (a *App) listNodes(r *http.Request) (interface{}, mw.Response) {
	filter, limit, err := a.handleNodeRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	dbNodes, nodesCount, err := a.db.GetNodes(filter, limit)
	if err != nil {
		return nil, mw.Error(err)
	}
	nodes := make([]types.Node, len(dbNodes))
	for idx, node := range dbNodes {
		nodes[idx] = nodeFromDBNode(node)
	}
	resp := mw.Ok()

	// return the number of pages and totalCount in the response headers
	if limit.RetCount {
		pages := math.Ceil(float64(nodesCount) / float64(limit.Size))
		resp = resp.WithHeader("count", fmt.Sprintf("%d", nodesCount)).
			WithHeader("size", fmt.Sprintf("%d", limit.Size)).
			WithHeader("pages", fmt.Sprintf("%d", int(pages)))
	}
	return nodes, resp
}

func (a *App) listNodesWithNestedCapacity(r *http.Request) (interface{}, mw.Response) {
	filter, limit, err := a.handleNodeRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	dbNodes, nodesCount, err := a.db.GetNodes(filter, limit)
	if err != nil {
		return nil, mw.Error(err)
	}
	nodes := make([]types.NodeWithNestedCapacity, len(dbNodes))
	for idx, node := range dbNodes {
		nodes[idx] = nodeWithNestedCapacityFromDBNode(node)
	}
	resp := mw.Ok()

	// return the number of pages and totalCount in the response headers
	if limit.RetCount {
		pages := math.Ceil(float64(nodesCount) / float64(limit.Size))
		resp = resp.WithHeader("count", fmt.Sprintf("%d", nodesCount)).
			WithHeader("size", fmt.Sprintf("%d", limit.Size)).
			WithHeader("pages", fmt.Sprintf("%d", int(pages)))
	}
	return nodes, resp
}

// getV1Node godoc
// @Summary Show the details for specific node
// @Description Get all details for specific node hardware, capacity, DMI, hypervisor
// @Tags GridProxy
// @Param node_id path int false "Node ID"
// @Accept  json
// @Produce  json
// @Success 200 {object} types.Node
// @Failure 400 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /nodes/{node_id} [get]
func (a *App) getV1Node(r *http.Request) (interface{}, mw.Response) {
	return a.loadNode(r)
}

// getV2Node godoc
// @Summary Show the details for specific node
// @Description Get all details for specific node hardware, capacity, DMI, hypervisor
// @Tags GridProxy
// @Param node_id path int false "Node ID"
// @Accept  json
// @Produce  json
// @Success 200 {object} types.Node
// @Failure 400 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /api/v2/nodes/{node_id} [get]
func (a *App) getV2Node(r *http.Request) (interface{}, mw.Response) {
	return a.loadNode(r)
}

// getV1Gateway godoc
// @Summary Show the details for specific gateway
// @Description Get all details for specific gateway hardware, capacity, DMI, hypervisor
// @Tags GridProxy
// @Param node_id path int false "Node ID"
// @Accept  json
// @Produce  json
// @Success 200 {object} types.Node
// @Failure 400 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /gateways/{node_id} [get]
func (a *App) getV1Gateway(r *http.Request) (interface{}, mw.Response) {
	return a.loadNode(r)
}

// getV2Gateway godoc
// @Summary Show the details for specific gateway
// @Description Get all details for specific gateway hardware, capacity, DMI, hypervisor
// @Tags GridProxy
// @Param node_id path int false "Node ID"
// @Accept  json
// @Produce  json
// @Success 200 {object} types.Node
// @Failure 400 {object} string
// @Failure 404 {object} string
// @Failure 500 {object} string
// @Router /api/v2/gateways/{node_id} [get]
func (a *App) getV2Gateway(r *http.Request) (interface{}, mw.Response) {
	return a.loadNode(r)
}

func (a *App) loadNode(r *http.Request) (interface{}, mw.Response) {
	nodeID := mux.Vars(r)["node_id"]
	nodeData, err := a.getNodeData(nodeID)
	if err != nil {
		return nil, errorReply(err)
	}
	return nodeData, nil
}

func (a *App) getNodeStatus(r *http.Request) (interface{}, mw.Response) {
	response := types.NodeStatus{}
	nodeID := mux.Vars(r)["node_id"]

	nodeData, err := a.getNodeData(nodeID)
	if err != nil {
		return nil, errorReply(err)
	}
	response.Status = nodeData.Status
	return response, nil
}

// getV2Twins godoc
// @Summary Show twins on the grid
// @Description Get all twins on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set twins' count on headers based on filter"
// @Param twin_id query int false "twin id"
// @Param account_id query string false "account address"
// @Success 200 {object} []types.Twin
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /twins [get]
func (a *App) getV1Twins(r *http.Request) (interface{}, mw.Response) {
	return a.listTwins(r)
}

// getV2Twins godoc
// @Summary Show twins on the grid
// @Description Get all twins on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set twins' count on headers based on filter"
// @Param twin_id query int false "twin id"
// @Param account_id query string false "account address"
// @Success 200 {object} []types.Twin
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/twins [get]
func (a *App) getV2Twins(r *http.Request) (interface{}, mw.Response) {
	return a.listTwins(r)
}

func (a *App) listTwins(r *http.Request) (interface{}, mw.Response) {
	filter, limit, err := a.handleTwinRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	twins, twinsCount, err := a.db.GetTwins(filter, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to query twin")
		return nil, mw.Error(err)
	}

	resp := mw.Ok()

	// return the number of pages and totalCount in the response headers
	if limit.RetCount {
		pages := math.Ceil(float64(twinsCount) / float64(limit.Size))
		resp = resp.WithHeader("count", fmt.Sprintf("%d", twinsCount)).
			WithHeader("size", fmt.Sprintf("%d", limit.Size)).
			WithHeader("pages", fmt.Sprintf("%d", int(pages)))
	}
	return twins, resp
}

// getV1Contracts godoc
// @Summary Show contracts on the grid
// @Description Get all contracts on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set contracts' count on headers based on filter"
// @Param contract_id query int false "contract id"
// @Param twin_id query int false "twin id"
// @Param node_id query int false "node id which contract is deployed on in case of ('rent' or 'node' contracts)"
// @Param name query string false "contract name in case of 'name' contracts"
// @Param type query string false "contract type 'node', 'name', or 'rent'"
// @Param state query string false "contract state 'Created', 'GracePeriod', or 'Deleted'"
// @Param deployment_data query string false "contract deployment data in case of 'node' contracts"
// @Param deployment_hash query string false "contract deployment hash in case of 'node' contracts"
// @Param number_of_public_ips query int false "Min number of public ips in the 'node' contract"
// @Success 200 {object} []types.Contract
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /contracts [get]
func (a *App) getV1Contracts(r *http.Request) (interface{}, mw.Response) {
	return a.listContracts(r)
}

// getV2Contracts godoc
// @Summary Show contracts on the grid
// @Description Get all contracts on the grid, It has pagination
// @Tags GridProxy
// @Accept  json
// @Produce  json
// @Param page query int false "Page number"
// @Param size query int false "Max result per page"
// @Param ret_count query bool false "Set contracts' count on headers based on filter"
// @Param contract_id query int false "contract id"
// @Param twin_id query int false "twin id"
// @Param node_id query int false "node id which contract is deployed on in case of ('rent' or 'node' contracts)"
// @Param name query string false "contract name in case of 'name' contracts"
// @Param type query string false "contract type 'node', 'name', or 'rent'"
// @Param state query string false "contract state 'Created', 'GracePeriod', or 'Deleted'"
// @Param deployment_data query string false "contract deployment data in case of 'node' contracts"
// @Param deployment_hash query string false "contract deployment hash in case of 'node' contracts"
// @Param number_of_public_ips query int false "Min number of public ips in the 'node' contract"
// @Success 200 {object} []types.Contract
// @Failure 400 {object} string
// @Failure 500 {object} string
// @Router /api/v2/contracts [get]
func (a *App) getV2Contracts(r *http.Request) (interface{}, mw.Response) {
	return a.listContracts(r)
}

func (a *App) listContracts(r *http.Request) (interface{}, mw.Response) {
	filter, limit, err := a.handleContractRequestsQueryParams(r)
	if err != nil {
		return nil, mw.BadRequest(err)
	}
	dbContracts, contractsCount, err := a.db.GetContracts(filter, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to query contract")
		return nil, mw.Error(err)
	}

	contracts := make([]types.Contract, len(dbContracts))
	for idx, contract := range dbContracts {
		contracts[idx], err = contractFromDBContract(contract)
		if err != nil {
			log.Err(err).Msg("failed to convert db contract to api contract")
		}
	}
	resp := mw.Ok()

	// return the number of pages and totalCount in the response headers
	if limit.RetCount {
		pages := math.Ceil(float64(contractsCount) / float64(limit.Size))
		resp = resp.WithHeader("count", fmt.Sprintf("%d", contractsCount)).
			WithHeader("size", fmt.Sprintf("%d", limit.Size)).
			WithHeader("pages", fmt.Sprintf("%d", int(pages)))
	}
	return contracts, resp
}
func (a *App) indexPage(m *mux.Router) mw.Action {
	return func(r *http.Request) (interface{}, mw.Response) {
		response := mw.Ok()
		var sb strings.Builder
		sb.WriteString("Welcome to threefold grid proxy server, available endpoints ")

		_ = m.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			path, err := route.GetPathTemplate()
			if err != nil {
				return nil
			}

			sb.WriteString("[" + path + "] ")
			return nil
		})
		return sb.String(), response
	}
}

func (a *App) version(r *http.Request) (interface{}, mw.Response) {
	response := mw.Ok()
	return types.Version{
		Version: a.releaseVersion,
	}, response
}

// Setup is the server and do initial configurations
// @title Grid Proxy Server API
// @version 1.0
// @description grid proxy server has the main methods to list farms, nodes, node details in the grid.
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /
func Setup(router *mux.Router, redisServer string, gitCommit string, database db.Database) error {
	log.Info().Str("redis address", redisServer).Msg("Preparing Redis Pool ...")

	rmbClient, err := rmb.NewClient(redisServer, 500)
	if err != nil {
		return errors.Wrap(err, "couldn't connect to rmb")
	}
	c := cache.New(2*time.Minute, 3*time.Minute)
	a := App{
		db:             database,
		rmb:            rmbClient,
		lruCache:       c,
		releaseVersion: gitCommit,
	}

	// v1 endpoints
	router.HandleFunc("/", mw.AsHandlerFunc(a.indexPage(router)))
	router.HandleFunc("/version", mw.AsHandlerFunc(a.version))
	router.PathPrefix("/swagger").Handler(httpSwagger.WrapHandler)
	router.HandleFunc("/stats", mw.AsHandlerFunc(a.getV1Stats))

	router.HandleFunc("/farms", mw.AsHandlerFunc(a.getV1Farms))
	router.HandleFunc("/nodes", mw.AsHandlerFunc(a.getV1Nodes))
	router.HandleFunc("/gateways", mw.AsHandlerFunc(a.getV1Gateways))
	router.HandleFunc("/twins", mw.AsHandlerFunc(a.getV1Twins))
	router.HandleFunc("/contracts", mw.AsHandlerFunc(a.getV1Contracts))
	router.HandleFunc("/nodes/{node_id:[0-9]+}", mw.AsHandlerFunc(a.getV1Node))
	router.HandleFunc("/gateways/{node_id:[0-9]+}", mw.AsHandlerFunc(a.getV1Gateway))
	router.HandleFunc("/nodes/{node_id:[0-9]+}/status", mw.AsHandlerFunc(a.getNodeStatus))
	router.HandleFunc("/gateways/{node_id:[0-9]+}/status", mw.AsHandlerFunc(a.getNodeStatus))

	// v2 endpoints
	router.HandleFunc("/api/v2/", mw.AsHandlerFunc(a.indexPage(router)))
	router.HandleFunc("/api/v2/version", mw.AsHandlerFunc(a.version))
	router.PathPrefix("/api/v2/swagger").Handler(httpSwagger.WrapHandler)
	router.HandleFunc("/api/v2/stats", mw.AsHandlerFunc(a.getV2Stats))

	router.HandleFunc("/api/v2/farms", mw.AsHandlerFunc(a.getV2Farms))
	router.HandleFunc("/api/v2/nodes", mw.AsHandlerFunc(a.getV2Nodes))
	router.HandleFunc("/api/v2/gateways", mw.AsHandlerFunc(a.getV2Gateways))
	router.HandleFunc("/api/v2/twins", mw.AsHandlerFunc(a.getV2Twins))
	router.HandleFunc("/api/v2/contracts", mw.AsHandlerFunc(a.getV2Contracts))
	router.HandleFunc("/api/v2/nodes/{node_id:[0-9]+}", mw.AsHandlerFunc(a.getV2Node))
	router.HandleFunc("/api/v2/gateways/{node_id:[0-9]+}", mw.AsHandlerFunc(a.getV2Gateway))
	router.HandleFunc("/api/v2/nodes/{node_id:[0-9]+}/status", mw.AsHandlerFunc(a.getNodeStatus))
	router.HandleFunc("/api/v2/gateways/{node_id:[0-9]+}/status", mw.AsHandlerFunc(a.getNodeStatus))

	return nil
}
