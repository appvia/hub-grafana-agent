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

type GrafanaCreateUserResponse struct {
	Id      int64  `json:"id"`
	Message string `json:"message"`
}

type GrafanaCreateTeamResponse struct {
	TeamId  int64  `json:"teamId"`
	Message string `json:"message"`
}

type GrafanaGetTeamResponse struct {
	Teams      []Team `json:"teams"`
	TotalCount int64  `json:"totalCount"`
}

type TeamMember struct {
	UserId int64  `json:"UserId"`
	Email  string `json:"email,omitempty"`
}

type GrafanaDashboard struct {
	Name    string `json:"name,omitempty"`
	Uid     string `json:"uid"`
	Id      int64  `json:"id"`
	Url     string `json:"url"`
	Version int64  `json:"version,omitempty"`
}

type GrafanaDashboardAlert struct {
	Name  string `json:"name,omitempty"`
	State string `json:"state"`
	Id    int64  `json:"id"`
	Url   string `json:"url"`
}
