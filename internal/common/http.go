package common

import (
	"io/ioutil"
	"fmt"
	"net/http"
	"encoding/json"
)

// http request executor
func DoHttpRequest(req *http.Request) ([]byte, error) {
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if 200 != resp.StatusCode {
		return nil, fmt.Errorf("%s", body)
	}
	return body, nil
}

func GetPublicIP() string {
	ip, err := getPublicIP()
	if err != nil {
		ip = "cannot get IP address: " + err.Error()
	}
	return ip
}
func getPublicIP() (string, error) {
		var url string
	// url = "http://" + ip + ":8080/v1/thorchain/pool_addresses"
	url = "https://api.ipify.org/?format=json"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	bytes, err := DoHttpRequest(req)
	if err != nil {
		return "", err
	}
	var ip map[string]string 
	err = json.Unmarshal(bytes, &ip)
	if err != nil {
		return "", err
	}
	return ip["ip"], nil
}