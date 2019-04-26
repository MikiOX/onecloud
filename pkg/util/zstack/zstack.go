package zstack

import (
	"context"
	"crypto/sha512"
	"fmt"
	"net/http"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

const (
	CLOUD_PROVIDER_ZSTACK = api.CLOUD_PROVIDER_ZSTACK
	ZSTACK_DEFAULT_REGION = "ZStack"
	ZSTACK_API_VERSION    = "v1"
)

var (
	SkipEsxi bool = true
)

type SZStackClient struct {
	providerID   string
	providerName string
	username     string
	password     string
	authURL      string

	sessionID string

	iregions []cloudprovider.ICloudRegion

	debug bool
}

func NewZStackClient(providerID string, providerName string, authURL string, username string, password string, isDebug bool) (*SZStackClient, error) {
	cli := &SZStackClient{
		providerID:   providerID,
		providerName: providerName,
		authURL:      authURL,
		username:     username,
		password:     password,
		debug:        isDebug,
	}
	if err := cli.connect(); err != nil {
		return nil, err
	}
	cli.iregions = []cloudprovider.ICloudRegion{&SRegion{client: cli, Name: ZSTACK_DEFAULT_REGION}}
	return cli, nil
}

func (cli *SZStackClient) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	subAccount := cloudprovider.SSubAccount{
		Account:      cli.username,
		Name:         cli.providerName,
		HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
	}
	return []cloudprovider.SSubAccount{subAccount}, nil
}

func (cli *SZStackClient) GetIRegions() []cloudprovider.ICloudRegion {
	return cli.iregions
}

func (cli *SZStackClient) GetIRegionById(id string) (cloudprovider.ICloudRegion, error) {
	for i := 0; i < len(cli.iregions); i++ {
		if cli.iregions[i].GetGlobalId() == id {
			return cli.iregions[i], nil
		}
	}
	return nil, cloudprovider.ErrNotFound
}

func (cli *SZStackClient) getRequestURL(resource string, params []string) string {
	return cli.authURL + fmt.Sprintf("/zstack/%s/%s", ZSTACK_API_VERSION, resource) + "?" + strings.Join(params, "&")
}

func (cli *SZStackClient) connect() error {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	authURL := cli.authURL + "/zstack/v1/accounts/login"
	body := jsonutils.Marshal(map[string]interface{}{
		"logInByAccount": map[string]string{
			"accountName": cli.username,
			"password":    fmt.Sprintf("%x", sha512.Sum512([]byte(cli.password))),
		},
	})
	_, resp, err := httputils.JSONRequest(client, context.Background(), "PUT", authURL, header, body, cli.debug)
	if err != nil {
		return err
	}
	cli.sessionID, err = resp.GetString("inventory", "uuid")
	return err
}

func (cli *SZStackClient) listAll(resource string, params []string, retVal interface{}) error {
	result := []jsonutils.JSONObject{}
	start, limit := 0, 50
	for {
		resp, err := cli._list(resource, start, limit, params)
		if err != nil {
			return err
		}
		objs, err := resp.GetArray("inventories")
		if err != nil {
			return err
		}
		result = append(result, objs...)
		if start+limit > len(result) {
			inventories := jsonutils.Marshal(map[string][]jsonutils.JSONObject{"inventories": result})
			return inventories.Unmarshal(retVal, "inventories")
		}
		start += limit
	}
}

func (cli *SZStackClient) _list(resource string, start int, limit int, params []string) (jsonutils.JSONObject, error) {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", "OAuth "+cli.sessionID)
	if params == nil {
		params = []string{}
	}
	params = append(params, "replyWithCount=true")
	params = append(params, fmt.Sprintf("start=%d", start))
	if limit == 0 {
		limit = 50
	}
	params = append(params, fmt.Sprintf("limit=%d", limit))
	requestURL := cli.getRequestURL(resource, params)
	_, resp, err := httputils.JSONRequest(client, context.Background(), "GET", requestURL, header, nil, cli.debug)
	return resp, err
}

func (cli *SZStackClient) getDeleteURL(resource, resourceId, deleteMode string) string {
	return cli.authURL + fmt.Sprintf("/zstack/%s/%s/%s?deleteMode=%s", ZSTACK_API_VERSION, resource, resourceId, deleteMode)
}

