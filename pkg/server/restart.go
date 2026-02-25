package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/dghubble/sling"
	"github.com/traPtitech/sakura-DevOpsBot/pkg/config"
)

type restartCommand struct {
}

func (sc *restartCommand) Execute(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("invalid arguments, expected server id or server name")
	}
	serverKey := strings.TrimSpace(args[0])
	if serverKey == "" {
		return fmt.Errorf("invalid arguments, expected non-empty server id or server name")
	}

	token := strings.TrimSpace(config.C.Servers.Sakura.BearerToken)
	if token == "" {
		return fmt.Errorf("API token has not been set yet")
	}

	if !strings.HasPrefix(config.C.Servers.Sakura.ServersAPIURLPath, "/") {
		config.C.Servers.Sakura.ServersAPIURLPath = "/" + config.C.Servers.Sakura.ServersAPIURLPath
	}

	serverID, serverName, err := resolveServerID(token, serverKey)
	if err != nil {
		return err
	}

	req, err := sling.New().
		Base(config.C.Servers.Sakura.Origin).
		Post(fmt.Sprintf("%s/%d/force-reboot", config.C.Servers.Sakura.ServersAPIURLPath, serverID)).
		Add("Authorization", "Bearer "+token).
		Request()
	if err != nil {
		return fmt.Errorf("failed to create restart request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return fmt.Errorf("failed to post restart request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("incorrect status code: %s", resp.Status)
	}

	log.Printf("Server restart requested successfully: ServerID=%d, ServerName=%s, Status=%s", serverID, serverName, resp.Status)

	return nil
}

func resolveServerID(token string, serverKey string) (int, string, error) {
	if serverID, err := strconv.Atoi(serverKey); err == nil {
		if serverID <= 0 {
			return 0, "", fmt.Errorf("invalid arguments, server id must be a positive integer: %q", serverKey)
		}
		return serverID, "(id specified)", nil
	}

	datasPerPage := 100
	page := 1
	matched := []serverResponse{}
	seen := map[int64]struct{}{}

	for {
		req, err := sling.New().
			Base(config.C.Servers.Sakura.Origin).
			Get(config.C.Servers.Sakura.ServersAPIURLPath).
			Add("Authorization", "Bearer "+token).
			QueryStruct(&params{PerPage: int64(datasPerPage), Page: int64(page), Search: serverKey}).
			Request()
		if err != nil {
			return 0, "", fmt.Errorf("failed to create server search request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("failed to search servers: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, "", fmt.Errorf("server search failed with status code: %s", resp.Status)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, "", fmt.Errorf("failed to read server search response body: %w", err)
		}

		var response serversResponse
		if err := json.Unmarshal(respBody, &response); err != nil {
			return 0, "", fmt.Errorf("failed to unmarshal server search response body: %w", err)
		}

		for _, server := range response.Results {
			if _, ok := seen[server.ID]; ok {
				continue
			}
			seen[server.ID] = struct{}{}
			matched = append(matched, server)
		}

		if response.Next == nil {
			break
		}
		page++
		if page > 100 {
			return 0, "", fmt.Errorf("too many pages while searching server: possible pagination loop")
		}
	}

	if len(matched) == 0 {
		return 0, "", fmt.Errorf("server not found by name: %q", serverKey)
	}

	exactMatches := make([]serverResponse, 0, len(matched))
	for _, server := range matched {
		if server.Name == serverKey {
			exactMatches = append(exactMatches, server)
		}
	}

	if len(exactMatches) == 1 {
		return int(exactMatches[0].ID), exactMatches[0].Name, nil
	}
	if len(exactMatches) > 1 {
		ids := make([]string, 0, len(exactMatches))
		for _, s := range exactMatches {
			ids = append(ids, strconv.FormatInt(s.ID, 10))
		}
		sort.Strings(ids)
		return 0, "", fmt.Errorf("multiple servers matched by exact name %q: ids=[%s]", serverKey, strings.Join(ids, ", "))
	}

	if len(matched) == 1 {
		return int(matched[0].ID), matched[0].Name, nil
	}

	candidates := make([]string, 0, len(matched))
	for _, s := range matched {
		candidates = append(candidates, fmt.Sprintf("%s(%d)", s.Name, s.ID))
	}
	sort.Strings(candidates)
	if len(candidates) > 10 {
		candidates = append(candidates[:10], "...")
	}

	return 0, "", fmt.Errorf("multiple servers matched by name %q. please specify id or exact name. candidates=[%s]", serverKey, strings.Join(candidates, ", "))
}
