package influxdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Client struct {
	host       string
	username   string
	password   string
	database   string
	httpClient *http.Client
	schema     string
}

type ClientConfig struct {
	Host       string
	Username   string
	Password   string
	Database   string
	HttpClient *http.Client
	IsSecure   bool
}

var defaults *ClientConfig

func init() {
	defaults = &ClientConfig{
		Host:       "localhost:8086",
		Username:   "root",
		Password:   "root",
		Database:   "",
		HttpClient: http.DefaultClient,
	}
}

func getDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func NewClient(config *ClientConfig) (*Client, error) {
	host := getDefault(config.Host, defaults.Host)
	username := getDefault(config.Username, defaults.Username)
	passowrd := getDefault(config.Password, defaults.Password)
	database := getDefault(config.Database, defaults.Database)
	if config.HttpClient == nil {
		config.HttpClient = defaults.HttpClient
	}

	schema := "http"
	if config.IsSecure {
		schema = "https"
	}
	return &Client{host, username, passowrd, database, config.HttpClient, schema}, nil
}

func (self *Client) getUrl(path string) string {
	return self.getUrlWithUserAndPass(path, self.username, self.password)
}

func (self *Client) getUrlWithUserAndPass(path, username, password string) string {
	return fmt.Sprintf("%s://%s%s?u=%s&p=%s", self.schema, self.host, path, username, password)
}

func responseToError(response *http.Response, err error, closeResponse bool) error {
	if err != nil {
		return err
	}
	if closeResponse {
		defer response.Body.Close()
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return fmt.Errorf("Server returned (%d): %s", response.StatusCode, string(body))
}

func (self *Client) CreateDatabase(name string) error {
	url := self.getUrl("/db")
	payload := map[string]string{"name": name}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) del(url string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	return self.httpClient.Do(req)
}

func (self *Client) DeleteDatabase(name string) error {
	url := self.getUrl("/db/" + name)
	resp, err := self.del(url)
	return responseToError(resp, err, true)
}

func (self *Client) listSomething(url string) ([]map[string]interface{}, error) {
	resp, err := self.httpClient.Get(url)
	err = responseToError(resp, err, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	somethings := []map[string]interface{}{}
	err = json.Unmarshal(body, &somethings)
	if err != nil {
		return nil, err
	}
	return somethings, nil
}

func (self *Client) GetDatabaseList() ([]map[string]interface{}, error) {
	url := self.getUrl("/db")
	return self.listSomething(url)
}

func (self *Client) CreateClusterAdmin(name, password string) error {
	url := self.getUrl("/cluster_admins")
	payload := map[string]string{"name": name, "password": password}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) UpdateClusterAdmin(name, password string) error {
	url := self.getUrl("/cluster_admins/" + name)
	payload := map[string]string{"password": password}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) DeleteClusterAdmin(name string) error {
	url := self.getUrl("/cluster_admins/" + name)
	resp, err := self.del(url)
	return responseToError(resp, err, true)
}

func (self *Client) GetClusterAdminList() ([]map[string]interface{}, error) {
	url := self.getUrl("/cluster_admins")
	return self.listSomething(url)
}

func (self *Client) CreateDatabaseUser(database, name, password string) error {
	url := self.getUrl("/db/" + database + "/users")
	payload := map[string]string{"name": name, "password": password}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) updateDatabaseUserCommon(database, name string, password *string, isAdmin *bool) error {
	url := self.getUrl("/db/" + database + "/users/" + name)
	payload := map[string]interface{}{}
	if password != nil {
		payload["password"] = *password
	}
	if isAdmin != nil {
		payload["admin"] = *isAdmin
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) UpdateDatabaseUser(database, name, password string) error {
	return self.updateDatabaseUserCommon(database, name, &password, nil)
}

func (self *Client) DeleteDatabaseUser(database, name string) error {
	url := self.getUrl("/db/" + database + "/users/" + name)
	resp, err := self.del(url)
	return responseToError(resp, err, true)
}

func (self *Client) GetDatabaseUserList(database string) ([]map[string]interface{}, error) {
	url := self.getUrl("/db/" + database + "/users")
	return self.listSomething(url)
}

func (self *Client) AlterDatabasePrivilege(database, name string, isAdmin bool) error {
	return self.updateDatabaseUserCommon(database, name, nil, &isAdmin)
}

type TimePrecision string

const (
	Second      TimePrecision = "s"
	Millisecond TimePrecision = "m"
	Microsecond TimePrecision = "u"
)

func (self *Client) WriteSeries(series []*Series) error {
	return self.writeSeriesCommon(series, nil)
}

func (self *Client) WriteSeriesWithTimePrecision(series []*Series, timePrecision TimePrecision) error {
	return self.writeSeriesCommon(series, map[string]string{"time_precision": string(timePrecision)})
}

func (self *Client) writeSeriesCommon(series []*Series, options map[string]string) error {
	data, err := json.Marshal(series)
	if err != nil {
		return err
	}
	url := self.getUrl("/db/" + self.database + "/series")
	for name, value := range options {
		url += fmt.Sprintf("&%s=%s", name, value)
	}
	resp, err := self.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return responseToError(resp, err, true)
}

func (self *Client) Query(query string, precision ...TimePrecision) ([]*Series, error) {
	escapedQuery := url.QueryEscape(query)
	url := self.getUrl("/db/" + self.database + "/series")
	if len(precision) > 0 {
		url += "&time_precision=" + string(precision[0])
	}
	url += "&q=" + escapedQuery
	resp, err := self.httpClient.Get(url)
	err = responseToError(resp, err, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	series := []*Series{}
	err = json.Unmarshal(data, &series)
	if err != nil {
		return nil, err
	}
	return series, nil
}

func (self *Client) Ping() error {
	url := self.getUrl("/ping")
	resp, err := self.httpClient.Get(url)
	return responseToError(resp, err, true)
}

func (self *Client) AuthenticateDatabaseUser(database, username, password string) error {
	url := self.getUrlWithUserAndPass(fmt.Sprintf("/db/%s/authenticate", database), username, password)
	resp, err := self.httpClient.Get(url)
	return responseToError(resp, err, true)
}
