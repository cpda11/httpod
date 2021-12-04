package proxy

import (
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// @Summary Do a GET request.
// @Tags Proxy Methods
// @Description Query httpod as reverse proxy to url.
// @Accept  json
// @Produce  json
// @Param url header string false "Full URL to use for the backend request. Mandatory. e.g. https://example.org/path "
// @Param method header string false "Method to use for the backend request. Optional, defaults to 'GET'."
// @Param body header string false "Body to use for a POST request. Optional."
// @Param additionalHeaders header string false "JSON of headers to add to the backend request. Optional."
// @Success 200
// @Failure 400
// @Failure 500
// @Router /proxy [get]
func GetHandler(context echo.Context) error {
	if err = Br.configureBackendRequest(context.Request()); err != nil {
		return context.String(http.StatusBadRequest, string(err.Error()))
	}

	if resp, err = Br.requestBackend(); err != nil {
		return context.String(http.StatusInternalServerError, string(err.Error()))
	}

	if jsonResp, err = json.MarshalIndent(resp, "", "   "); err != nil {
		return context.String(http.StatusInternalServerError, string(err.Error()))
	}
	return context.String(http.StatusOK, string(jsonResp))

}

var (
	jsonResp []byte
	resp     *BackendResponse
	err      error
	Br       BackendRequest
)

func (br *BackendRequest) configureBackendRequest(req *http.Request) error {
	var err error
	br.Method = strings.ToUpper(req.Header.Get("method"))
	if br.Method == "" {
		br.Method = "GET"
	}
	br.Url, err = url.Parse(req.Header.Get("url"))
	if err != nil {
		return err
	}
	if br.Url.Scheme == "" || br.Url.Host == "" {
		return errors.New("Invalid query input.")
	}
	additionalHeaders := []byte(req.Header.Get("additionalHeaders")) // TODO
	json.Unmarshal(additionalHeaders, &br.AdditionalHeaders)

	return nil
}

func (br *BackendRequest) requestBackend() (*BackendResponse, error) {
	var (
		backendTransport = &http.Transport{
			MaxIdleConns:       3,
			IdleConnTimeout:    5 * time.Second,
			DisableCompression: true,
		}
		backendHttpResponseBytes []byte
		backendResponse          = &BackendResponse{}
		res                      *http.Response
		err                      error
	)
	if br.HttpClient == nil {
		br.HttpClient = &http.Client{Transport: backendTransport}
	}

	if br.Request, err = http.NewRequest(br.Method, br.Url.String(), nil); err != nil {
		return nil, errors.New("Error on proxied request: " + err.Error())
	}

	for _, header := range br.AdditionalHeaders {
		br.Request.Header.Add(header, br.AdditionalHeaders["header"])
	}

	if res, err = br.HttpClient.Do(br.Request); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if backendHttpResponseBytes, err = ioutil.ReadAll(res.Body); err != nil {
		return nil, errors.New("Error parsing body: " + err.Error())
	}

	if res.StatusCode != 0 {
		backendResponse.StatusCode = res.StatusCode
	}

	if res.Request != nil {
		if res.Request.RequestURI != "" {
			backendResponse.URI = res.Request.RequestURI
		}
	}

	if res.Header != nil {
		backendResponse.Headers = &res.Header
	}

	responseBody := string(backendHttpResponseBytes)
	if responseBody != "" {
		backendResponse.Body = responseBody
	}

	return backendResponse, nil
}
