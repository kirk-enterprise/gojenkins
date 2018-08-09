package gojenkins

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Request Methods

type APIRequest struct {
	Method   string
	Endpoint string
	Payload  io.Reader
	Headers  http.Header
	Suffix   string
}

func (ar *APIRequest) SetHeader(key string, value string) *APIRequest {
	ar.Headers.Set(key, value)
	return ar
}

func NewAPIRequest(method string, endpoint string, payload io.Reader) *APIRequest {
	var headers = http.Header{}
	var suffix string
	ar := &APIRequest{method, endpoint, payload, headers, suffix}
	return ar
}

type Requester struct {
	Base      string
	BasicAuth *BasicAuth
	Client    *http.Client
	CACert    []byte
	SslVerify bool
}

func (r *Requester) SetCrumb(ar *APIRequest) error {
	crumbData := map[string]string{}
	response, _ := r.GetJSON("/crumbIssuer", &crumbData, nil)

	if response.StatusCode == 200 && crumbData["crumbRequestField"] != "" {
		ar.SetHeader(crumbData["crumbRequestField"], crumbData["crumb"])
	}

	return nil
}

func (r *Requester) PostJSON(endpoint string, payload io.Reader, responseStruct interface{}, querystring map[string]string) (*http.Response, error) {
	ar := NewAPIRequest("POST", endpoint, payload)
	if err := r.SetCrumb(ar); err != nil {
		return nil, err
	}
	ar.SetHeader("Content-Type", "application/x-www-form-urlencoded")
	ar.Suffix = "api/json"
	return r.Do(ar, responseStruct, querystring)
}

func (r *Requester) Post(endpoint string, payload io.Reader, responseStruct interface{}, querystring map[string]string) (*http.Response, error) {
	ar := NewAPIRequest("POST", endpoint, payload)
	if err := r.SetCrumb(ar); err != nil {
		return nil, err
	}
	ar.SetHeader("Content-Type", "application/x-www-form-urlencoded")
	ar.Suffix = ""
	return r.Do(ar, responseStruct, querystring)
}

func (r *Requester) PostFiles(endpoint string, payload io.Reader, responseStruct interface{}, querystring map[string]string, files []string) (*http.Response, error) {
	ar := NewAPIRequest("POST", endpoint, payload)
	if err := r.SetCrumb(ar); err != nil {
		return nil, err
	}
	return r.Do(ar, responseStruct, querystring, files)
}

func (r *Requester) PostXML(endpoint string, xml string, responseStruct interface{}, querystring map[string]string) (*http.Response, error) {
	payload := bytes.NewBuffer([]byte(xml))
	ar := NewAPIRequest("POST", endpoint, payload)
	if err := r.SetCrumb(ar); err != nil {
		return nil, err
	}
	ar.SetHeader("Content-Type", "application/xml")
	ar.Suffix = ""
	return r.Do(ar, responseStruct, querystring)
}

func (r *Requester) GetJSON(endpoint string, responseStruct interface{}, query map[string]string) (*http.Response, error) {
	ar := NewAPIRequest("GET", endpoint, nil)
	ar.SetHeader("Content-Type", "application/json")
	ar.Suffix = "api/json"
	return r.Do(ar, responseStruct, query)
}

func (r *Requester) GetXML(endpoint string, responseStruct interface{}, query map[string]string) (*http.Response, error) {
	ar := NewAPIRequest("GET", endpoint, nil)
	ar.SetHeader("Content-Type", "application/xml")
	ar.Suffix = ""
	return r.Do(ar, responseStruct, query)
}

func (r *Requester) Get(endpoint string, responseStruct interface{}, querystring map[string]string) (*http.Response, error) {
	ar := NewAPIRequest("GET", endpoint, nil)
	ar.Suffix = ""
	return r.Do(ar, responseStruct, querystring)
}

func (r *Requester) SetClient(client *http.Client) *Requester {
	r.Client = client
	return r
}

//Add auth on redirect if required.
func (r *Requester) redirectPolicyFunc(req *http.Request, via []*http.Request) error {
	if r.BasicAuth != nil {
		req.SetBasicAuth(r.BasicAuth.Username, r.BasicAuth.Password)
	}
	return nil
}

func (r *Requester) Do(ar *APIRequest, responseStruct interface{}, options ...interface{}) (*http.Response, error) {
	if !strings.HasSuffix(ar.Endpoint, "/") && ar.Method != "POST" {
		ar.Endpoint += "/"
	}

	fileUpload := false
	var files []string
	URL, err := url.Parse(r.Base + ar.Endpoint + ar.Suffix)

	if err != nil {
		return nil, err
	}

	for _, o := range options {
		switch v := o.(type) {
		case map[string]string:

			querystring := make(url.Values)
			for key, val := range v {
				querystring.Set(key, val)
			}

			URL.RawQuery = querystring.Encode()
		case []string:
			fileUpload = true
			files = v
		}
	}
	var req *http.Request

	if fileUpload {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		for _, file := range files {
			fileData, err := os.Open(file)
			if err != nil {
				Error.Println(err.Error())
				return nil, err
			}

			part, err := writer.CreateFormFile("file", filepath.Base(file))
			if err != nil {
				Error.Println(err.Error())
				return nil, err
			}
			if _, err = io.Copy(part, fileData); err != nil {
				return nil, err
			}
			defer fileData.Close()
		}
		var params map[string]string
		json.NewDecoder(ar.Payload).Decode(&params)
		for key, val := range params {
			if err = writer.WriteField(key, val); err != nil {
				return nil, err
			}
		}
		if err = writer.Close(); err != nil {
			return nil, err
		}
		req, err = http.NewRequest(ar.Method, URL.String(), body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
	} else {

		req, err = http.NewRequest(ar.Method, URL.String(), ar.Payload)
		if err != nil {
			return nil, err
		}
	}

	if r.BasicAuth != nil {
		req.SetBasicAuth(r.BasicAuth.Username, r.BasicAuth.Password)
	}

	for k := range ar.Headers {
		req.Header.Add(k, ar.Headers.Get(k))
	}

	if response, err := r.Client.Do(req); err != nil {
		return nil, err
	} else {
		defer response.Body.Close()

		errorText := response.Header.Get("X-Error")
		if errorText != "" {
			fmt.Printf("X-Error: %s %+v\n", errorText, *ar)
			return nil, errors.New(errorText)
		}

		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println("ioutil.ReadAll:", err)
			return nil, err
		}

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			fmt.Println(string(content))
			return nil, fmt.Errorf("response status code is not 200: %d", response.StatusCode)
		}

		// responseStruct为nil表示忽略响应内容，一般情况下，这是因为响应为html页面，对于api来说，无需处理，所以直接返回即可
		if responseStruct == nil {
			return response, nil
		}

		switch responseStruct.(type) {
		case *string:
			fmt.Println("responseStruct.(type) is *string")
			err = r.processStrContent(content, responseStruct)
		default:
			fmt.Println("responseStruct.(type) is json")
			err = r.processJSONContent(content, responseStruct)
		}
		return response, err
	}
}

func (r *Requester) processStrContent(content []byte, responseStruct interface{}) error {
	if str, ok := responseStruct.(*string); ok {
		*str = string(content)
	} else {
		return fmt.Errorf("Could not cast responseStruct to *string")
	}

	return nil
}

func (r *Requester) processJSONContent(content []byte, responseStruct interface{}) error {
	err := json.Unmarshal(content, responseStruct)
	if err != nil {
		fmt.Println(string(content))
	}
	return err
}