func (cli *SZStackClient) delete(resource, resourceId, deleteMode string) (jsonutils.JSONObject, error) {
	return cli._delete(resource, resourceId, deleteMode)
}

func (cli *SZStackClient) _delete(resource, resourceId, deleteMode string) (jsonutils.JSONObject, error) {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", "OAuth "+cli.sessionID)
	requestURL := cli.getDeleteURL(resource, resourceId, deleteMode)
	_, resp, err := httputils.JSONRequest(client, context.Background(), "DELETE", requestURL, header, nil, cli.debug)
	return resp, err
}

func (cli *SZStackClient) getURL(resource, resourceId, spec string) string {
	return cli.authURL + fmt.Sprintf("/zstack/%s/%s/%s/%s", ZSTACK_API_VERSION, resource, resourceId, spec)
}

func (cli *SZStackClient) getPostURL(resource string) string {
	return cli.authURL + fmt.Sprintf("/zstack/%s/%s", ZSTACK_API_VERSION, resource)
}

func (cli *SZStackClient) put(resource, resourceId string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	return cli._put(resource, resourceId, params)
}

func (cli *SZStackClient) _put(resource, resourceId string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", "OAuth "+cli.sessionID)
	requestURL := cli.getURL(resource, resourceId, "actions")
	_, resp, err := httputils.JSONRequest(client, context.Background(), "PUT", requestURL, header, params, cli.debug)
	return resp, err
}

func (cli *SZStackClient) get(resource, resourceId string, spec string) (jsonutils.JSONObject, error) {
	return cli._get(resource, resourceId, spec)
}

func (cli *SZStackClient) _get(resource, resourceId string, spec string) (jsonutils.JSONObject, error) {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", "OAuth "+cli.sessionID)
	requestURL := cli.getURL(resource, resourceId, spec)
	_, resp, err := httputils.JSONRequest(client, context.Background(), "GET", requestURL, header, nil, cli.debug)
	return resp, err
}

func (cli *SZStackClient) post(resource string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	return cli._post(resource, params)
}

func (cli *SZStackClient) _post(resource string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	client := httputils.GetDefaultClient()
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", "OAuth "+cli.sessionID)
	requestURL := cli.getPostURL(resource)
	_, resp, err := httputils.JSONRequest(client, context.Background(), "POST", requestURL, header, params, cli.debug)
	if err != nil {
		return nil, err
	}
	if resp.Contains("location") {
		startTime := time.Now()
		timeout := time.Minute * 30
		location, _ := resp.GetString("location")
		for {
			_resp, err := httputils.Request(client, context.Background(), "GET", location, header, nil, cli.debug)
			if err != nil {
				return nil, err
			}
			_, resp, err = httputils.ParseJSONResponse(_resp, err, cli.debug)
			if time.Now().Sub(startTime) > timeout {
				return nil, fmt.Errorf("timeout for waitting post %s params: %s", requestURL, params.PrettyString())
			}
			if _resp.StatusCode != 200 {
				log.Debugf("wait for job %s %s complete", requestURL, params.String())
				time.Sleep(5 * time.Second)
				continue
			}
			break
		}
	}
	return resp, err
}

func (cli *SZStackClient) list(baseURL string, start int, limit int, params []string, retVal interface{}) error {
	resp, err := cli._list(baseURL, start, limit, params)
	if err != nil {
		return err
	}
	return resp.Unmarshal(retVal, "inventories")
}

func (cli *SZStackClient) GetRegion(regionId string) *SRegion {
	for i := 0; i < len(cli.iregions); i++ {
		if cli.iregions[i].GetId() == regionId {
			return cli.iregions[i].(*SRegion)
		}
	}
	return nil
}

func (cli *SZStackClient) GetRegions() []SRegion {
	regions := make([]SRegion, len(cli.iregions))
	for i := 0; i < len(regions); i++ {
		region := cli.iregions[i].(*SRegion)
		regions[i] = *region
	}
	return regions
}

func (cli *SZStackClient) GetIProjects() ([]cloudprovider.ICloudProject, error) {
	return nil, cloudprovider.ErrNotImplemented
}
