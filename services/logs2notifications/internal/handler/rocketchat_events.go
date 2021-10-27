package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// RocketChatData .
type RocketChatData struct {
	Channel     string                 `json:"channel"`
	Attachments []RocketChatAttachment `json:"attachments"`
}

// RocketChatAttachment .
type RocketChatAttachment struct {
	Text   string                      `json:"text"`
	Color  string                      `json:"color"`
	Fields []RocketChatAttachmentField `json:"fields"`
}

// RocketChatAttachmentField .
type RocketChatAttachmentField struct {
	Short bool   `json:"short"`
	Title string `json:"title"`
	Value string `json:"value"`
}

// SendToRocketChat .
func SendToRocketChat(notification *Notification, channel, webhook, appID string) {

	emoji, color, template, err := getRocketChatEvent(notification.Event)
	if err != nil {
		return
	}

	var text string
	switch template {
	case "mergeRequestOpened":
		text = fmt.Sprintf("*[%s]* PR [#%s (%s)](%s) opened in [%s](%s)",
			notification.Meta.ProjectName,
			notification.Meta.PullrequestNumber,
			notification.Meta.PullrequestTitle,
			notification.Meta.PullrequestURL,
			notification.Meta.RepoName,
			notification.Meta.RepoURL,
		)
	case "mergeRequestUpdated":
		text = fmt.Sprintf("*[%s]* PR [#%s (%s)](%s) updated in [%s](%s)",
			notification.Meta.ProjectName,
			notification.Meta.PullrequestNumber,
			notification.Meta.PullrequestTitle,
			notification.Meta.PullrequestURL,
			notification.Meta.RepoName,
			notification.Meta.RepoURL,
		)
	case "mergeRequestClosed":
		text = fmt.Sprintf("*[%s]* PR [#%s (%s)](%s) closed in [%s](%s)",
			notification.Meta.ProjectName,
			notification.Meta.PullrequestNumber,
			notification.Meta.PullrequestTitle,
			notification.Meta.PullrequestURL,
			notification.Meta.RepoName,
			notification.Meta.RepoURL,
		)
	case "deleteEnvironment":
		text = fmt.Sprintf("*[%s]* Deleting environment `%s`",
			notification.Meta.ProjectName,
			notification.Meta.EnvironmentName,
		)
	case "repoPushHandled":
		text = fmt.Sprintf("*[%s]* [%s](%s/tree/%s)",
			notification.Meta.ProjectName,
			notification.Meta.BranchName,
			notification.Meta.RepoURL,
			notification.Meta.BranchName,
		)
		if notification.Meta.ShortSha != "" {
			text = fmt.Sprintf("%s ([%s](%s))",
				text,
				notification.Meta.ShortSha,
				notification.Meta.CommitURL,
			)
		}
		text = fmt.Sprintf("%s pushed in [%s](%s)",
			text,
			notification.Meta.RepoFullName,
			notification.Meta.RepoURL,
		)
	case "repoPushSkipped":
		text = fmt.Sprintf("*[%s]* [%s](%s/tree/%s)",
			notification.Meta.ProjectName,
			notification.Meta.BranchName,
			notification.Meta.RepoURL,
			notification.Meta.BranchName,
		)
		if notification.Meta.ShortSha != "" {
			text = fmt.Sprintf("%s ([%s](%s))",
				text,
				notification.Meta.ShortSha,
				notification.Meta.CommitURL,
			)
		}
		text = fmt.Sprintf("%s pushed in [%s](%s) *deployment skipped*",
			text,
			notification.Meta.RepoFullName,
			notification.Meta.RepoURL,
		)
	case "deployEnvironment":
		text = fmt.Sprintf("*[%s]* Deployment triggered `%s`",
			notification.Meta.ProjectName,
			notification.Meta.BranchName,
		)
		if notification.Meta.ShortSha != "" {
			text = fmt.Sprintf("%s (%s)",
				text,
				notification.Meta.ShortSha,
			)
		}
	case "removeFinished":
		text = fmt.Sprintf("*[%s]* Removed `%s`",
			notification.Meta.ProjectName,
			notification.Meta.OpenshiftProject,
		)
	case "removeRetry":
		text = fmt.Sprintf("*[%s]* Removed `%s`",
			notification.Meta.ProjectName,
			notification.Meta.OpenshiftProject,
		)
	case "notDeleted":
		text = fmt.Sprintf("*[%s]* `%s` not deleted. %s",
			notification.Meta.ProjectName,
			notification.Meta.BranchName,
			notification.Meta.Error,
		)
	case "deployError":
		text = fmt.Sprintf("*[%s]*",
			notification.Meta.ProjectName,
		)
		if notification.Meta.ShortSha != "" {
			text += fmt.Sprintf(" `%s` %s",
				notification.Meta.BranchName,
				notification.Meta.ShortSha,
			)
		} else {
			text += fmt.Sprintf(" `%s`",
				notification.Meta.BranchName,
			)
		}
		text += fmt.Sprintf(" Build `%s` Failed.",
			notification.Meta.BuildName,
		)
		if notification.Meta.LogLink != "" {
			text += fmt.Sprintf(" [Logs](%s) \r",
				notification.Meta.LogLink,
			)
		}
	case "deployFinished":
		text = fmt.Sprintf("*[%s]*",
			notification.Meta.ProjectName,
		)
		if notification.Meta.ShortSha != "" {
			text += fmt.Sprintf(" `%s` %s",
				notification.Meta.BranchName,
				notification.Meta.ShortSha,
			)
		} else {
			text += fmt.Sprintf(" `%s`",
				notification.Meta.BranchName,
			)
		}
		text += fmt.Sprintf(" Build `%s` Succeeded.",
			notification.Meta.BuildName,
		)
		if notification.Meta.LogLink != "" {
			text += fmt.Sprintf(" [Logs](%s) \r",
				notification.Meta.LogLink,
			)
		}
		text += fmt.Sprintf("* %s \n",
			notification.Meta.Route,
		)
		if len(notification.Meta.Routes) != 0 {
			for _, r := range notification.Meta.Routes {
				if r != notification.Meta.Route {
					text += fmt.Sprintf("* %s \n", r)
				}
			}
		}
	default:
		// do nothing
		return
	}

	data := RocketChatData{
		Channel: channel,
		Attachments: []RocketChatAttachment{
			{
				// Text:  fmt.Sprintf("%s %s", emoji, notification.Message),
				Text:  fmt.Sprintf("%s %s", emoji, text),
				Color: color,
				Fields: []RocketChatAttachmentField{
					{
						Short: true,
						Title: "Source",
						Value: appID,
					},
				},
			},
		},
	}

	jsonBytes, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", webhook, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(jsonBytes)))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending message to rocketchat: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Println(fmt.Sprintf("Sent %s message to rocketchat", notification.Event))
}

