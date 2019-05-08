/*
 * hub-grafana-agent
 *
 * an agent used to provision and configure Grafana resources
 *
 * API version: v1beta
 * Contact: support@appvia.io
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package swagger

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"text/template"

	"github.com/gorilla/mux"
	logrus "github.com/sirupsen/logrus"
)

const dashboardPrefix string = "hub-grafana-"

func handleSuccess(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(payload)
}

func handleDelete(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func handleInternalServerError(w http.ResponseWriter, reason string, detail string) {
	var apiError ApiError
	apiError = ApiError{Reason: reason, Detail: detail}
	payload, err := json.Marshal(apiError)
	_ = err
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(payload)
}

func handleBadRequest(w http.ResponseWriter, detail string) {
	var apiError ApiError
	apiError = ApiError{Reason: "bad request", Detail: detail}
	payload, err := json.Marshal(apiError)
	_ = err
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusBadRequest)
	w.Write(payload)
}

func handleNotFoundError(w http.ResponseWriter, detail string) {
	var apiError ApiError
	apiError = ApiError{Reason: "not found", Detail: detail}
	payload, err := json.Marshal(apiError)
	_ = err
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write(payload)
}

func callGrafana(url string, apiKey string, verb string, payload io.Reader) (int, []byte, error) {
	var statusCode int
	var body []byte
	var err error
	req, err := http.NewRequest(verb, url, payload)
	req.Header.Set("Authorization", "Bearer"+" "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		logrus.Println("Error calling Grafana:", verb, url, err)
	} else {
		body, _ = ioutil.ReadAll(resp.Body)
		statusCode = resp.StatusCode
		logrus.Printf("Response body from Grafana: %s", string(body))
		logrus.Printf("Response code from Grafana: %v", statusCode)
	}
	return statusCode, body, err
}

func getTemplateFromUrl(templateUrl string) ([]byte, error) {
	var templateBody []byte
	var err error
	req, err := http.NewRequest("GET", templateUrl, nil)
	req.Header.Set("Accept", "application/json,text/plain")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Println("Error fetching Grafana dashboard json from URL:", templateUrl)
		return templateBody, err
	}
	defer resp.Body.Close()
	templateBody, _ = ioutil.ReadAll(resp.Body)
	if os.Getenv("DEBUG") == "true" {
		logrus.Println("Template response body:", string(templateBody))
	}
	return templateBody, nil
}

func getDashboardByName(name string, grafanaUrl string, grafanaApiKey string) (url string, id int64, uid string, version int64, found bool, err error) {

	logrus.Println("Searching for dashboard using tag:", dashboardPrefix+name)
	status, body, err := callGrafana(grafanaUrl+"/api/search?tag="+dashboardPrefix+name, grafanaApiKey, "GET", nil)

	if status != 200 || err != nil {
		logrus.Println("Error getting dashboard for name:", name)
		return
	}

	type GrafanaDashboard struct {
		Uid     string `json:"uid"`
		Id      int64  `json:"id"`
		Url     string `json:"url"`
		Version int64  `json:"version"`
	}
	var g []GrafanaDashboard

	if err := json.Unmarshal(body, &g); err != nil {
		panic(err)
	}

	if len(g) == 0 {
		found = false
		return url, id, uid, version, found, err
	} else if len(g) == 1 {
		found = true
		logrus.Println("Found one dashboard matching search")
	} else if len(g) > 1 {
		found = true
		err := errors.New("more than one dashboard found")
		return url, id, uid, version, found, err
	}

	dash := g[0]

	url = grafanaUrl + dash.Url
	id = dash.Id
	uid = dash.Uid
	version = dash.Version

	return url, id, uid, version, found, err
}

func DashboardNameDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	decodedCert, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
	if err != nil {
		logrus.Println("decode error:", err)
	}
	_ = decodedCert
	grafanaUrl := r.Header.Get("X-Grafana-Url")
	grafanaApiKey := r.Header.Get("X-Grafana-API-Key")

	if err != nil {
		logrus.Println("decode error:", err)
	}

	url, id, uid, version, found, err := getDashboardByName(name, grafanaUrl, grafanaApiKey)
	_ = version

	if err != nil {
		handleInternalServerError(w, "internal server error", "error searching for dashboard in Grafana")
		return
	}

	if found == false {
		_, _, _ = id, uid, url
		handleDelete(w)
		return
	}

	if err != nil {
		handleInternalServerError(w, "internal server error", "error calling Grafana")
		return
	}

	if uid == "" {
		logrus.Println("Dashboard already deleted!")
		handleDelete(w)
		return
	}

	logrus.Printf("Attempting to delete dashboard with uid %s", string(uid))

	status, body, err := callGrafana(grafanaUrl+"/api/dashboards/uid/"+uid, grafanaApiKey, "DELETE", nil)
	_ = body

	if err != nil {
		handleInternalServerError(w, "internal server error", "error deleting dashboard from Grafana")
		return
	}

	if status == 200 {
		logrus.Println("Dashboard deleted!")
		w.WriteHeader(204)
		return
	} else {
		handleInternalServerError(w, "internal server error", "error deleting dashboard from Grafana")
		return
	}
}

func DashboardNameGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	decodedCert, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
	if err != nil {
		logrus.Println("decode error:", err)
	}
	_ = decodedCert
	grafanaUrl := r.Header.Get("X-Grafana-Url")
	grafanaApiKey := r.Header.Get("X-Grafana-API-Key")

	url, id, uid, version, found, err := getDashboardByName(name, grafanaUrl, grafanaApiKey)
	_ = version

	if err != nil {
		handleInternalServerError(w, "internal server error", "error searching for dashboard in Grafana")
		return
	}

	if found == false {
		handleNotFoundError(w, "dashboard not found")
		return
	}

	var dashboard Dashboard
	dashboard = Dashboard{Name: name, Url: url, Id: id, Uid: uid}
	payload, err := json.Marshal(dashboard)

	if err != nil {
		logrus.Println(err)
	}

	handleSuccess(w, payload)
	return
}

func renderTemplate(name string, id string, uid string, version string, templateAsString string) (renderedTemplate io.Reader, err error) {
	type Variables struct {
		Name    string
		Id      string
		Uid     string
		Version string
	}
	templateVars := Variables{name, id, uid, version}
	var payload bytes.Buffer
	t := template.New("dashboard")
	t.Parse(templateAsString)
	err = t.Execute(&payload, templateVars)
	if os.Getenv("DEBUG") == "true" {
		t.Execute(os.Stdout, templateVars)
	}
	if err != nil {
		return
	}
	renderedTemplate = &payload
	return renderedTemplate, err
}

func DashboardNamePut(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	decodedCert, err := base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
	if err != nil {
		logrus.Println("decode error:", err)
	}
	_ = decodedCert
	grafanaUrl := r.Header.Get("X-Grafana-Url")
	grafanaApiKey := r.Header.Get("X-Grafana-API-Key")
	reqBody, err := ioutil.ReadAll(r.Body)

	if len(reqBody) == 0 {
		logrus.Println("Malformed request body:", string(reqBody))
		handleBadRequest(w, "request body malformed")
		return
	}

	var t TemplateUrl
	err = json.Unmarshal(reqBody, &t)
	if err != nil || t.Url == "" {
		logrus.Println("Malformed request body:", string(reqBody))
		handleBadRequest(w, "request body malformed")
		return
	}

	templateUrl := t.Url
	logrus.Println("PUT request for name:", name, "templateUrl:", templateUrl)
	templateFromUrl, err := getTemplateFromUrl(templateUrl)

	if err != nil {
		handleInternalServerError(w, "internal server error", "error fetching template from templateUrl")
		return
	}

	url, id, uid, version, found, err := getDashboardByName(name, grafanaUrl, grafanaApiKey)

	if err != nil {
		handleInternalServerError(w, "internal server error", "error searching for dashboard in Grafana")
		return
	}

	var idVar, uidVar, versionVar string
	if found == false {
		idVar, uidVar, versionVar = "null", "null", "1"
		logrus.Println("Dashboard not found, creating new")
	} else {
		idVar, uidVar = strconv.FormatInt(id, 10), "\""+uid+"\""
		versionVar = strconv.FormatInt(version+1, 10)
		logrus.Printf("Dashboard found id: %s uid: %s version: %s", idVar, uidVar, strconv.FormatInt(version, 10))
		logrus.Printf("Updating existing dashboard id: %s uid: %s version: %s", idVar, uidVar, versionVar)
	}

	logrus.Printf("Attempting to create dashboard for name %s in grafana", name)

	renderedTemplateFromUrl, err := renderTemplate(name, idVar, uidVar, versionVar, string(templateFromUrl))

	if err != nil {
		logrus.Println(err)
		return
	}

	status, body, err := callGrafana(grafanaUrl+"/api/dashboards/db", grafanaApiKey, "POST", renderedTemplateFromUrl)

	if status != 200 || err != nil {
		logrus.Println(err)
		handleInternalServerError(w, "internal server error", "error creating dashboard from template: "+templateUrl)
		return
	}

	type GrafanaDashboard struct {
		Uid string `json:"uid"`
		Id  int64  `json:"id"`
		Url string `json:"url"`
	}
	var g GrafanaDashboard
	if err := json.Unmarshal(body, &g); err != nil {
		logrus.Println(err)
		return
	}
	url = grafanaUrl + g.Url
	id = g.Id
	uid = g.Uid

	var dashboard Dashboard
	dashboard = Dashboard{Name: name, Url: url, Id: id, Uid: uid}
	responseBody, err := json.Marshal(dashboard)

	handleSuccess(w, responseBody)
	return
}
