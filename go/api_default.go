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
	"crypto/x509"
	"crypto/tls"
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
const teamName string = "hub-team"

func handleSuccess(w http.ResponseWriter, payload []byte) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(payload)
}

func handleDelete(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func handleInternalServerError(w http.ResponseWriter, reason, detail string) {
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

func callGrafana(admin bool, url, auth string, grafanaCaCert []byte, verb string, payload io.Reader) (int, []byte, error) {
	var statusCode int
	var body []byte
	var err error

	if payload != nil {
		logrus.Debugln("---DEBUG enabled---")
		logrus.Debugln("URL: " + url)
		logrus.Debugln("Method: " + verb)
		buf := new(bytes.Buffer)
		buf.ReadFrom(payload)
		s := buf.String()
		logrus.Debugf("Request body: %s", s)
	}

	req, err := http.NewRequest(verb, url, payload)

	var authKey string
	if admin {
		logrus.Print("Using basic auth")
		authKey = "Basic"
	} else {
		logrus.Print("Using bearer auth")
		authKey = "Bearer"
	}

	req.Header.Set("Authorization", authKey+" "+auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if err != nil {
		logrus.Fatalf("Failed to append %q to RootCAs: %v", grafanaCaCert, err)
	}

	if grafanaCaCert != nil {
		if ok := rootCAs.AppendCertsFromPEM([]byte(grafanaCaCert)); !ok {
			logrus.Println("No certs appended, using system certs only")
		}
	}

	config := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            rootCAs,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)

	if err != nil {
		logrus.Printf("Error calling Grafana: %s %s %s", verb, url, err.Error())
		return statusCode, body, err
	}

	body, _ = ioutil.ReadAll(resp.Body)
	statusCode = resp.StatusCode

	logrus.Debugf("Response body from Grafana: %s", string(body))
	logrus.Debugf("Response code from Grafana: %v", statusCode)

	defer resp.Body.Close()

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

	logrus.Debugln("Template response body:", string(templateBody))

	return templateBody, nil
}

func getDashboardByName(name, grafanaURL, grafanaApiKey string, grafanaCaCert []byte) (url string, id int64, uid string, version int64, found bool, err error) {

	logrus.Println("Searching for dashboard using tag:", dashboardPrefix+name)
	status, body, err := callGrafana(false, grafanaURL+"/api/search?tag="+dashboardPrefix+name, grafanaApiKey, grafanaCaCert, "GET", nil)

	if status != 200 || err != nil {
		logrus.Println("Error getting dashboard for name:", name)
		return
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
		return url, id, uid, version, found, errors.New("more than one dashboard found")
	}

	dash := g[0]

	url = grafanaURL + dash.Url
	id = dash.Id
	uid = dash.Uid
	version = dash.Version

	return url, id, uid, version, found, err
}

func createUser(user User, grafanaURL, auth string, grafanaCaCert []byte) (createdUser User, err error) {
	user, found, err := getUserByEmail(user.Email, grafanaURL, auth, grafanaCaCert)

	if found {
		logrus.Printf("User %s exists already", user.Email)
		return user, nil
	}

	payload, err := json.Marshal(user)

	if err != nil {
		return createdUser, err
	}

	payloadReader := bytes.NewReader(payload)

	status, _, err := callGrafana(true, grafanaURL+"/api/admin/users", auth, grafanaCaCert, "POST", payloadReader)

	if status != 200 || err != nil {
		logrus.Println("Error creating user")
		return user, err
	}

	createdUser, _, err = getUserByEmail(user.Email, grafanaURL, auth, grafanaCaCert)

	return createdUser, nil
}

func createTeam(teamName, grafanaURL, auth string, grafanaCaCert []byte) (team Team, err error) {

	team, found, err := getTeamByName(teamName, grafanaURL, auth, grafanaCaCert)

	if err == nil && found == true {
		return team, nil
	}

	newTeam := Team{Name: teamName}
	teamObject, err := json.Marshal(newTeam)
	payloadReader := bytes.NewReader(teamObject)

	status, body, err := callGrafana(true, grafanaURL+"/api/teams", auth, grafanaCaCert, "POST", payloadReader)

	if err != nil {
		logrus.Println("Error creating team: " + err.Error())
		return team, err
	}

	var grafanaResponse GrafanaCreateTeamResponse
	err = json.Unmarshal(body, &grafanaResponse)

	if status != 200 && status != 409 {
		logrus.Println("Error creating team: " + teamName + "response status from grafana: " + strconv.Itoa(status))
		return team, errors.New("Error creating team:" + teamName)
	}
	teamResponse := Team{Name: teamName, TeamId: grafanaResponse.TeamId}
	return teamResponse, nil
}

func getUserByEmail(email, grafanaURL, auth string, grafanaCaCert []byte) (user User, found bool, err error) {
	logrus.Println("Searching for user using email:", email)

	status, body, err := callGrafana(true, grafanaURL+"/api/users/lookup?loginOrEmail="+email, auth, grafanaCaCert, "GET", nil)

	if err != nil {
		logrus.Println("Error getting user for email:", email)
		return
	}

	var u User

	if status == 404 {
		logrus.Println("User not found")
		found = false
		return u, found, err
	} else {
		logrus.Println("User found")
		found = true
		err := json.Unmarshal(body, &u)
		return u, found, err
	}
}

func deleteUserByEmail(email, grafanaURL, auth string, grafanaCaCert []byte) (err error) {
	logrus.Println("Searching for user using email:", email)

	user, found, err := getUserByEmail(email, grafanaURL, auth, grafanaCaCert)

	if err != nil {
		logrus.Println("Error deleting user for email:", email)
		return err
	}

	if found == false {
		logrus.Println("User not found")
		return nil
	}

	logrus.Println("User found, deleting...")
	status, _, err := callGrafana(true, grafanaURL+"/api/admin/users/"+strconv.FormatInt(user.Id, 10), auth, grafanaCaCert, "DELETE", nil)

	if err != nil || status != 200 {
		return err
	}
	return
}

func addUserToTeam(user User, team Team, grafanaURL, auth string, grafanaCaCert []byte) (err error) {
	logrus.Printf("Adding user %s to team %s", user.Email, team.Name)

	member := TeamMember{UserId: user.Id}
	memberObject, err := json.Marshal(member)

	teamId := strconv.FormatInt(team.TeamId, 10)

	memberPayload := bytes.NewReader(memberObject)

	status, _, err := callGrafana(true, grafanaURL+"/api/teams/"+teamId+"/members", auth, grafanaCaCert, "POST", memberPayload)

	if err != nil {
		logrus.Printf("Error adding user to team %s", teamId)
		return
	}

	if status != 400 && status != 200 {
		logrus.Printf("Error adding user %s to team %s", strconv.FormatInt(user.Id, 10), teamId)
		return
	}

	if status != 400 {
		logrus.Println("User already in team")
		return
	}

	if status == 200 {
		logrus.Println("Added user to team")
		return
	}

	return
}

func checkTeamMembership(email string, team Team, grafanaURL, auth string, grafanaCaCert []byte) (user User, found bool, err error) {
	logrus.Printf("Checking user %s is in team %s:", email, team.Name)

	teamId := strconv.FormatInt(team.TeamId, 10)

	status, body, err := callGrafana(true, grafanaURL+"/api/teams/"+teamId+"/members", auth, grafanaCaCert, "GET", nil)

	if err != nil || status != 200 {
		logrus.Println("Error checking team membership")
		return
	}

	var u User

	if len(body) == 0 {
		logrus.Println("User not found")
		found = false
		return u, found, err
	} else {
		logrus.Println("User found")
		found = true
		err := json.Unmarshal(body, &u)
		return u, found, err
	}
}

func getTeamMembers(team Team, grafanaURL, auth string, grafanaCaCert []byte) (users []User, err error) {
	logrus.Printf("Getting members of team: %s", team.Name)

	teamId := strconv.FormatInt(team.TeamId, 10)

	status, body, err := callGrafana(true, grafanaURL+"/api/teams/"+teamId+"/members", auth, grafanaCaCert, "GET", nil)

	logrus.Println("Response from membership list:")
	logrus.Println(string(body))

	if err != nil || status != 200 {
		logrus.Println("Error checking team membership")
		return users, err
	}

	var members []TeamMember
	err = json.Unmarshal(body, &members)

	for _, m := range members {
		user, _, err := getUserByEmail(m.Email, grafanaURL, auth, grafanaCaCert)
		if err != nil {
			return users, err
		}
		users = append(users, user)
	}

	if err != nil {
		logrus.Println("Malformed response from Grafana")
		return users, err
	}
	return users, nil
}

func getTeamByName(name, grafanaURL, auth string, grafanaCaCert []byte) (team Team, found bool, err error) {
	logrus.Println("Searching for team by name:", name)

	status, body, err := callGrafana(true, grafanaURL+"/api/teams/search?name="+name, auth, grafanaCaCert, "GET", nil)

	if err != nil {
		logrus.Println("Error getting team with the name:", name)
		return
	}

	var g GrafanaGetTeamResponse
	err = json.Unmarshal(body, &g)

	if err != nil {
		logrus.Println("Error getting team with the name:", name)
		return
	}

	var t Team

	if status == 404 {
		logrus.Println("Team not found")
		found = false
		return t, found, err
	}

	if status == 200 && len(g.Teams) == 1 {
		t = g.Teams[0]
		if err != nil {
			logrus.Println("Error getting team with the name:", name)
		}
		logrus.Println("Team found")
		found = true
		return t, found, err
	} else {
		logrus.Println("No team found:", name)
		found = false
		return t, found, err
	}
}

func renderTemplate(name, id, uid, version, templateAsString string) (renderedTemplate io.Reader, err error) {
	templateVars := Variables{name, id, uid, version}
	var payload bytes.Buffer
	t := template.New("dashboard")
	t.Parse(templateAsString)
	err = t.Execute(&payload, templateVars)

	logrus.Debugln("Rendered dashboard template:")
	logrus.Debugln(t.Execute(os.Stdout, templateVars))

	if err != nil {
		return
	}
	renderedTemplate = &payload
	return renderedTemplate, err
}

func userInList(user User, list []User) bool {
	for _, u := range list {
		if u.Id == user.Id {
			return true
		}
	}
	return false
}

func removeUserFromTeam(user User, team Team, grafanaURL, auth string, grafanaCaCert []byte) (err error) {
	status, _, err := callGrafana(true, grafanaURL+"/api/teams/"+strconv.FormatInt(team.TeamId, 10)+"/members/"+strconv.FormatInt(user.Id, 10), auth, grafanaCaCert, "DELETE", nil)

	if err != nil {
		logrus.Printf("Error removing user %s from team %s", user.Name, team.Name)
		return
	}

	if status == 404 {
		logrus.Printf("User not in team")
		return nil
	}

	if status == 200 {
		logrus.Printf("User %s removed from team %s", user.Name, team.Name)
		return nil
	}
	return
}

func DashboardNameGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var grafanaCaCert []byte
	var err error
    if r.Header.Get("X-Grafana-CA") != "" {
		grafanaCaCert, err = base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
		if err != nil {
			logrus.Println("decode error:", err)
		}
	}
	grafanaURL := r.Header.Get("X-Grafana-Url")
	grafanaApiKey := r.Header.Get("X-Grafana-API-Key")

	url, id, uid, _, found, err := getDashboardByName(name, grafanaURL, grafanaApiKey, grafanaCaCert)

	if err != nil {
		handleInternalServerError(w, "internal server error", "error searching for dashboard in Grafana")
		return
	}

	if found == false {
		handleNotFoundError(w, "dashboard not found")
		return
	}

	var dashboard GrafanaDashboard
	dashboard = GrafanaDashboard{Name: name, Url: url, Id: id, Uid: uid}
	payload, err := json.Marshal(dashboard)

	if err != nil {
		logrus.Println(err)
	}

	handleSuccess(w, payload)
	return
}

func DashboardNameDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	var grafanaCaCert []byte
	var err error
    if r.Header.Get("X-Grafana-CA") != "" {
		grafanaCaCert, err = base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
		if err != nil {
			logrus.Println("decode error:", err)
		}
	}
	grafanaURL := r.Header.Get("X-Grafana-Url")
	grafanaApiKey := r.Header.Get("X-Grafana-API-Key")

	url, id, uid, _, found, err := getDashboardByName(name, grafanaURL, grafanaApiKey, grafanaCaCert)

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

	status, _, err := callGrafana(false, grafanaURL+"/api/dashboards/uid/"+uid, grafanaApiKey, grafanaCaCert, "DELETE", nil)

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

func DashboardNamePut(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var grafanaCaCert []byte
	var err error
    if r.Header.Get("X-Grafana-CA") != "" {
		grafanaCaCert, err = base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
		if err != nil {
			logrus.Println("decode error:", err)
		}
	}
	grafanaURL := r.Header.Get("X-Grafana-Url")
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

	url, id, uid, version, found, err := getDashboardByName(name, grafanaURL, grafanaApiKey, grafanaCaCert)

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

	status, body, err := callGrafana(false, grafanaURL+"/api/dashboards/db", grafanaApiKey, grafanaCaCert, "POST", renderedTemplateFromUrl)

	if status != 200 || err != nil {
		logrus.Println(err)
		handleInternalServerError(w, "internal server error", "error creating dashboard from template: "+templateUrl)
		return
	}

	var g GrafanaDashboard
	if err := json.Unmarshal(body, &g); err != nil {
		logrus.Println(err)
		return
	}
	url = grafanaURL + g.Url
	id = g.Id
	uid = g.Uid

	var dashboard GrafanaDashboard
	dashboard = GrafanaDashboard{Name: name, Url: url, Id: id, Uid: uid}
	responseBody, err := json.Marshal(dashboard)

	handleSuccess(w, responseBody)
	return
}


func UserGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	email := vars["email"]

	var grafanaCaCert []byte
	var err error
    if r.Header.Get("X-Grafana-CA") != "" {
		grafanaCaCert, err = base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
		if err != nil {
			logrus.Println("decode error:", err)
		}
	}
	grafanaURL := r.Header.Get("X-Grafana-Url")
	grafanaBasicAuth := r.Header.Get("X-Grafana-Basic-Auth")

	logrus.Println("Getting user by email:", email)

	user, found, err := getUserByEmail(email, grafanaURL, grafanaBasicAuth, grafanaCaCert)

	if err != nil {
		logrus.Println("Error getting user for email:", email)
		return
	}

	if found == false {
		handleNotFoundError(w, "User not found")
		return
	}

	responseBody, err := json.Marshal(user)

	handleSuccess(w, responseBody)
}

