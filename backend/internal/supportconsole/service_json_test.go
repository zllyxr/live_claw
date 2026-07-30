package supportconsole

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnowflakeUserIDsMarshalAsExactDecimalStrings(t *testing.T) {
	const userID int64 = 1785252579710207004

	payload, err := json.Marshal(struct {
		Conversation Conversation `json:"conversation"`
		User         UserCard     `json:"user"`
		Message      Message      `json:"message"`
		Note         Note         `json:"note"`
	}{
		Conversation: Conversation{UserID: userID},
		User:         UserCard{ID: userID},
		Message:      Message{SenderType: 1, SenderID: userID},
		Note:         Note{UserID: userID},
	})
	if err != nil {
		t.Fatalf("marshal support payload: %v", err)
	}

	body := string(payload)
	for _, field := range []string{
		`"user_id":"1785252579710207004"`,
		`"id":"1785252579710207004"`,
		`"sender_id":"1785252579710207004"`,
	} {
		if !strings.Contains(body, field) {
			t.Fatalf("expected exact quoted ID field %s in %s", field, body)
		}
	}
	if strings.Contains(body, "1785252579710206976") {
		t.Fatalf("payload contains JavaScript-rounded user ID: %s", body)
	}
}

func TestAllSupportBigIntIDsMarshalAsExactDecimalStrings(t *testing.T) {
	const id int64 = 1785252579710207004

	tests := []struct {
		name     string
		value    any
		expected []string
	}{
		{
			name:     "agent",
			value:    Agent{ID: id},
			expected: []string{`"id":"1785252579710207004"`},
		},
		{
			name:  "conversation assignee",
			value: Conversation{AssignedAgentID: id},
			expected: []string{
				`"assigned_agent_id":"1785252579710207004"`,
			},
		},
		{
			name:     "message asset",
			value:    Message{AssetID: id},
			expected: []string{`"asset_id":"1785252579710207004"`},
		},
		{
			name:  "note",
			value: Note{ID: id, AgentID: id},
			expected: []string{
				`"id":"1785252579710207004"`,
				`"agent_id":"1785252579710207004"`,
			},
		},
		{
			name:     "quick reply",
			value:    QuickReply{ID: id},
			expected: []string{`"id":"1785252579710207004"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			body := string(payload)
			for _, field := range test.expected {
				if !strings.Contains(body, field) {
					t.Fatalf("expected exact quoted ID field %s in %s", field, body)
				}
			}
			if strings.Contains(body, "1785252579710206976") {
				t.Fatalf("payload contains JavaScript-rounded ID: %s", body)
			}
		})
	}
}
