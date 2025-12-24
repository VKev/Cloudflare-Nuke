package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/zones"

	"github.com/arafato/cf-nuke/infrastructure"
	"github.com/arafato/cf-nuke/types"
	"github.com/arafato/cf-nuke/utils"
)

func init() {
	infrastructure.RegisterCollector("cache-purge", CollectCachePurges)
}

type CachePurge struct {
	Client   *http.Client
	Mode     types.Mode
	APIToken string
	APIKey   string
	APIEmail string
}

type cachePurgeRequest struct {
	PurgeEverything bool `json:"purge_everything"`
}

type cachePurgeResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func CollectCachePurges(creds *types.Credentials) (types.Resources, error) {
	client := utils.CreateCFClient(creds)

	zonePage, err := client.Zones.List(context.TODO(), zones.ZoneListParams{
		Account: cloudflare.F(zones.ZoneListParamsAccount{ID: cloudflare.F(creds.AccountID)}),
	})
	if err != nil {
		return nil, err
	}

	var zonesList []zones.Zone
	for len(zonePage.Result) != 0 {
		zonesList = append(zonesList, zonePage.Result...)
		zonePage, err = zonePage.GetNextPage()
		if err != nil {
			return nil, err
		}
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	var resources types.Resources
	for _, zone := range zonesList {
		res := types.Resource{
			Removable: CachePurge{
				Client:   httpClient,
				Mode:     creds.Mode,
				APIToken: creds.APIKey,
				APIKey:   creds.APIKey,
				APIEmail: creds.User,
			},
			ResourceID:   zone.ID,
			ResourceName: zone.Name,
			AccountID:    creds.AccountID,
			ProductName:  "CachePurge",
			State:        types.Ready,
		}
		resources = append(resources, &res)
	}

	return resources, nil
}

func (c CachePurge) Remove(accountID string, resourceID string, resourceName string) error {
	payload, err := json.Marshal(cachePurgeRequest{PurgeEverything: true})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/purge_cache", resourceID)
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if c.Mode == types.Token {
		req.Header.Set("Authorization", "Bearer "+c.APIToken)
	} else {
		req.Header.Set("X-Auth-Email", c.APIEmail)
		req.Header.Set("X-Auth-Key", c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("purge cache request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if len(body) == 0 {
		return nil
	}

	var decoded cachePurgeResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return fmt.Errorf("purge cache request failed: %v", err)
	}
	if !decoded.Success {
		messages := make([]string, 0, len(decoded.Errors))
		for _, apiErr := range decoded.Errors {
			if apiErr.Message != "" {
				messages = append(messages, apiErr.Message)
			}
		}
		if len(messages) == 0 {
			return fmt.Errorf("purge cache request failed: %s", resp.Status)
		}
		return fmt.Errorf("purge cache request failed: %s", strings.Join(messages, "; "))
	}

	return nil
}
