package sync

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/api/gmail/v1"

	"github.com/veverkap/gmail-to-sqlite/db"
	"github.com/veverkap/gmail-to-sqlite/message"
)

// processMessage processes a single message from the Gmail API.
func processMessage(service *gmail.Service, database *db.DB, messageID string, labels map[string]string, index int, mu *sync.Mutex) error {
	// Get the raw message from Gmail API
	rawMsg, err := service.Users.Messages.Get("me", messageID).Do()
	if err != nil {
		return fmt.Errorf("could not get message from Gmail %s: %v", messageID, err)
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

	// Convert raw message to a Message object
	msg, err := message.FromRaw(rawMsgMap, labels)
	if err != nil {
		return fmt.Errorf("could not process message %s: %v", messageID, err)
	}

	// Save message to database (use mutex to prevent concurrent writes)
	mu.Lock()
	err = database.CreateMessage(msg)
	mu.Unlock()
	
	if err != nil {
		return fmt.Errorf("could not save message %s: %v", messageID, err)
	}

	// Print progress (use mutex to avoid garbled output)
	mu.Lock()
	fmt.Printf("Synced message %s from %v (Count: %d)\n", msg.ID, msg.Timestamp.Format(time.RFC3339), index+1)
	mu.Unlock()

	return nil
}
