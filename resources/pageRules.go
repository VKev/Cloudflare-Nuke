package resources

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go/v6"
	"github.com/cloudflare/cloudflare-go/v6/page_rules"
	"github.com/cloudflare/cloudflare-go/v6/zones"

	"github.com/arafato/cf-nuke/infrastructure"
	"github.com/arafato/cf-nuke/types"
	"github.com/arafato/cf-nuke/utils"
)

func init() {
	infrastructure.RegisterCollector("page-rule", CollectPageRules)
}

type PageRule struct {
	Client *page_rules.PageRuleService
	ZoneID string
}

func CollectPageRules(creds *types.Credentials) (types.Resources, error) {
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
		rules, err := client.PageRules.List(context.TODO(), page_rules.PageRuleListParams{
			ZoneID: cloudflare.F(zone.ID),
		})
		if err != nil {
			return nil, err
		}
		if rules == nil {
			continue
		}

		for _, rule := range *rules {
			res := types.Resource{
				Removable:    PageRule{Client: client.PageRules, ZoneID: zone.ID},
				ResourceID:   rule.ID,
				ResourceName: fmt.Sprintf("%s:%s", zone.Name, rule.ID),
				AccountID:    creds.AccountID,
				ProductName:  "PageRule",
				State:        types.Ready,
			}
			resources = append(resources, &res)
		}
	}

	return resources, nil
}

func (p PageRule) Remove(accountID string, resourceID string, resourceName string) error {
	_, err := p.Client.Delete(context.TODO(), resourceID, page_rules.PageRuleDeleteParams{
		ZoneID: cloudflare.F(p.ZoneID)})

	return err
}
