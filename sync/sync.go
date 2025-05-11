package sync

import (
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"

	"github.com/veverkap/gmail-to-sqlite/auth"
	"github.com/veverkap/gmail-to-sqlite/db"
	"github.com/veverkap/gmail-to-sqlite/message"
)

const (
	// MaxResults is the maximum number of results to fetch per page.
	MaxResults = 500
)

// GetLabels retrieves all labels from the Gmail API.
func GetLabels(service *gmail.Service) (map[string]string, error) {
	// Get all labels
	labels := make(map[string]string)
	labelsList, err := service.Users.Labels.List("me").Do()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch labels: %v", err)
	}

	for _, label := range labelsList.Labels {
		labels[label.Id] = label.Name
	}

	return labels, nil
}

// AllMessages fetches and saves all messages from the Gmail API.
func AllMessages(token *oauth2.Token, database *db.DB, fullSync bool) (int, error) {
	service, err := auth.GetService(token)
	if err != nil {
		return 0, fmt.Errorf("failed to create Gmail service: %v", err)
	}

	query := []string{}
	if !fullSync {
		last, err := database.LastIndexed()
		if err != nil {
			return 0, fmt.Errorf("failed to get last indexed timestamp: %v", err)
		}
		if last != nil {
			query = append(query, fmt.Sprintf("after:%d", last.Unix()))
		}

		first, err := database.FirstIndexed()
		if err != nil {
			return 0, fmt.Errorf("failed to get first indexed timestamp: %v", err)
		}
		if first != nil {
			query = append(query, fmt.Sprintf("before:%d", first.Unix()))
		}
	}

	labels, err := GetLabels(service)
	if err != nil {
		return 0, fmt.Errorf("failed to get labels: %v", err)
	}

	// Fetch messages
	pageToken := ""
	totalMessages := 0
	for {
		req := service.Users.Messages.List("me").MaxResults(int64(MaxResults))
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}
		if len(query) > 0 {
			req = req.Q(fmt.Sprintf("%s", query))
		}

		results, err := req.Do()
		if err != nil {
			return totalMessages, fmt.Errorf("failed to list messages: %v", err)
		}

		messages := results.Messages
		totalMessages += len(messages)

		for i, m := range messages {
			// Check if the message already exists
			rawMsg, err := service.Users.Messages.Get("me", m.Id).Do()
			if err != nil {
				fmt.Printf("Could not get message from Gmail %s: %v\n", m.Id, err)
				continue
			}

			// Convert the Gmail message to a map for parsing
			rawMsgMap := make(map[string]interface{})
			rawMsgMap["id"] = rawMsg.Id
			rawMsgMap["threadId"] = rawMsg.ThreadId
			rawMsgMap["labelIds"] = rawMsg.LabelIds
			rawMsgMap["sizeEstimate"] = float64(rawMsg.SizeEstimate)

			// Extract payload
			payload := make(map[string]interface{})
			headers := make([]interface{}, 0)
			for _, header := range rawMsg.Payload.Headers {
				headers = append(headers, map[string]interface{}{
					"name":  header.Name,
					"value": header.Value,
				})
			}
			payload["headers"] = headers

			// Extract body
			if rawMsg.Payload.Body != nil && rawMsg.Payload.Body.Data != "" {
				payload["body"] = map[string]interface{}{
					"data": rawMsg.Payload.Body.Data,
				}
			}

			// Extract parts
			if len(rawMsg.Payload.Parts) > 0 {
				parts := make([]interface{}, 0)
				for _, part := range rawMsg.Payload.Parts {
					partMap := make(map[string]interface{})
					partMap["mimeType"] = part.MimeType
					if part.Body != nil && part.Body.Data != "" {
						partMap["body"] = map[string]interface{}{
							"data": part.Body.Data,
						}
					}
					if len(part.Parts) > 0 {
						subParts := make([]interface{}, 0)
						for _, subPart := range part.Parts {
							subPartMap := make(map[string]interface{})
							subPartMap["mimeType"] = subPart.MimeType
							if subPart.Body != nil && subPart.Body.Data != "" {
								subPartMap["body"] = map[string]interface{}{
									"data": subPart.Body.Data,
								}
							}
							subParts = append(subParts, subPartMap)
						}
						partMap["parts"] = subParts
					}
					parts = append(parts, partMap)
				}
				payload["parts"] = parts
			}
			rawMsgMap["payload"] = payload

			msg, err := message.FromRaw(rawMsgMap, labels)
			if err != nil {
				fmt.Printf("Could not process message %s: %v\n", m.Id, err)
				continue
			}

			if err := database.CreateMessage(msg); err != nil {
				fmt.Printf("Could not save message %s: %v\n", m.Id, err)
				continue
			}

			fmt.Printf("Synced message %s from %v (Count: %d)\n", msg.ID, msg.Timestamp.Format(time.RFC3339), i+1)
		}

		if results.NextPageToken == "" {
			break
		}
		pageToken = results.NextPageToken
	}

	return totalMessages, nil
}

