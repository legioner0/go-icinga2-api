// Package iapi provides a client for interacting with an Icinga2 Server
package iapi

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"
)

// Server ... Use to be ClientConfig
type Server struct {
	Username           string
	Password           string
	BaseURL            string
	AllowUnverifiedSSL bool
	Retries            int
	RetryDelay         time.Duration
	httpClient         *http.Client
}

func New(username, password, url string, allowUnverifiedSSL bool, retries int, retryDelay time.Duration) (*Server, error) {
	return &Server{username, password, url, allowUnverifiedSSL, retries, retryDelay, nil}, nil
}

func (server *Server) Config(username, password, url string, allowUnverifiedSSL bool, retries int, retryDelay time.Duration) (*Server, error) {
	// TODO : Add code to verify parameters
	return &Server{username, password, url, allowUnverifiedSSL, retries, retryDelay, nil}, nil
}

func (server *Server) Connect() (error, int) {

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: server.AllowUnverifiedSSL,
		},
	}

	server.httpClient = &http.Client{
		Transport: t,
		Timeout:   time.Second * 60,
	}

	var err error
	request, err := http.NewRequest("GET", server.BaseURL, nil)
	if err != nil {
		server.httpClient = nil
	}

	request.SetBasicAuth(server.Username, server.Password)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	var response *http.Response
	retries := 0
	for {
		response, err = server.httpClient.Do(request)

		if !((err != nil) || (response == nil || response.StatusCode == 503)) {
			break
		}
		if retries >= server.Retries {
			break
		}
		retries++
		time.Sleep(server.RetryDelay)
	}
	if (err != nil) || (response == nil || response.StatusCode == 503) {
		server.httpClient = nil
		return err, retries
	}

	defer response.Body.Close()

	return nil, retries
}

// NewAPIRequest ...
func (server *Server) NewAPIRequest(method, APICall string, jsonString []byte) (*APIResult, error) {

	fullURL := server.BaseURL + APICall

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: server.AllowUnverifiedSSL,
		},
	}

	server.httpClient = &http.Client{
		Transport: t,
		Timeout:   time.Second * 60,
	}

	request, requestErr := http.NewRequest(method, fullURL, bytes.NewBuffer(jsonString))
	if requestErr != nil {
		return nil, requestErr
	}

	request.SetBasicAuth(server.Username, server.Password)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	var response *http.Response
	var doErr error
	retries := 0
	for {
		response, doErr = server.httpClient.Do(request)

		if !((doErr != nil) || (response == nil || response.StatusCode == 503)) {
			break
		}

		if retries >= server.Retries {
			break
		}
		retries++
		time.Sleep(server.RetryDelay)
	}

	if doErr != nil {
		results := APIResult{
			Code:        0,
			Status:      "Error : Request to server failed : " + doErr.Error(),
			ErrorString: doErr.Error(),
			Retries:     retries,
		}
		return &results, doErr
	}
	defer response.Body.Close()

	var results APIResult
	if decodeErr := json.NewDecoder(response.Body).Decode(&results); decodeErr != nil {
		return nil, decodeErr
	}

	if results.Retries == 0 { // results.Retries have default value so set it.
		results.Retries = retries
	}

	if results.Code == 0 { // results.Code has default value so set it.
		results.Code = response.StatusCode
	}

	if results.Status == "" { // results.Status has default value, so set it.
		results.Status = response.Status
	}

	switch results.Code {
	case 0:
		results.ErrorString = "Did not get a response code."
	case 404:
		results.ErrorString = results.Status
	case 200:
		results.ErrorString = results.Status
	default:
		results.ErrorString = results.Status
		//theError := strings.Replace(results.Results.([]interface{})[0].(map[string]interface{})["errors"].([]interface{})[0].(string), "\n", " ", -1)
		//results.ErrorString = strings.Replace(theError, "Error: ", "", -1)

	}

	return &results, nil

}
