package models

import (
	"os"

	mattermost "github.com/saygik/mattermost/client"
)

var client *mattermost.Client4

//var channelID string

type MattermostModel struct{}

func (m MattermostModel) Init() {
	client = mattermost.NewAPIv4Client(os.Getenv("GLPI_TO_MATT_MATT_URL"))
	client.SetToken(os.Getenv("GLPI_TO_MATT_TOKEN"))
	// channelID = os.Getenv("GLPI_TO_MATT_CHANNEL_ID")
}
func (m MattermostModel) CreatePostWithAttachtent(
	channelID, message, rootId string, msgProperties mattermost.MsgProperties) (*mattermost.Post, error) {
	createdPost, _, err := client.CreatePostWithAttachtent(channelID, message, "",
		msgProperties)
	if err != nil {
		return nil, err
	}
	return createdPost, nil
}
func (m MattermostModel) UpdatePostWithAttachtent(
	postId, message string, msgProperties mattermost.MsgProperties) (*mattermost.Post, error) {

	createdPost, _, err := client.UpdatePostWithAttachtent(postId, message, msgProperties)
	if err != nil {
		return nil, err
	}
	return createdPost, nil
}
func (m MattermostModel) CreateSimplePost(channelID, message, rootId string) (*mattermost.Post, error) {

	createdPost, _, err := client.CreateSimpleMessagePost(channelID, message, rootId)
	if err != nil {
		return nil, err
	}
	return createdPost, nil
}
func (m MattermostModel) UpdateThreadFollowAllUsersInChannel(channelID, PostId string) error {

	return client.UpdateThreadFollowAllUsersInChannel(channelID, PostId, true)
}
