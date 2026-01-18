package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/dghubble/sling"
	"github.com/traPtitech/sakura-DevOpsBot/pkg/config"
)

type hostsCommand struct {
}

type params struct {
	Page                 int64  `json:"page,omitempty"`
	PerPage              int64  `json:"per_page,omitempty"`
	ID                   string `json:"id,omitempty"`
	Switch               int64  `json:"switch,omitempty"`
	ZoneCode             string `json:"zone_code,omitempty"`
	ServiceType          string `json:"service_type,omitempty"`
	IPv4Address          string `json:"ipv4_address,omitempty"`
	MonitoringResourceID string `json:"monitoring_resource_id,omitempty"`
	Sort                 string `json:"sort,omitempty"`
	Search               string `json:"search,omitempty"`
}

type serversResponse struct {
	Results  []serverResponse `json:"results"`
	Count    int64            `json:"count"`
	Next     *string          `json:"next,omitempty"`
	Previous *string          `json:"previous,omitempty"`
}

type serverResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ServiceType   string `json:"service_type"`
	ServiceStatus string `json:"service_status"`
	CPUCores      int64  `json:"cpu_cores"`
	MemoryMiB     int64  `json:"memory_mebibytes"`
	Storage       []struct {
		Port    int64  `json:"port"`
		Type    string `json:"type"`
		SizeGiB int64  `json:"size_gibibytes"`
	} `json:"storage"`
	Zone struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"zone"`
	Options []string `json:"options"`
	Version string   `json:"version"`
	Ipv4    struct {
		Address     string   `json:"address"`
		Netmask     string   `json:"netmask"`
		Gateway     string   `json:"gateway"`
		NameServers []string `json:"nameservers"`
		HostName    string   `json:"hostname"`
		PTR         string   `json:"ptr"`
	} `json:"ipv4"`
	Ipv6 struct {
		Address     *string  `json:"address,omitempty"`
		PrefixLen   *string  `json:"prefixlen,omitempty"`
		Gateway     *string  `json:"gateway,omitempty"`
		NameServers []string `json:"nameservers"`
		HostName    *string  `json:"hostname,omitempty"`
		PTR         *string  `json:"ptr,omitempty"`
	} `json:"ipv6"`
	Contract struct {
		PlanCode    string `json:"plan_code"`
		PlanName    string `json:"plan_name"`
		ServiceCode string `json:"service_code"`
	} `json:"contract"`
	PowerStatus     string  `json:"power_status"`
	MigrationStatus *string `json:"migration_status,omitempty"`
}

type resultData struct {
	ID        int64
	Zone      string
	Name      string
	Ipv4      string
	Ipv6      string
	CpuCores  int64
	MemoryMiB int64
	Storage   []struct {
		Size int64
		Type string
	}
}

func (sc *hostsCommand) Execute(args []string) error {
	if len(strings.TrimSpace(config.C.Servers.Sakura.BearerToken)) == 0 {
		return fmt.Errorf("API token has not been set yet")
	}

	if !strings.HasPrefix(config.C.Servers.Sakura.ServersAPIURLPath, "/") {
		config.C.Servers.Sakura.ServersAPIURLPath = "/" + config.C.Servers.Sakura.ServersAPIURLPath
	}

	datasPerPage := 100
	req, err := sling.New().
		Base(config.C.Servers.Sakura.Origin).
		Get(config.C.Servers.Sakura.ServersAPIURLPath).
		Add("Authorization", "Bearer "+config.C.Servers.Sakura.BearerToken).
		QueryStruct(&params{PerPage: int64(datasPerPage)}).
		Request()
	if err != nil {
		return fmt.Errorf("failed to create hosts request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get hosts: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %s (expected: 200)", resp.Status)
	}

	var response serversResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	servers := []resultData{}

	for _, server := range response.Results {
		serverData := resultData{
			ID:        server.ID,
			Zone:      server.Zone.Name,
			Name:      server.Name,
			Ipv4:      server.Ipv4.Address,
			CpuCores:  server.CPUCores,
			MemoryMiB: server.MemoryMiB,
		}
		if server.Ipv6.Address != nil {
			serverData.Ipv6 = *server.Ipv6.Address
		} else {
			serverData.Ipv6 = "null"
		}
		for _, storage := range server.Storage {
			serverData.Storage = append(serverData.Storage, struct {
				Size int64
				Type string
			}{storage.SizeGiB, storage.Type})
		}
		servers = append(servers, serverData)
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	log.Printf("Server Name, \t Server ID, \t Zone Name, \t cpu cores, \t ipv4 address, \t ipv6 address, \t Memory Size (MiB), \t Storage Size (GiB) / type")
	for _, server := range servers {
		logMsg := fmt.Sprintf("%s, \t %d, \t %s, \t %d, \t %s, \t %s, \t %dMiB", server.Name, server.ID, server.Zone, server.CpuCores, server.Ipv4, server.Ipv6, server.MemoryMiB)
		for _, storage := range server.Storage {
			logMsg += fmt.Sprintf(", \t %dGiB / %s", storage.Size, storage.Type)
		}
		log.Printf(logMsg)
	}
	if response.Count > int64(datasPerPage) {
		log.Printf("Not all results are displayed. Total count: %d", response.Count)
	}

	return nil
}
