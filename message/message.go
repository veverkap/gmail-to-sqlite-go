package message

import (
	"encoding/base64"
	"net/mail"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Message represents an email message.
type Message struct {
	ID         string                 `json:"id"`
	ThreadID   string                 `json:"thread_id"`
	Sender     map[string]string      `json:"sender"`
	Recipients map[string]interface{} `json:"recipients"`
	Labels     []string               `json:"labels"`
	Subject    string                 `json:"subject"`
	Body       string                 `json:"body"`
	Size       int                    `json:"size"`
	Timestamp  time.Time              `json:"timestamp"`
	IsRead     bool                   `json:"is_read"`
	IsOutgoing bool                   `json:"is_outgoing"`
}

// NewMessage creates a new empty Message.
func NewMessage() *Message {
	return &Message{
		Sender:     make(map[string]string),
		Recipients: make(map[string]interface{}),
		Labels:     make([]string, 0),
	}
}

// FromRaw creates a Message from a raw Gmail API message.
func FromRaw(raw map[string]interface{}, labels map[string]string) (*Message, error) {
	msg := NewMessage()
	if err := msg.Parse(raw, labels); err != nil {
		return nil, err
	}
	return msg, nil
}

// ParseAddresses parses a list of email addresses.
func (m *Message) ParseAddresses(addresses string) []map[string]string {
	var parsedAddresses []map[string]string

	for _, address := range strings.Split(addresses, ",") {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}

		addr, err := mail.ParseAddress(address)
		if err != nil {
			// Skip invalid addresses
			continue
		}

		parsedAddresses = append(parsedAddresses, map[string]string{
			"email": strings.ToLower(addr.Address),
			"name":  addr.Name,
		})
	}

	return parsedAddresses
}

// DecodeBody decodes the body of a message part.
func (m *Message) DecodeBody(part map[string]interface{}) string {
	body, ok := part["body"].(map[string]interface{})
	if !ok {
		return ""
	}

	if data, ok := body["data"].(string); ok {
		decoded, err := base64.URLEncoding.DecodeString(data)
		if err != nil {
			return ""
		}
		return string(decoded)
	}

	if parts, ok := part["parts"].([]interface{}); ok {
		for _, subpart := range parts {
			if subpartMap, ok := subpart.(map[string]interface{}); ok {
				decodedBody := m.DecodeBody(subpartMap)
				if decodedBody != "" {
					return decodedBody
				}
			}
		}
	}

	return ""
}

// HTML2Text converts HTML to plain text.
func (m *Message) HTML2Text(html string) string {
	reader := strings.NewReader(html)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return html
	}
	return doc.Text()
}

// Parse parses a raw Gmail API message.
func (m *Message) Parse(msg map[string]interface{}, labels map[string]string) error {
	if id, ok := msg["id"].(string); ok {
		m.ID = id
	}

	if threadID, ok := msg["threadId"].(string); ok {
		m.ThreadID = threadID
	}

	if size, ok := msg["sizeEstimate"].(float64); ok {
		m.Size = int(size)
	}

	payload, ok := msg["payload"].(map[string]interface{})
	if !ok {
		return nil
	}

	headers, ok := payload["headers"].([]interface{})
	if ok {
		for _, header := range headers {
			headerMap, ok := header.(map[string]interface{})
			if !ok {
				continue
			}

			name, ok := headerMap["name"].(string)
			if !ok {
				continue
			}
			name = strings.ToLower(name)

			value, ok := headerMap["value"].(string)
			if !ok {
				continue
			}

			switch name {
			case "from":
				addr, err := mail.ParseAddress(value)
				if err == nil {
					m.Sender = map[string]string{"name": addr.Name, "email": addr.Address}
				}
			case "to":
				if m.Recipients["to"] == nil {
					m.Recipients["to"] = m.ParseAddresses(value)
				}
			case "cc":
				if m.Recipients["cc"] == nil {
					m.Recipients["cc"] = m.ParseAddresses(value)
				}
			case "bcc":
				if m.Recipients["bcc"] == nil {
					m.Recipients["bcc"] = m.ParseAddresses(value)
				}
			case "subject":
				m.Subject = value
			case "date":
				t, err := mail.ParseDate(value)
				if err == nil {
					m.Timestamp = t
				}
			}
		}
	}

	// Process labels
	if labelIDs, ok := msg["labelIds"].([]interface{}); ok {
		for _, labelID := range labelIDs {
			if id, ok := labelID.(string); ok {
				if label, exists := labels[id]; exists {
					m.Labels = append(m.Labels, label)
				}
			}
		}

		// Check specific labels
		for _, labelID := range labelIDs {
			id, ok := labelID.(string)
			if !ok {
				continue
			}

			if id == "UNREAD" {
				m.IsRead = false
			} else if id == "SENT" {
				m.IsOutgoing = true
			}
		}
	}

	// Extract body for non-multipart messages
	if body, ok := payload["body"].(map[string]interface{}); ok {
		if data, ok := body["data"].(string); ok {
			decoded, err := base64.URLEncoding.DecodeString(data)
			if err == nil {
				m.Body = m.HTML2Text(string(decoded))
			}
		}
	}

	// For multipart messages
	if m.Body == "" {
		if parts, ok := payload["parts"].([]interface{}); ok {
			for _, part := range parts {
				partMap, ok := part.(map[string]interface{})
				if !ok {
					continue
				}

				mimeType, ok := partMap["mimeType"].(string)
				if !ok {
					continue
				}

				if mimeType == "text/html" || mimeType == "text/plain" ||
					mimeType == "multipart/related" || mimeType == "multipart/alternative" {
					decodedBody := m.DecodeBody(partMap)
					if decodedBody != "" {
						m.Body = m.HTML2Text(decodedBody)
						break
					}
				}
			}
		}
	}

	return nil
}