// SingleMessage fetches and saves a single message from the Gmail API.
func SingleMessage(token *oauth2.Token, database *db.DB, messageID string) error {
	service, err := auth.GetService(token)
	if err != nil {
		return fmt.Errorf("failed to create Gmail service: %v", err)
	}

	labels, err := GetLabels(service)
	if err != nil {
		return fmt.Errorf("failed to get labels: %v", err)
	}

	rawMsg, err := service.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return fmt.Errorf("could not get message from Gmail: %v", err)
	}

	// Convert the Gmail message to a map for parsing
	rawMsgMap := make(map[string]interface{})
	rawMsgMap["id"] = rawMsg.Id
	rawMsgMap["threadId"] = rawMsg.ThreadId
	rawMsgMap["labelIds"] = rawMsg.LabelIds
	rawMsgMap["sizeEstimate"] = float64(rawMsg.SizeEstimate)

	// Extract payload
	payload := make(map[string]interface{})
	headers := make([]interface{}, 0)
	for _, header := range rawMsg.Payload.Headers {
		headers = append(headers, map[string]interface{}{
			"name":  header.Name,
			"value": header.Value,
		})
	}
	payload["headers"] = headers

	// Extract body
	if rawMsg.Payload.Body != nil && rawMsg.Payload.Body.Data != "" {
		payload["body"] = map[string]interface{}{
			"data": rawMsg.Payload.Body.Data,
		}
	}

	// Extract parts
	if len(rawMsg.Payload.Parts) > 0 {
		parts := make([]interface{}, 0)
		for _, part := range rawMsg.Payload.Parts {
			partMap := make(map[string]interface{})
			partMap["mimeType"] = part.MimeType
			if part.Body != nil && part.Body.Data != "" {
				partMap["body"] = map[string]interface{}{
					"data": part.Body.Data,
				}
			}
			if len(part.Parts) > 0 {
				subParts := make([]interface{}, 0)
				for _, subPart := range part.Parts {
					subPartMap := make(map[string]interface{})
					subPartMap["mimeType"] = subPart.MimeType
					if subPart.Body != nil && subPart.Body.Data != "" {
						subPartMap["body"] = map[string]interface{}{
							"data": subPart.Body.Data,
						}
					}
					subParts = append(subParts, subPartMap)
				}
				partMap["parts"] = subParts
			}
			parts = append(parts, partMap)
		}
		payload["parts"] = parts
	}
	rawMsgMap["payload"] = payload

	msg, err := message.FromRaw(rawMsgMap, labels)
	if err != nil {
		return fmt.Errorf("could not process message: %v", err)
	}

	if err := database.CreateMessage(msg); err != nil {
		return fmt.Errorf("could not save message: %v", err)
	}

	fmt.Printf("Synced message %s from %v\n", msg.ID, msg.Timestamp.Format(time.RFC3339))
	return nil
}