func getRocketChatEvent(msgEvent string) (string, string, string, error) {
	if val, ok := rocketChatEventTypeMap[msgEvent]; ok {
		return val.Emoji, val.Color, val.Template, nil
	}
	return "", "", "", fmt.Errorf("no matching event source")
}

var rocketChatEventTypeMap = map[string]EventMap{
	"github:pull_request:opened:handled":           {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestOpened"},
	"gitlab:merge_request:opened:handled":          {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestOpened"},
	"bitbucket:pullrequest:created:opened:handled": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestOpened"}, //not in slack
	"bitbucket:pullrequest:created:handled":        {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestOpened"}, //not in teams

	"github:pull_request:synchronize:handled":      {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestUpdated"},
	"gitlab:merge_request:updated:handled":         {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestUpdated"},
	"bitbucket:pullrequest:updated:opened:handled": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestUpdated"}, //not in slack
	"bitbucket:pullrequest:updated:handled":        {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestUpdated"}, //not in teams

	"github:pull_request:closed:handled":      {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestClosed"},
	"bitbucket:pullrequest:fulfilled:handled": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestClosed"},
	"bitbucket:pullrequest:rejected:handled":  {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestClosed"},
	"gitlab:merge_request:closed:handled":     {Emoji: ":information_source:", Color: "#E8E8E8", Template: "mergeRequestClosed"},

	"github:delete:handled":    {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deleteEnvironment"},
	"gitlab:remove:handled":    {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deleteEnvironment"}, //not in slack
	"bitbucket:delete:handled": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deleteEnvironment"}, //not in slack
	"api:deleteEnvironment":    {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deleteEnvironment"}, //not in teams

	"github:push:handled":         {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushHandled"},
	"bitbucket:repo:push:handled": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushHandled"},
	"gitlab:push:handled":         {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushHandled"},

	"github:push:skipped":    {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushSkipped"},
	"gitlab:push:skipped":    {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushSkipped"},
	"bitbucket:push:skipped": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "repoPushSkipped"},

	"api:deployEnvironmentLatest": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deployEnvironment"},
	"api:deployEnvironmentBranch": {Emoji: ":information_source:", Color: "#E8E8E8", Template: "deployEnvironment"},

	"task:deploy-openshift:finished":           {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "deployFinished"},
	"task:remove-openshift-resources:finished": {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "deployFinished"},
	"task:builddeploy-openshift:complete":      {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "deployFinished"},
	"task:builddeploy-kubernetes:complete":     {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "deployFinished"}, //not in teams

	"task:remove-openshift:finished":  {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "removeFinished"},
	"task:remove-kubernetes:finished": {Emoji: ":white_check_mark:", Color: "lawngreen", Template: "removeFinished"},

	"task:remove-openshift:error":        {Emoji: ":bangbang:", Color: "red", Template: "deployError"},
	"task:remove-kubernetes:error":       {Emoji: ":bangbang:", Color: "red", Template: "deployError"},
	"task:builddeploy-kubernetes:failed": {Emoji: ":bangbang:", Color: "red", Template: "deployError"}, //not in teams
	"task:builddeploy-openshift:failed":  {Emoji: ":bangbang:", Color: "red", Template: "deployError"},

	"github:pull_request:closed:CannotDeleteProductionEnvironment": {Emoji: ":warning:", Color: "gold", Template: "notDeleted"},
	"github:push:CannotDeleteProductionEnvironment":                {Emoji: ":warning:", Color: "gold", Template: "notDeleted"},
	"bitbucket:repo:push:CannotDeleteProductionEnvironment":        {Emoji: ":warning:", Color: "gold", Template: "notDeleted"},
	"gitlab:push:CannotDeleteProductionEnvironment":                {Emoji: ":warning:", Color: "gold", Template: "notDeleted"},

	// deprecated
	// "rest:remove:CannotDeleteProductionEnvironment": {Emoji: ":warning:", Color: "gold"},
	// "rest:deploy:receive":                           {Emoji: ":information_source:", Color: "#E8E8E8"},
	// "rest:remove:receive":                           {Emoji: ":information_source:", Color: "#E8E8E8"},
	// "rest:promote:receive":                          {Emoji: ":information_source:", Color: "#E8E8E8"},
	// "rest:pullrequest:deploy":                       {Emoji: ":information_source:", Color: "#E8E8E8"},
	// "rest:pullrequest:remove":                       {Emoji: ":information_source:", Color: "#E8E8E8"},

	// deprecated
	// "task:deploy-openshift:error":           {Emoji: ":bangbang:", Color: "red", Template: "deployError"},
	// "task:remove-openshift-resources:error": {Emoji: ":bangbang:", Color: "red", Template: "deployError"},

	// deprecated
	// "task:deploy-openshift:retry":           {Emoji: ":warning:", Color: "gold", Template: "removeRetry"},
	// "task:remove-openshift:retry":           {Emoji: ":warning:", Color: "gold", Template: "removeRetry"},
	// "task:remove-kubernetes:retry":          {Emoji: ":warning:", Color: "gold", Template: "removeRetry"},
	// "task:remove-openshift-resources:retry": {Emoji: ":warning:", Color: "gold", Template: "removeRetry"},
}
