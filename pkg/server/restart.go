package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/dghubble/sling"
	"github.com/traPtitech/sakura-DevOpsBot/pkg/config"
)

type restartCommand struct {
}

func (sc *restartCommand) Execute(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("invalid arguments, expected server id")
	}

	serverID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid arguments, server id must be an integer")
	}

	if len(config.C.Servers.Sakura.BearerToken) == 0 {
		return fmt.Errorf("API token has not been set yet")
	}

	req, err := sling.New().
		Base(config.C.Servers.Sakura.Origin).
		Post(fmt.Sprintf("%s/%d/force-reboot", config.C.Servers.Sakura.ServersAPIURLPath, serverID)).
		Add("Authorization", "Bearer "+config.C.Servers.Sakura.BearerToken).
		Request()
	if err != nil {
		return fmt.Errorf("failed to create restart request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return fmt.Errorf("failed to post restart request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	logStr := fmt.Sprintf(`Request
- URL: %s

Response
- Header: %+v
- Body: %s
- Status: %s (Expected: 202)
`, req.URL.String(), resp.Header, string(respBody), resp.Status)
	log.Println(logStr)

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("incorrect status code: %s", resp.Status)
	}

	return nil
}
