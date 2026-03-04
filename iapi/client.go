// Package iapi provides a client for interacting with an Icinga2 Server
package iapi

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// Server ... Use to be ClientConfig
type Server struct {
	Username           string
	Password           string
	BaseURL            string
	AllowUnverifiedSSL bool
	CACertFile         string
	Retries            int32
	RetryDelay         time.Duration
	httpClient         *http.Client
}

func New(username, password, url string, allowUnverifiedSSL bool, caCertFile string, retries int32, retryDelay time.Duration) (*Server, error) {
	return &Server{username, password, url, allowUnverifiedSSL, caCertFile, retries, retryDelay, nil}, nil
}

func (server *Server) Config(username, password, url string, allowUnverifiedSSL bool, caCertFile string, retries int32, retryDelay time.Duration) (*Server, error) {
	// TODO : Add code to verify parameters
	return &Server{username, password, url, allowUnverifiedSSL, caCertFile, retries, retryDelay, nil}, nil
}

func (server *Server) doRequest(method, fullURL string, body io.Reader) (*http.Response, error, int32) {

	var caCertPool *x509.CertPool
	if server.CACertFile != "" {
		caCert, err := os.ReadFile(server.CACertFile)
		if err != nil {
			return nil, err, 0
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
	}

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: server.AllowUnverifiedSSL,
			RootCAs:            caCertPool,
		},
	}

	server.httpClient = &http.Client{
		Transport: t,
		Timeout:   time.Second * 60,
	}

	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = io.ReadAll(body)
	}

	var response *http.Response
	var doErr error
	retries := int32(0)
	for {
		request, requestErr := http.NewRequest(method, fullURL, io.NopCloser(bytes.NewBuffer(bodyBytes)))
		if requestErr != nil {
			return nil, requestErr, retries
		}

		request.SetBasicAuth(server.Username, server.Password)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")

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

	return response, doErr, retries
}

func (server *Server) Connect() (error, int32) {

	response, doErr, retries := server.doRequest("GET", server.BaseURL, nil)

	if (doErr != nil) || (response == nil || response.StatusCode == 503) {
		server.httpClient = nil
		return doErr, retries
	}

	defer response.Body.Close()

	return nil, retries
}

// NewAPIRequest ...
func (server *Server) NewAPIRequest(method, APICall string, jsonString []byte) (*APIResult, error) {

	response, doErr, retries := server.doRequest(method, server.BaseURL+APICall, bytes.NewBuffer(jsonString))

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
