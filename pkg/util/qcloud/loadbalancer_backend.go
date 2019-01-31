package qcloud

import (
	"fmt"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/compute/models"
)

type SLBBackend struct {
	group *SLBBackendGroup

	PublicIPAddresses  []string `json:"PublicIpAddresses"`
	Weight             int      `json:"Weight"`
	InstanceID         string   `json:"InstanceId"`
	InstanceName       string   `json:"InstanceName"`
	PrivateIPAddresses []string `json:"PrivateIpAddresses"`
	RegisteredTime     string   `json:"RegisteredTime"`
	Type               string   `json:"Type"`
	Port               int      `json:"Port"`
}

// ==========================================================
type SListenerBackend struct {
	Rules      []rule       `json:"Rules"`
	Targets    []SLBBackend `json:"Targets"`
	Protocol   string       `json:"Protocol"`
	ListenerID string       `json:"ListenerId"`
	Port       int64        `json:"Port"`
}

type rule struct {
	URL        string       `json:"Url"`
	Domain     string       `json:"Domain"`
	LocationID string       `json:"LocationId"`
	Targets    []SLBBackend `json:"Targets"`
}

// ==========================================================

func (self *SLBBackend) GetId() string {
	return self.InstanceID
}

func (self *SLBBackend) GetName() string {
	return fmt.Sprintf("%s/%s", self.group.GetId(), self.GetId())
}

func (self *SLBBackend) GetGlobalId() string {
	return self.GetId()
}

// todo: status ??
func (self *SLBBackend) GetStatus() string {
	return ""
}

func (self *SLBBackend) Refresh() error {
	panic("implement me")
}

func (self *SLBBackend) IsEmulated() bool {
	return false
}

func (self *SLBBackend) GetMetadata() *jsonutils.JSONDict {
	return nil
}

func (self *SLBBackend) GetWeight() int {
	return self.Weight
}

func (self *SLBBackend) GetPort() int {
	return self.Port
}

// todo: self.Type ??
func (self *SLBBackend) GetBackendType() string {
	return models.LB_BACKEND_GUEST
}

func (self *SLBBackend) GetBackendRole() string {
	return models.LB_BACKEND_ROLE_DEFAULT
}

func (self *SLBBackend) GetBackendId() string {
	return self.InstanceID
}

// 传统型： https://cloud.tencent.com/document/product/214/31790
func (self *SRegion) getClassicBackends(lbId, listenerId string) ([]SLBBackend, error) {
	params := map[string]string{"LoadBalancerId": lbId}

	resp, err := self.clbRequest("DescribeClassicalLBTargets", params)
	if err != nil {
		return nil, err
	}

	backends := []SLBBackend{}
	err = resp.Unmarshal(&backends, "Targets")
	if err != nil {
		return nil, err
	}
	return backends, nil
}

// 应用型： https://cloud.tencent.com/document/product/214/30684
func (self *SRegion) getBackends(lbId, listenerId, ruleId string) ([]SLBBackend, error) {
	params := map[string]string{"LoadBalancerId": lbId}

	if len(listenerId) > 0 {
		params["ListenerIds.0"] = listenerId
	}

	if len(listenerId) > 0 {
		params["ListenerIds.0"] = listenerId
	}
	resp, err := self.clbRequest("DescribeTargets", params)
	if err != nil {
		return nil, err
	}

	lbackends := []SListenerBackend{}
	err = resp.Unmarshal(&lbackends, "Listeners")
	if err != nil {
		return nil, err
	}

	for _, entry := range lbackends {
		if (entry.Protocol == "HTTP" || entry.Protocol == "HTTPS") && len(ruleId) == 0 {
			return nil, fmt.Errorf("GetBackends http、https listener must specific rule id")
		}

		if len(ruleId) > 0 {
			for _, r := range entry.Rules {
				if r.LocationID == ruleId {
					return r.Targets, nil
				}
			}
		} else {
			return entry.Targets, nil
		}
	}

	// todo： 这里是返回空列表还是404？
	return []SLBBackend{}, nil
}

// 注意http、https监听器必须指定ruleId
func (self *SRegion) GetLBBackends(t LB_TYPE, lbId, listenerId, ruleId string) ([]SLBBackend, error) {
	if len(lbId) == 0 {
		return nil, fmt.Errorf("GetLBBackends loadbalancer id should not be empty")
	}

	if t == LB_TYPE_APPLICATION {
		return self.getBackends(lbId, listenerId, ruleId)
	} else if t == LB_TYPE_CLASSIC {
		return self.getClassicBackends(lbId, listenerId)
	} else {
		return nil, fmt.Errorf("GetLBBackends unsupported loadbalancer type %d", t)
	}
}
