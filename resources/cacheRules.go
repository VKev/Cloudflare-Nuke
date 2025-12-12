package resources

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/rulesets"
	"github.com/cloudflare/cloudflare-go/v6/zones"

	"github.com/arafato/cf-nuke/infrastructure"
	"github.com/arafato/cf-nuke/types"
	"github.com/arafato/cf-nuke/utils"
)

func init() {
	infrastructure.RegisterCollector("cache-rule", CollectCacheRules)
}

type CacheRule struct {
	Client *rulesets.RulesetService
	ZoneID string
}

func CollectCacheRules(creds *types.Credentials) (types.Resources, error) {
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

	var resources types.Resources
	for _, zone := range zonesList {
		page, err := client.Rulesets.List(context.TODO(), rulesets.RulesetListParams{
			ZoneID: cloudflare.F(zone.ID),
		})
		if err != nil {
			return nil, err
		}

		for page != nil {
			for _, rs := range page.Result {
				if rs.Phase != rulesets.PhaseHTTPRequestCacheSettings {
					continue
				}

				if rs.Kind == rulesets.KindManaged || rs.Kind == rulesets.KindRoot {
					continue
				}

				res := types.Resource{
					Removable:    CacheRule{Client: client.Rulesets, ZoneID: zone.ID},
					ResourceID:   rs.ID,
					ResourceName: fmt.Sprintf("%s:%s", zone.Name, rs.Name),
					AccountID:    creds.AccountID,
					ProductName:  "CacheRule",
					State:        types.Ready,
				}
				resources = append(resources, &res)
			}

			page, err = page.GetNextPage()
			if err != nil {
				return nil, err
			}
		}
	}

	return resources, nil
}

func (c CacheRule) Remove(accountID string, resourceID string, resourceName string) error {
	return c.Client.Delete(context.TODO(), resourceID, rulesets.RulesetDeleteParams{
		ZoneID: cloudflare.F(c.ZoneID)})
}
