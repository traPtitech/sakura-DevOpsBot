package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dghubble/sling"
	"github.com/mattn/go-runewidth"
	"github.com/traPtitech/sakura-DevOpsBot/pkg/config"
)

type hostsCommand struct {
}

type params struct {
	Page                 int64  `json:"page,omitempty" url:"page,omitempty"`
	PerPage              int64  `json:"per_page,omitempty" url:"per_page,omitempty"`
	ID                   string `json:"id,omitempty" url:"id,omitempty"`
	Switch               int64  `json:"switch,omitempty" url:"switch,omitempty"`
	ZoneCode             string `json:"zone_code,omitempty" url:"zone_code,omitempty"`
	ServiceType          string `json:"service_type,omitempty" url:"service_type,omitempty"`
	IPv4Address          string `json:"ipv4_address,omitempty" url:"ipv4_address,omitempty"`
	MonitoringResourceID string `json:"monitoring_resource_id,omitempty" url:"monitoring_resource_id,omitempty"`
	Sort                 string `json:"sort,omitempty" url:"sort,omitempty"`
	Search               string `json:"search,omitempty" url:"search,omitempty"`
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
		PrefixLen   *int64   `json:"prefixlen,omitempty"`
		Gateway     *string  `json:"gateway,omitempty"`
		NameServers []string `json:"nameservers"`
		HostName    *string  `json:"hostname,omitempty"`
		PTR         *string  `json:"ptr,omitempty"`
	} `json:"ipv6"`
	Contract struct {
		PlanCode    int64  `json:"plan_code"`
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
	servers := []resultData{}
	page := 1
	seen := map[int64]struct{}{}

	client := &http.Client{Timeout: 15 * time.Second}
	for {
		req, err := sling.New().
			Base(config.C.Servers.Sakura.Origin).
			Get(config.C.Servers.Sakura.ServersAPIURLPath).
			Add("Authorization", "Bearer "+config.C.Servers.Sakura.BearerToken).
			QueryStruct(&params{PerPage: int64(datasPerPage), Page: int64(page)}).
			Request()
		if err != nil {
			return fmt.Errorf("failed to create hosts request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get hosts: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("invalid status code: %s (expected: 200)", resp.Status)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		var response serversResponse
		if err := json.Unmarshal(respBody, &response); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}

		for _, server := range response.Results {
			if _, ok := seen[server.ID]; ok {
				continue
			}
			seen[server.ID] = struct{}{}

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

		if response.Next == nil {
			break
		}
		page++
		if page > 100 {
			return fmt.Errorf("too many pages: possible pagination loop")
		}
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	headers := []string{"NAME", "ID", "ZONE", "CPU", "IPv4", "IPv6", "MEM(MiB)", "STORAGE"}
	rows := make([][]string, 0, len(servers))

	for _, server := range servers {
		ipv6 := server.Ipv6
		if ipv6 == "" || ipv6 == "null" {
			ipv6 = "-"
		}
		rows = append(rows, []string{
			server.Name,
			fmt.Sprintf("%d", server.ID),
			server.Zone,
			fmt.Sprintf("%d", server.CpuCores),
			server.Ipv4,
			ipv6,
			fmt.Sprintf("%d", server.MemoryMiB),
			formatStorage(server.Storage),
		})
	}

	printAlignedTable(os.Stdout, headers, rows)
	log.Printf("Total servers: %d", len(servers))

	return nil
}

func printAlignedTable(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = runewidth.StringWidth(h)
	}
	for _, row := range rows {
		for i, c := range row {
			if cw := runewidth.StringWidth(c); cw > widths[i] {
				widths[i] = cw
			}
		}
	}

	writeRow := func(cols []string) {
		for i, c := range cols {
			if i == len(cols)-1 {
				fmt.Fprint(w, c)
				continue
			}
			pad := widths[i] - runewidth.StringWidth(c) + 2
			fmt.Fprint(w, c)
			fmt.Fprint(w, strings.Repeat(" ", pad))
		}
		fmt.Fprintln(w)
	}

	writeRow(headers)
	for _, row := range rows {
		writeRow(row)
	}
}

func formatStorage(storage []struct {
	Size int64
	Type string
}) string {
	if len(storage) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(storage))
	for _, s := range storage {
		parts = append(parts, fmt.Sprintf("%dGiB/%s", s.Size, s.Type))
	}
	return strings.Join(parts, ", ")
}