func UsersPut(w http.ResponseWriter, r *http.Request) {
	var grafanaCaCert []byte
	var err error
    if r.Header.Get("X-Grafana-CA") != "" {
		grafanaCaCert, err = base64.StdEncoding.DecodeString(r.Header.Get("X-Grafana-CA"))
		if err != nil {
			logrus.Println("decode error:", err)
		}
	}
	grafanaURL := r.Header.Get("X-Grafana-Url")
	grafanaBasicAuth := r.Header.Get("X-Grafana-Basic-Auth")
	reqBody, err := ioutil.ReadAll(r.Body)

	if len(reqBody) == 0 {
		logrus.Println("Missing request body:", string(reqBody))
		handleBadRequest(w, "request body malformed")
		return
	}

	var users []User
	err = json.Unmarshal(reqBody, &users)

	if err != nil {
		logrus.Println("Malformed request body:", string(reqBody), err.Error())
		handleBadRequest(w, "request body malformed")
		return
	}

	// Create team if it doesnt exist
	team, err := createTeam(teamName, grafanaURL, grafanaBasicAuth, grafanaCaCert)

	// List the users in the team
	usersInTeam, err := getTeamMembers(team, grafanaURL, grafanaBasicAuth, grafanaCaCert)

	logrus.Println("Listing users in the team")
	logrus.Println(usersInTeam)

	// Delete any members who are in the team but not the PUT payload
	for _, u := range usersInTeam {
		if userInList(u, users) == false {
			err = removeUserFromTeam(u, team, grafanaURL, grafanaBasicAuth, grafanaCaCert)
			if err != nil {
				handleInternalServerError(w, "Error removing user: "+u.Email+" from team: "+team.Name+" error: ", err.Error())
			}
		}
	}

	// Create a slice of users to form the response body which will include IDs not sent in PUT payload
	var agentResponse []User

	for _, u := range users {

		// Create global users for each email in the PUT payload
		user, err := createUser(u, grafanaURL, grafanaBasicAuth, grafanaCaCert)

		if err != nil {
			handleInternalServerError(w, "Error creating user: "+u.Email, err.Error())
			return
		}

		// Append the user including their ID to the user slice
		agentResponse = append(agentResponse, user)

		// Add the user to the team if they arent already in it
		logrus.Printf("Adding user with id %s to team %s", strconv.FormatInt(user.Id, 10), strconv.FormatInt(team.TeamId, 10))
		err = addUserToTeam(user, team, grafanaURL, grafanaBasicAuth, grafanaCaCert)

		if err != nil {
			handleInternalServerError(w, "Error adding user: "+u.Email+" to team: "+team.Name, err.Error())
			return
		}
	}
	// Return list of users created including IDs
	agentResponseBody, err := json.Marshal(agentResponse)
	handleSuccess(w, agentResponseBody)
}
